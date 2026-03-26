package handler

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type SubmissionHandler struct {
	subRepo           *repository.SubmissionRepository
	probRepo          *repository.ProblemRepository
	submissionService service.SubmissionService
}

func NewSubmissionHandler(
	subRepo *repository.SubmissionRepository,
	probRepo *repository.ProblemRepository,
	submissionService service.SubmissionService,
) *SubmissionHandler {
	return &SubmissionHandler{
		subRepo:           subRepo,
		probRepo:          probRepo,
		submissionService: submissionService,
	}
}

// SubmitFlagGin Gin 版本的提交 Flag
func (h *SubmissionHandler) SubmitFlagGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
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

	if req.ProblemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目 ID"})
		return
	}

	// 使用 Service 层提交（包含限流和排行榜更新）
	clientIP := realClientIP(c)
	submission, err := h.submissionService.SubmitFlag(claims.UserID, req.ProblemID, req.Flag, req.ContestID, clientIP)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, submission)
}

// GetMySubmissionsGin Gin 版本的获取当前用户的提交记录
func (h *SubmissionHandler) GetMySubmissionsGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	submissions, err := h.subRepo.GetSubmissionsByUserID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取提交记录失败"})
		return
	}

	c.JSON(http.StatusOK, submissions)
}
