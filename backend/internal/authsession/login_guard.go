package authsession

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// ErrLoginThrottled 表示当前登录请求已被防暴力破解策略拦截。
//
// 调用方应当将其转译成 429 Too Many Requests 并附 Retry-After。
var ErrLoginThrottled = errors.New("login throttled")

// LoginGuard 提供登录暴力破解防护：双维度（账户 + IP）失败计数与自动锁定。
//
// 设计：
//   - 失败计数与锁定状态都落在 Redis，TTL 自然过期；
//   - 不区分「账户不存在」与「密码错误」，对外消息保持一致，避免账户枚举；
//   - 锁定触发的同时清空对应的失败计数，避免锁定期外又立刻命中阈值。
type LoginGuard struct {
	redisClient *redis.Client
	cfg         config.LoginGuardConfig
}

// NewLoginGuard 构造 LoginGuard。redisClient 必传；cfg 字段为 0 时按默认值兜底。
func NewLoginGuard(redisClient *redis.Client, cfg config.LoginGuardConfig) *LoginGuard {
	return &LoginGuard{redisClient: redisClient, cfg: cfg}
}

// EnsureNotLocked 在执行密码校验前调用：如已锁定则返回 429 错误；ip 与 userID 至少一方不为空。
//
// userID 为空时仅做 IP 校验（用于尚未解析到具体账户的早期阶段）。
func (g *LoginGuard) EnsureNotLocked(ctx context.Context, clientIP, userID string) error {
	if g == nil || g.redisClient == nil {
		return nil
	}

	if normalizedIP := normalizeIP(clientIP); normalizedIP != "" {
		ttl, err := g.redisClient.TTL(ctx, cache.GetLoginLockIPKey(normalizedIP)).Result()
		if err != nil && err != redis.Nil {
			return fmt.Errorf("query login ip lock failed: %w", err)
		}
		if ttl > 0 {
			return apperr.NewWithRetryAfter(
				apperr.ErrTooManyRequests,
				fmt.Sprintf("登录失败次数过多，请 %s 后重试", humanizeDuration(ttl)),
				int(ttl.Seconds()),
			)
		}
	}

	if userID = strings.TrimSpace(userID); userID != "" {
		ttl, err := g.redisClient.TTL(ctx, cache.GetLoginLockAccountKey(userID)).Result()
		if err != nil && err != redis.Nil {
			return fmt.Errorf("query login account lock failed: %w", err)
		}
		if ttl > 0 {
			return apperr.NewWithRetryAfter(
				apperr.ErrTooManyRequests,
				fmt.Sprintf("账户登录失败次数过多，已临时锁定，请 %s 后重试", humanizeDuration(ttl)),
				int(ttl.Seconds()),
			)
		}
	}

	return nil
}

// RecordFailure 记录一次登录失败：累加 IP / 账户失败计数，并在达到阈值时设置锁定标记。
//
// userID 为空表示「未知账户」（如用户名不存在），此时仅累加 IP 维度计数。
// 返回的 lockedAccount / lockedIP 表示是否在本次调用中触发锁定。
func (g *LoginGuard) RecordFailure(ctx context.Context, clientIP, userID string) (lockedAccount bool, lockedIP bool) {
	if g == nil || g.redisClient == nil {
		return false, false
	}

	if normalizedIP := normalizeIP(clientIP); normalizedIP != "" {
		count, err := g.incrWithExpire(ctx, cache.GetLoginFailIPKey(normalizedIP), g.cfg.IPFailWindow)
		if err == nil && g.cfg.IPFailLimit > 0 && int(count) >= g.cfg.IPFailLimit {
			_ = g.redisClient.Set(ctx, cache.GetLoginLockIPKey(normalizedIP), "1", g.cfg.IPLockDuration).Err()
			_ = g.redisClient.Del(ctx, cache.GetLoginFailIPKey(normalizedIP)).Err()
			lockedIP = true
		}
	}

	if userID = strings.TrimSpace(userID); userID != "" {
		count, err := g.incrWithExpire(ctx, cache.GetLoginFailAccountKey(userID), g.cfg.AccountFailWindow)
		if err == nil && g.cfg.AccountFailLimit > 0 && int(count) >= g.cfg.AccountFailLimit {
			_ = g.redisClient.Set(ctx, cache.GetLoginLockAccountKey(userID), "1", g.cfg.AccountLockDuration).Err()
			_ = g.redisClient.Del(ctx, cache.GetLoginFailAccountKey(userID)).Err()
			lockedAccount = true
		}
	}

	return lockedAccount, lockedIP
}

// AccountLockTTL 返回账户锁定剩余时间，未锁定返回 0。
func (g *LoginGuard) AccountLockTTL(ctx context.Context, userID string) time.Duration {
	if g == nil || g.redisClient == nil || strings.TrimSpace(userID) == "" {
		return 0
	}
	ttl, err := g.redisClient.TTL(ctx, cache.GetLoginLockAccountKey(userID)).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return ttl
}

// IPLockTTL 返回 IP 锁定剩余时间，未锁定返回 0。
func (g *LoginGuard) IPLockTTL(ctx context.Context, clientIP string) time.Duration {
	if g == nil || g.redisClient == nil {
		return 0
	}
	ip := normalizeIP(clientIP)
	if ip == "" {
		return 0
	}
	ttl, err := g.redisClient.TTL(ctx, cache.GetLoginLockIPKey(ip)).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return ttl
}

// RecordSuccess 在登录成功后清除该账户与 IP 的失败计数（锁定标记不动，避免越权解锁）。
func (g *LoginGuard) RecordSuccess(ctx context.Context, clientIP, userID string) {
	if g == nil || g.redisClient == nil {
		return
	}
	if userID = strings.TrimSpace(userID); userID != "" {
		_ = g.redisClient.Del(ctx, cache.GetLoginFailAccountKey(userID)).Err()
	}
	if ip := normalizeIP(clientIP); ip != "" {
		_ = g.redisClient.Del(ctx, cache.GetLoginFailIPKey(ip)).Err()
	}
}

// incrWithExpire 原子地 INCR 计数并在第一次写入时设置 TTL。
func (g *LoginGuard) incrWithExpire(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	count, err := g.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("incr login fail counter failed: %w", err)
	}
	if count == 1 && ttl > 0 {
		_ = g.redisClient.Expire(ctx, key, ttl).Err()
	}
	return count, nil
}

// humanizeDuration 将剩余时间转换为「X 分钟」/「Y 秒」可读文案。
func humanizeDuration(d time.Duration) string {
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs <= 0 {
			secs = 1
		}
		return fmt.Sprintf("%d 秒", secs)
	}
	mins := int(d.Minutes())
	if mins <= 0 {
		mins = 1
	}
	return fmt.Sprintf("%d 分钟", mins)
}
