package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/authz"
	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type userPreferences struct {
	EmailPublic bool                `json:"email_public"`
	GithubURL   string              `json:"github_url"`
	SocialLinks []models.SocialLink `json:"social_links"`
}

func parseUserPreferences(raw string) userPreferences {
	pref := userPreferences{}
	if strings.TrimSpace(raw) == "" {
		return pref
	}
	_ = json.Unmarshal([]byte(raw), &pref)
	return pref
}

func parsePageParams(c *gin.Context, defaultSize, maxSize int) (int, int, error) {
	pageStr := strings.TrimSpace(c.DefaultQuery("page", "1"))
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		return 0, 0, apperr.New(apperr.ErrBadRequest, "page 必须是大于 0 的整数")
	}

	pageSizeStr := strings.TrimSpace(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		return 0, 0, apperr.New(apperr.ErrBadRequest, "page_size 必须是大于 0 的整数")
	}
	if pageSize > maxSize {
		return 0, 0, apperr.New(apperr.ErrBadRequest, "page_size 超出允许范围")
	}

	return page, pageSize, nil
}

func parseLimitParam(c *gin.Context, key string, defaultLimit, maxLimit int) (int, error) {
	value := strings.TrimSpace(c.DefaultQuery(key, strconv.Itoa(defaultLimit)))
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, apperr.New(apperr.ErrBadRequest, key+" 必须是大于 0 的整数")
	}
	if limit > maxLimit {
		return 0, apperr.New(apperr.ErrBadRequest, key+" 超出允许范围")
	}
	return limit, nil
}

func viewerUserID(c *gin.Context) string {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return ""
	}
	return claims.UserID
}

type UserHandler struct {
	userRepo    *repository.UserRepository
	authService service.AuthService
	redisClient *redis.Client
}

// ListAdminUsersGin root 查看用户列表。
func (h *UserHandler) ListAdminUsersGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 10, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	offset := (page - 1) * pageSize
	role := models.NormalizeRole(claims.Role)
	excludeRoot := role != models.RoleRoot
	excludeUserID := claims.UserID

	items, total, err := h.userRepo.ListAdminUsers(offset, pageSize, keyword, excludeRoot, excludeUserID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// PatchUserActiveGin 管理端封禁/解封用户。
func (h *UserHandler) PatchUserActiveGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	targetUserID := c.Param("id")
	if targetUserID == "" {
		writeBadRequest(c, "缺少用户ID")
		return
	}

	var req models.UpdateUserActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	if err := h.userRepo.UpdateUserActiveStatus(claims.UserID, targetUserID, req.IsActive, req.Reason, realClientIP(c)); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	if err := h.authService.RevokeUserSessions(targetUserID); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	statusText := "已封禁"
	if req.IsActive {
		statusText = "已解封"
	}

	c.JSON(http.StatusOK, gin.H{"message": statusText})
}

// ListRoleChangeLogsGin root 查看角色变更审计日志。
func (h *UserHandler) ListRoleChangeLogsGin(c *gin.Context) {
	page, pageSize, parseErr := parsePageParams(c, 10, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	offset := (page - 1) * pageSize
	items, total, err := h.userRepo.ListRoleChangeLogs(offset, pageSize)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// PatchUserRoleGin root 修改用户角色。
func (h *UserHandler) PatchUserRoleGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	targetUserID := c.Param("id")
	if targetUserID == "" {
		writeBadRequest(c, "缺少用户ID")
		return
	}

	var req models.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	req.Role = models.NormalizeRole(req.Role)
	if err := authz.CanPatchUserRole(claims.UserID, targetUserID, claims.Role, req.Role); err != nil {
		writeError(c, err, http.StatusForbidden)
		return
	}

	if err := h.userRepo.UpdateUserRole(claims.UserID, targetUserID, req.Role, req.Reason, realClientIP(c)); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	if err := h.authService.RevokeUserSessions(targetUserID); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "角色更新成功"})
}

func NewUserHandler(userRepo *repository.UserRepository, authService service.AuthService, redisClient *redis.Client) *UserHandler {
	return &UserHandler{
		userRepo:    userRepo,
		authService: authService,
		redisClient: redisClient,
	}
}

func (h *UserHandler) checkRateLimit(key string, limit int64, windowSeconds int) error {
	if h.redisClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pipe := h.redisClient.TxPipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(windowSeconds)*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	if incrCmd.Val() > limit {
		return apperr.New(apperr.ErrTooManyRequests, "操作过于频繁，请稍后再试")
	}

	return nil
}

func (h *UserHandler) setFollowCountsCache(userID string, followersCount int, followingCount int) {
	if h.redisClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pipe := h.redisClient.TxPipeline()
	pipe.Set(ctx, cache.GetUserFollowersCountKey(userID), followersCount, 5*time.Minute)
	pipe.Set(ctx, cache.GetUserFollowingCountKey(userID), followingCount, 5*time.Minute)
	_, _ = pipe.Exec(ctx)
}

