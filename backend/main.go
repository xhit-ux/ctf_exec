package main

import (
	"context"
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
	k8s "github.com/ctf_exec/backend/internal/k8s"
	"github.com/ctf_exec/backend/internal/middleware"
	minio "github.com/ctf_exec/backend/internal/minio"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
	wsmod "github.com/ctf_exec/backend/internal/ws"
	"github.com/gin-gonic/gin"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// ============================================================
	// 平台核心服务启动
	// ============================================================

	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogFatal("加载配置失败: %v", err)
	}
	if config.IsWeakJWTSecret(cfg.JWT.Secret) {
		log.Fatal(utils.Security.Sprintf("SECURITY: JWT_SECRET 为空或仍为默认值，服务拒绝启动"))
	}

	// -- Infrastructure Bootstrap --
	ctx := context.Background()

	k8sClient, err := k8s.NewClient(cfg.Container.KubeconfigPath)
	if err != nil {
		utils.LogFatal("Kubernetes 客户端初始化失败: %v", err)
	}
	utils.LogSuccess("Kubernetes 客户端已连接")

	regInfo := k8s.NewRegistryConfig()
	if err := k8s.EnsureRegistry(ctx, k8sClient, regInfo, cfg.Container.InfraNodeName, cfg.Registry.StorageSize); err != nil {
		utils.LogFatal("私有镜像仓库部署失败: %v", err)
	}
	cfg.SetRegistryInfo(regInfo.Host, regInfo.Username, regInfo.Password)
	utils.LogSuccess("私有镜像仓库已就绪: %s", regInfo.Host)
	utils.LogInfra("Registry 认证凭证 — 用户名: %s  密码: %s", regInfo.Username, regInfo.Password)

	if err := k8s.ConfigureDockerInsecureRegistry(regInfo.NodePort); err != nil {
		utils.LogWarn("Docker insecure registry 配置失败 (非致命): %v", err)
	} else {
		utils.LogSuccess("Docker insecure registry 已配置")
	}

	// 数据库
	if err := db.EnsureDatabaseExists(cfg.Database, "../db/final.sql"); err != nil {
		utils.LogFatal("数据库初始化检查失败: %v", err)
	}
	sqlDB, err := db.InitDB(cfg.Database)
	if err != nil {
		utils.LogFatal("数据库初始化失败: %v", err)
	}
	defer sqlDB.Close()
	utils.LogSuccess("数据库连接成功")

	// JWT
	utils.InitJWTConfig(cfg.JWT)
	utils.InitFlagHashSecret("", cfg.JWT.Secret)

	// Redis
	redisClient, err := cache.InitRedis(cfg.Redis)
	if err != nil {
		utils.LogFatal("Redis 初始化失败: %v", err)
	}
	defer redisClient.Close()
	utils.LogSuccess("Redis 连接成功")

	// MinIO
	var minioClient *minio.Client
	if cfg.MinIO.Enabled {
		var initErr error
		minioClient, initErr = minio.NewClient(cfg.MinIO)
		if initErr != nil {
			utils.LogFatal("MinIO 客户端初始化失败: %v", initErr)
		}
		if initErr := minioClient.EnsureBuckets(context.Background()); initErr != nil {
			utils.LogFatal("MinIO 桶初始化失败: %v", initErr)
		}
		utils.LogSuccess("MinIO 连接成功，桶已就绪")
	}

	// 仓库 & 服务层
	userRepo := repository.NewUserRepository(sqlDB)
	probRepo := repository.NewProblemRepository(sqlDB, k8sClient, cfg, redisClient, minioClient)
	subRepo := repository.NewSubmissionRepository(sqlDB)
	teamRepo := repository.NewTeamRepository(sqlDB)
	contestRepo := repository.NewContestRepository(sqlDB)
	contestAdminRepo := repository.NewContestAdminRepository(sqlDB)
	siteConfigRepo := repository.NewSiteConfigRepository(sqlDB)
	probRepo.StartDraftCleanupScheduler(24*time.Hour, 7*24*time.Hour); probRepo.StartTTLCleanupScheduler(5 * time.Minute)
	if err := service.EnsureDefaultRootUser(userRepo, cfg.Init.RootDefaultPassword); err != nil {
		utils.LogFatal("初始化 root 用户失败: %v", err)
	}

	sessionStore := authsession.NewStore(redisClient.Client, sqlDB)
	sessionStore.StartRefreshTokenCleanupScheduler(24*time.Hour, 7*24*time.Hour)
	authService := service.NewAuthService(userRepo, siteConfigRepo, redisClient.Client, sessionStore, cfg)
	userService := service.NewUserService(userRepo)
	_ = service.NewProblemService(probRepo)

	leaderboardService := service.NewLeaderboardService(redisClient.Client, userRepo, teamRepo, cfg)
	contestService := service.NewContestService(contestRepo, contestAdminRepo, teamRepo, leaderboardService)
	submissionService := service.NewSubmissionService(subRepo, probRepo, userRepo, contestService, redisClient.Client, cfg.RateLimit.SubmissionLimit, cfg.RateLimit.WindowSeconds)
	teamService := service.NewTeamService(teamRepo)

	// Handler
	authHandler := handler.NewAuthHandler(authService, userService, cfg.AuthCookie)
	userHandler := handler.NewUserHandler(userRepo, authService, redisClient.Client, sessionStore)
	probHandler := handler.NewProblemHandler(probRepo, contestRepo, contestAdminRepo, minioClient, cfg)
	subHandler := handler.NewSubmissionHandler(subRepo, probRepo, submissionService)
	uploadHandler := handler.NewUploadHandler(userRepo)
	teamHandler := handler.NewTeamHandler(teamRepo, teamService)
	contestHandler := handler.NewContestHandler(contestRepo, userRepo, contestAdminRepo, contestService)
	siteConfigHandler := handler.NewSiteConfigHandler(siteConfigRepo)
	wsHandler := wsmod.NewHandler(cfg.WS, sessionStore, cfg.Server.AllowedOrigins)

	// ============================================================
	// Gin HTTP 框架 & 路由 (debug 信息从此处开始输出)
	// ============================================================
	utils.LogInfo("── Gin HTTP 框架初始化 ──")

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Println(utils.Error.Sprintf("[panic-recovered] method=%s path=%s ip=%s panic=%v",
			c.Request.Method, c.Request.URL.Path, c.ClientIP(), recovered))
		apierror.Write(c, apperr.New(apperr.ErrInternal, "服务器开小差了，请稍后再试"), http.StatusInternalServerError)
		c.Abort()
	}))
	for _, proxy := range cfg.Server.TrustedProxies {
		normalized := strings.TrimSpace(proxy)
		switch normalized {
		case "0.0.0.0", "0.0.0.0/0", "::", "::/0":
			utils.LogFatal("SERVER_TRUSTED_PROXIES 不应包含 %s", normalized)
		}
	}
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		utils.LogFatal("配置受信任代理失败: %v", err)
	}

	middleware.SetAccessTokenCookieName(cfg.AuthCookie.AccessTokenName)
	r.Use(middleware.GinCORSMiddleware(cfg.Server.AllowedOrigins))
	if len(cfg.Server.AllowedOrigins) > 0 {
		r.Use(middleware.GinCSRFMiddleware(cfg.Server.AllowedOrigins))
	}

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
		c.JSON(httpStatus, gin.H{"status": overallStatus, "redis": redisHealthy, "database": dbHealthy})
	})

	api := r.Group("/api")
	{
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
		problems := api.Group("/problems")
		{
			problems.Use(middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore))
			problems.GET("", probHandler.GetProblemsGin)
			problems.GET("/detail", probHandler.GetProblemDetailGin)
			problems.GET(":id", probHandler.GetProblemDetailGin)
			problems.GET(":id/attachment", probHandler.GetProblemAttachmentDownloadGin)
			problems.GET(":id/hints", probHandler.ListProblemHintsGin)
			problems.GET(":id/prerequisites", probHandler.ListProblemPrerequisitesGin)
			problemAuth := problems.Group("")
			problemAuth.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			problemAuth.GET(":id/instance", probHandler.GetProblemInstanceGin)
			problemAuth.POST(":id/instance/start", probHandler.StartProblemInstanceGin)
			problemAuth.DELETE(":id/instance", probHandler.DestroyProblemInstanceGin)
			problemAuth.POST(":id/instance/extend", probHandler.ExtendProblemInstanceGin)
			problemAuth.POST("/hints/:hintID/unlock", probHandler.UnlockHintGin)
		}
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
		submissions := api.Group("/submissions")
		{
			submissions.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			submissions.POST("", subHandler.SubmitFlagGin)
			submissions.GET("/me", subHandler.GetMySubmissionsGin)
		}
		upload := api.Group("/upload")
		{
			upload.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			upload.POST("/avatar", uploadHandler.UploadAvatarGin)
			upload.POST("/contest-cover", middleware.RequireSiteAdminOrRoot(), uploadHandler.UploadContestCoverGin)
		}
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
				adminProblems.DELETE(":id", middleware.RequireSiteAdminOrRoot(), probHandler.DeleteProblemGin)
				adminProblems.POST(":id/hints", middleware.RequireSiteAdminOrRoot(), probHandler.CreateProblemHintGin)
				adminProblems.PUT(":id/hints/:hintID", middleware.RequireSiteAdminOrRoot(), probHandler.UpdateProblemHintGin)
				adminProblems.DELETE(":id/hints/:hintID", middleware.RequireSiteAdminOrRoot(), probHandler.DeleteProblemHintGin)
				adminProblems.PUT(":id/prerequisites", middleware.RequireSiteAdminOrRoot(), probHandler.SetProblemPrerequisitesGin)
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
				adminContests.PATCH(":id/challenges/:challengeID", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.UpdateContestChallengeGin)
				adminContests.DELETE(":id/challenges/:challengeID", middleware.RequireContestAdmin(userRepo, contestAdminRepo), contestHandler.RemoveChallengeFromContestGin)
				adminContests.GET(":id/submissions", middleware.RequireContestAdminView(userRepo, contestAdminRepo), contestHandler.ListContestSubmissionsGin)
			}
		}
		teams := api.Group("/teams")
		{
			teams.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			teams.POST("", teamHandler.CreateTeamGin)
			teams.POST("/join", teamHandler.JoinTeamGin)
			teams.GET("/my", teamHandler.GetMyTeamGin)
			teams.DELETE("/leave", teamHandler.LeaveTeamGin)
			teams.DELETE("/dissolve", teamHandler.DissolveTeamGin)
			teams.PUT("/update", teamHandler.UpdateTeamGin)
			teams.POST("/kick", teamHandler.KickMemberGin)
		}
		teamsPublic := api.Group("/teams")
		{
			teamsPublic.GET("/leaderboard", teamHandler.GetTeamLeaderboardGin)
			teamsPublic.GET("/public", middleware.GinOptionalAuthMiddlewareWithRepo(userRepo, sessionStore), teamHandler.GetTeamPublicGin)
		}
		if cfg.WS.Enabled {
			ws := api.Group("/ws")
			ws.Use(middleware.GinAuthMiddlewareWithRepo(userRepo, sessionStore))
			ws.GET("/connect", wsHandler.HandleConnect)
		}
	}

	r.Static("/static", "../static")
	leaderboardService.StartSnapshotScheduler(5 * time.Minute)

	// ============================================================
	// 服务端口监听
	// ============================================================
	utils.LogInfo("── 监听端口 :%s ──", cfg.Server.Port)
	utils.LogSuccess("所有核心服务已就绪，开始接受请求")

	if err := r.Run(":" + cfg.Server.Port); err != nil {
		utils.LogFatal("服务器启动失败: %v", err)
	}
}
