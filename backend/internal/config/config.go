package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config 全局配置结构体
type Config struct {
	Database    DatabaseConfig
	Redis       RedisConfig
	Server      ServerConfig
	JWT         JWTConfig
	Upload      UploadConfig
	RateLimit   RateLimitConfig
	Leaderboard LeaderboardConfig
	Email       EmailConfig
	Bot         BotProtectionConfig
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	Charset         string
	ParseTime       bool
	Local           string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string
	Host string
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// UploadConfig 上传配置
type UploadConfig struct {
	StaticPath    string
	MaxUploadSize int64
}

// RateLimitConfig 提交限流配置
type RateLimitConfig struct {
	SubmissionLimit int
	WindowSeconds   int
}

// LeaderboardConfig 排行榜配置
type LeaderboardConfig struct {
	CacheTTLSeconds int
}

// EmailConfig 邮件与邮箱验证码配置
type EmailConfig struct {
	ResendAPIKey                 string
	ResendFromEmail              string
	VerificationCodeTTLSeconds   int
	VerificationSendCooldownSecs int
	VerificationDailyLimit       int
}

// BotProtectionConfig 机器人验证配置（Turnstile）
type BotProtectionConfig struct {
	Enabled            bool
	TurnstileSecretKey string
	TurnstileVerifyURL string
}

// LoadConfig 从环境变量加载配置
func LoadConfig() (*Config, error) {
	// 尝试从多个位置加载 .env 文件
	// 1. 当前目录
	// 2. 项目根目录 (../)
	// 3. 用户主目录
	envPaths := []string{".env", "../.env"}
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	config := &Config{}

	// 加载数据库配置
	dbConfig, err := loadDatabaseConfig()
	if err != nil {
		return nil, fmt.Errorf("加载数据库配置失败: %w", err)
	}
	config.Database = *dbConfig

	// 加载Redis配置
	redisConfig, err := loadRedisConfig()
	if err != nil {
		return nil, fmt.Errorf("加载Redis配置失败: %w", err)
	}
	config.Redis = *redisConfig

	// 加载服务器配置
	serverConfig, err := loadServerConfig()
	if err != nil {
		return nil, fmt.Errorf("加载服务器配置失败: %w", err)
	}
	config.Server = *serverConfig

	// 加载JWT配置
	jwtConfig, err := loadJWTConfig()
	if err != nil {
		return nil, fmt.Errorf("加载JWT配置失败: %w", err)
	}
	config.JWT = *jwtConfig

	// 加载上传配置
	uploadConfig, err := loadUploadConfig()
	if err != nil {
		return nil, fmt.Errorf("加载上传配置失败: %w", err)
	}
	config.Upload = *uploadConfig

	// 加载限流配置
	rateLimitConfig, err := loadRateLimitConfig()
	if err != nil {
		return nil, fmt.Errorf("加载限流配置失败: %w", err)
	}
	config.RateLimit = *rateLimitConfig

	// 加载排行榜配置
	leaderboardConfig, err := loadLeaderboardConfig()
	if err != nil {
		return nil, fmt.Errorf("加载排行榜配置失败: %w", err)
	}
	config.Leaderboard = *leaderboardConfig

	// 加载邮件配置
	emailConfig, err := loadEmailConfig()
	if err != nil {
		return nil, fmt.Errorf("加载邮件配置失败: %w", err)
	}
	config.Email = *emailConfig

	// 加载机器人验证配置
	botConfig, err := loadBotProtectionConfig()
	if err != nil {
		return nil, fmt.Errorf("加载机器人验证配置失败: %w", err)
	}
	config.Bot = *botConfig

	return config, nil
}

// loadDatabaseConfig 加载数据库配置
func loadDatabaseConfig() (*DatabaseConfig, error) {
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnvInt("DB_PORT", 3306)
	user := getEnv("DB_USER", "root")
	password := getEnv("DB_PASSWORD", "")
	name := getEnv("DB_NAME", "ctf_exec")
	charset := getEnv("DB_CHARSET", "utf8mb4")
	parseTime := getEnvBool("DB_PARSE_TIME", true)
	local := getEnv("DB_LOCAL", "Local")
	maxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 25)
	maxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 10)
	connMaxLifetimeHours := getEnvInt("DB_CONN_MAX_LIFETIME_HOURS", 1)
	connMaxIdleTimeMinutes := getEnvInt("DB_CONN_MAX_IDLE_TIME_MINUTES", 10)

	return &DatabaseConfig{
		Host:            host,
		Port:            port,
		User:            user,
		Password:        password,
		Name:            name,
		Charset:         charset,
		ParseTime:       parseTime,
		Local:           local,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: time.Duration(connMaxLifetimeHours) * time.Hour,
		ConnMaxIdleTime: time.Duration(connMaxIdleTimeMinutes) * time.Minute,
	}, nil
}

