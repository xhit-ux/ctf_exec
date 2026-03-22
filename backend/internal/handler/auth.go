package handler

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService service.AuthService
	userService service.UserService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService service.AuthService, userService service.UserService) *AuthHandler {
	return &AuthHandler{authService: authService, userService: userService}
}

// SendRegisterCodeGin 发送注册邮箱验证码
func (h *AuthHandler) SendRegisterCodeGin(c *gin.Context) {
	var req models.SendRegisterCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	// 调用服务层
	response, err := h.authService.Register(&req, realClientIP(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// LoginGin Gin 版本的用户登录处理
func (h *AuthHandler) LoginGin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	// 调用服务层
	response, err := h.authService.Login(&req, realClientIP(c))
	if err != nil {
		writeError(c, err, http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCurrentUserGin Gin 版本的获取当前用户信息处理
func (h *AuthHandler) GetCurrentUserGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	// 使用用户服务获取用户信息
	user, err := h.userService.GetCurrentUser(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, user)
}
