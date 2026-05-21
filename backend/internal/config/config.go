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
	Session     SessionConfig
	WS          WSConfig
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
	LoginGuard  LoginGuardConfig
	Registry    RegistryConfig
}

// LoginGuardConfig 登录暴力破解防护配置：双维度（账户 + IP）失败计数 + 自动锁定。
type LoginGuardConfig struct {
	// AccountFailLimit 账户级失败次数阈值，达到即锁定。
	AccountFailLimit int
	// AccountFailWindow 账户失败计数窗口（计数器 TTL）。
	AccountFailWindow time.Duration
	// AccountLockDuration 账户锁定时长。
	AccountLockDuration time.Duration
	// IPFailLimit IP 级失败次数阈值（窗口内）。
	IPFailLimit int
	// IPFailWindow IP 失败计数窗口。
	IPFailWindow time.Duration
	// IPLockDuration IP 锁定时长。
	IPLockDuration time.Duration
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
	Secret string
	// AccessExpiration access token 有效期，所有角色统一短 TTL。
	AccessExpiration time.Duration
	// RefreshExpiration 默认 refresh token 有效期：未勾选 remember_me 时使用。
	RefreshExpiration time.Duration
	// RememberMeRefreshExpiration 勾选 remember_me 时使用的长 refresh TTL（仅 user 角色）。
	RememberMeRefreshExpiration time.Duration
	// Expiration 保留兼容字段，等同于 AccessExpiration。
	Expiration time.Duration
}

// SessionConfig 会话策略配置：admin 角色固定 single_session，user 可配置 multi_session 上限。
type SessionConfig struct {
	// AdminSessionMode 始终为 "single_session"（代码层硬编码，仅作记录用途）。
	AdminSessionMode string
	// UserSessionMode 普通用户会话模式："multi_session"。
	UserSessionMode string
	// UserMaxActiveSessions 普通用户最大并发活跃会话数（multi_session 上限）。
	UserMaxActiveSessions int
}

// WSConfig WebSocket 心跳与在线状态配置。
type WSConfig struct {
	// Enabled 是否启用 WS 心跳上报。
	Enabled bool
	// ClientHeartbeatInterval 服务端发送 ping 的间隔。
	ClientHeartbeatInterval time.Duration
	// PresenceTimeout 在线状态 TTL，应 >= ClientHeartbeatInterval * 2。
	PresenceTimeout time.Duration
	// MaxConnectionsPerUser 单用户最大并发 WS 连接数。
	MaxConnectionsPerUser int
	// SessionRecheckInterval 周期性校验 access 会话与单会话指针的间隔；
	// <=0 表示关闭主动再校验（仅依赖连接断开时检测）。
	SessionRecheckInterval time.Duration
}

// UploadConfig 上传配置
type UploadConfig struct {
	StaticPath          string
	MaxImagePackageSize int64
	MaxImageSize        int64
	MaxAttachmentSize   int64
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
	// CodeHMACSecret 用于对邮箱验证码做 HMAC-SHA256 哈希。
	// 为空时回退到 JWT_SECRET，保证升级现有部署时无需立刻改 env。
	CodeHMACSecret string
	// VerifyFailLimit 同一邮箱在验证码生命周期内允许的最大失败次数；超过后立即作废验证码。
	VerifyFailLimit int
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
	InfraNodeName           string // INFRA_NODE_NAME, registry 固定节点
}

// MinIOConfig MinIO 对象存储配置。
type MinIOConfig struct {
	Enabled       bool
	Endpoint      string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	BucketPrefix  string
	PresignExpiry int
}

// RegistryConfig 私有 Docker 镜像仓库配置。
type RegistryConfig struct {
	StorageSize string // REGISTRY_STORAGE_SIZE, PVC 大小
	Host        string // 启动时自动填充: "127.0.0.1:<nodePort>"
	Username    string // 启动时自动生成
	Password    string // 启动时自动生成
}

// SetRegistryInfo 启动时注入 registry 运行时信息。
func (c *Config) SetRegistryInfo(host, username, password string) {
	c.Registry.Host = host
	c.Registry.Username = username
	c.Registry.Password = password
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

		sessionConfig, err := loadSessionConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载会话策略配置失败: %w", err)
			return
		}
		cfg.Session = *sessionConfig

		wsConfig, err := loadWSConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载 WebSocket 配置失败: %w", err)
			return
		}
		cfg.WS = *wsConfig

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

		loginGuardConfig, err := loadLoginGuardConfig()
		if err != nil {
			loadConfigErr = fmt.Errorf("加载登录防护配置失败: %w", err)
			return
		}
		cfg.LoginGuard = *loginGuardConfig

			registryConfig, err := loadRegistryConfig()
			if err != nil {
				loadConfigErr = fmt.Errorf("加载仓库配置失败: %w", err)
				return
			}
			cfg.Registry = *registryConfig

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

