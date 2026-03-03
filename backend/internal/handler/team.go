package handler

import (
	"net/http"
	"strconv"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/service"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type TeamHandler struct {
	teamRepo    *repository.TeamRepository
	teamService service.TeamService
}

func NewTeamHandler(teamRepo *repository.TeamRepository, teamService service.TeamService) *TeamHandler {
	return &TeamHandler{
		teamRepo:    teamRepo,
		teamService: teamService,
	}
}

// CreateTeamGin Gin 版本的创建队伍
func (h *TeamHandler) CreateTeamGin(c *gin.Context) {
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

	var req models.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "队伍名称不能为空"})
		return
	}

	team, err := h.teamService.CreateTeam(claims.UserID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 加载团队成员列表
	members, err := h.teamRepo.GetTeamMembers(team.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载成员列表失败"})
		return
	}
	team.Members = members

	c.JSON(http.StatusOK, team)
}

// JoinTeamGin Gin 版本的加入队伍
func (h *TeamHandler) JoinTeamGin(c *gin.Context) {
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

	var req models.JoinTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if req.InviteCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邀请码不能为空"})
		return
	}

	err := h.teamService.JoinTeam(claims.UserID, req.InviteCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 加入成功后，获取并返回更新后的团队信息
	team, err := h.teamRepo.GetTeamByUserID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取团队信息失败"})
		return
	}

	c.JSON(http.StatusOK, team)
}

// GetMyTeamGin Gin 版本的获取我的队伍信息
func (h *TeamHandler) GetMyTeamGin(c *gin.Context) {
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

	team, err := h.teamService.GetMyTeam(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未加入任何队伍"})
		return
	}

	c.JSON(http.StatusOK, team)
}

// LeaveTeamGin Gin 版本的退出队伍
func (h *TeamHandler) LeaveTeamGin(c *gin.Context) {
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

	err := h.teamService.LeaveTeam(claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已成功退出队伍"})
}

// DissolveTeamGin Gin 版本的解散队伍
func (h *TeamHandler) DissolveTeamGin(c *gin.Context) {
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

	err := h.teamService.DissolveTeam(claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "队伍已解散"})
}

// GetTeamLeaderboardGin Gin 版本的获取团队排行榜
func (h *TeamHandler) GetTeamLeaderboardGin(c *gin.Context) {
	limitStr := c.Query("limit")
	limit := 10
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	ranking, err := h.teamService.GetTeamLeaderboard(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取排行榜失败"})
		return
	}

	c.JSON(http.StatusOK, ranking)
}

// GetTeamPublicGin Gin 版本的获取公共团队信息
func (h *TeamHandler) GetTeamPublicGin(c *gin.Context) {
	teamID := c.Query("id")
	if teamID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少队伍 ID"})
		return
	}

	team, err := h.teamRepo.GetTeamByID(teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "队伍不存在"})
		return
	}

	members, err := h.teamRepo.GetTeamMembers(teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取成员失败"})
		return
	}

	// 隐藏所有成员的敏感信息
	for _, member := range members {
		member.Email = ""
		member.PasswordHash = ""
		member.IsActive = false
		member.IsVerified = false
		member.LastLoginAt = nil
		member.Preferences = ""
	}

	team.Members = members

	// 脱敏处理：如果请求者不是成员，隐藏邀请码
	claimsInterface, exists := c.Get("claims")
	isMember := false
	if exists {
		if claims, ok := claimsInterface.(*utils.Claims); ok {
			for _, m := range members {
				if m.ID == claims.UserID {
					isMember = true
					break
				}
			}
		}
	}

	if !isMember {
		team.InviteCode = "" // 隐藏邀请码
	}

	c.JSON(http.StatusOK, team)
}

// UpdateTeamGin Gin 版本的更新团队信息 (仅队长)
func (h *TeamHandler) UpdateTeamGin(c *gin.Context) {
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

	var req models.UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "队伍名称不能为空"})
		return
	}

	err := h.teamService.UpdateTeam(claims.UserID, req.Name)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// KickMemberGin Gin 版本的踢出成员 (仅队长)
func (h *TeamHandler) KickMemberGin(c *gin.Context) {
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

	var req models.KickMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	err := h.teamService.KickMember(claims.UserID, req.UserID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "成员已移除"})
}
