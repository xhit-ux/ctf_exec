package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
)

type ContestService interface {
	GetContestStatus(contest *models.Contest) string
	JoinContest(contestID, userID string, teamID *string) error
	LeaveContest(contestID, userID string, teamID *string) error
	GetContestChallengesForUser(contestID, userID, userRole string) ([]*models.ContestChallengeView, error)
	GetContestScoreboard(contestID, boardType string, limit int) ([]*models.ContestScoreboardEntry, error)
	ValidateContestSubmission(contestID, userID string, problemID string) (*models.ContestParticipantScope, int, error)
	ApplyContestSolve(contestID string, scope *models.ContestParticipantScope, score int, solvedAt time.Time) error
}

type ContestServiceImpl struct {
	contestRepo      *repository.ContestRepository
	contestAdminRepo repository.ContestAdminRepository
	teamRepo         *repository.TeamRepository
	leaderboardSvc   LeaderboardService
}

func NewContestService(contestRepo *repository.ContestRepository, contestAdminRepo repository.ContestAdminRepository, teamRepo *repository.TeamRepository, leaderboardSvc LeaderboardService) *ContestServiceImpl {
	return &ContestServiceImpl{
		contestRepo:      contestRepo,
		contestAdminRepo: contestAdminRepo,
		teamRepo:         teamRepo,
		leaderboardSvc:   leaderboardSvc,
	}
}

func (s *ContestServiceImpl) GetContestStatus(contest *models.Contest) string {
	now := time.Now()
	if contest.StartTime != nil && now.Before(*contest.StartTime) {
		return "upcoming"
	}
	if contest.EndTime != nil && now.After(*contest.EndTime) {
		return "ended"
	}
	return "running"
}

func (s *ContestServiceImpl) JoinContest(contestID, userID string, teamID *string) error {
	contest, err := s.contestRepo.GetContestByID(contestID)
	if err != nil {
		return err
	}

	status := s.GetContestStatus(contest)
	if status == "ended" {
		return apperr.New(apperr.ErrBadRequest, "赛事已结束，无法报名")
	}

	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		normalizedTeamID := strings.TrimSpace(*teamID)
		member, err := s.contestRepo.IsTeamMember(normalizedTeamID, userID)
		if err != nil {
			return err
		}
		if !member {
			return apperr.New(apperr.ErrForbidden, "用户不属于该团队")
		}

		if err := s.contestRepo.JoinContest(contestID, userID, &normalizedTeamID); err != nil {
			return err
		}
		return s.contestRepo.EnsureContestTeamScore(contestID, normalizedTeamID)
	}

	if err := s.contestRepo.JoinContest(contestID, userID, nil); err != nil {
		return err
	}
	return s.contestRepo.EnsureContestUserScore(contestID, userID)
}

func (s *ContestServiceImpl) LeaveContest(contestID, userID string, teamID *string) error {
	if _, err := s.contestRepo.GetContestByID(contestID); err != nil {
		return err
	}

	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		normalizedTeamID := strings.TrimSpace(*teamID)
		member, err := s.contestRepo.IsTeamMember(normalizedTeamID, userID)
		if err != nil {
			return err
		}
		if !member {
			return apperr.New(apperr.ErrForbidden, "用户不属于该团队")
		}
		return s.contestRepo.LeaveContest(contestID, userID, &normalizedTeamID)
	}

	return s.contestRepo.LeaveContest(contestID, userID, nil)
}

