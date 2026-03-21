package handler

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
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

	err := h.authService.SendRegisterCode(&req, c.ClientIP())
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMsg := err.Error()

		if strings.Contains(errorMsg, "请求验证失败") || strings.Contains(errorMsg, "机器人验证") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(errorMsg, "已被使用") {
			statusCode = http.StatusConflict
		} else if strings.Contains(errorMsg, "频繁") || strings.Contains(errorMsg, "上限") {
			statusCode = http.StatusTooManyRequests
		}

		c.JSON(statusCode, gin.H{"error": errorMsg})
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
	response, err := h.authService.Register(&req, c.ClientIP())
	if err != nil {
		// 根据错误类型设置相应的 HTTP 状态码
		statusCode := http.StatusInternalServerError
		errorMsg := err.Error()

		// 根据错误消息判断状态码
		if strings.Contains(errorMsg, "用户名已被使用") || strings.Contains(errorMsg, "邮箱已被使用") {
			statusCode = http.StatusConflict
		} else if strings.Contains(errorMsg, "请求验证失败") || strings.Contains(errorMsg, "机器人验证") || strings.Contains(errorMsg, "验证码") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(errorMsg, "频繁") || strings.Contains(errorMsg, "上限") {
			statusCode = http.StatusTooManyRequests
		}

		c.JSON(statusCode, gin.H{"error": errorMsg})
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
	response, err := h.authService.Login(&req)
	if err != nil {
		statusCode := http.StatusUnauthorized
		errorMsg := err.Error()

		if strings.Contains(errorMsg, "请求验证失败") {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{"error": errorMsg})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCurrentUserGin Gin 版本的获取当前用户信息处理
func (h *AuthHandler) GetCurrentUserGin(c *gin.Context) {
	// 从 Gin 上下文中获取用户信息
	claimsInterface, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权的请求"})
		return
	}

	claims, ok := claimsInterface.(*utils.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
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
