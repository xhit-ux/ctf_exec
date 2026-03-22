package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ctf_exec/backend/internal/authz"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ContestHandler struct {
	repo             *repository.ContestRepository
	userRepo         *repository.UserRepository
	contestAdminRepo repository.ContestAdminRepository
	svc              service.ContestService
}

func NewContestHandler(repo *repository.ContestRepository, userRepo *repository.UserRepository, contestAdminRepo repository.ContestAdminRepository, svc service.ContestService) *ContestHandler {
	return &ContestHandler{repo: repo, userRepo: userRepo, contestAdminRepo: contestAdminRepo, svc: svc}
}

func (h *ContestHandler) ListContestsGin(c *gin.Context) {
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	contests, err := h.repo.ListContests(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取赛事列表失败"})
		return
	}

	c.JSON(http.StatusOK, contests)
}

func (h *ContestHandler) GetContestDetailGin(c *gin.Context) {
	contestID := c.Param("id")
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	contest, err := h.repo.GetContestByID(contestID)
	if err != nil {
		statusCode := statusCodeByError(err, http.StatusInternalServerError)
		if statusCode == http.StatusInternalServerError {
			c.JSON(statusCode, gin.H{"error": "获取赛事详情失败"})
			return
		}
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	if strings.EqualFold(contest.Visibility, "private") {
		if !h.canAccessPrivateContest(c, contestID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该私有赛事"})
			return
		}
	}

	c.JSON(http.StatusOK, contest)
}

func (h *ContestHandler) CreateContestGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	var req models.CreateContestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title 不能为空"})
		return
	}
	if req.StartTime != nil && req.EndTime != nil && req.EndTime.Before(*req.StartTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结束时间不能早于开始时间"})
		return
	}
	if !isValidContestVisibility(req.Visibility) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "visibility 仅支持 public/invite_only/private"})
		return
	}

	contest, err := h.repo.CreateContest(&req, claims.UserID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, contest)
}

func (h *ContestHandler) UpdateContestGin(c *gin.Context) {
	contestID := c.Param("id")
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	var req models.UpdateContestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title 不能为空"})
		return
	}
	if req.StartTime != nil && req.EndTime != nil && req.EndTime.Before(*req.StartTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结束时间不能早于开始时间"})
		return
	}
	if !isValidContestVisibility(req.Visibility) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "visibility 仅支持 public/invite_only/private"})
		return
	}

	if err := h.repo.UpdateContest(contestID, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事更新成功"})
}

func (h *ContestHandler) DeleteContestGin(c *gin.Context) {
	contestID := c.Param("id")
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	if err := h.repo.DeleteContest(contestID); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事删除成功"})
}

func (h *ContestHandler) AddChallengeToContestGin(c *gin.Context) {
	if ok := h.ensureCanEditContestProblems(c); !ok {
		return
	}

	contestID := c.Param("id")
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	var req models.ContestChallengeBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	if req.ChallengeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 challenge_id"})
		return
	}

	if err := h.repo.AddChallengeToContest(contestID, req.ChallengeID, req.ScoreOverride); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事题目编排成功"})
}

func (h *ContestHandler) RemoveChallengeFromContestGin(c *gin.Context) {
	if ok := h.ensureCanEditContestProblems(c); !ok {
		return
	}

	contestID := c.Param("id")
	challengeIDStr := c.Param("challengeID")
	challengeID, err := strconv.Atoi(challengeIDStr)
	if contestID == "" || err != nil || challengeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if err := h.repo.RemoveChallengeFromContest(contestID, challengeID); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事题目移除成功"})
}

func (h *ContestHandler) JoinContestGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	contestID := c.Param("id")
	if strings.TrimSpace(contestID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	var req models.JoinContestRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	if err := h.svc.JoinContest(contestID, claims.UserID, req.TeamID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "报名成功"})
}

func (h *ContestHandler) LeaveContestGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	contestID := c.Param("id")
	if strings.TrimSpace(contestID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	var req models.LeaveContestRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	if err := h.svc.LeaveContest(contestID, claims.UserID, req.TeamID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "退赛成功"})
}

func (h *ContestHandler) GetContestChallengesGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	contestID := c.Param("id")
	if strings.TrimSpace(contestID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	items, err := h.svc.GetContestChallengesForUser(contestID, claims.UserID)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *ContestHandler) GetContestScoreboardGin(c *gin.Context) {
	contestID := c.Param("id")
	if strings.TrimSpace(contestID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	boardType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("type", "user")))
	if boardType != "user" && boardType != "team" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 仅支持 user 或 team"})
		return
	}

	limit := 50
	if limitStr := strings.TrimSpace(c.Query("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	items, err := h.svc.GetContestScoreboard(contestID, boardType, limit)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, items)
}

// ListContestAdminsGin 查询赛事管理员列表。
func (h *ContestHandler) ListContestAdminsGin(c *gin.Context) {
	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	items, err := h.contestAdminRepo.ListContestAdmins(contestID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, items)
}

// AddContestAdminGin 为赛事添加管理员。
func (h *ContestHandler) AddContestAdminGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	var req models.AddContestAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id 不能为空"})
		return
	}

	targetUser, err := h.userRepo.GetUserByID(strings.TrimSpace(req.UserID))
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	if models.NormalizeRole(targetUser.Role) == models.RoleRoot {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能将 root 加入 contest_admin"})
		return
	}

	count, err := h.contestAdminRepo.CountContestAdmins(contestID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if count >= 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "赛事管理员人数已达上限(10)"})
		return
	}

	if err := h.contestAdminRepo.AddContestAdmin(req.UserID, contestID, req.Role, claims.UserID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事管理员添加成功"})
}

// RemoveContestAdminGin 移除赛事管理员。
func (h *ContestHandler) RemoveContestAdminGin(c *gin.Context) {
	contestID := strings.TrimSpace(c.Param("id"))
	userID := strings.TrimSpace(c.Param("user_id"))
	if contestID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	if err := h.contestAdminRepo.RemoveContestAdmin(userID, contestID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事管理员移除成功"})
}

func isValidContestVisibility(visibility string) bool {
	normalized := strings.ToLower(strings.TrimSpace(visibility))
	if normalized == "" {
		return true
	}

	return normalized == "public" || normalized == "invite_only" || normalized == "private"
}

func (h *ContestHandler) ensureCanEditContestProblems(c *gin.Context) bool {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return false
	}

	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return false
	}

	contest, err := h.repo.GetContestByID(contestID)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return false
	}

	user, err := h.userRepo.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户状态异常，请重新登录"})
		return false
	}

	allowed, err := authz.CanEditProblemInContest(user, contest, contestID, h.contestAdminRepo)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "赛事已结束或无权编辑赛事题目"})
		return false
	}

	return true
}

func (h *ContestHandler) canAccessPrivateContest(c *gin.Context, contestID string) bool {
	claims, err := middleware.GetClaims(c)
	if err != nil {
		return false
	}

	user, err := h.userRepo.GetUserByID(claims.UserID)
	if err != nil {
		return false
	}

	ok, err := authz.CanManageContest(user, contestID, h.contestAdminRepo)
	if err != nil {
		return false
	}

	return ok
}
