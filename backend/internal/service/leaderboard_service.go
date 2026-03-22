package service

import (
	"context"
	"encoding/json"
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
	// 全站用户排行榜
	UpdateGlobalUserScore(userID string, score float64) error
	GetGlobalUserTopN(ctx context.Context, n int64) ([]redis.Z, error)
	GetGlobalUserRank(ctx context.Context, userID string) (int64, error)
	GetGlobalUserScore(ctx context.Context, userID string) (float64, error)

	// 全站团队排行榜
	UpdateGlobalTeamScore(teamID string, score float64) error
	GetGlobalTeamTopN(ctx context.Context, n int64) ([]redis.Z, error)
	GetGlobalTeamRank(ctx context.Context, teamID string) (int64, error)
	GetGlobalTeamScore(ctx context.Context, teamID string) (float64, error)

	// 赛事排行榜
	UpdateContestUserScore(contestID, userID string, score float64) error
	GetContestUserTopN(ctx context.Context, contestID string, n int64) ([]redis.Z, error)
	GetContestUserRank(ctx context.Context, contestID, userID string) (int64, error)

	UpdateContestTeamScore(contestID, teamID string, score float64) error
	GetContestTeamTopN(ctx context.Context, contestID string, n int64) ([]redis.Z, error)
	GetContestTeamRank(ctx context.Context, contestID, teamID string) (int64, error)

	// 同步方法
	SyncUserScoreFromDB(userID string) error
	SyncTeamScoreFromDB(teamID string) error
	SyncAllScoresFromDB() error
}

// LeaderboardServiceImpl 排行榜服务实现
type LeaderboardServiceImpl struct {
	redisClient    *redis.Client
	userRepo       *repository.UserRepository
	teamRepo       *repository.TeamRepository
	leaderboardCfg config.LeaderboardConfig
}

// NewLeaderboardService 创建排行榜服务实例
func NewLeaderboardService(
	redisClient *redis.Client,
	userRepo *repository.UserRepository,
	teamRepo *repository.TeamRepository,
	cfg *config.Config,
) *LeaderboardServiceImpl {
	return &LeaderboardServiceImpl{
		redisClient:    redisClient,
		userRepo:       userRepo,
		teamRepo:       teamRepo,
		leaderboardCfg: cfg.Leaderboard,
	}
}

// StartSnapshotScheduler 启动全站榜单快照定时任务。
// 按 interval 周期刷新。
func (s *LeaderboardServiceImpl) StartSnapshotScheduler(interval time.Duration) {
	go func() {
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

	cachePayload := map[string]string{
		"mode":         "snapshot",
		"refreshed_at": now.Format(time.RFC3339),
	}
	raw, err := json.Marshal(cachePayload)
	if err == nil {
		_ = s.redisClient.Set(context.Background(), "leaderboard:global", raw, time.Duration(s.leaderboardCfg.CacheTTLSeconds)*time.Second).Err()
	}

	return nil
}

// UpdateGlobalUserScore 更新全站用户分数
func (s *LeaderboardServiceImpl) UpdateGlobalUserScore(userID string, score float64) error {
	ctx := context.Background()
	key := cache.KeyGlobalUserLeaderboard

	// 更新Redis ZSET
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: userID,
	}).Err()
	if err != nil {
		return fmt.Errorf("更新用户排行榜失败: %w", err)
	}

	// 设置过期时间（自动续期）
	expireTime := time.Duration(s.leaderboardCfg.CacheTTLSeconds) * time.Second
	err = s.redisClient.Expire(ctx, key, expireTime).Err()
	if err != nil {
		// 过期时间设置失败不影响主要功能，只记录日志
		fmt.Printf("设置排行榜过期时间失败: %v\n", err)
	}

	return nil
}

// GetGlobalUserTopN 获取全站用户排行榜前N名
func (s *LeaderboardServiceImpl) GetGlobalUserTopN(ctx context.Context, n int64) ([]redis.Z, error) {
	key := cache.KeyGlobalUserLeaderboard

	// 从Redis获取前N名
	result, err := s.redisClient.ZRevRangeWithScores(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("获取用户排行榜前N名失败: %w", err)
	}

	return result, nil
}

// GetGlobalUserRank 获取全站用户排名
func (s *LeaderboardServiceImpl) GetGlobalUserRank(ctx context.Context, userID string) (int64, error) {
	key := cache.KeyGlobalUserLeaderboard

	// 从Redis获取排名（降序排名，0为第一名）
	rank, err := s.redisClient.ZRevRank(ctx, key, userID).Result()
	if err != nil {
		if err == redis.Nil {
			// 用户不在排行榜中，返回-1表示未上榜
			return -1, nil
		}
		return -1, fmt.Errorf("获取用户排名失败: %w", err)
	}

	return rank, nil
}

