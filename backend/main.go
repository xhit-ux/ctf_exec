package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/authsession"
	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/db"
	"github.com/ctf_exec/backend/internal/handler"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	sqlDB, err := db.InitDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer sqlDB.Close()
	log.Println("✓ 数据库连接成功")

	// 初始化 JWT 运行时配置缓存（启动后全局复用）。
	utils.InitJWTConfig(cfg.JWT)

	// 初始化Redis
	redisClient, err := cache.InitRedis()
	if err != nil {
		log.Fatalf("Redis初始化失败: %v", err)
	}
	defer redisClient.Close()
	log.Println("✓ Redis连接成功")

	// 初始化仓库
	userRepo := repository.NewUserRepository(sqlDB)
	probRepo := repository.NewProblemRepository(sqlDB)
	subRepo := repository.NewSubmissionRepository(sqlDB)
	teamRepo := repository.NewTeamRepository(sqlDB)
	contestRepo := repository.NewContestRepository(sqlDB)
	contestAdminRepo := repository.NewContestAdminRepository(sqlDB)
	siteConfigRepo := repository.NewSiteConfigRepository(sqlDB)

	// 初始化服务层
	sessionStore := authsession.NewStore(redisClient.Client)
	authService := service.NewAuthService(userRepo, redisClient.Client, sessionStore, cfg)
	userService := service.NewUserService(userRepo)
	_ = service.NewProblemService(probRepo) // 暂时未使用

	// 初始化排行榜服务
	leaderboardService := service.NewLeaderboardService(
		redisClient.Client,
		userRepo,
		teamRepo,
		cfg,
	)

	// 初始化提交服务（注入排行榜服务和Redis客户端）
	contestService := service.NewContestService(contestRepo, teamRepo, leaderboardService)

	// 初始化提交服务（注入排行榜服务和Redis客户端）
	submissionService := service.NewSubmissionService(
		subRepo,
		probRepo,
		userRepo,
		leaderboardService,
		contestService,
		redisClient.Client,
		cfg.RateLimit.SubmissionLimit,
		cfg.RateLimit.WindowSeconds,
	)

	// 初始化团队服务
	teamService := service.NewTeamService(teamRepo)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(authService, userService)
	userHandler := handler.NewUserHandler(userRepo, leaderboardService, authService)
	probHandler := handler.NewProblemHandler(probRepo)
	subHandler := handler.NewSubmissionHandler(subRepo, probRepo, submissionService)
	uploadHandler := handler.NewUploadHandler(userRepo)
	teamHandler := handler.NewTeamHandler(teamRepo, teamService)
	contestHandler := handler.NewContestHandler(contestRepo, userRepo, contestAdminRepo, contestService)
	siteConfigHandler := handler.NewSiteConfigHandler(siteConfigRepo)

	// 创建 Gin 引擎
	r := gin.Default()
	for _, proxy := range cfg.Server.TrustedProxies {
		normalized := strings.TrimSpace(proxy)
		switch normalized {
		case "0.0.0.0", "0.0.0.0/0", "::", "::/0":
			log.Fatalf("SECURITY: SERVER_TRUSTED_PROXIES 不应包含 %s，这会导致客户端 IP 伪造风险", normalized)
		}
	}
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Fatalf("配置受信任代理失败: %v", err)
	}

	// 添加 CORS 中间件（限制为显式允许的前端来源）
	r.Use(middleware.GinCORSMiddleware(cfg.Server.AllowedOrigins))

	// 健康检查（包含Redis健康检查）
	r.GET("/health", func(c *gin.Context) {
		redisHealthy := true
		if err := redisClient.HealthCheck(c.Request.Context()); err != nil {
			redisHealthy = false
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"redis":  redisHealthy,
		})
	})

	// API 路由组
	api := r.Group("/api")
	{
		// 认证路由
		auth := api.Group("/auth")
		{
			auth.POST("/send-register-code", authHandler.SendRegisterCodeGin)
			auth.POST("/register", authHandler.RegisterGin)
			auth.POST("/login", authHandler.LoginGin)
			auth.POST("/refresh", authHandler.RefreshGin)
			auth.POST("/logout", middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore), authHandler.LogoutGin)
			auth.GET("/me", middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore), authHandler.GetCurrentUserGin)
		}

		// 用户路由
		users := api.Group("/users")
		{
			users.GET("/leaderboard", userHandler.GetLeaderboardGin)
			users.GET("/leaderboard/search", userHandler.GetLeaderboardUserByUsernameGin)
			users.PUT("/profile", middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.UpdateProfileGin)
			users.GET("/public", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.GetPublicProfileGin)
		}

		// 题目路由
		problems := api.Group("/problems")
		{
			problems.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			problems.GET("", probHandler.GetProblemsGin)
			problems.GET("/detail", probHandler.GetProblemDetailGin)
		}

		// 赛事公开路由
		contests := api.Group("/contests")
		{
			contests.Use(middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore))
			contests.GET("", contestHandler.ListContestsGin)
			contests.GET(":id", contestHandler.GetContestDetailGin)
			contests.GET(":id/scoreboard", contestHandler.GetContestScoreboardGin)

			contestAuth := contests.Group("")
			contestAuth.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			contestAuth.POST(":id/join", contestHandler.JoinContestGin)
			contestAuth.POST(":id/leave", contestHandler.LeaveContestGin)
			contestAuth.GET(":id/challenges", contestHandler.GetContestChallengesGin)
		}

		// 提交路由
		submissions := api.Group("/submissions")
		{
			submissions.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			submissions.POST("", subHandler.SubmitFlagGin)
			submissions.GET("/me", subHandler.GetMySubmissionsGin)
		}

		// 文件上传路由
		upload := api.Group("/upload")
		{
			upload.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			upload.POST("/avatar", uploadHandler.UploadAvatarGin)
			upload.POST("/contest-cover", middleware.RequireSiteAdminOrRoot(userRepo), uploadHandler.UploadContestCoverGin)
		}

		// 管理端路由
		admin := api.Group("/admin")
		{
			admin.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))

			adminUsers := admin.Group("/users")
			{
				adminUsers.GET("", middleware.RequireSiteAdminOrRoot(userRepo), userHandler.ListAdminUsersGin)
				adminUsers.GET("/role-change-logs", middleware.RequireRoot(userRepo), userHandler.ListRoleChangeLogsGin)
				adminUsers.PATCH(":id/active", middleware.RequireSiteAdminOrRoot(userRepo), userHandler.PatchUserActiveGin)
				adminUsers.PATCH(":id/role", middleware.RequireRoot(userRepo), userHandler.PatchUserRoleGin)
			}

			adminSiteConfig := admin.Group("/site-config")
			{
				adminSiteConfig.Use(middleware.RequireRoot(userRepo))
				adminSiteConfig.GET("", siteConfigHandler.GetSiteConfigGin)
				adminSiteConfig.PUT("", siteConfigHandler.UpdateSiteConfigGin)
			}

			adminProblems := admin.Group("/problems")
			{
				adminProblems.Use(middleware.RequireSiteAdminOrRoot(userRepo))
				adminProblems.POST("", probHandler.CreateProblemGin)
				adminProblems.PUT(":id", probHandler.UpdateProblemGin)
				adminProblems.DELETE(":id", probHandler.DeleteProblemGin)
			}

			adminContests := admin.Group("/contests")
			{
				adminContests.POST("", middleware.RequireSiteAdminOrRoot(userRepo), contestHandler.CreateContestGin)
				adminContests.DELETE(":id", middleware.RequireSiteAdminOrRoot(userRepo), contestHandler.DeleteContestGin)

				adminContests.GET(":id/admins", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.ListContestAdminsGin)
				adminContests.POST(":id/admins", middleware.RequireSiteAdminOrRoot(userRepo), contestHandler.AddContestAdminGin)
				adminContests.DELETE(":id/admins/:user_id", middleware.RequireSiteAdminOrRoot(userRepo), contestHandler.RemoveContestAdminGin)

				adminContests.PUT(":id", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.UpdateContestGin)
				adminContests.POST(":id/challenges", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.AddChallengeToContestGin)
				adminContests.DELETE(":id/challenges/:challengeID", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.RemoveChallengeFromContestGin)
			}
		}

		// 团队路由
		teams := api.Group("/teams")
		{
			// 需要认证的路由
			teams.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			teams.POST("", teamHandler.CreateTeamGin)
			teams.POST("/join", teamHandler.JoinTeamGin)
			teams.GET("/my", teamHandler.GetMyTeamGin)
			teams.DELETE("/leave", teamHandler.LeaveTeamGin)
			teams.DELETE("/dissolve", teamHandler.DissolveTeamGin)
			teams.PUT("/update", teamHandler.UpdateTeamGin)
			teams.POST("/kick", teamHandler.KickMemberGin)
		}

		// 团队公开路由（不需要认证或可选认证）
		teamsPublic := api.Group("/teams")
		{
			teamsPublic.GET("/leaderboard", teamHandler.GetTeamLeaderboardGin)
			teamsPublic.GET("/public", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), teamHandler.GetTeamPublicGin)
		}
	}

	// 静态文件服务
	r.Static("/static", "../static")

	// 启动时同步排行榜数据
	go func() {
		if err := leaderboardService.SyncAllScoresFromDB(); err != nil {
			log.Printf("初始化同步排行榜数据失败: %v", err)
		} else {
			log.Println("✓ 排行榜数据同步完成")
		}
	}()

	// 全站榜单采用延迟快照模式：每 5 分钟定时刷新一次。
	leaderboardService.StartSnapshotScheduler(5 * time.Minute)

	log.Println("✓ 服务器启动在 :" + cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
