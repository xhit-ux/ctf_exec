package authsession

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ctf_exec/backend/internal/cache"
	"github.com/redis/go-redis/v9"
)

// ErrWSConnectionLimitExceeded 表示用户已达到 WS 连接数上限。
var ErrWSConnectionLimitExceeded = errors.New("ws connection limit exceeded")

// setUserCurrentSessionScript 原子地将 user_current 指针指向新 jti，并返回旧 jti（不存在则空串）。
//
// KEYS[1] = auth:user_current:<uid>
// ARGV[1] = newJTI
// ARGV[2] = TTL seconds
//
// 单 KEY 操作，集群兼容。
var setUserCurrentSessionScript = redis.NewScript(`
local old = redis.call('GET', KEYS[1])
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
if old then return old else return '' end
`)

// SetUserCurrentSession 原子地把用户当前会话指针置换为 newJTI，返回被替换的旧 jti。
// 若返回的 oldJTI 与 newJTI 相同，说明刚刚由本次调用写入，调用方无需撤销。
func (s *Store) SetUserCurrentSession(ctx context.Context, userID, newJTI string, ttl time.Duration) (string, error) {
	if s == nil || s.redisClient == nil {
		return "", fmt.Errorf("session store unavailable")
	}
	if userID == "" || newJTI == "" {
		return "", fmt.Errorf("invalid args for SetUserCurrentSession")
	}
	if ttl <= 0 {
		return "", fmt.Errorf("invalid ttl for SetUserCurrentSession")
	}

	key := cache.GetUserCurrentSessionKey(userID)
	res, err := setUserCurrentSessionScript.Run(ctx, s.redisClient, []string{key}, newJTI, int(ttl.Seconds())).Result()
	if err != nil {
		return "", fmt.Errorf("set user current session failed: %w", err)
	}

	old, ok := res.(string)
	if !ok {
		return "", nil
	}
	return old, nil
}

// IsUserCurrentSession 校验给定 jti 是否为该用户的当前有效会话。
//
// 当指针 key 不存在（未启用单会话或已过期）时返回 (true, nil)，调用方按「不强制单会话」处理。
// 由中间件结合角色策略决定是否调用此方法。
func (s *Store) IsUserCurrentSession(ctx context.Context, userID, jti string) (bool, error) {
	if s == nil || s.redisClient == nil {
		return false, fmt.Errorf("session store unavailable")
	}
	if userID == "" || jti == "" {
		return false, nil
	}

	key := cache.GetUserCurrentSessionKey(userID)
	stored, err := s.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		// 指针不存在：旧链路或已过期，不强制互踢。
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("query user current session failed: %w", err)
	}

	return stored == jti, nil
}

// ClearUserCurrentSession 清除用户当前会话指针（登出 / 撤销时使用）。
func (s *Store) ClearUserCurrentSession(ctx context.Context, userID string) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" {
		return nil
	}

	key := cache.GetUserCurrentSessionKey(userID)
	if err := s.redisClient.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("clear user current session failed: %w", err)
	}
	return nil
}

// SetUserOnline 刷新用户在线状态 TTL（无则创建）。
func (s *Store) SetUserOnline(ctx context.Context, userID string, ttl time.Duration) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" {
		return nil
	}
	if ttl <= 0 {
		return fmt.Errorf("invalid ttl for SetUserOnline")
	}

	key := cache.GetUserOnlineKey(userID)
	if err := s.redisClient.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("set user online failed: %w", err)
	}
	return nil
}

// IsUserOnline 查询用户是否在线（在线 key 仍未过期）。
func (s *Store) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	if s == nil || s.redisClient == nil {
		return false, fmt.Errorf("session store unavailable")
	}
	if userID == "" {
		return false, nil
	}

	key := cache.GetUserOnlineKey(userID)
	n, err := s.redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("query user online failed: %w", err)
	}
	return n > 0, nil
}

