package main

import (
	"log"
	"net/http"

	"github.com/ctf_exec/backend/internal/db"
	"github.com/ctf_exec/backend/internal/handler"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化数据库
	sqlDB, err := db.InitDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer sqlDB.Close()
	log.Println("✓ 数据库连接成功")

	// 初始化仓库
	userRepo := repository.NewUserRepository(sqlDB)
	probRepo := repository.NewProblemRepository(sqlDB)
	subRepo := repository.NewSubmissionRepository(sqlDB)
	teamRepo := repository.NewTeamRepository(sqlDB)

	// 初始化服务层
	authService := service.NewAuthService(userRepo)
	userService := service.NewUserService(userRepo)
	_ = service.NewProblemService(probRepo) // 暂时未使用
	_ = service.NewTeamService(teamRepo)    // 暂时未使用

	// 初始化处理器（使用服务层）
	authHandler := handler.NewAuthHandler(authService, userService)
	userHandler := handler.NewUserHandler(userRepo)
	probHandler := handler.NewProblemHandler(probRepo)
	subHandler := handler.NewSubmissionHandler(subRepo, probRepo)
	uploadHandler := handler.NewUploadHandler(userRepo)
	teamHandler := handler.NewTeamHandler(teamRepo)

	// 创建 Gin 引擎
	r := gin.Default()

	// 添加 CORS 中间件（使用 Gin 中间件）
	r.Use(middleware.GinCORSMiddleware())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API 路由组
	api := r.Group("/api")
	{
		// 认证路由
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.RegisterGin)
			auth.POST("/login", authHandler.LoginGin)
			auth.GET("/me", middleware.GinAuthMiddleware(), authHandler.GetCurrentUserGin)
		}

		// 用户路由
		users := api.Group("/users")
		{
			users.GET("/leaderboard", userHandler.GetLeaderboardGin)
			users.GET("/profile", middleware.GinAuthMiddleware(), userHandler.UpdateProfileGin)
			users.GET("/public", userHandler.GetPublicProfileGin)
		}

		// 题目路由
		problems := api.Group("/problems")
		{
			problems.Use(middleware.GinAuthMiddleware())
			problems.GET("", probHandler.GetProblemsGin)
			problems.GET("/detail", probHandler.GetProblemDetailGin)
		}

		// 提交路由
		submissions := api.Group("/submissions")
		{
			submissions.Use(middleware.GinAuthMiddleware())
			submissions.POST("", subHandler.SubmitFlagGin)
			submissions.GET("/me", subHandler.GetMySubmissionsGin)
		}

		// 文件上传路由
		upload := api.Group("/upload")
		{
			upload.Use(middleware.GinAuthMiddleware())
			upload.POST("/avatar", uploadHandler.UploadAvatarGin)
			upload.POST("/contest-cover", uploadHandler.UploadContestCoverGin)
		}

		// 团队路由
		teams := api.Group("/teams")
		{
			// 需要认证的路由
			teams.Use(middleware.GinAuthMiddleware())
			teams.POST("", teamHandler.CreateTeamGin)
			teams.POST("/join", teamHandler.JoinTeamGin)
			teams.GET("/my", teamHandler.GetMyTeamGin)
			teams.POST("/leave", teamHandler.LeaveTeamGin)
			teams.POST("/dissolve", teamHandler.DissolveTeamGin)
			teams.POST("/update", teamHandler.UpdateTeamGin)
			teams.POST("/kick", teamHandler.KickMemberGin)

			// 不需要认证的路由（需要特殊处理）
			// 对于可选认证的路由，我们需要单独处理
			// 先移除认证中间件，然后添加可选认证中间件
		}

		// 团队公开路由（不需要认证或可选认证）
		teamsPublic := api.Group("/teams")
		{
			teamsPublic.GET("/leaderboard", teamHandler.GetTeamLeaderboardGin)
			teamsPublic.GET("/public", middleware.GinOptionalAuthMiddleware(), teamHandler.GetTeamPublicGin)
		}
	}

	// 静态文件服务
	r.Static("/static", "../static")

	log.Println("✓ 服务器启动在 :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}