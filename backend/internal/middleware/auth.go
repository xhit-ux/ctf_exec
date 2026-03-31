package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apierror"
	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/authsession"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

func extractBearerToken(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil
}

func ensureClaimsUserValid(claims *utils.Claims, userRepo *repository.UserRepository) error {
	if userRepo == nil {
		return nil
	}

	user, err := userRepo.GetUserByID(claims.UserID)
	if err != nil {
		return err
	}

	if models.NormalizeRole(claims.Role) != models.NormalizeRole(user.Role) {
		return fmt.Errorf("role mismatch")
	}

	return nil
}

func ensureClaimsSessionValid(claims *utils.Claims, sessionStore *authsession.Store) error {
	if sessionStore == nil {
		return nil
	}
	if claims == nil || claims.UserID == "" || claims.ID == "" {
		return fmt.Errorf("invalid jwt claims")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	active, err := sessionStore.IsSessionActive(ctx, claims.UserID, claims.ID)
	if err != nil {
		return err
	}
	if !active {
		return fmt.Errorf("jwt session revoked")
	}

	return nil
}

// GinAuthMiddleware Gin 认证中间件。
func GinAuthMiddleware() gin.HandlerFunc {
	return GinAuthMiddlewareWithRepo(nil, nil)
}

// GinAuthMiddlewareWithRepo Gin 认证中间件（包含用户状态校验）。
func GinAuthMiddlewareWithRepo(userRepo *repository.UserRepository, sessionStore *authsession.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "缺少认证信息"), http.StatusUnauthorized)
			c.Abort()
			return
		}

		tokenString, err := extractBearerToken(authHeader)
		if err != nil {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "认证信息格式错误"), http.StatusUnauthorized)
			c.Abort()
			return
		}

		claims, err := utils.VerifyTokenByType(tokenString, utils.TokenTypeAccess)
		if err != nil {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "登录凭证无效"), http.StatusUnauthorized)
			c.Abort()
			return
		}

		if err := ensureClaimsUserValid(claims, userRepo); err != nil {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "账户不存在、已被封禁或权限已变更，请重新登录"), http.StatusUnauthorized)
			c.Abort()
			return
		}

		if err := ensureClaimsSessionValid(claims, sessionStore); err != nil {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "登录已失效，请重新登录"), http.StatusUnauthorized)
			c.Abort()
			return
		}

		claims.Role = models.NormalizeRole(claims.Role)
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

// GinOptionalAuthMiddleware Gin 可选认证中间件。
func GinOptionalAuthMiddleware() gin.HandlerFunc {
	return GinOptionalAuthMiddlewareWithRepo(nil, nil)
}

// GinOptionalAuthMiddlewareWithRepo Gin 可选认证中间件（包含用户状态校验，失败时降级为匿名请求）。
func GinOptionalAuthMiddlewareWithRepo(userRepo *repository.UserRepository, sessionStore *authsession.Store) gin.HandlerFunc {
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

		claims, err := utils.VerifyTokenByType(tokenString, utils.TokenTypeAccess)
		if err != nil {
			c.Next()
			return
		}

		if err := ensureClaimsUserValid(claims, userRepo); err != nil {
			c.Next()
			return
		}

		if err := ensureClaimsSessionValid(claims, sessionStore); err != nil {
			c.Next()
			return
		}

		claims.Role = models.NormalizeRole(claims.Role)
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}