// GetOnlineUserIDs 批量查询用户在线状态。使用 Pipeline 一次往返完成。
func (s *Store) GetOnlineUserIDs(ctx context.Context, userIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(userIDs))
	if s == nil || s.redisClient == nil {
		return result, fmt.Errorf("session store unavailable")
	}
	if len(userIDs) == 0 {
		return result, nil
	}

	pipe := s.redisClient.Pipeline()
	cmds := make([]*redis.IntCmd, len(userIDs))
	for i, uid := range userIDs {
		if uid == "" {
			cmds[i] = nil
			continue
		}
		cmds[i] = pipe.Exists(ctx, cache.GetUserOnlineKey(uid))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return result, fmt.Errorf("batch query online failed: %w", err)
	}
	for i, uid := range userIDs {
		if cmds[i] == nil {
			continue
		}
		if cmds[i].Val() > 0 {
			result[uid] = true
		} else {
			result[uid] = false
		}
	}
	return result, nil
}

// registerWSConnectionScript 检查用户 WS 连接数上限并原子注册新连接。
//
// KEYS[1] = auth:ws:user_conns:<uid>
// KEYS[2] = auth:ws:conn:<connID>
// KEYS[3] = auth:online:<uid>
// ARGV[1] = connID
// ARGV[2] = userID
// ARGV[3] = maxConns
// ARGV[4] = ttl seconds (presence + meta)
// ARGV[5] = nowMs
//
// 多 KEY 操作但所有 key 都通过同一 user_id 派生（仅 conn meta 例外）；本系统当前为
// 单实例 Redis，集群部署时需将连接元数据 key 与索引 key 合并为同槽（可用 hashtag）。
//
// 返回：1 = 注册成功；0 = 已超限。
var registerWSConnectionScript = redis.NewScript(`
local count = redis.call('SCARD', KEYS[1])
if count >= tonumber(ARGV[3]) then
    return 0
end
redis.call('SADD', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[1], ARGV[4])
redis.call('HSET', KEYS[2], 'user_id', ARGV[2], 'connected_at_ms', ARGV[5], 'last_heartbeat_ms', ARGV[5])
redis.call('EXPIRE', KEYS[2], ARGV[4])
redis.call('SET', KEYS[3], '1', 'EX', ARGV[4])
return 1
`)

// RegisterWSConnection 注册一个 WS 连接。超过 maxConns 时返回 ErrWSConnectionLimitExceeded。
func (s *Store) RegisterWSConnection(ctx context.Context, userID, connID string, maxConns int, ttl time.Duration) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" || connID == "" {
		return fmt.Errorf("invalid args for RegisterWSConnection")
	}
	if maxConns <= 0 {
		maxConns = 1
	}
	if ttl <= 0 {
		return fmt.Errorf("invalid ttl for RegisterWSConnection")
	}

	keys := []string{
		cache.GetWSUserConnectionsKey(userID),
		cache.GetWSConnectionMetaKey(connID),
		cache.GetUserOnlineKey(userID),
	}
	args := []any{
		connID,
		userID,
		maxConns,
		int(ttl.Seconds()),
		strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	res, err := registerWSConnectionScript.Run(ctx, s.redisClient, keys, args...).Int()
	if err != nil {
		return fmt.Errorf("register ws connection failed: %w", err)
	}
	if res == 0 {
		return ErrWSConnectionLimitExceeded
	}
	return nil
}

// UnregisterWSConnection 注销一个 WS 连接。
//
// 注意：用户在线 key 不在此处主动 DEL，而是依赖自然过期，避免「最后一个连接断开瞬间，
// 同一时刻另一连接尚未上报心跳」造成的 presence 抖动。
func (s *Store) UnregisterWSConnection(ctx context.Context, userID, connID string) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" || connID == "" {
		return nil
	}

	pipe := s.redisClient.Pipeline()
	pipe.SRem(ctx, cache.GetWSUserConnectionsKey(userID), connID)
	pipe.Del(ctx, cache.GetWSConnectionMetaKey(connID))
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return fmt.Errorf("unregister ws connection failed: %w", err)
	}
	return nil
}

