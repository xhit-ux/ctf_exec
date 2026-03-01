package main

import (
	"log"
	"net/http"

	"github.com/ctf_exec/backend/internal/db"
	"github.com/ctf_exec/backend/internal/handler"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
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
	// TODO: 其他 Handler 需要类似的改造，暂时保持原样
	userHandler := handler.NewUserHandler(userRepo)
	probHandler := handler.NewProblemHandler(probRepo)
	subHandler := handler.NewSubmissionHandler(subRepo, probRepo)
	uploadHandler := handler.NewUploadHandler(userRepo)
	teamHandler := handler.NewTeamHandler(teamRepo)

	// CORS 中间件包装所有处理程序
	corsHandler := middleware.CORSMiddleware(http.DefaultServeMux)

	// 健康检查
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 认证路由（使用服务层）
	http.HandleFunc("/api/auth/register", authHandler.Register)
	http.HandleFunc("/api/auth/login", authHandler.Login)
	http.HandleFunc("/api/auth/me", middleware.AuthMiddleware(http.HandlerFunc(authHandler.GetCurrentUser)).ServeHTTP)

	// 用户路由（暂时保持原样）
	http.HandleFunc("/api/users/leaderboard", userHandler.GetLeaderboard)
	http.HandleFunc("/api/users/profile", middleware.AuthMiddleware(http.HandlerFunc(userHandler.UpdateProfile)).ServeHTTP)
	http.HandleFunc("/api/users/public", userHandler.GetPublicProfile)

	// 题目路由（暂时保持原样）
	http.HandleFunc("/api/problems", middleware.AuthMiddleware(http.HandlerFunc(probHandler.GetProblems)).ServeHTTP)
	http.HandleFunc("/api/problems/detail", middleware.AuthMiddleware(http.HandlerFunc(probHandler.GetProblemDetail)).ServeHTTP)

	// 提交路由（暂时保持原样）
	http.HandleFunc("/api/submissions", middleware.AuthMiddleware(http.HandlerFunc(subHandler.SubmitFlag)).ServeHTTP)
	http.HandleFunc("/api/submissions/me", middleware.AuthMiddleware(http.HandlerFunc(subHandler.GetMySubmissions)).ServeHTTP)

	// 文件上传路由（暂时保持原样）
	http.HandleFunc("/api/upload/avatar", middleware.AuthMiddleware(http.HandlerFunc(uploadHandler.UploadAvatar)).ServeHTTP)
	http.HandleFunc("/api/upload/contest-cover", middleware.AuthMiddleware(http.HandlerFunc(uploadHandler.UploadContestCover)).ServeHTTP)

	// 团队路由（暂时保持原样）
	http.HandleFunc("/api/teams", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.CreateTeam)).ServeHTTP)
	http.HandleFunc("/api/teams/join", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.JoinTeam)).ServeHTTP)
	http.HandleFunc("/api/teams/my", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.GetMyTeam)).ServeHTTP)
	http.HandleFunc("/api/teams/leave", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.LeaveTeam)).ServeHTTP)
	http.HandleFunc("/api/teams/dissolve", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.DissolveTeam)).ServeHTTP)
	http.HandleFunc("/api/teams/leaderboard", teamHandler.GetTeamLeaderboard)
	http.HandleFunc("/api/teams/public", middleware.OptionalAuthMiddleware(http.HandlerFunc(teamHandler.GetTeamPublic)).ServeHTTP)
	http.HandleFunc("/api/teams/update", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.UpdateTeam)).ServeHTTP)
	http.HandleFunc("/api/teams/kick", middleware.AuthMiddleware(http.HandlerFunc(teamHandler.KickMember)).ServeHTTP)

	// 静态文件服务
	fs := http.FileServer(http.Dir("../static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Println("✓ 服务器启动在 :8080")
	if err := http.ListenAndServe(":8080", corsHandler); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
