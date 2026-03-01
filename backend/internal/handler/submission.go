package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

type SubmissionHandler struct {
	subRepo  *repository.SubmissionRepository
	probRepo *repository.ProblemRepository
}

func NewSubmissionHandler(subRepo *repository.SubmissionRepository, probRepo *repository.ProblemRepository) *SubmissionHandler {
	return &SubmissionHandler{subRepo: subRepo, probRepo: probRepo}
}

// SubmitFlag 处理提交 Flag
func (h *SubmissionHandler) SubmitFlag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求体失败")
		return
	}
	defer r.Body.Close()

	var req models.CreateSubmissionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	if req.Flag == "" {
		writeError(w, http.StatusBadRequest, "Flag 不能为空")
		return
	}

	// 服务器端验证 Flag
	correctFlag, err := h.probRepo.GetFlagByProblemID(req.ProblemID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效的题目 ID")
		return
	}

	// 简单的字符串比较，实际上可能需要防时序攻击，但对于 CTF flag 比较通常不需要那么严格，除非是 check server
	isCorrect := (req.Flag == correctFlag)

	// 更新请求体中的校验结果，传给 repo (或者修改 repo 接口)
	// 这里简单起见，我们信任 handler 传递给 repo 的结果
	req.IsCorrect = isCorrect

	submission, err := h.subRepo.CreateSubmission(claims.UserID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "提交失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(submission)
}

// GetMySubmissions 获取当前用户的提交记录
func (h *SubmissionHandler) GetMySubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	submissions, err := h.subRepo.GetSubmissionsByUserID(claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取提交记录失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(submissions)
}
