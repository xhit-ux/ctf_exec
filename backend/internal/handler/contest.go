package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ctf_exec/backend/internal/apperr"
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
	limit, err := parseLimitParam(c, "limit", 20, 200)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	contests, err := h.repo.ListContests(limit)
	if err != nil {
		writeInternal(c, "获取赛事列表失败")
		return
	}

	c.JSON(http.StatusOK, contests)
}

func (h *ContestHandler) GetContestDetailGin(c *gin.Context) {
	contestID := c.Param("id")
	if contestID == "" {
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	contest, err := h.repo.GetContestByID(contestID)
	if err != nil {
		if !errors.Is(err, apperr.ErrNotFound) {
			writeInternal(c, "获取赛事详情失败")
			return
		}
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	if strings.EqualFold(contest.Visibility, "private") {
		if !h.canAccessPrivateContest(c, contestID) {
			writeForbidden(c, "无权访问该私有赛事")
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
		writeBadRequest(c, "请求体格式错误")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeBadRequest(c, "title 不能为空")
		return
	}
	if req.StartTime != nil && req.EndTime != nil && req.EndTime.Before(*req.StartTime) {
		writeBadRequest(c, "结束时间不能早于开始时间")
		return
	}
	if !isValidContestVisibility(req.Visibility) {
		writeBadRequest(c, "visibility 仅支持 public/invite_only/private")
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
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	var req models.UpdateContestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeBadRequest(c, "title 不能为空")
		return
	}
	if req.StartTime != nil && req.EndTime != nil && req.EndTime.Before(*req.StartTime) {
		writeBadRequest(c, "结束时间不能早于开始时间")
		return
	}
	if !isValidContestVisibility(req.Visibility) {
		writeBadRequest(c, "visibility 仅支持 public/invite_only/private")
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
		writeBadRequest(c, "缺少赛事ID")
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
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	var req models.ContestChallengeBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if req.ChallengeID <= 0 {
		writeBadRequest(c, "无效的 challenge_id")
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
		writeBadRequest(c, "无效的参数")
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
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	var req models.JoinContestRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		writeBadRequest(c, "请求体格式错误")
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
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	var req models.LeaveContestRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		writeBadRequest(c, "请求体格式错误")
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
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	items, err := h.svc.GetContestChallengesForUser(contestID, claims.UserID, claims.Role)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *ContestHandler) GetContestScoreboardGin(c *gin.Context) {
	contestID := c.Param("id")
	if strings.TrimSpace(contestID) == "" {
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	boardType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("type", "user")))
	if boardType != "user" && boardType != "team" {
		writeBadRequest(c, "type 仅支持 user 或 team")
		return
	}

	limit := 50
	if strings.TrimSpace(c.Query("limit")) != "" {
		parsed, parseErr := parseLimitParam(c, "limit", 50, 200)
		if parseErr != nil {
			writeError(c, parseErr, http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	items, err := h.svc.GetContestScoreboard(contestID, boardType, limit)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, items)
}

// GetContestParticipantsGin 查询赛事参赛者列表（已报名用户或赛事管理员可查看）。
func (h *ContestHandler) GetContestParticipantsGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 20, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	allowed, err := h.canAccessContestMemberData(claims.UserID, claims.Role, contestID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if !allowed {
		writeForbidden(c, "未报名该赛事或无管理权限")
		return
	}

	offset := (page - 1) * pageSize
	items, total, err := h.repo.ListContestParticipants(contestID, offset, pageSize)
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

// GetContestMySubmissionsGin 查询当前用户在赛事中的提交记录。
func (h *ContestHandler) GetContestMySubmissionsGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 20, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	allowed, err := h.canAccessContestMemberData(claims.UserID, claims.Role, contestID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if !allowed {
		writeForbidden(c, "未报名该赛事或无管理权限")
		return
	}

	offset := (page - 1) * pageSize
	items, total, err := h.repo.ListContestMySubmissions(contestID, claims.UserID, offset, pageSize)
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

// ListAdminContestsGin 管理端分页查询赛事。
func (h *ContestHandler) ListAdminContestsGin(c *gin.Context) {
	page, pageSize, parseErr := parsePageParams(c, 20, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	visibility := strings.ToLower(strings.TrimSpace(c.Query("visibility")))
	if visibility != "" && visibility != "public" && visibility != "invite_only" && visibility != "private" {
		writeBadRequest(c, "visibility 仅支持 public/invite_only/private")
		return
	}

	offset := (page - 1) * pageSize
	items, total, err := h.repo.ListAdminContests(offset, pageSize, keyword, visibility)
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

// ListContestSubmissionsGin 查询赛事提交审计记录（contest_admin+）。
func (h *ContestHandler) ListContestSubmissionsGin(c *gin.Context) {
	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	page, pageSize, parseErr := parsePageParams(c, 20, 100)
	if parseErr != nil {
		writeError(c, parseErr, http.StatusBadRequest)
		return
	}

	problemID := 0
	problemIDRaw := strings.TrimSpace(c.Query("problem_id"))
	if problemIDRaw != "" {
		parsed, err := strconv.Atoi(problemIDRaw)
		if err != nil || parsed <= 0 {
			writeBadRequest(c, "problem_id 必须是大于 0 的整数")
			return
		}
		problemID = parsed
	}

	result := strings.ToLower(strings.TrimSpace(c.Query("result")))
	if result != "" && result != "correct" && result != "wrong" {
		writeBadRequest(c, "result 仅支持 correct/wrong")
		return
	}

	userID := strings.TrimSpace(c.Query("user_id"))
	offset := (page - 1) * pageSize
	items, total, err := h.repo.ListContestSubmissions(contestID, offset, pageSize, userID, problemID, result)
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

// ListContestAdminsGin 查询赛事管理员列表。
func (h *ContestHandler) ListContestAdminsGin(c *gin.Context) {
	contestID := strings.TrimSpace(c.Param("id"))
	if contestID == "" {
		writeBadRequest(c, "缺少赛事ID")
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
		writeBadRequest(c, "缺少赛事ID")
		return
	}

	var req models.AddContestAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeBadRequest(c, "username 不能为空")
		return
	}

	targetUser, err := h.userRepo.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}
	if models.NormalizeRole(targetUser.Role) == models.RoleRoot {
		writeBadRequest(c, "不能将 root 加入 contest_admin")
		return
	}

	count, err := h.contestAdminRepo.CountContestAdmins(contestID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}
	if count >= 10 {
		writeBadRequest(c, "赛事管理员人数已达上限(10)")
		return
	}

	if err := h.contestAdminRepo.AddContestAdmin(targetUser.ID, contestID, req.Role, claims.UserID); err != nil {
		writeError(c, err, http.StatusBadRequest)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事管理员添加成功"})
}

// RemoveContestAdminGin 移除赛事管理员。
func (h *ContestHandler) RemoveContestAdminGin(c *gin.Context) {
	contestID := strings.TrimSpace(c.Param("id"))
	username := strings.TrimSpace(c.Param("username"))
	if contestID == "" || username == "" {
		writeBadRequest(c, "参数不完整")
		return
	}

	if err := h.contestAdminRepo.RemoveContestAdminByUsername(username, contestID); err != nil {
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
		writeBadRequest(c, "缺少赛事ID")
		return false
	}

	contest, err := h.repo.GetContestByID(contestID)
	if err != nil {
		writeError(c, err, http.StatusBadRequest)
		return false
	}

	user, err := h.userRepo.GetUserByID(claims.UserID)
	if err != nil {
		writeUnauthorized(c, "用户状态异常，请重新登录")
		return false
	}

	allowed, err := authz.CanEditProblemInContest(user, contest, contestID, h.contestAdminRepo)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return false
	}
	if !allowed {
		writeForbidden(c, "赛事已结束或无权编辑赛事题目")
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

func (h *ContestHandler) canAccessContestMemberData(userID, role, contestID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, nil
	}

	participant, err := h.repo.IsUserContestParticipant(contestID, userID)
	if err != nil {
		return false, err
	}
	if participant {
		return true, nil
	}

	allowed, err := authz.CanViewContestAdmin(&models.User{ID: userID, Role: role}, contestID, h.contestAdminRepo)
	if err != nil {
		return false, err
	}

	return allowed, nil
}