// loadRedisConfig 加载Redis配置
func loadRedisConfig() (*RedisConfig, error) {
	host := getEnv("REDIS_HOST", "127.0.0.1")
	port := getEnvInt("REDIS_PORT", 6379)
	password := getEnv("REDIS_PASSWORD", "")
	db := getEnvInt("REDIS_DB", 0)

	return &RedisConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
	}, nil
}

// loadServerConfig 加载服务器配置
func loadServerConfig() (*ServerConfig, error) {
	port := getEnv("SERVER_PORT", "8080")
	host := getEnv("SERVER_HOST", "0.0.0.0")

	return &ServerConfig{
		Port: port,
		Host: host,
	}, nil
}

// loadJWTConfig 加载JWT配置
func loadJWTConfig() (*JWTConfig, error) {
	secret := getEnv("JWT_SECRET", "your-secret-key-change-this-in-production")
	expirationHours := getEnvInt("JWT_EXPIRATION_HOURS", 168) // 7天

	return &JWTConfig{
		Secret:     secret,
		Expiration: time.Duration(expirationHours) * time.Hour,
	}, nil
}

// loadUploadConfig 加载上传配置
func loadUploadConfig() (*UploadConfig, error) {
	staticPath := getEnv("STATIC_PATH", "../static")
	maxUploadSize := getEnvInt64("MAX_UPLOAD_SIZE", 10*1024*1024) // 10MB

	return &UploadConfig{
		StaticPath:    staticPath,
		MaxUploadSize: maxUploadSize,
	}, nil
}

// loadRateLimitConfig 加载限流配置
func loadRateLimitConfig() (*RateLimitConfig, error) {
	limit := getEnvInt("SUBMISSION_RATE_LIMIT", 10)
	windowSeconds := getEnvInt("SUBMISSION_RATE_WINDOW_SECONDS", 60)

	return &RateLimitConfig{
		SubmissionLimit: limit,
		WindowSeconds:   windowSeconds,
	}, nil
}

// loadLeaderboardConfig 加载排行榜配置
func loadLeaderboardConfig() (*LeaderboardConfig, error) {
	cacheTTL := getEnvInt("LEADERBOARD_CACHE_TTL_SECONDS", 30)

	return &LeaderboardConfig{
		CacheTTLSeconds: cacheTTL,
	}, nil
}

// loadEmailConfig 加载邮件配置
func loadEmailConfig() (*EmailConfig, error) {
	return &EmailConfig{
		ResendAPIKey:                 getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:              getEnv("RESEND_FROM_EMAIL", ""),
		VerificationCodeTTLSeconds:   getEnvInt("EMAIL_VERIFY_CODE_TTL_SECONDS", 300),
		VerificationSendCooldownSecs: getEnvInt("EMAIL_VERIFY_SEND_COOLDOWN_SECONDS", 60),
		VerificationDailyLimit:       getEnvInt("EMAIL_VERIFY_DAILY_LIMIT", 20),
	}, nil
}

// loadBotProtectionConfig 加载机器人验证配置
func loadBotProtectionConfig() (*BotProtectionConfig, error) {
	secretKey := getEnv("TURNSTILE_SECRET_KEY", "")
	enabled := getEnvBool("TURNSTILE_ENABLED", secretKey != "")

	return &BotProtectionConfig{
		Enabled:            enabled,
		TurnstileSecretKey: secretKey,
		TurnstileVerifyURL: getEnv("TURNSTILE_VERIFY_URL", "https://challenges.cloudflare.com/turnstile/v0/siteverify"),
	}, nil
}

// 辅助函数：获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
