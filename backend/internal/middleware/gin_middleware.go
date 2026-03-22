package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

const claimsContextKey = "claims"

func extractBearerToken(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil
}

// GetClaims 从上下文获取并校验 claims。
func GetClaims(c *gin.Context) (*utils.Claims, error) {
	claimsInterface, exists := c.Get(claimsContextKey)
	if !exists {
		return nil, fmt.Errorf("claims not found")
	}

	claims, ok := claimsInterface.(*utils.Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	claims.Role = models.NormalizeRole(claims.Role)
	return claims, nil
}

// MustGetClaims 从上下文读取 claims，失败时自动返回 401 并终止请求。
func MustGetClaims(c *gin.Context) (*utils.Claims, bool) {
	claims, err := GetClaims(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
		c.Abort()
		return nil, false
	}

	return claims, true
}

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

		tokenString, err := extractBearerToken(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		// 验证 token
		claims, err := utils.VerifyToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims.Role = models.NormalizeRole(claims.Role)

		// 将 claims 存储在 Gin 上下文中
		c.Set(claimsContextKey, claims)
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

		tokenString, err := extractBearerToken(authHeader)
		if err != nil {
			c.Next()
			return
		}

		claims, err := utils.VerifyToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		claims.Role = models.NormalizeRole(claims.Role)

		// 将 claims 存储在 Gin 上下文中
		c.Set(claimsContextKey, claims)
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
		claims, ok := MustGetClaims(c)
		if !ok {
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

// RequireSiteAdminOrRoot 要求角色为 root 或 site_admin。
func RequireSiteAdminOrRoot(userRepo *repository.UserRepository) gin.HandlerFunc {
	return RequireRoles(userRepo, models.RoleRoot, models.RoleSiteAdmin)
}

// RequireRoot 要求角色为 root。
func RequireRoot(userRepo *repository.UserRepository) gin.HandlerFunc {
	return RequireRoles(userRepo, models.RoleRoot)
}
