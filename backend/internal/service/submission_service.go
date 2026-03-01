package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/redis/go-redis/v9"
)

// SubmissionService 提交服务接口
type SubmissionService interface {
	SubmitFlag(userID string, problemID int, flag string) (*models.Submission, error)
	GetMySubmissions(userID string) ([]*models.Submission, error)
	CheckRateLimit(userID string) (bool, int64, error)
}

// SubmissionServiceImpl 提交服务实现
type SubmissionServiceImpl struct {
	submissionRepo *repository.SubmissionRepository
	problemRepo    *repository.ProblemRepository
	redisClient    *redis.Client
	rateLimit      int
	windowSeconds  int
}

// NewSubmissionService 创建提交服务实例
func NewSubmissionService(
	submissionRepo *repository.SubmissionRepository,
	problemRepo *repository.ProblemRepository,
	redisClient *redis.Client,
	rateLimit int,
	windowSeconds int,
) *SubmissionServiceImpl {
	return &SubmissionServiceImpl{
		submissionRepo: submissionRepo,
		problemRepo:    problemRepo,
		redisClient:    redisClient,
		rateLimit:      rateLimit,
		windowSeconds:  windowSeconds,
	}
}

// SubmitFlag 提交 flag
func (s *SubmissionServiceImpl) SubmitFlag(userID string, problemID int, flag string) (*models.Submission, error) {
	// 检查提交频率限制
	allowed, remaining, err := s.CheckRateLimit(userID)
	if err != nil {
		return nil, fmt.Errorf("检查提交限制失败: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("提交过于频繁，请稍后再试 (剩余时间: %d秒)", remaining)
	}

	// 获取题目信息
	problem, err := s.problemRepo.GetProblemByID(problemID)
	if err != nil {
		return nil, fmt.Errorf("题目不存在: %w", err)
	}

	// 获取题目的正确 flag
	correctFlag, err := s.problemRepo.GetFlagByProblemID(problemID)
	if err != nil {
		return nil, fmt.Errorf("获取题目 Flag 失败: %w", err)
	}

	// 验证 flag
	isCorrect := (flag == correctFlag)

	// 检查是否已经正确提交过该题目
	if isCorrect {
		submissions, err := s.submissionRepo.GetSubmissionsByUserID(userID)
		if err == nil {
			for _, sub := range submissions {
				if sub.ProblemID == problemID && sub.IsCorrect {
					return nil, fmt.Errorf("你已经解决过这个题目了")
				}
			}
		}
	}

	// 创建提交请求
	req := &models.CreateSubmissionRequest{
		ProblemID: problemID,
		Flag:      flag,
		IsCorrect: isCorrect,
	}

	// 保存提交记录
	submission, err := s.submissionRepo.CreateSubmission(userID, req)
	if err != nil {
		return nil, fmt.Errorf("保存提交记录失败: %w", err)
	}

	// 更新排行榜（如果提交正确）
	if isCorrect {
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
	submission.ProblemTitle = problem.Title

	return submission, nil
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
	ctx := context.Background()
	key := "leaderboard:user"

	// 从数据库获取用户分数
	// 注意：这里我们假设分数已经在数据库更新了
	// 为了简单，我们可以直接从数据库读取用户信息
	// 或者传递分数参数，这里先简单实现

	// 临时方案：先设置一个默认分数（实际应从数据库读取）
	// 我们暂时使用占位符，实际实现应该获取真实分数
	score := 100.0

	// 更新 ZSET
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: userID,
	}).Err()
	if err != nil {
		return fmt.Errorf("更新排行榜失败: %w", err)
	}

	return nil
}