func (s *ContestServiceImpl) GetContestChallengesForUser(contestID, userID, userRole string) ([]*models.ContestChallengeView, error) {
	contest, err := s.contestRepo.GetContestByID(contestID)
	if err != nil {
		return nil, err
	}

	if s.GetContestStatus(contest) == "upcoming" {
		role := models.NormalizeRole(userRole)
		if role != models.RoleRoot && role != models.RoleSiteAdmin {
			adminRole, err := s.contestAdminRepo.GetContestAdminRole(userID, contestID)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(adminRole) == "" {
				return nil, apperr.New(apperr.ErrForbidden, "赛事未开始，仅赛事管理员可查看题目")
			}
		}
	}

	items, err := s.contestRepo.ListContestChallengesForUser(contestID, userID)
	if err != nil {
		return nil, err
	}
	// 对动态计分题目：用 CTFd 公式覆盖 current_score。
	// repo 阶段返回的 CurrentScore=contest_score（占位），这里精确计算。
	for _, it := range items {
		if it == nil {
			continue
		}
		if it.IsDynamicScore && it.InitialScore != nil && it.MinScore != nil && it.DecayConstant != nil {
			cfg := &models.ContestChallenge{
				IsDynamicScore: true,
				ContestScore:   it.CurrentScore,
				InitialScore:   it.InitialScore,
				MinScore:       it.MinScore,
				DecayConstant:  it.DecayConstant,
			}
			it.CurrentScore = ComputeContestCurrentScore(cfg, it.SolveCount)
		}
	}
	return items, nil
}

func (s *ContestServiceImpl) GetContestScoreboard(contestID, boardType string, limit int) ([]*models.ContestScoreboardEntry, error) {
	if _, err := s.contestRepo.GetContestByID(contestID); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 200 {
		limit = 50
	}

	if boardType == "team" {
		if s.leaderboardSvc != nil {
			redisRows, err := s.leaderboardSvc.GetContestTeamTopN(context.Background(), contestID, int64(limit))
			if err == nil && len(redisRows) > 0 {
				result := make([]*models.ContestScoreboardEntry, 0, len(redisRows))
				for i, row := range redisRows {
					member := fmt.Sprintf("%v", row.Member)
					score := int(row.Score)
					entry := &models.ContestScoreboardEntry{Rank: i + 1, Score: score}
					entry.TeamID = &member
					result = append(result, entry)
				}
				return result, nil
			}
		}
		return s.contestRepo.GetContestTeamScoreboardFromDB(contestID, limit)
	}

	if s.leaderboardSvc != nil {
		redisRows, err := s.leaderboardSvc.GetContestUserTopN(context.Background(), contestID, int64(limit))
		if err == nil && len(redisRows) > 0 {
			result := make([]*models.ContestScoreboardEntry, 0, len(redisRows))
			for i, row := range redisRows {
				member := fmt.Sprintf("%v", row.Member)
				score := int(row.Score)
				entry := &models.ContestScoreboardEntry{Rank: i + 1, Score: score}
				entry.UserID = &member
				result = append(result, entry)
			}
			return result, nil
		}
	}

	return s.contestRepo.GetContestUserScoreboardFromDB(contestID, limit)
}

