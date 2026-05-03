package authsession

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Store 提供 JWT 会话创建、校验与撤销能力。
type Store struct {
	redisClient *redis.Client
	db          *sql.DB
}

// NewStore 创建会话存储。
func NewStore(redisClient *redis.Client, db *sql.DB) *Store {
	return &Store{redisClient: redisClient, db: db}
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

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateRefreshSession 注册 refresh token 会话到持久化存储。
func (s *Store) CreateRefreshSession(ctx context.Context, claims *utils.Claims, rawToken, clientIP, userAgent string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session store unavailable")
	}
	if claims == nil || claims.UserID == "" || claims.ID == "" || claims.ExpiresAt == nil || rawToken == "" {
		return fmt.Errorf("invalid claims for refresh session creation")
	}

	expiresAt := claims.ExpiresAt.Time
	if time.Until(expiresAt) <= 0 {
		return fmt.Errorf("refresh token already expired")
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, jti, expires_at, revoked, ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, NOW())
	`
	if _, err := s.db.ExecContext(
		ctx,
		query,
		uuid.New().String(),
		claims.UserID,
		hashRefreshToken(rawToken),
		claims.ID,
		expiresAt,
		normalizeIP(clientIP),
		truncateUserAgent(userAgent),
	); err != nil {
		return fmt.Errorf("create refresh session failed: %w", err)
	}

	return nil
}

func normalizeIP(raw string) string {
	ip := strings.TrimSpace(raw)
	if ip == "" {
		return ""
	}
	if len(ip) > 45 {
		return ip[:45]
	}
	return ip
}

func truncateUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	if len(ua) > 512 {
		return ua[:512]
	}
	return ua
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

// IsRefreshSessionActive 校验 refresh 会话是否仍有效且归属于当前用户。
func (s *Store) IsRefreshSessionActive(ctx context.Context, rawToken, userID, jti string) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("session store unavailable")
	}
	if rawToken == "" || userID == "" || jti == "" {
		return false, nil
	}

	var count int
	query := `
		SELECT COUNT(1)
		FROM refresh_tokens
		WHERE user_id = ? AND jti = ? AND token_hash = ? AND revoked = 0 AND expires_at > NOW()
	`
	if err := s.db.QueryRowContext(ctx, query, userID, jti, hashRefreshToken(rawToken)).Scan(&count); err != nil {
		return false, fmt.Errorf("query refresh session failed: %w", err)
	}

	return count > 0, nil
}

// RevokeRefreshSession 撤销单个 refresh 会话。
func (s *Store) RevokeRefreshSession(ctx context.Context, rawToken, userID, jti string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("session store unavailable")
	}
	if rawToken == "" || userID == "" || jti == "" {
		return nil
	}

	query := `
		UPDATE refresh_tokens
		SET revoked = 1
		WHERE user_id = ? AND jti = ? AND token_hash = ? AND revoked = 0
	`
	if _, err := s.db.ExecContext(ctx, query, userID, jti, hashRefreshToken(rawToken)); err != nil {
		return fmt.Errorf("revoke refresh session failed: %w", err)
	}

	return nil
}

// CleanupExpiredRefreshSessions 清理过期超过 retention 的 refresh token 记录。
func (s *Store) CleanupExpiredRefreshSessions(ctx context.Context, retention time.Duration) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("session store unavailable")
	}
	if retention < 0 {
		retention = 0
	}

	query := `
		DELETE FROM refresh_tokens
		WHERE expires_at < DATE_SUB(NOW(), INTERVAL ? SECOND)
	`
	result, err := s.db.ExecContext(ctx, query, int(retention.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("cleanup expired refresh sessions failed: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read cleanup affected rows failed: %w", err)
	}

	return affected, nil
}

// StartRefreshTokenCleanupScheduler 启动 refresh token 清理任务。
func (s *Store) StartRefreshTokenCleanupScheduler(interval, retention time.Duration) {
	if s == nil || s.db == nil {
		log.Println("[refresh-cleanup] session store unavailable, scheduler skipped")
		return
	}
	if interval <= 0 {
		interval = 24 * time.Hour // 默认每天执行一次
	}

	go func() {
		ctx := context.Background()
		deleted, err := s.CleanupExpiredRefreshSessions(ctx, retention)
		if err != nil {
			log.Printf("[refresh-cleanup] initial cleanup failed: %v", err)
		} else {
			log.Printf("[refresh-cleanup] initial cleanup completed, deleted=%d", deleted)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			deleted, runErr := s.CleanupExpiredRefreshSessions(context.Background(), retention)
			if runErr != nil {
				log.Printf("[refresh-cleanup] scheduled cleanup failed: %v", runErr)
				continue
			}
			if deleted > 0 {
				log.Printf("[refresh-cleanup] scheduled cleanup completed, deleted=%d", deleted)
			}
		}
	}()
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

	pipe := s.redisClient.TxPipeline()
	for _, jti := range jtis {
		if jti == "" {
			continue
		}
		pipe.Del(ctx, cache.GetJWTSessionByJTIKey(jti))
	}
	pipe.Del(ctx, userSetKey)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("revoke all jwt sessions failed: %w", err)
	}

	if s.db != nil {
		if _, err := s.db.ExecContext(ctx, `
			UPDATE refresh_tokens
			SET revoked = 1
			WHERE user_id = ? AND revoked = 0
		`, userID); err != nil {
			return fmt.Errorf("revoke all refresh sessions failed: %w", err)
		}
	}

	return nil
}