// RefreshWSHeartbeat 刷新连接最后心跳时间与 TTL，并续期用户在线状态。
func (s *Store) RefreshWSHeartbeat(ctx context.Context, userID, connID string, ttl time.Duration) error {
	if s == nil || s.redisClient == nil {
		return fmt.Errorf("session store unavailable")
	}
	if userID == "" || connID == "" {
		return nil
	}
	if ttl <= 0 {
		return fmt.Errorf("invalid ttl for RefreshWSHeartbeat")
	}

	pipe := s.redisClient.Pipeline()
	pipe.HSet(ctx, cache.GetWSConnectionMetaKey(connID), "last_heartbeat_ms", time.Now().UnixMilli())
	pipe.Expire(ctx, cache.GetWSConnectionMetaKey(connID), ttl)
	pipe.Expire(ctx, cache.GetWSUserConnectionsKey(userID), ttl)
	pipe.Set(ctx, cache.GetUserOnlineKey(userID), "1", ttl)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return fmt.Errorf("refresh ws heartbeat failed: %w", err)
	}
	return nil
}

// CountActiveSessions 返回用户当前活跃会话数（剔除已过期的 jti 残留）。
//
// 实现：先列出 set 内全部 jti，再用 Pipeline 检查每个 jti 对应的 session key 是否存在；
// 不存在的 jti 顺手从 set 中清理，避免长期残留导致 SCARD 失真。
func (s *Store) CountActiveSessions(ctx context.Context, userID string) (int, error) {
	if s == nil || s.redisClient == nil {
		return 0, fmt.Errorf("session store unavailable")
	}
	if userID == "" {
		return 0, nil
	}

	setKey := cache.GetJWTUserSessionSetKey(userID)
	jtis, err := s.redisClient.SMembers(ctx, setKey).Result()
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("list user jwt sessions failed: %w", err)
	}
	if len(jtis) == 0 {
		return 0, nil
	}

	pipe := s.redisClient.Pipeline()
	exists := make([]*redis.IntCmd, len(jtis))
	for i, jti := range jtis {
		exists[i] = pipe.Exists(ctx, cache.GetJWTSessionByJTIKey(jti))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return 0, fmt.Errorf("check jwt session existence failed: %w", err)
	}

	stale := make([]any, 0)
	active := 0
	for i, cmd := range exists {
		if cmd.Val() > 0 {
			active++
		} else {
			stale = append(stale, jtis[i])
		}
	}
	if len(stale) > 0 {
		_ = s.redisClient.SRem(ctx, setKey, stale...).Err()
	}
	return active, nil
}

// EvictOldestSession 驱逐用户最旧的活跃会话（按 refresh_tokens.created_at 排序），返回被驱逐的 jti。
//
// 用于 multi_session 上限治理：发现活跃会话数 >= 上限时，先淘汰最旧的，再放新会话进来。
func (s *Store) EvictOldestSession(ctx context.Context, userID string) (string, error) {
	if s == nil || s.redisClient == nil {
		return "", fmt.Errorf("session store unavailable")
	}
	if userID == "" {
		return "", nil
	}

	setKey := cache.GetJWTUserSessionSetKey(userID)
	jtis, err := s.redisClient.SMembers(ctx, setKey).Result()
	if err != nil && err != redis.Nil {
		return "", fmt.Errorf("list user jwt sessions failed: %w", err)
	}
	if len(jtis) == 0 {
		return "", nil
	}

	// 通过 refresh_tokens.created_at 找到最早的 jti（refresh_tokens.jti 与 access jti 1:1 对应）。
	// 即便 refresh 已 revoked，仍可作为「会话签发顺序」的依据。
	if s.db == nil {
		// 退化路径：随机选一个。
		victim := jtis[0]
		if revokeErr := s.RevokeSession(ctx, userID, victim); revokeErr != nil {
			return "", revokeErr
		}
		return victim, nil
	}

	placeholders := ""
	args := []any{userID}
	for i, jti := range jtis {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, jti)
	}

	query := `
		SELECT jti
		FROM refresh_tokens
		WHERE user_id = ? AND jti IN (` + placeholders + `)
		ORDER BY created_at ASC
		LIMIT 1
	`
	var victim string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&victim); err != nil {
		// 找不到时退化为随机选第一个。
		victim = jtis[0]
	}
	if victim == "" {
		victim = jtis[0]
	}
	if err := s.RevokeSession(ctx, userID, victim); err != nil {
		return "", err
	}
	// access 已撤销，关联的 refresh 也一并失效。
	_, _ = s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ? AND jti = ? AND revoked = 0`, userID, victim)
	return victim, nil
}
