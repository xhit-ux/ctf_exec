package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/ctf_exec/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// RedisClient 包装redis.Client提供应用特定的方法
type RedisClient struct {
	*redis.Client
}

// InitRedis 初始化Redis连接
func InitRedis() (*RedisClient, error) {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	redisCfg := cfg.Redis

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisCfg.Host, redisCfg.Port),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接测试失败: %w", err)
	}

	return &RedisClient{Client: client}, nil
}

// HealthCheck Redis健康检查
func (rc *RedisClient) HealthCheck(ctx context.Context) error {
	return rc.Client.Ping(ctx).Err()
}

// Close 关闭Redis连接
func (rc *RedisClient) Close() error {
	return rc.Client.Close()
}