package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/redis/go-redis/v9"
)

// SubmissionService 提交服务接口
type SubmissionService interface {
	SubmitFlag(userID string, problemID int, flag string, contestID *string, clientIP string) (*models.Submission, error)
	GetMySubmissions(userID string) ([]*models.Submission, error)
	CheckRateLimit(userID string) (bool, int64, error)
}

// SubmissionServiceImpl 提交服务实现
type SubmissionServiceImpl struct {
	submissionRepo *repository.SubmissionRepository
	problemRepo    *repository.ProblemRepository
	userRepo       *repository.UserRepository
	leaderboardSvc LeaderboardService
	contestSvc     ContestService
	redisClient    *redis.Client
	rateLimit      int
	windowSeconds  int
}

// NewSubmissionService 创建提交服务实例
func NewSubmissionService(
	submissionRepo *repository.SubmissionRepository,
	problemRepo *repository.ProblemRepository,
	userRepo *repository.UserRepository,
	leaderboardSvc LeaderboardService,
	contestSvc ContestService,
	redisClient *redis.Client,
	rateLimit int,
	windowSeconds int,
) *SubmissionServiceImpl {
	return &SubmissionServiceImpl{
		submissionRepo: submissionRepo,
		problemRepo:    problemRepo,
		userRepo:       userRepo,
		leaderboardSvc: leaderboardSvc,
		contestSvc:     contestSvc,
		redisClient:    redisClient,
		rateLimit:      rateLimit,
		windowSeconds:  windowSeconds,
	}
}

// SubmitFlag 提交 flag
func (s *SubmissionServiceImpl) SubmitFlag(userID string, problemID int, flag string, contestID *string, clientIP string) (*models.Submission, error) {
	// 检查提交频率限制
	allowed, remaining, err := s.CheckRateLimit(userID)
	if err != nil {
		return nil, fmt.Errorf("检查提交限制失败: %w", err)
	}
	if !allowed {
		return nil, apperr.New(apperr.ErrBadRequest, fmt.Sprintf("提交过于频繁，请稍后再试 (剩余时间: %d秒)", remaining))
	}

	// 获取题目元数据并校验提交可见性策略。
	problemMeta, err := s.problemRepo.GetProblemSubmissionMeta(problemID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, apperr.New(apperr.ErrBadRequest, "题目不存在")
		}
		return nil, fmt.Errorf("查询题目失败: %w", err)
	}

	if err := validateProblemSubmissionPolicy(problemMeta, contestID); err != nil {
		return nil, err
	}

	// 获取题目的正确 flag hash
	correctFlagHash, err := s.problemRepo.GetFlagHashByProblemID(problemID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, apperr.New(apperr.ErrNotFound, "获取题目 Flag 失败")
		}
		return nil, fmt.Errorf("获取题目 Flag Hash 失败: %w", err)
	}

	// 验证 flag (SHA256 哈希比较)
	isCorrect := utils.VerifyFlagHash(flag, correctFlagHash)

	var contestScope *models.ContestParticipantScope
	var contestScore int
	if contestID != nil && *contestID != "" {
		if s.contestSvc == nil {
			return nil, apperr.New(apperr.ErrBadRequest, "赛事服务不可用")
		}
		scope, score, err := s.contestSvc.ValidateContestSubmission(*contestID, userID, problemID)
		if err != nil {
			return nil, err
		}
		contestScope = scope
		contestScore = score
	}

	alreadySolved, err := s.submissionRepo.HasCorrectSubmissionByUserProblem(userID, problemID)
	if err != nil {
		return nil, fmt.Errorf("查询历史解题记录失败: %w", err)
	}

	// 非赛事场景：检查是否已经正确提交过该题目
	if isCorrect && alreadySolved && (contestID == nil || *contestID == "") {
		return nil, apperr.New(apperr.ErrConflict, "你已经解决过这个题目了")
	}

	// 创建提交请求
	req := &models.CreateSubmissionRequest{
		ProblemID: problemID,
		Flag:      flag,
		TeamID:    nil,
		ContestID: contestID,
		IsCorrect: isCorrect,
		IPAddress: clientIP,
	}
	if contestScope != nil {
		req.TeamID = contestScope.TeamID
	}

	// 保存提交记录
	submission, err := s.submissionRepo.CreateSubmission(userID, req)
	if err != nil {
		return nil, fmt.Errorf("保存提交记录失败: %w", err)
	}

	// 更新排行榜（如果提交正确）
	if isCorrect {
		if !alreadySolved && s.userRepo != nil {
			err = s.userRepo.IncreaseUserScoreAndSolvedCount(userID, problemMeta.Points)
			if err != nil {
				fmt.Printf("更新全站用户积分失败: %v\n", err)
			}
		}

		if contestID != nil && *contestID != "" {
			err = s.contestSvc.ApplyContestSolve(*contestID, contestScope, contestScore, time.Now())
			if err != nil {
				fmt.Printf("更新赛事榜失败: %v\n", err)
			}
		}

		// 更新用户分数到 Redis ZSET
		err = s.updateLeaderboard(userID)
		if err != nil {
			// 仅记录错误，不影响主要功能
			fmt.Printf("更新排行榜失败: %v\n", err)
		}
	}

	// 记录提交频率
	err = s.recordSubmission(userID)
	if err != nil {
		fmt.Printf("记录提交频率失败: %v\n", err)
	}

	// 设置提交标题
	submission.ProblemTitle = problemMeta.Title

	return submission, nil
}

