package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

type ProblemHandler struct {
	repo *repository.ProblemRepository
}

func NewProblemHandler(repo *repository.ProblemRepository) *ProblemHandler {
	return &ProblemHandler{repo: repo}
}

// GetProblems 获取题目列表
func (h *ProblemHandler) GetProblems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	var problems []*models.Problem
	var err error

	// 尝试从 context 获取 claims (AuthMiddleware 会填充)
	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if ok {
		problems, err = h.repo.GetProblemsWithStatus(claims.UserID)
	} else {
		problems, err = h.repo.GetProblems()
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取题目列表失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problems)
}

// GetProblemDetail 获取题目详情
func (h *ProblemHandler) GetProblemDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 简单的从 URL query 获取 id (例如 /api/problems?id=1)
	// 或者更好的方式是路由参数 /api/problems/1，但标准库 mux 处理比较麻烦，这里先用 query param 演示
	// 这里我们假设 main.go 里面用 http.ServeMux 没有路径参数解析，所以我们解析 Path。
	// 但是 standard mux 在 Go 1.22 增强了，如果是旧版本可能需要手动解析。
	// 假设我们使用 Query 参数 ?id=X 兼容性好。

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		// 尝试从 Path 解析, 需要 main.go 配合 HandleFunc("/api/problems/", ...)
		// 这里简化，只支持 Query 参数 ?id=
		writeError(w, http.StatusBadRequest, "缺少 id 参数")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效的 id")
		return
	}

	problem, err := h.repo.GetProblemByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(problem)
}
