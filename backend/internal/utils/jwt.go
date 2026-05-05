package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	jwtCfgMu sync.RWMutex
	jwtCfg   *config.JWTConfig
)

// Claims JWT 声明
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"`
	// RememberMe 仅在 refresh token 上有意义，表示该 refresh 是否走「记住我」的长 TTL。
	// 用于 refresh 轮换时保持 remember_me 状态，无需依赖 TTL 长度推断。
	RememberMe bool `json:"remember,omitempty"`
	jwt.RegisteredClaims
}

// InitJWTConfig 初始化 JWT 运行时配置缓存，应在服务启动时调用一次。
func InitJWTConfig(cfg config.JWTConfig) {
	jwtCfgMu.Lock()
	defer jwtCfgMu.Unlock()
	local := cfg
	jwtCfg = &local
}

func getJWTConfig() (*config.JWTConfig, error) {
	jwtCfgMu.RLock()
	defer jwtCfgMu.RUnlock()
	if jwtCfg == nil {
		return nil, fmt.Errorf("jwt 配置未初始化，请在 main 启动阶段调用 InitJWTConfig")
	}
	cfgCopy := *jwtCfg
	return &cfgCopy, nil
}

func generateToken(userID, username, role, tokenType string, expiration time.Duration, rememberMe bool) (string, *Claims, error) {
	if tokenType != TokenTypeAccess && tokenType != TokenTypeRefresh {
		return "", nil, fmt.Errorf("invalid token type")
	}
	cfg, err := getJWTConfig()
	if err != nil {
		return "", nil, err
	}

	now := time.Now()
	expirationTime := now.Add(expiration)
	claims := &Claims{
		UserID:     userID,
		Username:   username,
		Role:       role,
		Type:       tokenType,
		RememberMe: rememberMe,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	return tokenString, claims, nil
}

// GenerateAccessToken 生成短时 access token。
func GenerateAccessToken(userID, username, role string) (string, *Claims, error) {
	cfg, err := getJWTConfig()
	if err != nil {
		return "", nil, err
	}
	return generateToken(userID, username, role, TokenTypeAccess, cfg.AccessExpiration, false)
}

// GenerateRefreshToken 生成默认 TTL 的 refresh token（不勾选 remember_me）。
func GenerateRefreshToken(userID, username, role string) (string, *Claims, error) {
	cfg, err := getJWTConfig()
	if err != nil {
		return "", nil, err
	}
	return generateToken(userID, username, role, TokenTypeRefresh, cfg.RefreshExpiration, false)
}

// GenerateRefreshTokenWithTTL 以指定 TTL 生成 refresh token；rememberMe 会被写入 claims 以便轮换时保持。
func GenerateRefreshTokenWithTTL(userID, username, role string, ttl time.Duration, rememberMe bool) (string, *Claims, error) {
	if ttl <= 0 {
		return "", nil, fmt.Errorf("invalid refresh token ttl")
	}
	return generateToken(userID, username, role, TokenTypeRefresh, ttl, rememberMe)
}

// GenerateToken 兼容旧调用，等同于 GenerateAccessToken。
func GenerateToken(userID, username, role string) (string, *Claims, error) {
	return GenerateAccessToken(userID, username, role)
}

// VerifyToken 验证 JWT token。
func VerifyToken(tokenString string) (*Claims, error) {
	cfg, err := getJWTConfig()
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("token 验证失败: %w", err)
	}

	if !token.Valid {
		return nil, apperr.New(apperr.ErrUnauthorized, "token 无效")
	}

	return claims, nil
}

// VerifyTokenByType 验证 token 且要求类型匹配。
func VerifyTokenByType(tokenString, expectedType string) (*Claims, error) {
	claims, err := VerifyToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Type != expectedType {
		return nil, apperr.New(apperr.ErrUnauthorized, "token 类型不匹配")
	}

	return claims, nil
}
