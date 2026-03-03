package handler

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
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

	if req.ProblemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的题目 ID"})
		return
	}

	// 使用 Service 层提交（包含限流和排行榜更新）
	submission, err := h.submissionService.SubmitFlag(claims.UserID, req.ProblemID, req.Flag)
	if err != nil {
		// 根据错误类型返回适当的 HTTP 状态码
		statusCode := http.StatusInternalServerError
		errorMsg := err.Error()
		
		// 特定错误处理
		if errorMsg == "你已经解决过这个题目了" {
			statusCode = http.StatusConflict
		} else if errorMsg == "提交过于频繁，请稍后再试" || errorMsg == "题目不存在" {
			statusCode = http.StatusBadRequest
		} else if errorMsg == "获取题目 Flag 失败" {
			statusCode = http.StatusNotFound
		}
		
		c.JSON(statusCode, gin.H{"error": errorMsg})
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