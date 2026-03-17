package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userRepo           *repository.UserRepository
	leaderboardService service.LeaderboardService
}

// PatchUserRoleGin root 修改用户角色。
func (h *UserHandler) PatchUserRoleGin(c *gin.Context) {
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

	targetUserID := c.Param("id")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户ID"})
		return
	}

	var req models.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	req.Role = models.NormalizeRole(req.Role)
	if !models.IsValidRole(req.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法角色值"})
		return
	}

	if claims.UserID == targetUserID && req.Role != models.RoleRoot {
		c.JSON(http.StatusBadRequest, gin.H{"error": "禁止 root 自降级"})
		return
	}

	if err := h.userRepo.UpdateUserRole(claims.UserID, targetUserID, req.Role, req.Reason, c.ClientIP()); err != nil {
		if err.Error() == "目标用户不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "角色未发生变化" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "角色更新成功"})
}

func NewUserHandler(userRepo *repository.UserRepository, leaderboardService service.LeaderboardService) *UserHandler {
	return &UserHandler{
		userRepo:           userRepo,
		leaderboardService: leaderboardService,
	}
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

	// 解析用户偏好设置
	type UserPreferences struct {
		EmailPublic bool `json:"email_public"`
	}
	var preferences UserPreferences
	if user.Preferences != "" {
		// 忽略解析错误，默认为 false
		json.Unmarshal([]byte(user.Preferences), &preferences)
	}

	// 公开个人资料结构
	type PublicUser struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Bio         string `json:"bio"`
		AvatarURL   string `json:"avatar_url"`
		Score       int    `json:"score"`
		SolvedCount int    `json:"solved_count"`
		Role        string `json:"role"`
		CreatedAt   string `json:"created_at"`
		Email       string `json:"email,omitempty"`       // 根据隐私设置决定是否包含
		IsVerified  bool   `json:"is_verified,omitempty"` // 邮箱验证状态，当邮箱公开时包含
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

	// 检查用户是否已登录
	claimsInterface, exists := c.Get("claims")
	if exists {
		claims, ok := claimsInterface.(*utils.Claims)
		if ok {
			// 用户已登录
			if claims.UserID == user.ID {
				// 查看自己的个人资料：始终显示邮箱（如果存在）
				if user.Email != "" {
					publicUser.Email = user.Email
					publicUser.IsVerified = user.IsVerified
				}
			} else {
				// 查看其他用户的个人资料：根据隐私设置决定
				if preferences.EmailPublic && user.Email != "" {
					publicUser.Email = user.Email
					publicUser.IsVerified = user.IsVerified
				}
			}
		} else {
			// claims 类型错误，视为未登录
			// 不包含邮箱
		}
	}
	// 如果未登录，不包含邮箱

	c.JSON(http.StatusOK, publicUser)
}
