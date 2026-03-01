package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

type UserHandler struct {
	userRepo *repository.UserRepository
}

func NewUserHandler(userRepo *repository.UserRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

// GetLeaderboard 获取排行榜
func (h *UserHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // 默认 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	users, err := h.userRepo.GetLeaderboard(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取排行榜失败")
		return
	}

	// 敏感信息脱敏（虽然 User struct tag 控制了 json，但 Email 还是 public 的，前端可能需要 mask）
	// 这里简单处理，不返回 email 如果不需要
	// Note: models.User definition `json:"email"` means it sends email.
	// For leaderboard, we probably shouldn't expose email.
	// Create a SafeUser struct or just nil the email?
	// Using a temporary anonymous struct slice or modification is safer.

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safeUsers)
}

// UpdateProfile 更新当前用户信息
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 PUT 方法")
		return
	}

	// 从 context 中获取用户信息
	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权的请求")
		return
	}

	// 解析请求体
	var req models.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	// 简单验证
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "显示名称不能为空")
		return
	}

	// 更新用户信息前，如果是修改了邮箱，需要检查邮箱是否已存在
	user, _ := h.userRepo.GetUserByID(claims.UserID)
	if user != nil && user.Email != req.Email {
		exists, err := h.userRepo.UserExistsByEmail(req.Email)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "检查邮箱失败")
			return
		}
		if exists {
			writeError(w, http.StatusBadRequest, "该邮箱已被其他账号使用")
			return
		}
	}

	// 更新用户信息
	err := h.userRepo.UpdateProfile(claims.UserID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "更新成功",
	})
}

// GetPublicProfile 获取公开个人资料
func (h *UserHandler) GetPublicProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "缺少用户名参数")
		return
	}

	user, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(w, http.StatusNotFound, "用户不存在")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(publicUser)
}
