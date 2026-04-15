package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService service.AuthService
	userService service.UserService
	cookieCfg   config.AuthCookieConfig
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService service.AuthService, userService service.UserService, cookieCfg config.AuthCookieConfig) *AuthHandler {
	return &AuthHandler{authService: authService, userService: userService, cookieCfg: cookieCfg}
}

func sameSiteByConfig(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, accessToken, refreshToken string, accessTTL, refreshTTL int64) {
	sameSite := sameSiteByConfig(h.cookieCfg.SameSite)
	secure := h.cookieCfg.Secure
	domain := strings.TrimSpace(h.cookieCfg.Domain)

	maxAccessAge := int(accessTTL)
	if maxAccessAge < 0 {
		maxAccessAge = 0
	}
	maxRefreshAge := int(refreshTTL)
	if maxRefreshAge < 0 {
		maxRefreshAge = 0
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieCfg.AccessTokenName,
		Value:    accessToken,
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   maxAccessAge,
		Expires:  time.Now().Add(time.Duration(maxAccessAge) * time.Second),
	})

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieCfg.RefreshTokenName,
		Value:    refreshToken,
		Path:     "/api/auth",
		Domain:   domain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   maxRefreshAge,
		Expires:  time.Now().Add(time.Duration(maxRefreshAge) * time.Second),
	})
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	sameSite := sameSiteByConfig(h.cookieCfg.SameSite)
	secure := h.cookieCfg.Secure
	domain := strings.TrimSpace(h.cookieCfg.Domain)

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieCfg.AccessTokenName,
		Value:    "",
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieCfg.RefreshTokenName,
		Value:    "",
		Path:     "/api/auth",
		Domain:   domain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (h *AuthHandler) writeAuthResponse(c *gin.Context, status int, response *models.AuthResponse) {
	h.setAuthCookies(c, response.Token, response.RefreshToken, response.AccessExpiresInSecs, response.RefreshExpiresInSecs)

	response.Token = ""
	response.RefreshToken = ""
	c.JSON(status, response)
}

// SendRegisterCodeGin 发送注册邮箱验证码
func (h *AuthHandler) SendRegisterCodeGin(c *gin.Context) {
	var req models.SendRegisterCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	err := h.authService.SendRegisterCode(&req, realClientIP(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送，请前往邮箱查收"})
}

// RegisterGin Gin 版本的用户注册处理
func (h *AuthHandler) RegisterGin(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	// 调用服务层
	response, err := h.authService.Register(&req, realClientIP(c), c.Request.UserAgent())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	h.writeAuthResponse(c, http.StatusCreated, response)
}

// LoginGin Gin 版本的用户登录处理
func (h *AuthHandler) LoginGin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	// 调用服务层
	response, err := h.authService.Login(&req, realClientIP(c), c.Request.UserAgent())
	if err != nil {
		writeError(c, err, http.StatusUnauthorized)
		return
	}

	h.writeAuthResponse(c, http.StatusOK, response)
}

// RefreshGin 使用 refresh token 轮换签发新 token 对。
func (h *AuthHandler) RefreshGin(c *gin.Context) {
	var req models.RefreshTokenRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, "请求体格式错误")
			return
		}
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		cookieToken, err := c.Cookie(h.cookieCfg.RefreshTokenName)
		if err == nil {
			refreshToken = strings.TrimSpace(cookieToken)
		}
	}

	response, err := h.authService.Refresh(refreshToken, realClientIP(c), c.Request.UserAgent())
	if err != nil {
		h.clearAuthCookies(c)
		writeError(c, err, http.StatusUnauthorized)
		return
	}

	h.writeAuthResponse(c, http.StatusOK, response)
}

// LogoutGin Gin 版本的用户登出处理。
func (h *AuthHandler) LogoutGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	var req models.LogoutRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			writeBadRequest(c, "请求体格式错误")
			return
		}
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		cookieToken, err := c.Cookie(h.cookieCfg.RefreshTokenName)
		if err == nil {
			refreshToken = strings.TrimSpace(cookieToken)
		}
	}

	if err := h.authService.Logout(claims.UserID, claims.ID, refreshToken); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	h.clearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

// GetCurrentUserGin Gin 版本的获取当前用户信息处理
func (h *AuthHandler) GetCurrentUserGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	if claims.ExpiresAt != nil {
		remaining := int(time.Until(claims.ExpiresAt.Time).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-Access-Expires-In-Secs", strconv.Itoa(remaining))
	}

	// 使用用户服务获取用户信息
	user, err := h.userService.GetCurrentUser(claims.UserID)
	if err != nil {
		writeNotFound(c, "用户不存在")
		return
	}

	c.JSON(http.StatusOK, user)
}
