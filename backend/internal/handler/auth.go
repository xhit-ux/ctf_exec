package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
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

// Register 处理用户注册
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}
	defer r.Body.Close()

	var req models.RegisterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	// 调用服务层
	response, err := h.authService.Register(&req)
	if err != nil {
		// 根据错误类型设置相应的 HTTP 状态码
		statusCode := http.StatusInternalServerError
		errorMsg := err.Error()
		
		// 根据错误消息判断状态码
		if strings.Contains(errorMsg, "用户名已被使用") || strings.Contains(errorMsg, "邮箱已被使用") {
			statusCode = http.StatusConflict
		} else if strings.Contains(errorMsg, "请求验证失败") {
			statusCode = http.StatusBadRequest
		}
		
		writeError(w, statusCode, errorMsg)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Login 处理用户登陆
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}
	defer r.Body.Close()

	var req models.LoginRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误")
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
		
		writeError(w, statusCode, errorMsg)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetCurrentUser 获取当前用户信息
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 从 context 中获取用户信息
	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权的请求")
		return
	}

	// 使用用户服务获取用户信息
	user, err := h.userService.GetCurrentUser(claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

// 写入错误响应
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Code:    statusCode,
		Message: message,
	})
}
