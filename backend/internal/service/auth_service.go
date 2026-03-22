package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/redis/go-redis/v9"
)

// AuthService 认证服务接口
type AuthService interface {
	SendRegisterCode(req *models.SendRegisterCodeRequest, clientIP string) error
	Register(req *models.RegisterRequest, clientIP string) (*models.AuthResponse, error)
	Login(req *models.LoginRequest, clientIP string) (*models.AuthResponse, error)
}

// AuthServiceImpl 认证服务实现
type AuthServiceImpl struct {
	userRepo    *repository.UserRepository
	redisClient *redis.Client
	emailCfg    config.EmailConfig
	botCfg      config.BotProtectionConfig
	httpClient  *http.Client
}

// NewAuthService 创建认证服务实例
func NewAuthService(userRepo *repository.UserRepository, redisClient *redis.Client, cfg *config.Config) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepo:    userRepo,
		redisClient: redisClient,
		emailCfg:    cfg.Email,
		botCfg:      cfg.Bot,
		httpClient:  &http.Client{Timeout: 8 * time.Second},
	}
}

// SendRegisterCode 发送注册验证码
func (s *AuthServiceImpl) SendRegisterCode(req *models.SendRegisterCodeRequest, clientIP string) error {
	if err := validateSendCodeRequest(req); err != nil {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	normalizedEmail := normalizeEmail(req.Email)

	if err := s.verifyBotToken(req.BotToken, clientIP); err != nil {
		return err
	}

	exists, err := s.userRepo.UserExistsByEmail(normalizedEmail)
	if err != nil {
		return fmt.Errorf("检查邮箱失败: %w", err)
	}
	if exists {
		return apperr.New(apperr.ErrConflict, "邮箱已被使用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cooldownKey := cache.GetEmailVerifyCooldownKey(normalizedEmail)
	ttl, err := s.redisClient.TTL(ctx, cooldownKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("验证码限流检查失败: %w", err)
	}
	if ttl > 0 {
		return apperr.New(apperr.ErrTooManyRequests, fmt.Sprintf("验证码发送过于频繁，请 %d 秒后重试", int(ttl.Seconds())))
	}

	if err := s.checkAndIncreaseDailyLimit(ctx, normalizedEmail); err != nil {
		return err
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return fmt.Errorf("验证码生成失败: %w", err)
	}

	codeKey := cache.GetEmailVerifyCodeKey(normalizedEmail)
	codeHash := hashEmailCode(normalizedEmail, code)
	codeTTL := time.Duration(s.emailCfg.VerificationCodeTTLSeconds) * time.Second
	cooldownTTL := time.Duration(s.emailCfg.VerificationSendCooldownSecs) * time.Second

	if err := s.redisClient.Set(ctx, codeKey, codeHash, codeTTL).Err(); err != nil {
		return fmt.Errorf("验证码存储失败: %w", err)
	}
	if err := s.redisClient.Set(ctx, cooldownKey, "1", cooldownTTL).Err(); err != nil {
		return fmt.Errorf("验证码冷却存储失败: %w", err)
	}

	if err := s.sendVerificationEmail(normalizedEmail, code); err != nil {
		_ = s.redisClient.Del(ctx, codeKey, cooldownKey).Err()
		return err
	}

	return nil
}

// Register 处理用户注册
func (s *AuthServiceImpl) Register(req *models.RegisterRequest, clientIP string) (*models.AuthResponse, error) {
	// 验证输入
	if err := validateRegisterRequest(req); err != nil {
		return nil, apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	normalizedEmail := normalizeEmail(req.Email)

	if err := s.verifyEmailCode(normalizedEmail, req.EmailCode); err != nil {
		return nil, err
	}

	// 检查用户名是否存在
	exists, err := s.userRepo.UserExistsByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("检查用户名失败: %w", err)
	}
	if exists {
		return nil, apperr.New(apperr.ErrConflict, "用户名已被使用")
	}

	// 检查邮箱是否存在
	if normalizedEmail != "" {
		exists, err := s.userRepo.UserExistsByEmail(normalizedEmail)
		if err != nil {
			return nil, fmt.Errorf("检查邮箱失败: %w", err)
		}
		if exists {
			return nil, apperr.New(apperr.ErrConflict, "邮箱已被使用")
		}
	}

	req.Email = normalizedEmail

	// 加密密码
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user, err := s.userRepo.CreateUser(req, hashedPassword, clientIP)
	if err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	// 生成 token
	token, err := utils.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// Login 处理用户登录
func (s *AuthServiceImpl) Login(req *models.LoginRequest, clientIP string) (*models.AuthResponse, error) {
	// 验证输入
	if err := validateLoginRequest(req); err != nil {
		return nil, apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	// 查询用户
	user, err := s.userRepo.GetUserByUsername(req.Username)
	if err != nil {
		return nil, apperr.New(apperr.ErrUnauthorized, "用户名或密码错误")
	}

	// 验证密码
	if err := utils.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		return nil, apperr.New(apperr.ErrUnauthorized, "用户名或密码错误")
	}

	// 更新最后登陆时间
	_ = s.userRepo.UpdateLastLogin(user.ID, clientIP)

	// 生成 token
	token, err := utils.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("生成 token 失败: %w", err)
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

func (s *AuthServiceImpl) verifyEmailCode(email, rawCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	codeKey := cache.GetEmailVerifyCodeKey(email)
	storedHash, err := s.redisClient.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return apperr.New(apperr.ErrBadRequest, "验证码已过期或不存在，请重新获取")
	}
	if err != nil {
		return fmt.Errorf("验证码校验失败: %w", err)
	}

	computedHash := hashEmailCode(email, strings.TrimSpace(rawCode))
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(computedHash)) != 1 {
		return apperr.New(apperr.ErrBadRequest, "邮箱验证码错误")
	}

	if err := s.redisClient.Del(ctx, codeKey).Err(); err != nil {
		return fmt.Errorf("验证码状态更新失败: %w", err)
	}

	return nil
}

func (s *AuthServiceImpl) checkAndIncreaseDailyLimit(ctx context.Context, email string) error {
	day := time.Now().Format("20060102")
	dailyKey := cache.GetEmailVerifyDailyLimitKey(email, day)

	count, err := s.redisClient.Incr(ctx, dailyKey).Result()
	if err != nil {
		return fmt.Errorf("验证码频率统计失败: %w", err)
	}
	if count == 1 {
		_ = s.redisClient.Expire(ctx, dailyKey, 24*time.Hour).Err()
	}

	if int(count) > s.emailCfg.VerificationDailyLimit {
		return apperr.New(apperr.ErrTooManyRequests, "验证码发送已达当日上限，请明日再试")
	}

	return nil
}

func (s *AuthServiceImpl) verifyBotToken(botToken, clientIP string) error {
	if !s.botCfg.Enabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		ip = "unknown"
	}

	if strings.TrimSpace(botToken) == "" {
		return apperr.New(apperr.ErrBadRequest, "请先完成机器人验证")
	}

	if err := s.limitBotVerifyByIP(ctx, ip); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("secret", s.botCfg.TurnstileSecretKey)
	form.Set("response", botToken)
	if ip != "unknown" {
		form.Set("remoteip", ip)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.botCfg.TurnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("机器人验证请求构建失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return apperr.New(apperr.ErrBadRequest, "机器人验证请求失败")
	}
	defer resp.Body.Close()

	var verifyResp struct {
		Success bool     `json:"success"`
		Errors  []string `json:"error-codes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return apperr.New(apperr.ErrBadRequest, "机器人验证响应解析失败")
	}

	if !verifyResp.Success {
		return apperr.New(apperr.ErrBadRequest, "机器人验证未通过")
	}

	return nil
}

func (s *AuthServiceImpl) limitBotVerifyByIP(ctx context.Context, clientIP string) error {
	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		ip = "unknown"
	}

	key := cache.GetBotVerifyRateLimitKey(ip)
	count, err := s.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("机器人验证限流失败: %w", err)
	}
	if count == 1 {
		_ = s.redisClient.Expire(ctx, key, 60*time.Second).Err()
	}
	if count > 30 {
		return apperr.New(apperr.ErrTooManyRequests, "请求过于频繁，请稍后再试")
	}

	return nil
}

func (s *AuthServiceImpl) sendVerificationEmail(toEmail, code string) error {
	if strings.TrimSpace(s.emailCfg.ResendAPIKey) == "" || strings.TrimSpace(s.emailCfg.ResendFromEmail) == "" {
		return fmt.Errorf("邮件服务未配置，请联系管理员")
	}

	payload := map[string]any{
		"from":    s.emailCfg.ResendFromEmail,
		"to":      []string{toEmail},
		"subject": "CTF 平台注册验证码",
		"html": fmt.Sprintf(
			"<div style=\"font-family:Arial,sans-serif;line-height:1.7;\"><h2>邮箱验证码</h2><p>你的验证码为：</p><p style=\"font-size:24px;font-weight:700;letter-spacing:3px;\">%s</p><p>验证码 %d 秒内有效，请勿泄露给他人。</p></div>",
			code,
			s.emailCfg.VerificationCodeTTLSeconds,
		),
		"text": fmt.Sprintf("你的验证码是 %s，%d 秒内有效。", code, s.emailCfg.VerificationCodeTTLSeconds),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("邮件请求构建失败: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("邮件请求创建失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.emailCfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("邮件发送失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("邮件发送失败，请稍后重试")
	}

	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func generateNumericCode(length int) (string, error) {
	if length <= 0 {
		return "", apperr.New(apperr.ErrBadRequest, "验证码长度无效")
	}

	b := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b[i] = byte('0' + n.Int64())
	}

	return string(b), nil
}

func hashEmailCode(email, code string) string {
	raw := email + ":" + code
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// 验证注册请求
func validateRegisterRequest(req *models.RegisterRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return apperr.New(apperr.ErrBadRequest, "用户名不能为空")
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return apperr.New(apperr.ErrBadRequest, "用户名长度应在 3-32 个字符之间")
	}
	if strings.TrimSpace(req.Email) == "" {
		return apperr.New(apperr.ErrBadRequest, "邮箱不能为空")
	}
	if !isValidEmail(req.Email) {
		return apperr.New(apperr.ErrBadRequest, "邮箱格式不正确")
	}
	if len(req.Password) < 6 {
		return apperr.New(apperr.ErrBadRequest, "密码长度至少 6 个字符")
	}
	if len(req.Password) > 128 {
		return apperr.New(apperr.ErrBadRequest, "密码长度不能超过 128 个字符")
	}
	if len(strings.TrimSpace(req.EmailCode)) != 6 {
		return apperr.New(apperr.ErrBadRequest, "邮箱验证码格式不正确")
	}
	return nil
}

func validateSendCodeRequest(req *models.SendRegisterCodeRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return apperr.New(apperr.ErrBadRequest, "邮箱不能为空")
	}
	if !isValidEmail(req.Email) {
		return apperr.New(apperr.ErrBadRequest, "邮箱格式不正确")
	}

	return nil
}

// 验证登录请求
func validateLoginRequest(req *models.LoginRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return apperr.New(apperr.ErrBadRequest, "用户名不能为空")
	}
	if strings.TrimSpace(req.Password) == "" {
		return apperr.New(apperr.ErrBadRequest, "密码不能为空")
	}
	return nil
}