func validateProblemSubmissionPolicy(problem *models.ProblemSubmissionMeta, contestID *string) error {
	if problem == nil {
		return apperr.New(apperr.ErrBadRequest, "题目元数据为空")
	}

	if problem.Status != "published" {
		return apperr.New(apperr.ErrForbidden, "题目当前不可提交")
	}

	now := time.Now()
	hasContest := contestID != nil && *contestID != ""

	if hasContest {
		if problem.Scope == "contest" {
			if problem.ContestID == nil || *problem.ContestID != *contestID {
				return apperr.New(apperr.ErrForbidden, "题目不属于当前赛事")
			}
			return nil
		}

		if problem.Scope != "platform" {
			return apperr.New(apperr.ErrForbidden, "题目类型不支持提交")
		}

		if problem.VisibleFrom != nil && now.Before(*problem.VisibleFrom) {
			return apperr.New(apperr.ErrForbidden, "题目尚未开放")
		}
		if problem.VisibleTo != nil && now.After(*problem.VisibleTo) {
			return apperr.New(apperr.ErrForbidden, "题目已关闭")
		}
		return nil
	}

	if problem.Scope != "platform" {
		return apperr.New(apperr.ErrForbidden, "该题仅支持在赛事内提交")
	}
	if problem.VisibleFrom != nil && now.Before(*problem.VisibleFrom) {
		return apperr.New(apperr.ErrForbidden, "题目尚未开放")
	}
	if problem.VisibleTo != nil && now.After(*problem.VisibleTo) {
		return apperr.New(apperr.ErrForbidden, "题目已关闭")
	}

	return nil
}

// GetMySubmissions 获取用户的提交记录
func (s *SubmissionServiceImpl) GetMySubmissions(userID string) ([]*models.Submission, error) {
	submissions, err := s.submissionRepo.GetSubmissionsByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取提交记录失败: %w", err)
	}

	return submissions, nil
}

// CheckRateLimit 检查提交频率限制
func (s *SubmissionServiceImpl) CheckRateLimit(userID string) (bool, int64, error) {
	ctx := context.Background()
	key := fmt.Sprintf("submission:rate_limit:%s", userID)

	// 获取当前时间戳
	now := time.Now().Unix()
	windowStart := now - int64(s.windowSeconds)

	// 删除窗口期之前的记录
	err := s.redisClient.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart)).Err()
	if err != nil {
		return false, 0, fmt.Errorf("清理旧记录失败: %w", err)
	}

	// 获取窗口期内的提交数量
	count, err := s.redisClient.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf").Result()
	if err != nil {
		return false, 0, fmt.Errorf("获取提交数量失败: %w", err)
	}

	// 检查是否超过限制
	if count >= int64(s.rateLimit) {
		// 获取最早提交的时间
		oldest, err := s.redisClient.ZRangeWithScores(ctx, key, 0, 0).Result()
		if err != nil || len(oldest) == 0 {
			return false, int64(s.windowSeconds), nil
		}
		// 计算剩余时间
		remaining := int64(s.windowSeconds) - (now - int64(oldest[0].Score))
		if remaining < 0 {
			remaining = 0
		}
		return false, remaining, nil
	}

	return true, 0, nil
}

// recordSubmission 记录提交
func (s *SubmissionServiceImpl) recordSubmission(userID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("submission:rate_limit:%s", userID)
	now := time.Now().Unix()

	// 添加当前时间戳到有序集合
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: now,
	}).Err()
	if err != nil {
		return fmt.Errorf("记录提交失败: %w", err)
	}

	// 设置键的过期时间
	expireTime := time.Duration(s.windowSeconds+60) * time.Second // 额外60秒缓冲
	err = s.redisClient.Expire(ctx, key, expireTime).Err()
	if err != nil {
		return fmt.Errorf("设置过期时间失败: %w", err)
	}

	return nil
}

// updateLeaderboard 更新排行榜
func (s *SubmissionServiceImpl) updateLeaderboard(userID string) error {
	// 从数据库同步用户最新分数到Redis排行榜
	// 这会自动从数据库读取用户的真实分数（Score字段）
	// 并更新到Global User Leaderboard中
	err := s.leaderboardSvc.SyncUserScoreFromDB(userID)
	if err != nil {
		return fmt.Errorf("同步用户分数到排行榜失败: %w", err)
	}

	return nil
}
