package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apierror"
	"github.com/ctf_exec/backend/internal/apperr"
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
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("[ctf-backend] ")

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if config.IsWeakJWTSecret(cfg.JWT.Secret) {
		log.Fatal("SECURITY: JWT_SECRET 为空或仍为默认值，服务拒绝启动")
	}

	// 初始化数据库
	sqlDB, err := db.InitDB(cfg.Database)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer sqlDB.Close()
	log.Println("✓ 数据库连接成功")

	// 初始化 JWT 运行时配置缓存（启动后全局复用）。
	utils.InitJWTConfig(cfg.JWT)
	utils.InitFlagHashSecret("", cfg.JWT.Secret)

	// 初始化Redis
	redisClient, err := cache.InitRedis(cfg.Redis)
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
	probRepo.StartDraftCleanupScheduler(24*time.Hour, 7*24*time.Hour)
	if err := service.EnsureDefaultRootUser(userRepo, cfg.Init.RootDefaultPassword); err != nil {
		log.Fatalf("初始化 root 用户失败: %v", err)
	}

	// 初始化服务层
	sessionStore := authsession.NewStore(redisClient.Client, sqlDB)
	sessionStore.StartRefreshTokenCleanupScheduler(24*time.Hour, 7*24*time.Hour)
	authService := service.NewAuthService(userRepo, siteConfigRepo, redisClient.Client, sessionStore, cfg)
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
	contestService := service.NewContestService(contestRepo, contestAdminRepo, teamRepo, leaderboardService)

	// 初始化提交服务（注入排行榜服务和Redis客户端）
	submissionService := service.NewSubmissionService(
		subRepo,
		probRepo,
		userRepo,
		contestService,
		redisClient.Client,
		cfg.RateLimit.SubmissionLimit,
		cfg.RateLimit.WindowSeconds,
	)

	// 初始化团队服务
	teamService := service.NewTeamService(teamRepo)

	// 初始化处理器
	authHandler := handler.NewAuthHandler(authService, userService, cfg.AuthCookie)
	userHandler := handler.NewUserHandler(userRepo, authService, redisClient.Client)
	probHandler := handler.NewProblemHandler(probRepo, contestRepo, contestAdminRepo)
	subHandler := handler.NewSubmissionHandler(subRepo, probRepo, submissionService)
	uploadHandler := handler.NewUploadHandler(userRepo)
	teamHandler := handler.NewTeamHandler(teamRepo, teamService)
	contestHandler := handler.NewContestHandler(contestRepo, userRepo, contestAdminRepo, contestService)
	siteConfigHandler := handler.NewSiteConfigHandler(siteConfigRepo)

	// 创建 Gin 引擎：统一日志 + JSON panic 恢复。
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("[panic-recovered] method=%s path=%s ip=%s panic=%v", c.Request.Method, c.Request.URL.Path, c.ClientIP(), recovered)
		apierror.Write(c, apperr.New(apperr.ErrInternal, "服务器开小差了，请稍后再试"), http.StatusInternalServerError)
		c.Abort()
	}))
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
	middleware.SetAccessTokenCookieName(cfg.AuthCookie.AccessTokenName)
	r.Use(middleware.GinCORSMiddleware(cfg.Server.AllowedOrigins))

	// 健康检查（包含 Redis 与 MySQL 健康检查）
	r.GET("/health", func(c *gin.Context) {
		redisHealthy := true
		if err := redisClient.HealthCheck(c.Request.Context()); err != nil {
			redisHealthy = false
		}
		dbHealthy := true
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			dbHealthy = false
		}

		httpStatus := http.StatusOK
		overallStatus := "ok"
		if !redisHealthy || !dbHealthy {
			httpStatus = http.StatusServiceUnavailable
			overallStatus = "degraded"
		}

		c.JSON(httpStatus, gin.H{
			"status":   overallStatus,
			"redis":    redisHealthy,
			"database": dbHealthy,
		})
	})

	// API 路由组
	api := r.Group("/api")
	{
		// 认证路由
		auth := api.Group("/auth")
		{
			auth.GET("/registration-hints", authHandler.RegistrationHintsGin)
			auth.POST("/send-register-code", authHandler.SendRegisterCodeGin)
			auth.POST("/send-password-reset-code", authHandler.SendPasswordResetCodeGin)
			auth.POST("/reset-password", authHandler.ResetPasswordGin)
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
			users.GET("/profile/:username", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.GetProfileGin)
			users.POST("/:username/follow", middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.FollowUserGin)
			users.DELETE("/:username/follow", middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.UnfollowUserGin)
			users.GET("/:username/followers", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.GetFollowersGin)
			users.GET("/:username/following", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.GetFollowingGin)
			users.GET("/:username/mutual", middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.GetMutualFollowingGin)
			users.GET("/public", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), userHandler.GetPublicProfileGin)
		}

		// 题目路由
		problems := api.Group("/problems")
		{
			problems.Use(middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore))
			problems.GET("", probHandler.GetProblemsGin)
			problems.GET("/detail", probHandler.GetProblemDetailGin)
			problems.GET(":id", probHandler.GetProblemDetailGin)

			problemAuth := problems.Group("")
			problemAuth.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			problemAuth.GET(":id/instance", probHandler.GetProblemInstanceGin)
			problemAuth.POST(":id/instance/start", probHandler.StartProblemInstanceGin)
			problemAuth.DELETE(":id/instance", probHandler.DestroyProblemInstanceGin)
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
			contestAuth.GET(":id/participants", contestHandler.GetContestParticipantsGin)
			contestAuth.GET(":id/my-submissions", contestHandler.GetContestMySubmissionsGin)
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
			upload.POST("/contest-cover", middleware.RequireSiteAdminOrRoot(), uploadHandler.UploadContestCoverGin)
		}

		// 管理端路由
		admin := api.Group("/admin")
		{
			admin.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))

			adminUsers := admin.Group("/users")
			{
				adminUsers.GET("", middleware.RequireSiteAdminOrRoot(), userHandler.ListAdminUsersGin)
				adminUsers.GET("/role-change-logs", middleware.RequireRoot(), userHandler.ListRoleChangeLogsGin)
				adminUsers.PATCH(":id/active", middleware.RequireSiteAdminOrRoot(), userHandler.PatchUserActiveGin)
				adminUsers.PATCH(":id/role", middleware.RequireRoot(), userHandler.PatchUserRoleGin)
			}

			adminSiteConfig := admin.Group("/site-config")
			{
				adminSiteConfig.Use(middleware.RequireRoot())
				adminSiteConfig.GET("", siteConfigHandler.GetSiteConfigGin)
				adminSiteConfig.PUT("", siteConfigHandler.UpdateSiteConfigGin)
			}

			adminProblems := admin.Group("/problems")
			{
				adminProblems.GET("", middleware.RequireRoles("root", "site_admin", "contest_admin"), probHandler.ListAdminProblemsGin)
				adminProblems.GET(":id", middleware.RequireRoles("root", "site_admin", "contest_admin"), probHandler.GetAdminProblemDetailGin)
				adminProblems.POST("", middleware.RequireSiteAdminOrRoot(), probHandler.CreateProblemDraftGin)
				adminProblems.PATCH(":id/basic", middleware.RequireSiteAdminOrRoot(), probHandler.SaveProblemBasicGin)
				adminProblems.POST(":id/flag", middleware.RequireSiteAdminOrRoot(), probHandler.SaveProblemFlagGin)
				adminProblems.PATCH(":id/container", middleware.RequireSiteAdminOrRoot(), probHandler.SaveProblemContainerGin)
				adminProblems.POST(":id/image", middleware.RequireSiteAdminOrRoot(), probHandler.SaveProblemImageGin)
				adminProblems.POST(":id/attachment", middleware.RequireSiteAdminOrRoot(), probHandler.SaveProblemAttachmentGin)
				adminProblems.GET(":id/build-task", middleware.RequireSiteAdminOrRoot(), probHandler.GetProblemBuildTaskGin)
				adminProblems.POST(":id/publish", middleware.RequireSiteAdminOrRoot(), probHandler.PublishProblemGin)
				adminProblems.PUT(":id", middleware.RequireRoles("root", "site_admin", "contest_admin"), probHandler.UpdateProblemGin)
				adminProblems.DELETE(":id", middleware.RequireSiteAdminOrRoot(), probHandler.DeleteProblemGin)
			}

			adminContests := admin.Group("/contests")
			{
				adminContests.GET("", middleware.RequireSiteAdminOrRoot(), contestHandler.ListAdminContestsGin)
				adminContests.POST("", middleware.RequireSiteAdminOrRoot(), contestHandler.CreateContestGin)
				adminContests.DELETE(":id", middleware.RequireSiteAdminOrRoot(), contestHandler.DeleteContestGin)

				adminContests.GET(":id/admins", middleware.RequireContestAdminView(userRepo, contestAdminRepo), contestHandler.ListContestAdminsGin)
				adminContests.POST(":id/admins", middleware.RequireSiteAdminOrRoot(), contestHandler.AddContestAdminGin)
				adminContests.DELETE(":id/admins/:username", middleware.RequireSiteAdminOrRoot(), contestHandler.RemoveContestAdminGin)

				adminContests.PUT(":id", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.UpdateContestGin)
				adminContests.POST(":id/challenges", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.AddChallengeToContestGin)
				adminContests.DELETE(":id/challenges/:challengeID", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.RemoveChallengeFromContestGin)
				adminContests.GET(":id/submissions", middleware.RequireContestAdminView(userRepo, contestAdminRepo), contestHandler.ListContestSubmissionsGin)
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

	// 全站榜单采用延迟快照模式：每 5 分钟定时刷新一次。
	leaderboardService.StartSnapshotScheduler(5 * time.Minute)

	log.Println("✓ 服务器启动在 :" + cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
