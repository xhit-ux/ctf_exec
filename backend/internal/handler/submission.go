package handler

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type SubmissionHandler struct {
	subRepo  *repository.SubmissionRepository
	probRepo *repository.ProblemRepository
}

func NewSubmissionHandler(subRepo *repository.SubmissionRepository, probRepo *repository.ProblemRepository) *SubmissionHandler {
	return &SubmissionHandler{subRepo: subRepo, probRepo: probRepo}
}

// SubmitFlagGin Gin 版本的提交 Flag
func (h *SubmissionHandler) SubmitFlagGin(c *gin.Context) {
	// 从 Gin 上下文中获取 claims
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

	var req models.CreateSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}

	if req.Flag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Flag 不能为空"})
		return
	}

	// 服务器端验证 Flag
	correctFlag, err := h.probRepo.GetFlagByProblemID(req.ProblemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目 ID"})
		return
	}

	// 简单的字符串比较
	isCorrect := (req.Flag == correctFlag)
	req.IsCorrect = isCorrect

	submission, err := h.subRepo.CreateSubmission(claims.UserID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交失败"})
		return
	}

	c.JSON(http.StatusCreated, submission)
}

// GetMySubmissionsGin Gin 版本的获取当前用户的提交记录
func (h *SubmissionHandler) GetMySubmissionsGin(c *gin.Context) {
	// 从 Gin 上下文中获取 claims
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

	submissions, err := h.subRepo.GetSubmissionsByUserID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取提交记录失败"})
		return
	}

	c.JSON(http.StatusOK, submissions)
}