// GetGlobalUserScore 获取全站用户分数
func (s *LeaderboardServiceImpl) GetGlobalUserScore(ctx context.Context, userID string) (float64, error) {
	key := cache.KeyGlobalUserLeaderboard

	// 从Redis获取分数
	score, err := s.redisClient.ZScore(ctx, key, userID).Result()
	if err != nil {
		if err == redis.Nil {
			// 用户不在排行榜中，返回0分
			return 0, nil
		}
		return 0, fmt.Errorf("获取用户分数失败: %w", err)
	}

	return score, nil
}

// UpdateGlobalTeamScore 更新全站团队分数
func (s *LeaderboardServiceImpl) UpdateGlobalTeamScore(teamID string, score float64) error {
	ctx := context.Background()
	key := cache.KeyGlobalTeamLeaderboard

	// 更新Redis ZSET
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: teamID,
	}).Err()
	if err != nil {
		return fmt.Errorf("更新团队排行榜失败: %w", err)
	}

	// 设置过期时间（自动续期）
	expireTime := time.Duration(s.leaderboardCfg.CacheTTLSeconds) * time.Second
	err = s.redisClient.Expire(ctx, key, expireTime).Err()
	if err != nil {
		fmt.Printf("设置团队排行榜过期时间失败: %v\n", err)
	}

	return nil
}

// GetGlobalTeamTopN 获取全站团队排行榜前N名
func (s *LeaderboardServiceImpl) GetGlobalTeamTopN(ctx context.Context, n int64) ([]redis.Z, error) {
	key := cache.KeyGlobalTeamLeaderboard

	// 从Redis获取前N名
	result, err := s.redisClient.ZRevRangeWithScores(ctx, key, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("获取团队排行榜前N名失败: %w", err)
	}

	return result, nil
}

// GetGlobalTeamRank 获取全站团队排名
func (s *LeaderboardServiceImpl) GetGlobalTeamRank(ctx context.Context, teamID string) (int64, error) {
	key := cache.KeyGlobalTeamLeaderboard

	// 从Redis获取排名（降序排名，0为第一名）
	rank, err := s.redisClient.ZRevRank(ctx, key, teamID).Result()
	if err != nil {
		if err == redis.Nil {
			// 团队不在排行榜中，返回-1表示未上榜
			return -1, nil
		}
		return -1, fmt.Errorf("获取团队排名失败: %w", err)
	}

	return rank, nil
}

// GetGlobalTeamScore 获取全站团队分数
func (s *LeaderboardServiceImpl) GetGlobalTeamScore(ctx context.Context, teamID string) (float64, error) {
	key := cache.KeyGlobalTeamLeaderboard

	// 从Redis获取分数
	score, err := s.redisClient.ZScore(ctx, key, teamID).Result()
	if err != nil {
		if err == redis.Nil {
			// 团队不在排行榜中，返回0分
			return 0, nil
		}
		return 0, fmt.Errorf("获取团队分数失败: %w", err)
	}

	return score, nil
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

// SyncUserScoreFromDB 从数据库同步用户分数到Redis
func (s *LeaderboardServiceImpl) SyncUserScoreFromDB(userID string) error {
	// 保留提交后增量同步能力：仅更新单个用户分数到 Redis，避免全量重算。
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}

	return s.UpdateGlobalUserScore(user.ID, float64(user.Score))
}

// SyncTeamScoreFromDB 从数据库同步团队分数到Redis
func (s *LeaderboardServiceImpl) SyncTeamScoreFromDB(teamID string) error {
	// 保留提交后增量同步能力：仅更新单个团队分数到 Redis，避免全量重算。
	team, err := s.teamRepo.GetTeamByID(teamID)
	if err != nil {
		return fmt.Errorf("获取团队信息失败: %w", err)
	}

	return s.UpdateGlobalTeamScore(team.ID, float64(team.Score))
}

// SyncAllScoresFromDB 从数据库同步所有用户和团队分数到Redis
func (s *LeaderboardServiceImpl) SyncAllScoresFromDB() error {
	if err := s.RefreshGlobalLeaderboardSnapshot(); err != nil {
		return err
	}

	// 获取所有用户
	users, err := s.userRepo.GetAllUsers()
	if err != nil {
		return fmt.Errorf("获取用户列表失败: %w", err)
	}

	// 同步所有用户分数
	for _, user := range users {
		err := s.UpdateGlobalUserScore(user.ID, float64(user.Score))
		if err != nil {
			// 记录错误但继续执行
			fmt.Printf("同步用户分数失败 (用户ID: %s): %v\n", user.ID, err)
		}
	}

	// 获取所有团队
	teams, err := s.teamRepo.GetAllTeams()
	if err != nil {
		return fmt.Errorf("获取团队列表失败: %w", err)
	}

	// 同步所有团队分数
	for _, team := range teams {
		err := s.UpdateGlobalTeamScore(team.ID, float64(team.Score))
		if err != nil {
			// 记录错误但继续执行
			fmt.Printf("同步团队分数失败 (团队ID: %s): %v\n", team.ID, err)
		}
	}

	return nil
}
