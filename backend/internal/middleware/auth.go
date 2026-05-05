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

var accessTokenCookieName = "ctf_access_token"

// SetAccessTokenCookieName 设置 access token 的 Cookie 名称。
func SetAccessTokenCookieName(name string) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return
	}
	accessTokenCookieName = normalized
}

func extractBearerToken(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil
}

func extractAccessToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if strings.TrimSpace(authHeader) != "" {
		return extractBearerToken(authHeader)
	}

	cookieToken, err := c.Cookie(accessTokenCookieName)
	if err != nil || strings.TrimSpace(cookieToken) == "" {
		return "", fmt.Errorf("missing access token")
	}

	return cookieToken, nil
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

// ensureClaimsIsCurrentSession 仅对 single_session 角色（admin 系）生效：
// 若 auth:user_current:<uid> 指针不再指向当前 jti，说明已被新登录互踢。
//
// 注意：当指针不存在（key 过期 / 旧链路）时视为通过，避免中间件升级造成存量 token 全 401。
func ensureClaimsIsCurrentSession(claims *utils.Claims, sessionStore *authsession.Store) error {
	if sessionStore == nil || claims == nil {
		return nil
	}

	normalized := models.NormalizeRole(claims.Role)
	if normalized != models.RoleRoot && normalized != models.RoleSiteAdmin {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	isCurrent, err := sessionStore.IsUserCurrentSession(ctx, claims.UserID, claims.ID)
	if err != nil {
		return err
	}
	if !isCurrent {
		return fmt.Errorf("session superseded by newer login")
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
		tokenString, err := extractAccessToken(c)
		if err != nil {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "缺少认证信息"), http.StatusUnauthorized)
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

		if err := ensureClaimsIsCurrentSession(claims, sessionStore); err != nil {
			apierror.Write(c, apperr.New(apperr.ErrUnauthorized, "你的账号已在其他设备登录，请重新登录"), http.StatusUnauthorized)
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
		tokenString, err := extractAccessToken(c)
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

		// 可选认证：单会话被互踢时降级为匿名而不是 401。
		if err := ensureClaimsIsCurrentSession(claims, sessionStore); err != nil {
			c.Next()
			return
		}

		claims.Role = models.NormalizeRole(claims.Role)
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}
