package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

type TeamHandler struct {
	teamRepo *repository.TeamRepository
}

func NewTeamHandler(teamRepo *repository.TeamRepository) *TeamHandler {
	return &TeamHandler{teamRepo: teamRepo}
}

// CreateTeam 创建队伍
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	var req models.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "队伍名称不能为空")
		return
	}

	team, err := h.teamRepo.CreateTeam(claims.UserID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// JoinTeam 加入队伍
func (h *TeamHandler) JoinTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	var req models.JoinTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.InviteCode == "" {
		writeError(w, http.StatusBadRequest, "邀请码不能为空")
		return
	}

	err := h.teamRepo.JoinTeam(claims.UserID, req.InviteCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "成功加入队伍"})
}

// GetMyTeam 获取我的队伍信息
func (h *TeamHandler) GetMyTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	team, err := h.teamRepo.GetTeamByUserID(claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "未加入任何队伍")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// LeaveTeam 退出队伍
func (h *TeamHandler) LeaveTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 DELETE 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	err := h.teamRepo.LeaveTeam(claims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "已成功退出队伍"})
}

// DissolveTeam 解散队伍
func (h *TeamHandler) DissolveTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 DELETE 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	err := h.teamRepo.DissolveTeam(claims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "队伍已解散"})
}

// GetTeamLeaderboard 获取团队排行榜
func (h *TeamHandler) GetTeamLeaderboard(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	ranking, err := h.teamRepo.GetTeamRanking(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取排行榜失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ranking)
}

// GetTeamPublic 获取公共团队信息
func (h *TeamHandler) GetTeamPublic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	teamID := r.URL.Query().Get("id")
	if teamID == "" {
		writeError(w, http.StatusBadRequest, "缺少队伍 ID")
		return
	}

	team, err := h.teamRepo.GetTeamByID(teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "队伍不存在")
		return
	}

	members, err := h.teamRepo.GetTeamMembers(teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取成员失败")
		return
	}
	team.Members = members

	// 脱敏处理：如果请求者不是成员，隐藏邀请码
	claims, _ := r.Context().Value("claims").(*utils.Claims)
	isMember := false
	if claims != nil {
		for _, m := range members {
			if m.ID == claims.UserID {
				isMember = true
				break
			}
		}
	}

	if !isMember {
		team.InviteCode = "" // 隐藏邀请码
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// UpdateTeam 更新团队信息 (仅队长)
func (h *TeamHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 PUT 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	// 1. 验证是否是该队队长
	team, err := h.teamRepo.GetTeamByUserID(claims.UserID)
	if err != nil || team.LeaderID != claims.UserID {
		writeError(w, http.StatusForbidden, "只有队长可以修改信息")
		return
	}

	var req models.UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "队伍名称不能为空")
		return
	}

	err = h.teamRepo.UpdateTeam(team.ID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "更新成功"})
}

// KickMember 踢出成员 (仅队长)
func (h *TeamHandler) KickMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	claims, ok := r.Context().Value("claims").(*utils.Claims)
	if !ok {
		writeError(w, http.StatusUnauthorized, "未授权")
		return
	}

	// 1. 验证是否是队长
	team, err := h.teamRepo.GetTeamByUserID(claims.UserID)
	if err != nil || team.LeaderID != claims.UserID {
		writeError(w, http.StatusForbidden, "只有队长可以踢出成员")
		return
	}

	var req models.KickMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	err = h.teamRepo.KickMember(team.ID, req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "成员已移除"})
}
