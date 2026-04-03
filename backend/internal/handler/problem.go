package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

type ProblemHandler struct {
	repo             *repository.ProblemRepository
	contestRepo      *repository.ContestRepository
	contestAdminRepo repository.ContestAdminRepository
}

// CreateProblemGin 管理端创建题目。
func (h *ProblemHandler) CreateProblemGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

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

	req.Type = normalizeProblemType(req.Type)
	req.FlagRule = normalizeFlagRule(req.FlagRule)
	req.Status = normalizeProblemStatus(req.Status)

	if !isSupportedFlagRule(req.FlagRule) {
		writeBadRequest(c, "flag_rule 不合法")
		return
	}

	if !isSupportedProblemStatus(req.Status) {
		writeBadRequest(c, "status 不合法")
		return
	}

	role := models.NormalizeRole(claims.Role)
	req.CreatedBy = claims.UserID
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	req.ContestID = strings.TrimSpace(req.ContestID)

	if req.Scope == "" {
		req.Scope = "platform"
	}

	if req.Scope != "platform" && req.Scope != "contest" {
		writeBadRequest(c, "scope 仅支持 platform/contest")
		return
	}

	if role != models.RoleRoot && role != models.RoleSiteAdmin {
		if req.ContestID == "" {
			writeForbidden(c, "contest_admin 创建题目时必须指定 contest_id")
			return
		}

		isAdmin, err := h.contestAdminRepo.IsContestAdmin(claims.UserID, req.ContestID)
		if err != nil {
			writeError(c, err, http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			writeForbidden(c, "无权在该赛事发布题目")
			return
		}

		req.Scope = "contest"
		if req.Status == "published" || req.Status == "archived" {
			writeForbidden(c, "contest_admin 仅可创建 draft/review 状态题目")
			return
		}
	} else if req.Scope == "contest" && req.ContestID == "" {
		writeBadRequest(c, "scope=contest 时 contest_id 不能为空")
		return
	}

	if req.Scope == "contest" {
		contest, err := h.contestRepo.GetContestByID(req.ContestID)
		if err != nil {
			writeError(c, err, http.StatusBadRequest)
			return
		}

		if contest.EndTime != nil && time.Now().After(*contest.EndTime) {
			writeBadRequest(c, "赛事已结束，不能发布新的赛事题目")
			return
		}
	}

	problem, err := h.repo.CreateProblem(&req)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	if req.Scope == "contest" {
		if err := h.contestRepo.AddChallengeToContest(req.ContestID, problem.ID, nil); err != nil {
			writeError(c, err, http.StatusInternalServerError)
			return
		}
	}

	c.JSON(http.StatusCreated, problem)
}

// UpdateProblemGin 管理端更新题目。
func (h *ProblemHandler) UpdateProblemGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

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

	req.Type = normalizeProblemType(req.Type)
	req.FlagRule = normalizeFlagRule(req.FlagRule)
	req.Status = normalizeProblemStatus(req.Status)

	if !isSupportedFlagRule(req.FlagRule) {
		writeBadRequest(c, "flag_rule 不合法")
		return
	}

	if !isSupportedProblemStatus(req.Status) {
		writeBadRequest(c, "status 不合法")
		return
	}

	role := models.NormalizeRole(claims.Role)
	if role != models.RoleRoot && role != models.RoleSiteAdmin {
		scope, contestID, _, metaErr := h.repo.GetProblemAccessMeta(id)
		if metaErr != nil {
			writeError(c, metaErr, http.StatusNotFound)
			return
		}

		if scope != "contest" || contestID == "" {
			writeForbidden(c, "仅可编辑所属赛事题目")
			return
		}

		isAdmin, adminErr := h.contestAdminRepo.IsContestAdmin(claims.UserID, contestID)
		if adminErr != nil {
			writeError(c, adminErr, http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			writeForbidden(c, "无权编辑该题目")
			return
		}

		if req.Status == "published" || req.Status == "archived" {
			writeForbidden(c, "contest_admin 仅可更新为 draft/review 状态")
			return
		}
	}

	if err := h.repo.UpdateProblem(id, &req); err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "题目更新成功"})
}

// ListAdminProblemsGin 管理端题目列表。
func (h *ProblemHandler) ListAdminProblemsGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	items, err := h.repo.ListAdminProblems(
		strings.TrimSpace(c.Query("keyword")),
		strings.TrimSpace(c.Query("scope")),
		strings.TrimSpace(c.Query("status")),
	)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError)
		return
	}

	role := models.NormalizeRole(claims.Role)
	if role == models.RoleContestAdmin {
		filtered := make([]*models.Problem, 0, len(items))
		for _, item := range items {
			if item.Scope != "contest" || item.ContestID == nil || strings.TrimSpace(*item.ContestID) == "" {
				continue
			}
			isAdmin, adminErr := h.contestAdminRepo.IsContestAdmin(claims.UserID, *item.ContestID)
			if adminErr != nil {
				writeError(c, adminErr, http.StatusInternalServerError)
				return
			}
			if isAdmin {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetAdminProblemDetailGin 管理端题目详情。
func (h *ProblemHandler) GetAdminProblemDetailGin(c *gin.Context) {
	claims, ok := middleware.MustGetClaims(c)
	if !ok {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeBadRequest(c, "无效的题目ID")
		return
	}

	problem, err := h.repo.GetAdminProblemByID(id)
	if err != nil {
		writeError(c, err, http.StatusNotFound)
		return
	}

	if models.NormalizeRole(claims.Role) == models.RoleContestAdmin {
		if problem.Scope != "contest" || problem.ContestID == nil || strings.TrimSpace(*problem.ContestID) == "" {
			writeForbidden(c, "无权查看该题目")
			return
		}
		isAdmin, adminErr := h.contestAdminRepo.IsContestAdmin(claims.UserID, *problem.ContestID)
		if adminErr != nil {
			writeError(c, adminErr, http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			writeForbidden(c, "无权查看该题目")
			return
		}
	}

	c.JSON(http.StatusOK, problem)
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

func NewProblemHandler(repo *repository.ProblemRepository, contestRepo *repository.ContestRepository, contestAdminRepo repository.ContestAdminRepository) *ProblemHandler {
	return &ProblemHandler{repo: repo, contestRepo: contestRepo, contestAdminRepo: contestAdminRepo}
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

func normalizeProblemType(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v != "dynamic" {
		return "static"
	}
	return v
}

func normalizeFlagRule(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "exact"
	}
	return v
}

func isSupportedFlagRule(rule string) bool {
	v := strings.ToLower(strings.TrimSpace(rule))
	if v == "exact" || v == "case_insensitive" {
		return true
	}
	return strings.HasPrefix(v, "contains:") || strings.HasPrefix(v, "regex:")
}

func normalizeProblemStatus(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "published"
	}
	return v
}

func isSupportedProblemStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft", "review", "published", "archived":
		return true
	default:
		return false
	}
}
