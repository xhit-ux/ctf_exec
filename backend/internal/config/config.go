package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// Config 全局配置结构体
type Config struct {
	Database    DatabaseConfig
	Redis       RedisConfig
	Server      ServerConfig
	JWT         JWTConfig
	Init        InitConfig
	AuthCookie  AuthCookieConfig
	Upload      UploadConfig
	RateLimit   RateLimitConfig
	Leaderboard LeaderboardConfig
	Email       EmailConfig
	Bot         BotProtectionConfig
	Container   ContainerConfig
	MinIO       MinIOConfig
	Flag        FlagConfig
	Ingress     IngressConfig
}

// InitConfig 启动初始化配置。
type InitConfig struct {
	RootDefaultPassword string
}

// AuthCookieConfig 认证 Cookie 配置。
type AuthCookieConfig struct {
	AccessTokenName  string
	RefreshTokenName string
	Domain           string
	Secure           bool
	SameSite         string
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
	Port           string
	Host           string
	TrustedProxies []string
	AllowedOrigins []string
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret            string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
	// Expiration 保留兼容字段，等同于 AccessExpiration。
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

// ContainerConfig 容器实例与 k3s 运行配置。
type ContainerConfig struct {
	KubeconfigPath          string
	MaxConcurrentContainers int
	DefaultContainerTTL     int
	NamespacePrefix         string
	KataRuntimeClass        string
}

// MinIOConfig MinIO 对象存储配置。
type MinIOConfig struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	BucketPrefix  string
	PresignExpiry int
}

// FlagConfig 动态 flag 生成配置。
type FlagConfig struct {
	Template string
}

// IngressConfig Web 题域名与 TLS 配置。
type IngressConfig struct {
	DomainTemplate string
	Class          string
	TLSEnabled     bool
	TLSSecret      string
}

const DefaultJWTSecret = "your-secret-key-change-this-in-production"

var (
	loadConfigOnce sync.Once
	loadedConfig   *Config
	loadConfigErr  error
)

// LoadConfig 从环境变量加载配置
func LoadConfig() (*Config, error) {
	loadConfigOnce.Do(func() {
		// 尝试从多个位置加载 .env 文件
		// 1. 当前目录
		// 2. 项目根目录 (../)
		envPaths := []string{".env", "../.env"}
		for _, path := range envPaths {
			if err := godotenv.Load(path); err == nil {
				break
			}
		}

		cfg := &Config{}

		// 加载数据库配置
		dbConfig, err := loadDatabaseConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载数据库配置失败: %w", err)
			return
		}
		cfg.Database = *dbConfig

		// 加载Redis配置
		redisConfig, err := loadRedisConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载Redis配置失败: %w", err)
			return
		}
		cfg.Redis = *redisConfig

		// 加载服务器配置
		serverConfig, err := loadServerConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载服务器配置失败: %w", err)
			return
		}
		cfg.Server = *serverConfig

		// 加载JWT配置
		jwtConfig, err := loadJWTConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载JWT配置失败: %w", err)
			return
		}
		cfg.JWT = *jwtConfig

		initConfig, err := loadInitConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载初始化配置失败: %w", err)
			return
		}
		cfg.Init = *initConfig

		cookieConfig, err := loadAuthCookieConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载认证 Cookie 配置失败: %w", err)
			return
		}
		cfg.AuthCookie = *cookieConfig

		// 加载上传配置
		uploadConfig, err := loadUploadConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载上传配置失败: %w", err)
			return
		}
		cfg.Upload = *uploadConfig

		// 加载限流配置
		rateLimitConfig, err := loadRateLimitConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载限流配置失败: %w", err)
			return
		}
		cfg.RateLimit = *rateLimitConfig

		// 加载排行榜配置
		leaderboardConfig, err := loadLeaderboardConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载排行榜配置失败: %w", err)
			return
		}
		cfg.Leaderboard = *leaderboardConfig

		// 加载邮件配置
		emailConfig, err := loadEmailConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载邮件配置失败: %w", err)
			return
		}
		cfg.Email = *emailConfig

		// 加载机器人验证配置
		botConfig, err := loadBotProtectionConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载机器人验证配置失败: %w", err)
			return
		}
		cfg.Bot = *botConfig

		containerConfig, err := loadContainerConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载容器配置失败: %w", err)
			return
		}
		cfg.Container = *containerConfig

		minioConfig, err := loadMinIOConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载MinIO配置失败: %w", err)
			return
		}
		cfg.MinIO = *minioConfig

		flagConfig, err := loadFlagConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载Flag配置失败: %w", err)
			return
		}
		cfg.Flag = *flagConfig

		ingressConfig, err := loadIngressConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载Ingress配置失败: %w", err)
			return
		}
		cfg.Ingress = *ingressConfig

		loadedConfig = cfg
	})

	if loadConfigErr != nil {
		return nil, loadConfigErr
	}
	if loadedConfig == nil {
		return nil, fmt.Errorf("配置未初始化")
	}

	return loadedConfig, nil
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
	trustedProxies := parseCSV(getEnv("SERVER_TRUSTED_PROXIES", "127.0.0.1,::1"))
	allowedOrigins := parseCSV(getEnv("SERVER_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"))

	return &ServerConfig{
		Port:           port,
		Host:           host,
		TrustedProxies: trustedProxies,
		AllowedOrigins: allowedOrigins,
	}, nil
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// loadJWTConfig 加载JWT配置
func loadJWTConfig() (*JWTConfig, error) {
	secret := getEnv("JWT_SECRET", DefaultJWTSecret)

	accessMinutes := getEnvInt("JWT_ACCESS_EXPIRATION_MINUTES", 15) // 15分钟
	refreshHours := getEnvInt("JWT_REFRESH_EXPIRATION_HOURS", 168)  // 7天

	accessExpiration := time.Duration(accessMinutes) * time.Minute
	refreshExpiration := time.Duration(refreshHours) * time.Hour

	return &JWTConfig{
		Secret:            secret,
		AccessExpiration:  accessExpiration,
		RefreshExpiration: refreshExpiration,
		Expiration:        accessExpiration,
	}, nil
}

