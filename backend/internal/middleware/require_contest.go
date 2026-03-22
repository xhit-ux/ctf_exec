package middleware

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/authz"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// RequireContestAdmin 要求当前用户拥有赛事管理权限（root/site_admin/contest_admin）。
func RequireContestAdmin(userRepo *repository.UserRepository, contestAdminRepo repository.ContestAdminRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := MustGetClaims(c)
		if !ok {
			return
		}

		contestID := strings.TrimSpace(c.Param("id"))
		if contestID == "" {
			contestID = strings.TrimSpace(c.Param("contest_id"))
		}
		if contestID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
			c.Abort()
			return
		}

		user := &models.User{ID: claims.UserID, Role: claims.Role}
		if userRepo != nil {
			dbUser, err := userRepo.GetUserByID(claims.UserID)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "用户状态异常，请重新登录"})
				c.Abort()
				return
			}
			user = dbUser
		}

		ok, err := authz.CanManageContest(user, contestID, contestAdminRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "权限校验失败"})
			c.Abort()
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权操作该赛事"})
			c.Abort()
			return
		}

		c.Next()
	}
}