func (s *ContestServiceImpl) ValidateContestSubmission(contestID, userID string, problemID string) (*models.ContestParticipantScope, int, error) {
	contest, err := s.contestRepo.GetContestByID(contestID)
	if err != nil {
		return nil, 0, err
	}

	if s.GetContestStatus(contest) != "running" {
		return nil, 0, apperr.New(apperr.ErrBadRequest, "赛事未处于进行中")
	}

	allowed, err := s.contestRepo.IsProblemInContest(contestID, problemID)
	if err != nil {
		return nil, 0, err
	}
	if !allowed {
		return nil, 0, apperr.New(apperr.ErrBadRequest, "题目不在该赛事中")
	}

	var scope models.ContestParticipantScope
	isUserParticipant, err := s.contestRepo.IsParticipant(contestID, userID, nil)
	if err != nil {
		return nil, 0, err
	}
	if isUserParticipant {
		scope.UserID = &userID
	} else {
		team, teamErr := s.teamRepo.GetTeamByUserID(userID)
		if teamErr != nil || team == nil {
			return nil, 0, apperr.New(apperr.ErrBadRequest, "未报名该赛事")
		}
		isTeamParticipant, err := s.contestRepo.IsParticipant(contestID, userID, &team.ID)
		if err != nil {
			return nil, 0, err
		}
		if !isTeamParticipant {
			return nil, 0, apperr.New(apperr.ErrBadRequest, "未报名该赛事")
		}
		scope.TeamID = &team.ID
	}

	if scope.TeamID != nil {
		solved, err := s.contestRepo.HasSolvedInContest(contestID, problemID, userID, scope.TeamID)
		if err != nil {
			return nil, 0, err
		}
		if solved {
			return nil, 0, apperr.New(apperr.ErrConflict, "该题在赛事中已解出")
		}
	} else {
		solved, err := s.contestRepo.HasSolvedInContest(contestID, problemID, userID, nil)
		if err != nil {
			return nil, 0, err
		}
		if solved {
			return nil, 0, apperr.New(apperr.ErrConflict, "该题在赛事中已解出")
		}
	}

	cfg, err := s.contestRepo.GetContestChallengeScoringConfig(contestID, problemID)
	if err != nil {
		return nil, 0, err
	}
	solveCount, err := s.contestRepo.CountContestSolves(contestID, problemID)
	if err != nil {
		return nil, 0, err
	}
	// 当前提交者也算一次解题，按 v2.1 公式 N=solveCount+1 计入。
	score := ComputeContestCurrentScore(cfg, solveCount+1)

	return &scope, score, nil
}

// ComputeContestCurrentScore 计算赛事题目的当前分值。
//
// CTFd 多项式衰减公式（v2.1 Part 3.2）：
//
//	current = max(min, ⌈ (min - initial) / decay² · N² + initial ⌉)
//
// 其中 N 为该题已解题主体数（含本次提交）。
// 当 is_dynamic_score = false 时，固定返回 contest_score。
func ComputeContestCurrentScore(cfg *models.ContestChallenge, solveCount int) int {
	if cfg == nil {
		return 0
	}
	if !cfg.IsDynamicScore {
		return cfg.ContestScore
	}
	if cfg.InitialScore == nil || cfg.MinScore == nil || cfg.DecayConstant == nil {
		return cfg.ContestScore
	}
	if solveCount <= 0 {
		return *cfg.InitialScore
	}
	initial := float64(*cfg.InitialScore)
	minScore := float64(*cfg.MinScore)
	decay := float64(*cfg.DecayConstant)
	if decay <= 0 {
		return *cfg.InitialScore
	}
	n := float64(solveCount)
	current := (minScore-initial)/(decay*decay)*n*n + initial
	rounded := int(current + 0.999999) // 向上取整
	if rounded < *cfg.MinScore {
		rounded = *cfg.MinScore
	}
	if rounded > *cfg.InitialScore {
		rounded = *cfg.InitialScore
	}
	return rounded
}

func (s *ContestServiceImpl) ApplyContestSolve(contestID string, scope *models.ContestParticipantScope, score int, solvedAt time.Time) error {
	if scope == nil {
		return apperr.New(apperr.ErrBadRequest, "赛事主体信息为空")
	}

	if scope.TeamID != nil {
		latest, err := s.contestRepo.UpsertContestTeamScore(contestID, *scope.TeamID, score, solvedAt)
		if err != nil {
			return err
		}
		if s.leaderboardSvc != nil {
			_ = s.leaderboardSvc.UpdateContestTeamScore(contestID, *scope.TeamID, float64(latest))
		}
		return nil
	}

	if scope.UserID == nil {
		return apperr.New(apperr.ErrBadRequest, "赛事个人主体为空")
	}
	latest, err := s.contestRepo.UpsertContestUserScore(contestID, *scope.UserID, score, solvedAt)
	if err != nil {
		return err
	}
	if s.leaderboardSvc != nil {
		_ = s.leaderboardSvc.UpdateContestUserScore(contestID, *scope.UserID, float64(latest))
	}

	return nil
}

func normalizeScoreMemberID(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