func IsWeakJWTSecret(secret string) bool {
	trimmed := strings.TrimSpace(secret)
	return trimmed == "" || trimmed == DefaultJWTSecret
}

func loadInitConfig() (*InitConfig, error) {
	return &InitConfig{
		RootDefaultPassword: getEnv("ROOT_DEFAULT_PASSWORD", ""),
	}, nil
}

func loadAuthCookieConfig() (*AuthCookieConfig, error) {
	return &AuthCookieConfig{
		AccessTokenName:  getEnv("AUTH_ACCESS_COOKIE_NAME", "ctf_access_token"),
		RefreshTokenName: getEnv("AUTH_REFRESH_COOKIE_NAME", "ctf_refresh_token"),
		Domain:           getEnv("AUTH_COOKIE_DOMAIN", ""),
		Secure:           getEnvBool("AUTH_COOKIE_SECURE", false),
		SameSite:         strings.ToLower(strings.TrimSpace(getEnv("AUTH_COOKIE_SAMESITE", "lax"))),
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
		VerificationDailyLimit:       getEnvInt("EMAIL_VERIFY_DAILY_LIMIT", 5),
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

func loadContainerConfig() (*ContainerConfig, error) {
	return &ContainerConfig{
		KubeconfigPath:          getEnv("KUBECONFIG_PATH", ""),
		MaxConcurrentContainers: getEnvInt("MAX_CONCURRENT_CONTAINERS", 5),
		DefaultContainerTTL:     getEnvInt("DEFAULT_CONTAINER_TTL", 3600),
		NamespacePrefix:         getEnv("NAMESPACE_PREFIX", "ctf"),
		KataRuntimeClass:        getEnv("KATA_RUNTIME_CLASS", "kata"),
	}, nil
}

func loadMinIOConfig() (*MinIOConfig, error) {
	return &MinIOConfig{
		Endpoint:      getEnv("MINIO_ENDPOINT", ""),
		AccessKey:     getEnv("MINIO_ACCESS_KEY", ""),
		SecretKey:     getEnv("MINIO_SECRET_KEY", ""),
		UseSSL:        getEnvBool("MINIO_USE_SSL", false),
		BucketPrefix:  getEnv("MINIO_BUCKET_PREFIX", "ctf"),
		PresignExpiry: getEnvInt("MINIO_PRESIGN_EXPIRY", 600),
	}, nil
}

func loadFlagConfig() (*FlagConfig, error) {
	return &FlagConfig{
		Template: getEnv("FLAG_TEMPLATE", "flag{%s}"),
	}, nil
}

func loadIngressConfig() (*IngressConfig, error) {
	return &IngressConfig{
		DomainTemplate: getEnv("INGRESS_DOMAIN_TEMPLATE", "{id}.chal.example.com"),
		Class:          getEnv("INGRESS_CLASS", "traefik"),
		TLSEnabled:     getEnvBool("INGRESS_TLS_ENABLED", false),
		TLSSecret:      getEnv("INGRESS_TLS_SECRET", ""),
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
