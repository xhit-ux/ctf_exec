package middleware

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/apierror"
	"github.com/ctf_exec/backend/internal/apperr"
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
			apierror.Write(c, apperr.New(apperr.ErrBadRequest, "缺少赛事ID"), http.StatusBadRequest)
			c.Abort()
			return
		}

		user := &models.User{ID: claims.UserID, Role: claims.Role}
		if userRepo != nil {
			dbUser, err := userRepo.GetUserByID(claims.UserID)
			if err != nil {
				apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "用户状态异常，请重新登录"), http.StatusUnauthorized)
				c.Abort()
				return
			}
			user = dbUser
		}

		ok, err := authz.CanManageContest(user, contestID, contestAdminRepo)
		if err != nil {
			apierror.Write(c, apperr.Wrap(apperr.ErrInternal, "权限校验失败", err), http.StatusInternalServerError)
			c.Abort()
			return
		}
		if !ok {
			apierror.Write(c, apperr.New(apperr.ErrForbidden, "无权操作该赛事"), http.StatusForbidden)
			c.Abort()
			return
		}

		c.Next()
	}
}
