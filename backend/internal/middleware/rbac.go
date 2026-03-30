package middleware

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/authz"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// RequireRoles 要求当前登录用户属于指定角色之一。
// 角色直接以 JWT claims 为准；会话撤销由认证中间件统一保证。
func RequireRoles(roles ...string) gin.HandlerFunc {
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

		if !authz.CanAccessRole(effectiveRole, allowed...) {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSiteAdminOrRoot 要求角色为 root 或 site_admin。
func RequireSiteAdminOrRoot() gin.HandlerFunc {
	return RequireRoles(models.RoleRoot, models.RoleSiteAdmin)
}

// RequireRoot 要求角色为 root。
func RequireRoot() gin.HandlerFunc {
	return RequireRoles(models.RoleRoot)
}
