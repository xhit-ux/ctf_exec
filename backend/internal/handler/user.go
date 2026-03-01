package handler

import (
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo *repository.UserRepository
}

func NewUserHandler(userRepo *repository.UserRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

// GetLeaderboardGin Gin 版本的获取排行榜
func (h *UserHandler) GetLeaderboardGin(c *gin.Context) {
	limitStr := c.Query("limit")
	limit := 10 // 默认 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	users, err := h.userRepo.GetLeaderboard(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取排行榜失败"})
		return
	}

	type SafeUser struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Score       int    `json:"score"`
		SolvedCount int    `json:"solved_count"`
		AvatarURL   string `json:"avatar_url"`
		Rank        int    `json:"rank"`
	}

	safeUsers := make([]SafeUser, len(users))
	for i, u := range users {
		safeUsers[i] = SafeUser{
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Score:       u.Score,
			SolvedCount: u.SolvedCount,
			AvatarURL:   u.AvatarURL,
			Rank:        i + 1,
		}
	}

	c.JSON(http.StatusOK, safeUsers)
}

// UpdateProfileGin Gin 版本的更新当前用户信息
func (h *UserHandler) UpdateProfileGin(c *gin.Context) {
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

	// 解析请求体
	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	// 简单验证
	if req.DisplayName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "显示名称不能为空"})
		return
	}

	// 更新用户信息前，如果是修改了邮箱，需要检查邮箱是否已存在
	user, _ := h.userRepo.GetUserByID(claims.UserID)
	if user != nil && user.Email != req.Email {
		exists, err := h.userRepo.UserExistsByEmail(req.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "检查邮箱失败"})
			return
		}
		if exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该邮箱已被其他账号使用"})
			return
		}
	}

	// 更新用户信息
	err := h.userRepo.UpdateProfile(claims.UserID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// GetPublicProfileGin Gin 版本的获取公开个人资料
func (h *UserHandler) GetPublicProfileGin(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户名参数"})
		return
	}

	user, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 脱敏处理，公开资料不显示邮箱
	type PublicUser struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Bio         string `json:"bio"`
		AvatarURL   string `json:"avatar_url"`
		Score       int    `json:"score"`
		SolvedCount int    `json:"solved_count"`
		Role        string `json:"role"`
		CreatedAt   string `json:"created_at"`
	}

	publicUser := PublicUser{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Bio:         user.Bio,
		AvatarURL:   user.AvatarURL,
		Score:       user.Score,
		SolvedCount: user.SolvedCount,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt.Format("2006-01-02"),
	}

	c.JSON(http.StatusOK, publicUser)
}