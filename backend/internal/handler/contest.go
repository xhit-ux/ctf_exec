package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type ContestHandler struct {
	repo *repository.ContestRepository
	svc  service.ContestService
}

func NewContestHandler(repo *repository.ContestRepository, svc service.ContestService) *ContestHandler {
	return &ContestHandler{repo: repo, svc: svc}
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
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contest)
}

func (h *ContestHandler) CreateContestGin(c *gin.Context) {
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

	contest, err := h.repo.CreateContest(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	if err := h.repo.UpdateContest(contestID, &req); err != nil {
		c.JSON(statusCodeByError(err, http.StatusInternalServerError), gin.H{"error": err.Error()})
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
		c.JSON(statusCodeByError(err, http.StatusInternalServerError), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事删除成功"})
}

func (h *ContestHandler) AddChallengeToContestGin(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事题目编排成功"})
}

func (h *ContestHandler) RemoveChallengeFromContestGin(c *gin.Context) {
	contestID := c.Param("id")
	challengeIDStr := c.Param("challengeID")
	challengeID, err := strconv.Atoi(challengeIDStr)
	if contestID == "" || err != nil || challengeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if err := h.repo.RemoveChallengeFromContest(contestID, challengeID); err != nil {
		c.JSON(statusCodeByError(err, http.StatusInternalServerError), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "赛事题目移除成功"})
}

func (h *ContestHandler) JoinContestGin(c *gin.Context) {
	claimsInterface, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	claims, ok := claimsInterface.(*utils.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
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
		c.JSON(statusCodeByError(err, http.StatusBadRequest), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "报名成功"})
}

func (h *ContestHandler) LeaveContestGin(c *gin.Context) {
	claimsInterface, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	claims, ok := claimsInterface.(*utils.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
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
		c.JSON(statusCodeByError(err, http.StatusBadRequest), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "退赛成功"})
}

func (h *ContestHandler) GetContestChallengesGin(c *gin.Context) {
	claimsInterface, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	claims, ok := claimsInterface.(*utils.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的用户信息"})
		return
	}

	contestID := c.Param("id")
	if strings.TrimSpace(contestID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少赛事ID"})
		return
	}

	items, err := h.svc.GetContestChallengesForUser(contestID, claims.UserID)
	if err != nil {
		c.JSON(statusCodeByError(err, http.StatusBadRequest), gin.H{"error": err.Error()})
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
		c.JSON(statusCodeByError(err, http.StatusBadRequest), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}
