package authsession

import (
	"context"
	"fmt"
	"time"

	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/redis/go-redis/v9"
)

// Store 提供 JWT 会话创建、校验与撤销能力。
type Store struct {
	redisClient *redis.Client
}

// NewStore 创建会话存储。
func NewStore(redisClient *redis.Client) *Store {
	return &Store{redisClient: redisClient}
}

// CreateSession 注册新 JWT 会话，生命周期与 token 过期时间一致。
func (s *Store) CreateSession(ctx context.Context, claims *utils.Claims) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if claims == nil || claims.UserID == "" || claims.ID == "" || claims.ExpiresAt == nil {
		return fmt.Errorf("invalid claims for session creation")
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return fmt.Errorf("token already expired")
	}

	sessionKey := cache.GetJWTSessionByJTIKey(claims.ID)
	userSetKey := cache.GetJWTUserSessionSetKey(claims.UserID)

	pipe := s.redisClient.TxPipeline()
	pipe.Set(ctx, sessionKey, claims.UserID, ttl)
	pipe.SAdd(ctx, userSetKey, claims.ID)
	pipe.Expire(ctx, userSetKey, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create jwt session failed: %w", err)
	}

	return nil
}

// CreateRefreshSession 注册 refresh token 会话。
func (s *Store) CreateRefreshSession(ctx context.Context, claims *utils.Claims) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if claims == nil || claims.UserID == "" || claims.ID == "" || claims.ExpiresAt == nil {
		return fmt.Errorf("invalid claims for refresh session creation")
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return fmt.Errorf("refresh token already expired")
	}

	sessionKey := cache.GetJWTRefreshSessionByJTIKey(claims.ID)
	userSetKey := cache.GetJWTRefreshUserSessionSetKey(claims.UserID)

	pipe := s.redisClient.TxPipeline()
	pipe.Set(ctx, sessionKey, claims.UserID, ttl)
	pipe.SAdd(ctx, userSetKey, claims.ID)
	pipe.Expire(ctx, userSetKey, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create refresh session failed: %w", err)
	}

	return nil
}

// IsSessionActive 校验 jti 会话是否仍有效且归属于当前用户。
func (s *Store) IsSessionActive(ctx context.Context, userID, jti string) (bool, error) {
	if s == nil || s.redisClient == nil {
		return false, fmt.Errorf("session store unavailable")
	}
	if userID == "" || jti == "" {
		return false, nil
	}

	sessionKey := cache.GetJWTSessionByJTIKey(jti)
	storedUserID, err := s.redisClient.Get(ctx, sessionKey).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query jwt session failed: %w", err)
	}

	return storedUserID == userID, nil
}

// RevokeSession 撤销单个 jti 会话。
func (s *Store) RevokeSession(ctx context.Context, userID, jti string) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" || jti == "" {
		return nil
	}

	sessionKey := cache.GetJWTSessionByJTIKey(jti)
	userSetKey := cache.GetJWTUserSessionSetKey(userID)

	pipe := s.redisClient.TxPipeline()
	pipe.Del(ctx, sessionKey)
	pipe.SRem(ctx, userSetKey, jti)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke jwt session failed: %w", err)
	}

	return nil
}

// IsRefreshSessionActive 校验 refresh jti 是否仍有效且归属于当前用户。
func (s *Store) IsRefreshSessionActive(ctx context.Context, userID, jti string) (bool, error) {
	if s == nil || s.redisClient == nil {
		return false, fmt.Errorf("session store unavailable")
	}
	if userID == "" || jti == "" {
		return false, nil
	}

	sessionKey := cache.GetJWTRefreshSessionByJTIKey(jti)
	storedUserID, err := s.redisClient.Get(ctx, sessionKey).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query refresh session failed: %w", err)
	}

	return storedUserID == userID, nil
}

// RevokeRefreshSession 撤销单个 refresh 会话。
func (s *Store) RevokeRefreshSession(ctx context.Context, userID, jti string) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" || jti == "" {
		return nil
	}

	sessionKey := cache.GetJWTRefreshSessionByJTIKey(jti)
	userSetKey := cache.GetJWTRefreshUserSessionSetKey(userID)

	pipe := s.redisClient.TxPipeline()
	pipe.Del(ctx, sessionKey)
	pipe.SRem(ctx, userSetKey, jti)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke refresh session failed: %w", err)
	}

	return nil
}

// RevokeAllUserSessions 撤销用户全部会话，常用于封禁、角色变更等高风险操作。
func (s *Store) RevokeAllUserSessions(ctx context.Context, userID string) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" {
		return nil
	}

	userSetKey := cache.GetJWTUserSessionSetKey(userID)
	jtis, err := s.redisClient.SMembers(ctx, userSetKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("list user jwt sessions failed: %w", err)
	}

	refreshUserSetKey := cache.GetJWTRefreshUserSessionSetKey(userID)
	refreshJTIs, err := s.redisClient.SMembers(ctx, refreshUserSetKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("list user refresh sessions failed: %w", err)
	}

	pipe := s.redisClient.TxPipeline()
	for _, jti := range jtis {
		if jti == "" {
			continue
		}
		pipe.Del(ctx, cache.GetJWTSessionByJTIKey(jti))
	}
	for _, jti := range refreshJTIs {
		if jti == "" {
			continue
		}
		pipe.Del(ctx, cache.GetJWTRefreshSessionByJTIKey(jti))
	}
	pipe.Del(ctx, userSetKey)
	pipe.Del(ctx, refreshUserSetKey)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke all jwt sessions failed: %w", err)
	}

	return nil
}
