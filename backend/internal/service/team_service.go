package service

import (
	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/authz"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
)

// TeamService 团队服务接口
type TeamService interface {
	CreateTeam(userID string, name string) (*models.Team, error)
	JoinTeam(userID string, inviteCode string) error
	GetMyTeam(userID string) (*models.Team, error)
	LeaveTeam(userID string) error
	DissolveTeam(userID string) error
	GetTeamLeaderboard(limit int) ([]*models.TeamLeaderboardEntry, error)
	GetTeamPublic(teamID string) (*models.Team, error)
	UpdateTeam(userID string, name string) error
	KickMember(userID string, targetUserID string) error
}

// TeamServiceImpl 团队服务实现
type TeamServiceImpl struct {
	teamRepo *repository.TeamRepository
}

// NewTeamService 创建团队服务实例
func NewTeamService(teamRepo *repository.TeamRepository) *TeamServiceImpl {
	return &TeamServiceImpl{teamRepo: teamRepo}
}

// CreateTeam 创建队伍
func (s *TeamServiceImpl) CreateTeam(userID string, name string) (*models.Team, error) {
	// 验证队伍名称
	if name == "" {
		return nil, apperr.New(apperr.ErrBadRequest, "队伍名称不能为空")
	}
	if len(name) < 2 || len(name) > 32 {
		return nil, apperr.New(apperr.ErrBadRequest, "队伍名称长度应在 2-32 个字符之间")
	}

	team, err := s.teamRepo.CreateTeam(userID, name)
	if err != nil {
		return nil, wrapServiceError("创建队伍失败", err)
	}

	return team, nil
}

// JoinTeam 加入队伍
func (s *TeamServiceImpl) JoinTeam(userID string, inviteCode string) error {
	if inviteCode == "" {
		return apperr.New(apperr.ErrBadRequest, "邀请码不能为空")
	}

	err := s.teamRepo.JoinTeam(userID, inviteCode)
	if err != nil {
		return wrapServiceError("加入队伍失败", err)
	}

	return nil
}

// GetMyTeam 获取用户所属队伍
func (s *TeamServiceImpl) GetMyTeam(userID string) (*models.Team, error) {
	team, err := s.teamRepo.GetTeamByUserID(userID)
	if err != nil {
		return nil, wrapServiceError("获取队伍信息失败", err)
	}

	// 获取成员列表
	members, err := s.teamRepo.GetTeamMembers(team.ID)
	if err != nil {
		return nil, wrapServiceError("获取成员列表失败", err)
	}

	team.Members = members
	return team, nil
}

// LeaveTeam 退出队伍
func (s *TeamServiceImpl) LeaveTeam(userID string) error {
	err := s.teamRepo.LeaveTeam(userID)
	if err != nil {
		return wrapServiceError("退出队伍失败", err)
	}

	return nil
}

// DissolveTeam 解散队伍
func (s *TeamServiceImpl) DissolveTeam(userID string) error {
	err := s.teamRepo.DissolveTeam(userID)
	if err != nil {
		return wrapServiceError("解散队伍失败", err)
	}

	return nil
}

// GetTeamLeaderboard 获取团队排行榜
func (s *TeamServiceImpl) GetTeamLeaderboard(limit int) ([]*models.TeamLeaderboardEntry, error) {
	if limit <= 0 {
		limit = 20 // 默认值
	}
	if limit > 100 {
		limit = 100 // 限制最大值
	}

	ranking, err := s.teamRepo.GetTeamRanking(limit)
	if err != nil {
		return nil, wrapServiceError("获取团队排行榜失败", err)
	}

	return ranking, nil
}

// GetTeamPublic 获取公开队伍信息
func (s *TeamServiceImpl) GetTeamPublic(teamID string) (*models.Team, error) {
	if teamID == "" {
		return nil, apperr.New(apperr.ErrBadRequest, "队伍ID不能为空")
	}

	team, err := s.teamRepo.GetTeamByID(teamID)
	if err != nil {
		return nil, wrapServiceError("获取队伍信息失败", err)
	}

	// 获取成员列表（仅公开信息）
	members, err := s.teamRepo.GetTeamMembers(teamID)
	if err != nil {
		return nil, wrapServiceError("获取成员列表失败", err)
	}

	// 隐藏敏感信息
	for _, member := range members {
		member.Email = ""
		member.PasswordHash = ""
		member.IsActive = false
		member.IsVerified = false
		member.LastLoginAt = nil
	}

	team.Members = members
	return team, nil
}

// UpdateTeam 更新队伍信息
func (s *TeamServiceImpl) UpdateTeam(userID string, name string) error {
	// 验证权限：必须是队长
	team, err := s.teamRepo.GetTeamByUserID(userID)
	if err != nil {
		return wrapServiceError("获取队伍信息失败", err)
	}

	// 检查是否是队长
	if !authz.IsTeamLeader(userID, team.LeaderID) {
		return apperr.New(apperr.ErrForbidden, "只有队长可以修改队伍信息")
	}

	// 验证队伍名称
	if name == "" {
		return apperr.New(apperr.ErrBadRequest, "队伍名称不能为空")
	}
	if len(name) < 2 || len(name) > 32 {
		return apperr.New(apperr.ErrBadRequest, "队伍名称长度应在 2-32 个字符之间")
	}

	err = s.teamRepo.UpdateTeam(team.ID, name)
	if err != nil {
		return wrapServiceError("更新队伍信息失败", err)
	}

	return nil
}

// KickMember 踢出成员
func (s *TeamServiceImpl) KickMember(userID string, targetUserID string) error {
	if targetUserID == "" {
		return apperr.New(apperr.ErrBadRequest, "目标用户ID不能为空")
	}

	// 不能踢自己
	if userID == targetUserID {
		return apperr.New(apperr.ErrBadRequest, "不能踢出自己")
	}

	// 验证权限：必须是队长
	team, err := s.teamRepo.GetTeamByUserID(userID)
	if err != nil {
		return wrapServiceError("获取队伍信息失败", err)
	}

	if !authz.IsTeamLeader(userID, team.LeaderID) {
		return apperr.New(apperr.ErrForbidden, "只有队长可以踢出成员")
	}

	err = s.teamRepo.KickMember(team.ID, targetUserID)
	if err != nil {
		return wrapServiceError("踢出成员失败", err)
	}

	return nil
}