func (h *UserHandler) getFollowCounts(userID string) (int, int, error) {
	if h.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		followerRes, followerErr := h.redisClient.Get(ctx, cache.GetUserFollowersCountKey(userID)).Int()
		followingRes, followingErr := h.redisClient.Get(ctx, cache.GetUserFollowingCountKey(userID)).Int()
		if followerErr == nil && followingErr == nil {
			return followerRes, followingRes, nil
		}

		if followerErr != nil && !errors.Is(followerErr, redis.Nil) {
			return 0, 0, followerErr
		}
		if followingErr != nil && !errors.Is(followingErr, redis.Nil) {
			return 0, 0, followingErr
		}
	}

	followersCount, followingCount, err := h.userRepo.GetFollowCounts(userID)
	if err != nil {
		return 0, 0, err
	}
	h.setFollowCountsCache(userID, followersCount, followingCount)

	return followersCount, followingCount, nil
}

// GetLeaderboardGin Gin 版本的获取排行榜
func (h *UserHandler) GetLeaderboardGin(c *gin.Context) {
	page, pageSize, parseErr := parsePageParams(c, 5, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	offset := (page - 1) * pageSize
	users, total, err := h.userRepo.GetLeaderboardPage(offset, pageSize)
	if err != nil {
		writeInternal(c, "获取排行榜失败")
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
			Rank:        offset + i + 1,
		}
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       safeUsers,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetLeaderboardUserByUsernameGin 按用户名检索排行榜用户
func (h *UserHandler) GetLeaderboardUserByUsernameGin(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	u, rank, err := h.userRepo.GetLeaderboardUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
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

	c.JSON(http.StatusOK, SafeUser{
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Score:       u.Score,
		SolvedCount: u.SolvedCount,
		AvatarURL:   u.AvatarURL,
		Rank:        rank,
	})
}

// GetProfileGin 获取用户完整 profile（支持可选认证）。
func (h *UserHandler) GetProfileGin(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	user, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	viewerID := viewerUserID(c)
	isOwnProfile := viewerID != "" && viewerID == user.ID
	lastActiveAt, lastActiveErr := h.userRepo.GetLastActiveAt(user.ID)
	if lastActiveErr != nil {
		writeError(c, lastActiveErr, http.StatusInternalServerError)
		return
	}

	isOnline := false
	if lastActiveAt != nil {
		isOnline = time.Since(*lastActiveAt) < 15*time.Minute
	}

	followersCount, followingCount, err := h.getFollowCounts(user.ID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	contestsParticipated, err := h.userRepo.GetContestsParticipatedCount(user.ID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	rankNum := user.RankNum
	if _, rank, rankErr := h.userRepo.GetLeaderboardUserByUsername(user.Username); rankErr == nil && rank > 0 {
		rankVal := rank
		rankNum = &rankVal
	}
	user.RankNum = rankNum

	prefs := parseUserPreferences(user.Preferences)
	if !isOwnProfile {
		if viewerID == "" || !prefs.EmailPublic {
			user.Email = ""
		}
	}

	var isFollowing *bool
	var isFollowedBy *bool
	if viewerID != "" && !isOwnProfile {
		fwd, followErr := h.userRepo.IsFollowing(viewerID, user.ID)
		if followErr != nil {
			writeError(c, followErr, http.StatusInternalServerError)
			return
		}
		rev, reverseErr := h.userRepo.IsFollowing(user.ID, viewerID)
		if reverseErr != nil {
			writeError(c, reverseErr, http.StatusInternalServerError)
			return
		}
		isFollowing = &fwd
		isFollowedBy = &rev
	}

	resp := models.UserProfileResponse{
		User: *user,
		Stats: models.ProfileStats{
			FollowersCount:       followersCount,
			FollowingCount:       followingCount,
			ContestsParticipated: contestsParticipated,
			RankNum:              rankNum,
		},
		IsOwnProfile: isOwnProfile,
		IsFollowing:  isFollowing,
		IsFollowedBy: isFollowedBy,
		IsOnline:     isOnline,
		LastActiveAt: lastActiveAt,
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateProfileGin Gin 版本的更新当前用户信息
func (h *UserHandler) UpdateProfileGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	if err := h.checkRateLimit(cache.GetProfileEditRateLimitKey(claims.UserID), 5, 60); err != nil {
		writeError(c, err, http.StatusTooManyRequests)
		return
	}

	// 解析请求体
	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	// 简单验证
	if req.DisplayName == "" {
		writeBadRequest(c, "显示名称不能为空")
		return
	}
	if len(req.DisplayName) > 64 {
		writeBadRequest(c, "显示名称不能超过 64 字符")
		return
	}

	// 更新用户信息前，如果是修改了邮箱，需要检查邮箱是否已存在
	user, _ := h.userRepo.GetUserByID(claims.UserID)
	if user != nil && user.Email != req.Email {
		exists, err := h.userRepo.UserExistsByEmail(req.Email)
		if err != nil {
			writeInternal(c, "检查邮箱失败")
			return
		}
		if exists {
			writeBadRequest(c, "该邮箱已被其他账号使用")
			return
		}
	}

	// 更新用户信息
	err := h.userRepo.UpdateProfile(claims.UserID, &req)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// FollowUserGin 关注用户。
func (h *UserHandler) FollowUserGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	if err := h.checkRateLimit(cache.GetFollowRateLimitKey(claims.UserID), 30, 60); err != nil {
		writeError(c, err, http.StatusTooManyRequests)
		return
	}

	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	targetUser, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	if targetUser.ID == claims.UserID {
		writeBadRequest(c, "不能关注自己")
		return
	}

	if err = h.userRepo.FollowUser(claims.UserID, targetUser.ID); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	followersCount, targetFollowingCount, countErr := h.userRepo.GetFollowCounts(targetUser.ID)
	if countErr != nil {
		writeError(c, countErr, http.StatusInternalServerError)
		return
	}
	h.setFollowCountsCache(targetUser.ID, followersCount, targetFollowingCount)

	actorFollowersCount, actorFollowingCount, actorCountErr := h.userRepo.GetFollowCounts(claims.UserID)
	if actorCountErr == nil {
		h.setFollowCountsCache(claims.UserID, actorFollowersCount, actorFollowingCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "关注成功",
		"followers_count": followersCount,
		"is_following":    true,
	})
}

// UnfollowUserGin 取消关注用户。
func (h *UserHandler) UnfollowUserGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	if err := h.checkRateLimit(cache.GetFollowRateLimitKey(claims.UserID), 30, 60); err != nil {
		writeError(c, err, http.StatusTooManyRequests)
		return
	}

	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	targetUser, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	if err = h.userRepo.UnfollowUser(claims.UserID, targetUser.ID); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	followersCount, targetFollowingCount, countErr := h.userRepo.GetFollowCounts(targetUser.ID)
	if countErr != nil {
		writeError(c, countErr, http.StatusInternalServerError)
		return
	}
	h.setFollowCountsCache(targetUser.ID, followersCount, targetFollowingCount)

	actorFollowersCount, actorFollowingCount, actorCountErr := h.userRepo.GetFollowCounts(claims.UserID)
	if actorCountErr == nil {
		h.setFollowCountsCache(claims.UserID, actorFollowersCount, actorFollowingCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "取消关注成功",
		"followers_count": followersCount,
		"is_following":    false,
	})
}

// GetFollowersGin 获取用户粉丝列表。
func (h *UserHandler) GetFollowersGin(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	targetUser, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 20, 50)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}
	offset := (page - 1) * pageSize
	keyword := strings.TrimSpace(c.Query("keyword"))
	viewerID := viewerUserID(c)

	items, total, listErr := h.userRepo.GetFollowers(targetUser.ID, viewerID, offset, pageSize, keyword)
	if listErr != nil {
		writeError(c, listErr, http.StatusInternalServerError)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetFollowingGin 获取用户关注列表。
func (h *UserHandler) GetFollowingGin(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	targetUser, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 20, 50)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}
	offset := (page - 1) * pageSize
	keyword := strings.TrimSpace(c.Query("keyword"))
	viewerID := viewerUserID(c)

	items, total, listErr := h.userRepo.GetFollowing(targetUser.ID, viewerID, offset, pageSize, keyword)
	if listErr != nil {
		writeError(c, listErr, http.StatusInternalServerError)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetMutualFollowingGin 获取共同关注列表（登录后可用）。
func (h *UserHandler) GetMutualFollowingGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	targetUser, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 20, 50)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}
	offset := (page - 1) * pageSize
	keyword := strings.TrimSpace(c.Query("keyword"))

	items, total, listErr := h.userRepo.GetMutualFollowing(targetUser.ID, claims.UserID, offset, pageSize, keyword)
	if listErr != nil {
		writeError(c, listErr, http.StatusInternalServerError)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetPublicProfileGin Gin 版本的获取公开个人资料
func (h *UserHandler) GetPublicProfileGin(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	if username == "" {
		writeBadRequest(c, "缺少用户名参数")
		return
	}

	user, err := h.userRepo.GetUserByUsername(username)
	if err != nil {
		writeNotFound(c, "用户不存在")
		return
	}

	prefs := parseUserPreferences(user.Preferences)

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
	if claims, err := middleware.GetClaims(c); err == nil {
		// 用户已登录
		if claims.UserID == user.ID {
			// 查看自己的个人资料：始终显示邮箱（如果存在）
			if user.Email != "" {
				publicUser.Email = user.Email
				publicUser.IsVerified = user.IsVerified
			}
		} else {
			// 查看其他用户的个人资料：根据隐私设置决定
			if prefs.EmailPublic && user.Email != "" {
				publicUser.Email = user.Email
				publicUser.IsVerified = user.IsVerified
			}
		}
	}
	// 如果未登录，不包含邮箱

	c.JSON(http.StatusOK, publicUser)
}