// loadJWTConfig 加载JWT配置。
//
// 兼容性：保留 JWT_REFRESH_EXPIRATION_HOURS 作为「未勾选记住我」的默认 refresh TTL；
// AUTH_NO_REMEMBER_ME_REFRESH_TTL_MINUTES 优先级更高，便于按分钟精细控制；
// AUTH_REMEMBER_ME_REFRESH_TTL_DAYS 控制「勾选记住我」的长 refresh TTL（仅 user 角色生效）。
func loadJWTConfig() (*JWTConfig, error) {
	secret := getEnv("JWT_SECRET", DefaultJWTSecret)

	accessMinutes := getEnvInt("JWT_ACCESS_EXPIRATION_MINUTES", 15)

	noRememberMeMinutes := getEnvInt("AUTH_NO_REMEMBER_ME_REFRESH_TTL_MINUTES", 0)
	var refreshExpiration time.Duration
	if noRememberMeMinutes > 0 {
		refreshExpiration = time.Duration(noRememberMeMinutes) * time.Minute
	} else {
		refreshHours := getEnvInt("JWT_REFRESH_EXPIRATION_HOURS", 168)
		refreshExpiration = time.Duration(refreshHours) * time.Hour
	}

	rememberDays := getEnvInt("AUTH_REMEMBER_ME_REFRESH_TTL_DAYS", 30)
	rememberMeRefreshExpiration := time.Duration(rememberDays) * 24 * time.Hour
	if rememberMeRefreshExpiration < refreshExpiration {
		// 记住我的 TTL 必须 >= 默认 TTL，否则配置无意义。
		rememberMeRefreshExpiration = refreshExpiration
	}

	accessExpiration := time.Duration(accessMinutes) * time.Minute

	return &JWTConfig{
		Secret:                      secret,
		AccessExpiration:            accessExpiration,
		RefreshExpiration:           refreshExpiration,
		RememberMeRefreshExpiration: rememberMeRefreshExpiration,
		Expiration:                  accessExpiration,
	}, nil
}

// loadSessionConfig 加载会话策略配置。
func loadSessionConfig() (*SessionConfig, error) {
	userMode := strings.ToLower(strings.TrimSpace(getEnv("AUTH_USER_SESSION_MODE", "multi_session")))
	if userMode != "multi_session" && userMode != "single_session" {
		userMode = "multi_session"
	}
	maxSessions := getEnvInt("AUTH_USER_MAX_ACTIVE_SESSIONS", 5)
	if maxSessions <= 0 {
		maxSessions = 5
	}

	return &SessionConfig{
		AdminSessionMode:      "single_session",
		UserSessionMode:       userMode,
		UserMaxActiveSessions: maxSessions,
	}, nil
}

// loadWSConfig 加载 WebSocket 心跳配置。
func loadWSConfig() (*WSConfig, error) {
	enabled := getEnvBool("WS_ENABLED", true)
	heartbeatSec := getEnvInt("WS_CLIENT_HEARTBEAT_INTERVAL_SEC", 30)
	if heartbeatSec <= 0 {
		heartbeatSec = 30
	}
	presenceSec := getEnvInt("WS_PRESENCE_TIMEOUT_SEC", 120)
	if presenceSec < heartbeatSec*2 {
		// 在线判定超时必须 >= 心跳间隔 * 2，避免抖动误判。
		presenceSec = heartbeatSec * 2
	}
	maxConns := getEnvInt("WS_MAX_CONNECTIONS_PER_USER", 3)
	if maxConns <= 0 {
		maxConns = 3
	}
	// 默认 60s 主动再校验一次会话状态；设置为 0 可关闭。
	recheckSec := getEnvInt("WS_SESSION_RECHECK_INTERVAL_SEC", 60)
	if recheckSec < 0 {
		recheckSec = 0
	}

	return &WSConfig{
		Enabled:                 enabled,
		ClientHeartbeatInterval: time.Duration(heartbeatSec) * time.Second,
		PresenceTimeout:         time.Duration(presenceSec) * time.Second,
		MaxConnectionsPerUser:   maxConns,
		SessionRecheckInterval:  time.Duration(recheckSec) * time.Second,
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
		SameSite:         strings.ToLower(strings.TrimSpace(getEnv("AUTH_COOKIE_SAMESITE", "strict"))),
	}, nil
}

