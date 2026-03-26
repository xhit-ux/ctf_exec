package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/redis/go-redis/v9"
)

// LeaderboardService 排行榜服务接口
type LeaderboardService interface {
	// 赛事排行榜
	UpdateContestUserScore(contestID, userID string, score float64) error
	GetContestUserTopN(ctx context.Context, contestID string, n int64) ([]redis.Z, error)
	GetContestUserRank(ctx context.Context, contestID, userID string) (int64, error)

	UpdateContestTeamScore(contestID, teamID string, score float64) error
	GetContestTeamTopN(ctx context.Context, contestID string, n int64) ([]redis.Z, error)
	GetContestTeamRank(ctx context.Context, contestID, teamID string) (int64, error)

	// 全站榜单快照（纯 DB 刷新）
	RefreshGlobalLeaderboardSnapshot() error
	StartSnapshotScheduler(interval time.Duration)
}

// LeaderboardServiceImpl 排行榜服务实现
type LeaderboardServiceImpl struct {
	redisClient *redis.Client
	userRepo    *repository.UserRepository
	teamRepo    *repository.TeamRepository
}

// NewLeaderboardService 创建排行榜服务实例
func NewLeaderboardService(
	redisClient *redis.Client,
	userRepo *repository.UserRepository,
	teamRepo *repository.TeamRepository,
	cfg *config.Config,
) *LeaderboardServiceImpl {
	return &LeaderboardServiceImpl{
		redisClient: redisClient,
		userRepo:    userRepo,
		teamRepo:    teamRepo,
	}
}

// StartSnapshotScheduler 启动全站榜单快照定时任务。
// 按 interval 周期刷新。
func (s *LeaderboardServiceImpl) StartSnapshotScheduler(interval time.Duration) {
	go func() {
		if err := s.RefreshGlobalLeaderboardSnapshot(); err != nil {
			log.Printf("启动时刷新全站榜单快照失败: %v", err)
		} else {
			log.Println("✓ 全站榜单快照已刷新")
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := s.RefreshGlobalLeaderboardSnapshot(); err != nil {
				log.Printf("定时刷新全站榜单快照失败: %v", err)
				continue
			}
			log.Println("✓ 全站榜单快照已刷新")
		}
	}()
}

// RefreshGlobalLeaderboardSnapshot 刷新用户与团队全站榜单快照。
func (s *LeaderboardServiceImpl) RefreshGlobalLeaderboardSnapshot() error {
	now := time.Now()

	if err := s.userRepo.RefreshGlobalUserLeaderboardSnapshot(now); err != nil {
		return fmt.Errorf("刷新用户榜单快照失败: %w", err)
	}

	if err := s.teamRepo.RefreshGlobalTeamLeaderboardSnapshot(now); err != nil {
		return fmt.Errorf("刷新团队榜单快照失败: %w", err)
	}

	return nil
}

// UpdateContestUserScore 更新赛事用户分数
func (s *LeaderboardServiceImpl) UpdateContestUserScore(contestID, userID string, score float64) error {
	ctx := context.Background()
	key := cache.GetContestUserLeaderboardKey(contestID)

	// 更新Redis ZSET
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: userID,
	}).Err()
	if err != nil {
		return fmt.Errorf("更新赛事用户排行榜失败: %w", err)
	}

	return nil
}

// GetContestUserTopN 获取赛事用户排行榜前N名
func (s *LeaderboardServiceImpl) GetContestUserTopN(ctx context.Context, contestID string, n int64) ([]redis.Z, error) {
	key := cache.GetContestUserLeaderboardKey(contestID)

	// 从Redis获取前N名
	result, err := s.redisClient.ZRevRangeWithScores(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("获取赛事用户排行榜前N名失败: %w", err)
	}

	return result, nil
}

// GetContestUserRank 获取赛事用户排名
func (s *LeaderboardServiceImpl) GetContestUserRank(ctx context.Context, contestID, userID string) (int64, error) {
	key := cache.GetContestUserLeaderboardKey(contestID)

	// 从Redis获取排名（降序排名，0为第一名）
	rank, err := s.redisClient.ZRevRank(ctx, key, userID).Result()
	if err != nil {
		if err == redis.Nil {
			// 用户不在排行榜中，返回-1表示未上榜
			return -1, nil
		}
		return -1, fmt.Errorf("获取赛事用户排名失败: %w", err)
	}

	return rank, nil
}

// UpdateContestTeamScore 更新赛事团队分数
func (s *LeaderboardServiceImpl) UpdateContestTeamScore(contestID, teamID string, score float64) error {
	ctx := context.Background()
	key := cache.GetContestTeamLeaderboardKey(contestID)

	// 更新Redis ZSET
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: teamID,
	}).Err()
	if err != nil {
		return fmt.Errorf("更新赛事团队排行榜失败: %w", err)
	}

	return nil
}

// GetContestTeamTopN 获取赛事团队排行榜前N名
func (s *LeaderboardServiceImpl) GetContestTeamTopN(ctx context.Context, contestID string, n int64) ([]redis.Z, error) {
	key := cache.GetContestTeamLeaderboardKey(contestID)

	// 从Redis获取前N名
	result, err := s.redisClient.ZRevRangeWithScores(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("获取赛事团队排行榜前N名失败: %w", err)
	}

	return result, nil
}

// GetContestTeamRank 获取赛事团队排名
func (s *LeaderboardServiceImpl) GetContestTeamRank(ctx context.Context, contestID, teamID string) (int64, error) {
	key := cache.GetContestTeamLeaderboardKey(contestID)

	// 从Redis获取排名（降序排名，0为第一名）
	rank, err := s.redisClient.ZRevRank(ctx, key, teamID).Result()
	if err != nil {
		if err == redis.Nil {
			// 团队不在排行榜中，返回-1表示未上榜
			return -1, nil
		}
		return -1, fmt.Errorf("获取赛事团队排名失败: %w", err)
	}

	return rank, nil
}
