package handler

import (
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type ProblemHandler struct {
	repo *repository.ProblemRepository
}

func NewProblemHandler(repo *repository.ProblemRepository) *ProblemHandler {
	return &ProblemHandler{repo: repo}
}

// GetProblemsGin Gin 版本的获取题目列表
func (h *ProblemHandler) GetProblemsGin(c *gin.Context) {
	var problems []*models.Problem
	var err error

	// 从 Gin 上下文中获取 claims
	claimsInterface, exists := c.Get("claims")
	if exists {
		if claims, ok := claimsInterface.(*utils.Claims); ok {
			problems, err = h.repo.GetProblemsWithStatus(claims.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目列表失败"})
				return
			}
			c.JSON(http.StatusOK, problems)
			return
		}
	}

	// 如果没有认证信息，获取普通题目列表
	problems, err = h.repo.GetProblems()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取题目列表失败"})
		return
	}

	c.JSON(http.StatusOK, problems)
}

// GetProblemDetailGin Gin 版本的获取题目详情
func (h *ProblemHandler) GetProblemDetailGin(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 id 参数"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}

	problem, err := h.repo.GetProblemByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, problem)
}