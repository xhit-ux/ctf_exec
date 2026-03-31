package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

type ProblemHandler struct {
	repo *repository.ProblemRepository
}

// CreateProblemGin 管理端创建题目。
func (h *ProblemHandler) CreateProblemGin(c *gin.Context) {
	var req models.CreateProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Difficulty) == "" {
		writeBadRequest(c, "title/category/difficulty 不能为空")
		return
	}
	if req.Points < 0 {
		writeBadRequest(c, "points 不能为负数")
		return
	}
	if strings.TrimSpace(req.Flag) == "" {
		writeBadRequest(c, "flag 不能为空")
		return
	}

	problem, err := h.repo.CreateProblem(&req)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, problem)
}

// UpdateProblemGin 管理端更新题目。
func (h *ProblemHandler) UpdateProblemGin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	var req models.UpdateProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c, "请求体格式错误")
		return
	}

	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Difficulty) == "" {
		writeBadRequest(c, "title/category/difficulty 不能为空")
		return
	}
	if req.Points < 0 {
		writeBadRequest(c, "points 不能为负数")
		return
	}

	if err := h.repo.UpdateProblem(id, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "题目更新成功"})
}

// DeleteProblemGin 管理端删除题目。
func (h *ProblemHandler) DeleteProblemGin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	if err := h.repo.DeleteProblem(id); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "题目删除成功"})
}

func NewProblemHandler(repo *repository.ProblemRepository) *ProblemHandler {
	return &ProblemHandler{repo: repo}
}

// GetProblemsGin Gin 版本的获取题目列表
func (h *ProblemHandler) GetProblemsGin(c *gin.Context) {
	var problems []*models.Problem
	var err error

	// 从 Gin 上下文中获取 claims
	if claims, err := middleware.GetClaims(c); err == nil {
		problems, err = h.repo.GetProblemsWithStatus(claims.UserID)
		if err != nil {
			writeInternal(c, "获取题目列表失败")
			return
		}
		c.JSON(http.StatusOK, problems)
		return
	}

	// 如果没有认证信息，获取普通题目列表
	problems, err = h.repo.GetProblems()
	if err != nil {
		writeInternal(c, "获取题目列表失败")
		return
	}

	c.JSON(http.StatusOK, problems)
}

// GetProblemDetailGin Gin 版本的获取题目详情
func (h *ProblemHandler) GetProblemDetailGin(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		writeBadRequest(c, "缺少 id 参数")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeBadRequest(c, "无效的 id")
		return
	}

	problem, err := h.repo.GetProblemByID(id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, problem)
}