// loadUploadConfig 加载上传配置
func loadUploadConfig() (*UploadConfig, error) {
	staticPath := getEnv("STATIC_PATH", "../static")
	maxImagePackageSize := getEnvInt64("MAX_UPLOAD_IMAGE_PACKAGE_SIZE", 2*1024*1024*1024) // 2GB
	maxImageSize := getEnvInt64("MAX_UPLOAD_IMAGE_SIZE", 10*1024*1024)                    // 10MB
	maxAttachmentSize := getEnvInt64("MAX_UPLOAD_ATTACHMENT_SIZE", 200*1024*1024)         // 200MB

	return &UploadConfig{
		StaticPath:          staticPath,
		MaxImagePackageSize: maxImagePackageSize,
		MaxImageSize:        maxImageSize,
		MaxAttachmentSize:   maxAttachmentSize,
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
	codeSecret := strings.TrimSpace(getEnv("EMAIL_CODE_HMAC_SECRET", ""))
	failLimit := getEnvInt("EMAIL_VERIFY_FAIL_LIMIT", 10)
	if failLimit <= 0 {
		failLimit = 10
	}
	return &EmailConfig{
		ResendAPIKey:                 getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:              getEnv("RESEND_FROM_EMAIL", ""),
		VerificationCodeTTLSeconds:   getEnvInt("EMAIL_VERIFY_CODE_TTL_SECONDS", 300),
		VerificationSendCooldownSecs: getEnvInt("EMAIL_VERIFY_SEND_COOLDOWN_SECONDS", 60),
		VerificationDailyLimit:       getEnvInt("EMAIL_VERIFY_DAILY_LIMIT", 5),
		CodeHMACSecret:               codeSecret,
		VerifyFailLimit:              failLimit,
	}, nil
}

// loadBotProtectionConfig 加载机器人验证配置
func loadBotProtectionConfig() (*BotProtectionConfig, error) {
	secretKey := strings.TrimSpace(getEnv("TURNSTILE_SECRET_KEY", ""))
	enabled := getEnvBool("TURNSTILE_ENABLED", secretKey != "")

	return &BotProtectionConfig{
		Enabled:            enabled,
		TurnstileSecretKey: secretKey,
		TurnstileVerifyURL: strings.TrimSpace(getEnv("TURNSTILE_VERIFY_URL", "https://challenges.cloudflare.com/turnstile/v0/siteverify")),
	}, nil
}

func loadContainerConfig() (*ContainerConfig, error) {
	return &ContainerConfig{
		KubeconfigPath:          getEnv("KUBECONFIG_PATH", ""),
		MaxConcurrentContainers: getEnvInt("MAX_CONCURRENT_CONTAINERS", 5),
		DefaultContainerTTL:     getEnvInt("DEFAULT_CONTAINER_TTL", 3600),
		NamespacePrefix:         getEnv("NAMESPACE_PREFIX", "ctf"),
		KataRuntimeClass:        getEnv("KATA_RUNTIME_CLASS", "kata"),
			InfraNodeName: getEnv("INFRA_NODE_NAME", ""),
	}, nil
}

func loadMinIOConfig() (*MinIOConfig, error) {
	return &MinIOConfig{
		Enabled:       getEnvBool("MINIO_ENABLED", false),
		Endpoint:      getEnv("MINIO_ENDPOINT", ""),
		AccessKey:     getEnv("MINIO_ACCESS_KEY", ""),
		SecretKey:     getEnv("MINIO_SECRET_KEY", ""),
		UseSSL:        getEnvBool("MINIO_USE_SSL", false),
		BucketPrefix:  getEnv("MINIO_BUCKET_PREFIX", "ctf"),
		PresignExpiry: getEnvInt("MINIO_PRESIGN_EXPIRY", 600),
	}, nil
}

func loadRegistryConfig() (*RegistryConfig, error) {
	return &RegistryConfig{
		StorageSize: getEnv("REGISTRY_STORAGE_SIZE", "30Gi"),
	}, nil
}

func loadFlagConfig() (*FlagConfig, error) {
	return &FlagConfig{
		Template: getEnv("FLAG_TEMPLATE", "flag{%s}"),
	}, nil
}

// loadLoginGuardConfig 加载登录暴力破解防护配置。
//
// 默认值经验估算：账户级 10 次失败 / 15 分钟窗口 → 锁 15 分钟；
// IP 级 60 次失败 / 1 小时窗口 → 锁 30 分钟；后者主要拦截分布式枚举。
func loadLoginGuardConfig() (*LoginGuardConfig, error) {
	accountLimit := getEnvInt("LOGIN_FAIL_ACCOUNT_LIMIT", 10)
	if accountLimit <= 0 {
		accountLimit = 10
	}
	accountWindowMin := getEnvInt("LOGIN_FAIL_ACCOUNT_WINDOW_MINUTES", 15)
	if accountWindowMin <= 0 {
		accountWindowMin = 15
	}
	accountLockMin := getEnvInt("LOGIN_FAIL_ACCOUNT_LOCK_MINUTES", 15)
	if accountLockMin <= 0 {
		accountLockMin = 15
	}

	ipLimit := getEnvInt("LOGIN_FAIL_IP_LIMIT", 60)
	if ipLimit <= 0 {
		ipLimit = 60
	}
	ipWindowMin := getEnvInt("LOGIN_FAIL_IP_WINDOW_MINUTES", 60)
	if ipWindowMin <= 0 {
		ipWindowMin = 60
	}
	ipLockMin := getEnvInt("LOGIN_FAIL_IP_LOCK_MINUTES", 30)
	if ipLockMin <= 0 {
		ipLockMin = 30
	}

	return &LoginGuardConfig{
		AccountFailLimit:    accountLimit,
		AccountFailWindow:   time.Duration(accountWindowMin) * time.Minute,
		AccountLockDuration: time.Duration(accountLockMin) * time.Minute,
		IPFailLimit:         ipLimit,
		IPFailWindow:        time.Duration(ipWindowMin) * time.Minute,
		IPLockDuration:      time.Duration(ipLockMin) * time.Minute,
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
