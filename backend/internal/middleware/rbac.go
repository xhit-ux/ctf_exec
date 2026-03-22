package middleware

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/authz"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// RequireRoles 要求当前登录用户属于指定角色之一。
// 若传入 userRepo，会额外进行数据库角色一致性校验，降低角色变更后的旧 token 风险。
func RequireRoles(userRepo *repository.UserRepository, roles ...string) gin.HandlerFunc {
	allowed := make([]string, 0, len(roles))
	for _, role := range roles {
		normalized := models.NormalizeRole(role)
		if models.IsValidRole(normalized) {
			allowed = append(allowed, normalized)
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

		if !authz.CanAccessRole(effectiveRole, allowed...) {
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
