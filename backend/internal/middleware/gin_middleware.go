package middleware

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

// GinAuthMiddleware Gin 认证中间件
func GinAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取 Authorization 头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			c.Abort()
			return
		}

		// 提取 bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 验证 token
		claims, err := utils.VerifyToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// 将 claims 存储在 Gin 上下文中
		c.Set("claims", claims)
		c.Next()
	}
}

// GinOptionalAuthMiddleware Gin 可选认证中间件 (如果提供了 token 则解析，否则继续)
func GinOptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := utils.VerifyToken(parts[1])
		if err != nil {
			c.Next()
			return
		}

		// 将 claims 存储在 Gin 上下文中
		c.Set("claims", claims)
		c.Next()
	}
}

// GinCORSMiddleware Gin CORS 中间件
func GinCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// RequireRoles 要求当前登录用户属于指定角色之一。
// 若传入 userRepo，会额外进行数据库角色一致性校验，降低角色变更后的旧 token 风险。
func RequireRoles(userRepo *repository.UserRepository, roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		normalized := models.NormalizeRole(role)
		if models.IsValidRole(normalized) {
			allowed[normalized] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		claimsInterface, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权的请求"})
			c.Abort()
			return
		}

		claims, ok := claimsInterface.(*utils.Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
			c.Abort()
			return
		}

		effectiveRole := models.NormalizeRole(claims.Role)
		if userRepo != nil {
			user, err := userRepo.GetUserByID(claims.UserID)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "用户状态异常，请重新登录"})
				c.Abort()
				return
			}

			dbRole := models.NormalizeRole(user.Role)
			if effectiveRole != dbRole {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "角色已变更，请重新登录"})
				c.Abort()
				return
			}

			effectiveRole = dbRole
		}

		if _, ok := allowed[effectiveRole]; !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}

		c.Next()
	}
}
