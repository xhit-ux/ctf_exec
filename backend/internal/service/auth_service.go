package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/authsession"
	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/redis/go-redis/v9"
)

// 与 db/final.sql 中 chk_username_format 一致：^[a-zA-Z0-9_]{3,32}$
var usernameRegisterPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

// AuthService 认证服务接口
type AuthService interface {
	GetRegistrationHints() (*models.RegistrationHints, error)
	SendRegisterCode(req *models.SendRegisterCodeRequest, clientIP string) error
	SendPasswordResetCode(req *models.SendPasswordResetCodeRequest, clientIP string) error
	ResetPasswordWithEmailCode(req *models.ResetPasswordWithCodeRequest) error
	Register(req *models.RegisterRequest, clientIP, userAgent string) (*models.AuthResponse, error)
	Login(req *models.LoginRequest, clientIP, userAgent string) (*models.AuthResponse, error)
	Refresh(refreshToken, clientIP, userAgent string) (*models.AuthResponse, error)
	Logout(userID, accessJTI, refreshToken string) error
	RevokeUserSessions(userID string) error
}

// AuthServiceImpl 认证服务实现
type AuthServiceImpl struct {
	userRepo       *repository.UserRepository
	siteConfigRepo *repository.SiteConfigRepository
	redisClient    *redis.Client
	sessionStore   *authsession.Store
	loginGuard     *authsession.LoginGuard
	jwtCfg         config.JWTConfig
	sessionCfg     config.SessionConfig
	emailCfg       config.EmailConfig
	botCfg         config.BotProtectionConfig
	httpClient     *http.Client
	// codeHMACKey 用于对邮箱验证码做 HMAC-SHA256；缺省回退到 JWT_SECRET。
	codeHMACKey []byte
}

// NewAuthService 创建认证服务实例
func NewAuthService(userRepo *repository.UserRepository, siteConfigRepo *repository.SiteConfigRepository, redisClient *redis.Client, sessionStore *authsession.Store, cfg *config.Config) *AuthServiceImpl {
	codeSecret := strings.TrimSpace(cfg.Email.CodeHMACSecret)
	if codeSecret == "" {
		codeSecret = cfg.JWT.Secret
	}
	return &AuthServiceImpl{
		userRepo:       userRepo,
		siteConfigRepo: siteConfigRepo,
		redisClient:    redisClient,
		sessionStore:   sessionStore,
		loginGuard:     authsession.NewLoginGuard(redisClient, cfg.LoginGuard),
		jwtCfg:         cfg.JWT,
		sessionCfg:     cfg.Session,
		emailCfg:       cfg.Email,
		botCfg:         cfg.Bot,
		httpClient:     &http.Client{Timeout: 8 * time.Second},
		codeHMACKey:    []byte(codeSecret),
	}
}

// GetRegistrationHints 返回注册页所需的公开策略（站点配置 + 是否启用人机验证）。
func (s *AuthServiceImpl) GetRegistrationHints() (*models.RegistrationHints, error) {
	siteCfg, err := s.siteConfigRepo.GetSiteConfig()
	if err != nil {
		return nil, wrapServiceError("读取站点配置失败", err)
	}
	return &models.RegistrationHints{
		RegistrationEnabled:       siteCfg.RegistrationEnabled,
		EmailVerificationRequired: siteCfg.EmailVerificationRequired,
		BotProtectionEnabled:      s.botCfg.Enabled,
	}, nil
}

// isSingleSessionRole 判断角色是否走单会话互踢策略。admin 系角色（root/site_admin）固定单会话。
func (s *AuthServiceImpl) isSingleSessionRole(role string) bool {
	normalized := models.NormalizeRole(role)
	return normalized == models.RoleRoot || normalized == models.RoleSiteAdmin
}

// resolveRememberMe 根据角色规范化 remember_me：admin 强制为 false，user 透传请求值。
func (s *AuthServiceImpl) resolveRememberMe(role string, requested bool) bool {
	if s.isSingleSessionRole(role) {
		return false
	}
	return requested
}

// resolveRefreshTTL 根据角色和 remember_me 决定 refresh token TTL。
func (s *AuthServiceImpl) resolveRefreshTTL(role string, rememberMe bool) time.Duration {
	if !rememberMe {
		return s.jwtCfg.RefreshExpiration
	}
	if s.isSingleSessionRole(role) {
		return s.jwtCfg.RefreshExpiration
	}
	return s.jwtCfg.RememberMeRefreshExpiration
}

// issueTokenPair 签发 access + refresh token 并落地会话状态，按角色应用单/多会话策略。
//
// 策略矩阵：
//   - admin (root/site_admin): single_session，原子置换 user_current 指针并撤销旧 access；
//     remember_me 强制 false，refresh TTL 始终为短 TTL。
//   - user: multi_session，活跃会话数超过 AUTH_USER_MAX_ACTIVE_SESSIONS 时驱逐最旧会话；
//     remember_me 透传，决定 refresh TTL。
func (s *AuthServiceImpl) issueTokenPair(user *models.User, clientIP, userAgent string, requestedRememberMe bool) (*models.AuthResponse, error) {
	rememberMe := s.resolveRememberMe(user.Role, requestedRememberMe)
	refreshTTL := s.resolveRefreshTTL(user.Role, rememberMe)

	accessToken, accessClaims, err := utils.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, wrapServiceError("生成 access token 失败", err)
	}

	refreshToken, refreshClaims, err := utils.GenerateRefreshTokenWithTTL(user.ID, user.Username, user.Role, refreshTTL, rememberMe)
	if err != nil {
		return nil, wrapServiceError("生成 refresh token 失败", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.sessionStore.CreateSession(ctx, accessClaims); err != nil {
		return nil, wrapServiceError("创建 access 会话失败", err)
	}
	if err := s.sessionStore.CreateRefreshSession(ctx, refreshClaims, refreshToken, clientIP, userAgent, accessClaims.ID); err != nil {
		return nil, wrapServiceError("创建 refresh 会话失败", err)
	}

	if s.isSingleSessionRole(user.Role) {
		// 原子地把指针指向新 jti，并撤销旧 access；refresh 同步标记 revoked=1。
		oldJTI, err := s.sessionStore.SetUserCurrentSession(ctx, user.ID, accessClaims.ID, s.jwtCfg.AccessExpiration)
		if err != nil {
			return nil, wrapServiceError("设置当前会话指针失败", err)
		}
		if oldJTI != "" && oldJTI != accessClaims.ID {
			_ = s.sessionStore.RevokeSession(ctx, user.ID, oldJTI)
			_ = s.sessionStore.RevokeRefreshSessionByJTI(ctx, user.ID, oldJTI)
		}
	} else {
		// multi_session：超限时驱逐最旧会话。
		if err := s.enforceSessionLimit(ctx, user.ID); err != nil {
			return nil, err
		}
	}

	return &models.AuthResponse{
		Token:                accessToken,
		RefreshToken:         refreshToken,
		AccessExpiresInSecs:  int64(s.jwtCfg.AccessExpiration / time.Second),
		RefreshExpiresInSecs: int64(refreshTTL / time.Second),
		TokenType:            "Bearer",
		User:                 user,
		SessionID:            accessClaims.ID,
		RememberMe:           rememberMe,
	}, nil
}

// enforceSessionLimit 在多会话场景下强制活跃会话数不超过上限：
// 上限-1 之内放行，达到上限时驱逐最旧会话以腾出位置。
//
// 调用时机：发出新会话之前，确保新会话签发后总数 == 上限。
func (s *AuthServiceImpl) enforceSessionLimit(ctx context.Context, userID string) error {
	limit := s.sessionCfg.UserMaxActiveSessions
	if limit <= 0 {
		return nil
	}

	active, err := s.sessionStore.CountActiveSessions(ctx, userID)
	if err != nil {
		return wrapServiceError("查询用户活跃会话数失败", err)
	}

	// active 此时已包含刚刚通过 CreateSession 写入的新会话。
	for active > limit {
		victim, err := s.sessionStore.EvictOldestSession(ctx, userID)
		if err != nil {
			return wrapServiceError("驱逐过期会话失败", err)
		}
		if victim == "" {
			break
		}
		active--
	}
	return nil
}

// SendPasswordResetCode 发送密码重置验证码（邮箱不存在时静默成功，避免枚举）。
func (s *AuthServiceImpl) SendPasswordResetCode(req *models.SendPasswordResetCodeRequest, clientIP string) error {
	if err := validateSendPasswordResetCodeRequest(req); err != nil {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	normalizedEmail := normalizeEmail(req.Email)

	if err := s.verifyBotToken(req.BotToken, clientIP); err != nil {
		return err
	}

	user, err := s.userRepo.GetActiveUserByEmail(normalizedEmail)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil
		}
		return wrapServiceError("查询用户失败", err)
	}
	if strings.TrimSpace(user.Email) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cooldownKey := cache.GetPasswordResetCooldownKey(normalizedEmail)
	ttl, err := s.redisClient.TTL(ctx, cooldownKey).Result()
	if err != nil && err != redis.Nil {
		return wrapServiceError("验证码限流检查失败", err)
	}
	if ttl > 0 {
		return apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, fmt.Sprintf("验证码发送过于频繁，请 %d 秒后重试", int(ttl.Seconds())), int(ttl.Seconds()))
	}

	if err := s.checkPasswordResetDailyLimit(ctx, normalizedEmail); err != nil {
		return err
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return wrapServiceError("验证码生成失败", err)
	}

	codeKey := cache.GetPasswordResetCodeKey(normalizedEmail)
	codeHash := s.hashPasswordResetCode(normalizedEmail, code)
	codeTTL := time.Duration(s.emailCfg.VerificationCodeTTLSeconds) * time.Second
	cooldownTTL := time.Duration(s.emailCfg.VerificationSendCooldownSecs) * time.Second

	if err := s.redisClient.Set(ctx, codeKey, codeHash, codeTTL).Err(); err != nil {
		return wrapServiceError("验证码存储失败", err)
	}
	// 重新发码 → 清空旧的失败计数，避免历史失败累计影响新验证码。
	_ = s.redisClient.Del(ctx, cache.GetPasswordResetVerifyFailKey(normalizedEmail)).Err()
	if err := s.redisClient.Set(ctx, cooldownKey, "1", cooldownTTL).Err(); err != nil {
		return wrapServiceError("验证码冷却存储失败", err)
	}

	if err := s.sendPasswordResetEmail(normalizedEmail, code); err != nil {
		_ = s.redisClient.Del(ctx, codeKey, cooldownKey).Err()
		return err
	}

	return nil
}

// ResetPasswordWithEmailCode 校验邮箱验证码后更新密码并撤销全部会话。
func (s *AuthServiceImpl) ResetPasswordWithEmailCode(req *models.ResetPasswordWithCodeRequest) error {
	if err := validateResetPasswordWithCodeRequest(req); err != nil {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	normalizedEmail := normalizeEmail(req.Email)

	if err := s.verifyPasswordResetCode(normalizedEmail, req.EmailCode); err != nil {
		return err
	}

	user, err := s.userRepo.GetActiveUserByEmail(normalizedEmail)
	if err != nil {
		return err
	}

	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return wrapServiceError("密码加密失败", err)
	}

	if err := s.userRepo.UpdatePasswordHash(user.ID, hashedPassword); err != nil {
		return err
	}

	if err := s.RevokeUserSessions(user.ID); err != nil {
		return wrapServiceError("重置密码后撤销会话失败", err)
	}

	return nil
}

// SendRegisterCode 发送注册验证码
func (s *AuthServiceImpl) SendRegisterCode(req *models.SendRegisterCodeRequest, clientIP string) error {
	if err := validateSendCodeRequest(req); err != nil {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	siteCfg, err := s.siteConfigRepo.GetSiteConfig()
	if err != nil {
		return wrapServiceError("读取站点配置失败", err)
	}
	if !siteCfg.RegistrationEnabled {
		return apperr.New(apperr.ErrForbidden, "管理员已关闭新用户注册")
	}
	if !siteCfg.EmailVerificationRequired {
		return apperr.New(apperr.ErrBadRequest, "当前站点未启用邮箱验证码注册，请直接提交注册表单完成账号创建")
	}

	normalizedEmail := normalizeEmail(req.Email)

	if err := s.verifyBotToken(req.BotToken, clientIP); err != nil {
		return err
	}

	exists, err := s.userRepo.UserExistsByEmail(normalizedEmail)
	if err != nil {
		return wrapServiceError("检查邮箱失败", err)
	}
	if exists {
		return apperr.New(apperr.ErrConflict, "邮箱已被使用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cooldownKey := cache.GetEmailVerifyCooldownKey(normalizedEmail)
	ttl, err := s.redisClient.TTL(ctx, cooldownKey).Result()
	if err != nil && err != redis.Nil {
		return wrapServiceError("验证码限流检查失败", err)
	}
	if ttl > 0 {
		return apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, fmt.Sprintf("验证码发送过于频繁，请 %d 秒后重试", int(ttl.Seconds())), int(ttl.Seconds()))
	}

	if err := s.checkAndIncreaseDailyLimit(ctx, normalizedEmail); err != nil {
		return err
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return wrapServiceError("验证码生成失败", err)
	}

	codeKey := cache.GetEmailVerifyCodeKey(normalizedEmail)
	codeHash := s.hashEmailCode(normalizedEmail, code)
	codeTTL := time.Duration(s.emailCfg.VerificationCodeTTLSeconds) * time.Second
	cooldownTTL := time.Duration(s.emailCfg.VerificationSendCooldownSecs) * time.Second

	if err := s.redisClient.Set(ctx, codeKey, codeHash, codeTTL).Err(); err != nil {
		return wrapServiceError("验证码存储失败", err)
	}
	// 重新发码 → 清空旧的失败计数。
	_ = s.redisClient.Del(ctx, cache.GetEmailVerifyFailKey(normalizedEmail)).Err()
	if err := s.redisClient.Set(ctx, cooldownKey, "1", cooldownTTL).Err(); err != nil {
		return wrapServiceError("验证码冷却存储失败", err)
	}

	if err := s.sendVerificationEmail(normalizedEmail, code); err != nil {
		_ = s.redisClient.Del(ctx, codeKey, cooldownKey).Err()
		return err
	}

	return nil
}

// Register 处理用户注册
func (s *AuthServiceImpl) Register(req *models.RegisterRequest, clientIP, userAgent string) (*models.AuthResponse, error) {
	siteCfg, err := s.siteConfigRepo.GetSiteConfig()
	if err != nil {
		return nil, wrapServiceError("读取站点配置失败", err)
	}
	if !siteCfg.RegistrationEnabled {
		return nil, apperr.New(apperr.ErrForbidden, "管理员已关闭新用户注册")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if err := validateRegisterRequest(req, siteCfg.EmailVerificationRequired); err != nil {
		return nil, apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	normalizedEmail := normalizeEmail(req.Email)

	if siteCfg.EmailVerificationRequired {
		if err := s.verifyEmailCode(normalizedEmail, req.EmailCode); err != nil {
			return nil, err
		}
	}

	// 检查用户名是否存在
	exists, err := s.userRepo.UserExistsByUsername(req.Username)
	if err != nil {
		return nil, wrapServiceError("检查用户名失败", err)
	}
	if exists {
		return nil, apperr.New(apperr.ErrConflict, "用户名已被使用")
	}

	// 检查邮箱是否存在
	if normalizedEmail != "" {
		exists, err := s.userRepo.UserExistsByEmail(normalizedEmail)
		if err != nil {
			return nil, wrapServiceError("检查邮箱失败", err)
		}
		if exists {
			return nil, apperr.New(apperr.ErrConflict, "邮箱已被使用")
		}
	}

	req.Email = normalizedEmail

	// 加密密码
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, wrapServiceError("密码加密失败", err)
	}

	// 创建用户
	user, err := s.userRepo.CreateUser(req, hashedPassword, clientIP)
	if err != nil {
		return nil, wrapServiceError("创建用户失败", err)
	}

	now := time.Now()
	if updateErr := s.userRepo.UpdateLastLogin(user.ID, clientIP); updateErr == nil {
		user.LastLoginAt = &now
	}
	_ = s.userRepo.LogActivity(user.ID, "login", clientIP)

	// 注册场景默认开启「记住我」（user 角色），方便新用户跨会话保持登录。
	rememberMe := req.RememberMe
	if !rememberMe {
		rememberMe = true
	}
	return s.issueTokenPair(user, clientIP, userAgent, rememberMe)
}

// Login 处理用户登录。
//
// 防暴力破解策略：
//  1. 入口先校验 IP 锁定状态；
//  2. 解析账户后再校验账户锁定状态；
//  3. 密码校验失败时累加 IP 与账户级失败计数，达到阈值即触发对应锁定；
//  4. 登录成功立即清空双维度失败计数（锁定标记不会被解除，由 TTL 自然过期）。
//
// 失败文案统一为「用户名或密码错误」，避免账户枚举；触发锁定时改返 429 + Retry-After。
func (s *AuthServiceImpl) Login(req *models.LoginRequest, clientIP, userAgent string) (*models.AuthResponse, error) {
	if err := validateLoginRequest(req); err != nil {
		return nil, apperr.New(apperr.ErrBadRequest, fmt.Sprintf("请求验证失败: %v", err))
	}

	guardCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.loginGuard.EnsureNotLocked(guardCtx, clientIP, ""); err != nil {
		return nil, err
	}

	identifier := resolveLoginIdentifier(req)
	var user *models.User
	var err error
	if isValidEmail(identifier) {
		normalizedEmail := normalizeEmail(identifier)
		user, err = s.userRepo.GetActiveUserByEmail(normalizedEmail)
	} else {
		user, err = s.userRepo.GetUserByUsername(identifier)
	}
	if err != nil {
		// 未知账户：仍然累加 IP 失败计数，避免攻击者通过不存在的账户低成本枚举。
		if locked, _ := s.loginGuard.RecordFailure(guardCtx, clientIP, ""); locked {
			ttl := s.loginGuard.IPLockTTL(guardCtx, clientIP)
			return nil, apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, "登录失败次数过多，请稍后重试", int(ttl.Seconds()))
		}
		return nil, apperr.New(apperr.ErrUnauthorized, "用户名或密码错误")
	}

	if err := s.loginGuard.EnsureNotLocked(guardCtx, clientIP, user.ID); err != nil {
		return nil, err
	}

	if err := utils.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		lockedAccount, lockedIP := s.loginGuard.RecordFailure(guardCtx, clientIP, user.ID)
		if lockedAccount || lockedIP {
			retryAfter := 0
			if lockedAccount {
				retryAfter = int(s.loginGuard.AccountLockTTL(guardCtx, user.ID).Seconds())
			}
			if retryAfter <= 0 {
				retryAfter = int(s.loginGuard.IPLockTTL(guardCtx, clientIP).Seconds())
			}
			return nil, apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, "登录失败次数过多，已临时锁定", retryAfter)
		}
		return nil, apperr.New(apperr.ErrUnauthorized, "用户名或密码错误")
	}

	s.loginGuard.RecordSuccess(guardCtx, clientIP, user.ID)

	now := time.Now()
	if updateErr := s.userRepo.UpdateLastLogin(user.ID, clientIP); updateErr == nil {
		user.LastLoginAt = &now
	}
	_ = s.userRepo.LogLogin(user.ID, clientIP)

	return s.issueTokenPair(user, clientIP, userAgent, req.RememberMe)
}

// Refresh 使用 refresh token 轮换签发新令牌对。
func (s *AuthServiceImpl) Refresh(refreshToken, clientIP, userAgent string) (*models.AuthResponse, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, apperr.New(apperr.ErrBadRequest, "缺少 refresh_token")
	}

	refreshClaims, err := utils.VerifyTokenByType(refreshToken, utils.TokenTypeRefresh)
	if err != nil {
		return nil, apperr.New(apperr.ErrUnauthorized, "refresh token 无效")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	active, err := s.sessionStore.IsRefreshSessionActive(ctx, refreshToken, refreshClaims.UserID)
	if err != nil {
		return nil, wrapServiceError("刷新会话校验失败", err)
	}
	if !active {
		return nil, apperr.New(apperr.ErrUnauthorized, "refresh token 已失效")
	}

	user, err := s.userRepo.GetUserByID(refreshClaims.UserID)
	if err != nil {
		return nil, apperr.New(apperr.ErrUnauthorized, "用户状态异常，请重新登录")
	}

	if err := s.sessionStore.RevokeRefreshSession(ctx, refreshToken, refreshClaims.UserID); err != nil {
		return nil, wrapServiceError("撤销旧 refresh token 失败", err)
	}

	_ = s.userRepo.LogActivity(user.ID, "online", clientIP)

	// 沿用旧 refresh claims 中记录的 remember_me，避免依赖 TTL 长度推断。
	return s.issueTokenPair(user, clientIP, userAgent, refreshClaims.RememberMe)
}

// Logout 登出当前会话。
func (s *AuthServiceImpl) Logout(userID, accessJTI, refreshToken string) error {
	if userID == "" || accessJTI == "" {
		return apperr.New(apperr.ErrBadRequest, "无效的会话标识")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.sessionStore.RevokeSession(ctx, userID, accessJTI); err != nil {
		return wrapServiceError("登出失败", err)
	}

	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken != "" {
		refreshClaims, err := utils.VerifyTokenByType(refreshToken, utils.TokenTypeRefresh)
		if err == nil && refreshClaims.UserID == userID {
			if err := s.sessionStore.RevokeRefreshSession(ctx, refreshToken, userID); err != nil {
				return wrapServiceError("撤销 refresh 会话失败", err)
			}
		}
	}

	// 单会话角色：登出当前 jti 时若指针仍指向它则一并清除，避免 user_current 残留。
	if user, err := s.userRepo.GetUserByID(userID); err == nil && s.isSingleSessionRole(user.Role) {
		if isCurrent, _ := s.sessionStore.IsUserCurrentSession(ctx, userID, accessJTI); isCurrent {
			_ = s.sessionStore.ClearUserCurrentSession(ctx, userID)
		}
	}

	_ = s.userRepo.LogActivity(userID, "logout", "")

	return nil
}

// RevokeUserSessions 撤销指定用户全部会话。
func (s *AuthServiceImpl) RevokeUserSessions(userID string) error {
	if userID == "" {
		return apperr.New(apperr.ErrBadRequest, "缺少用户ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.sessionStore.RevokeAllUserSessions(ctx, userID); err != nil {
		return wrapServiceError("撤销用户会话失败", err)
	}

	return nil
}

// verifyPasswordResetCode 校验密码重置验证码。
//
// 防暴力破解：失败计数器与 code key 同生命周期，达到 EmailCfg.VerifyFailLimit 时立即作废
// code key，要求重新申请。计数器在比较失败「之后」累加，在比较成功后随 code key 一并删除。
func (s *AuthServiceImpl) verifyPasswordResetCode(email, rawCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	codeKey := cache.GetPasswordResetCodeKey(email)
	storedHash, err := s.redisClient.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return apperr.New(apperr.ErrBadRequest, "验证码已过期或不存在，请重新获取")
	}
	if err != nil {
		return wrapServiceError("验证码校验失败", err)
	}

	computedHash := s.hashPasswordResetCode(email, strings.TrimSpace(rawCode))
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(computedHash)) != 1 {
		return s.handleEmailCodeMismatch(ctx, codeKey, cache.GetPasswordResetVerifyFailKey(email))
	}

	pipe := s.redisClient.Pipeline()
	pipe.Del(ctx, codeKey)
	pipe.Del(ctx, cache.GetPasswordResetVerifyFailKey(email))
	if _, err := pipe.Exec(ctx); err != nil {
		return wrapServiceError("验证码状态更新失败", err)
	}

	return nil
}

// verifyEmailCode 校验注册验证码，与密码重置共享暴力破解防护逻辑。
func (s *AuthServiceImpl) verifyEmailCode(email, rawCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	codeKey := cache.GetEmailVerifyCodeKey(email)
	storedHash, err := s.redisClient.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return apperr.New(apperr.ErrBadRequest, "验证码已过期或不存在，请重新获取")
	}
	if err != nil {
		return wrapServiceError("验证码校验失败", err)
	}

	computedHash := s.hashEmailCode(email, strings.TrimSpace(rawCode))
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(computedHash)) != 1 {
		return s.handleEmailCodeMismatch(ctx, codeKey, cache.GetEmailVerifyFailKey(email))
	}

	pipe := s.redisClient.Pipeline()
	pipe.Del(ctx, codeKey)
	pipe.Del(ctx, cache.GetEmailVerifyFailKey(email))
	if _, err := pipe.Exec(ctx); err != nil {
		return wrapServiceError("验证码状态更新失败", err)
	}

	return nil
}

// handleEmailCodeMismatch 在哈希比对失败时累加失败计数，超阈值即作废验证码并返回统一错误。
//
// 失败计数 TTL 与验证码 TTL 同步（不超过验证码生命周期），保证计数器随 code 一同消亡。
func (s *AuthServiceImpl) handleEmailCodeMismatch(ctx context.Context, codeKey, failCounterKey string) error {
	codeTTL, _ := s.redisClient.TTL(ctx, codeKey).Result()
	count, err := s.redisClient.Incr(ctx, failCounterKey).Result()
	if err == nil && count == 1 {
		expire := codeTTL
		if expire <= 0 {
			expire = time.Duration(s.emailCfg.VerificationCodeTTLSeconds) * time.Second
		}
		_ = s.redisClient.Expire(ctx, failCounterKey, expire).Err()
	}

	limit := s.emailCfg.VerifyFailLimit
	if err == nil && limit > 0 && int(count) >= limit {
		// 达到阈值：作废验证码并清理计数器，强制用户重新发码。
		pipe := s.redisClient.Pipeline()
		pipe.Del(ctx, codeKey)
		pipe.Del(ctx, failCounterKey)
		_, _ = pipe.Exec(ctx)
		return apperr.New(apperr.ErrTooManyRequests, "验证码错误次数过多，请重新获取验证码")
	}

	return apperr.New(apperr.ErrBadRequest, "邮箱验证码错误")
}

func (s *AuthServiceImpl) checkPasswordResetDailyLimit(ctx context.Context, email string) error {
	day := time.Now().Format("20060102")
	dailyKey := cache.GetPasswordResetDailyLimitKey(email, day)

	count, err := s.redisClient.Incr(ctx, dailyKey).Result()
	if err != nil {
		return wrapServiceError("验证码频率统计失败", err)
	}
	if count == 1 {
		_ = s.redisClient.Expire(ctx, dailyKey, 24*time.Hour).Err()
	}

	if int(count) > s.emailCfg.VerificationDailyLimit {
		return apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, "验证码发送已达当日上限，请明日再试", 86400)
	}

	return nil
}

func (s *AuthServiceImpl) checkAndIncreaseDailyLimit(ctx context.Context, email string) error {
	day := time.Now().Format("20060102")
	dailyKey := cache.GetEmailVerifyDailyLimitKey(email, day)

	count, err := s.redisClient.Incr(ctx, dailyKey).Result()
	if err != nil {
		return wrapServiceError("验证码频率统计失败", err)
	}
	if count == 1 {
		_ = s.redisClient.Expire(ctx, dailyKey, 24*time.Hour).Err()
	}

	if int(count) > s.emailCfg.VerificationDailyLimit {
		return apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, "验证码发送已达当日上限，请明日再试", 86400)
	}

	return nil
}

func (s *AuthServiceImpl) verifyBotToken(botToken, clientIP string) error {
	if !s.botCfg.Enabled {
		return nil
	}

	token := strings.TrimSpace(botToken)
	if token == "" {
		return apperr.New(apperr.ErrBadRequest, "请先完成机器人验证")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		ip = "unknown"
	}

	form := url.Values{}
	form.Set("secret", strings.TrimSpace(s.botCfg.TurnstileSecretKey))
	form.Set("response", token)
	// 不向 siteverify 传 remoteip：反代环境下常见 127.0.0.1 与用户真实出口 IP 不一致，会导致校验失败。
	// 不在调用 Cloudflare 之前做本地 IP 限流：失败场景（如 timeout-or-duplicate）会反复消耗配额，误锁正常用户。

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(s.botCfg.TurnstileVerifyURL), strings.NewReader(form.Encode()))
	if err != nil {
		return wrapServiceError("机器人验证请求构建失败", err)
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
		log.Printf("[turnstile] siteverify failed status=%d error_codes=%v client_ip_seen=%q", resp.StatusCode, verifyResp.Errors, ip)
		codes := strings.TrimSpace(strings.Join(verifyResp.Errors, ","))
		if codes != "" {
			return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("机器人验证未通过（%s）", codes))
		}
		return apperr.New(apperr.ErrBadRequest, "机器人验证未通过")
	}

	if err := s.limitBotVerifyByIP(ctx, ip); err != nil {
		return err
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
		return wrapServiceError("机器人验证限流失败", err)
	}
	if count == 1 {
		_ = s.redisClient.Expire(ctx, key, 60*time.Second).Err()
	}
	if count > 30 {
		return apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, "请求过于频繁，请稍后再试", 60)
	}

	return nil
}

func (s *AuthServiceImpl) sendPasswordResetEmail(toEmail, code string) error {
	if strings.TrimSpace(s.emailCfg.ResendAPIKey) == "" || strings.TrimSpace(s.emailCfg.ResendFromEmail) == "" {
		return apperr.New(apperr.ErrInternal, "邮件服务未配置，请联系管理员")
	}

	payload := map[string]any{
		"from":    s.emailCfg.ResendFromEmail,
		"to":      []string{toEmail},
		"subject": "CTF 平台密码重置验证码",
		"html": fmt.Sprintf(
			"<div style=\"font-family:Arial,sans-serif;line-height:1.7;\"><h2>密码重置</h2><p>你正在重置登录密码，验证码为：</p><p style=\"font-size:24px;font-weight:700;letter-spacing:3px;\">%s</p><p>验证码 %d 秒内有效。如非本人操作请忽略本邮件。</p></div>",
			code,
			s.emailCfg.VerificationCodeTTLSeconds,
		),
		"text": fmt.Sprintf("密码重置验证码：%s，%d 秒内有效。如非本人操作请忽略。", code, s.emailCfg.VerificationCodeTTLSeconds),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return wrapServiceError("邮件请求构建失败", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(body)))
	if err != nil {
		return wrapServiceError("邮件请求创建失败", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.emailCfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return apperr.New(apperr.ErrInternal, "邮件发送失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apperr.New(apperr.ErrInternal, "邮件发送失败，请稍后重试")
	}

	return nil
}

func (s *AuthServiceImpl) sendVerificationEmail(toEmail, code string) error {
	if strings.TrimSpace(s.emailCfg.ResendAPIKey) == "" || strings.TrimSpace(s.emailCfg.ResendFromEmail) == "" {
		return apperr.New(apperr.ErrInternal, "邮件服务未配置，请联系管理员")
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
		return wrapServiceError("邮件请求构建失败", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(body)))
	if err != nil {
		return wrapServiceError("邮件请求创建失败", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.emailCfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return apperr.New(apperr.ErrInternal, "邮件发送失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apperr.New(apperr.ErrInternal, "邮件发送失败，请稍后重试")
	}

	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func resolveLoginIdentifier(req *models.LoginRequest) string {
	if req == nil {
		return ""
	}
	identifier := strings.TrimSpace(req.Identifier)
	if identifier != "" {
		return identifier
	}
	return strings.TrimSpace(req.Username)
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

// hashPasswordResetCode 使用 HMAC-SHA256(secret, "pwdreset:" + email + ":" + code) 生成验证码哈希。
//
// 引入服务端密钥后，即便数据库泄露，攻击者也无法对 6 位数字空间做彩虹表/枚举攻击。
func (s *AuthServiceImpl) hashPasswordResetCode(email, code string) string {
	mac := hmac.New(sha256.New, s.codeHMACKey)
	mac.Write([]byte("pwdreset:"))
	mac.Write([]byte(email))
	mac.Write([]byte(":"))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// hashEmailCode 使用 HMAC-SHA256(secret, "register:" + email + ":" + code) 生成注册验证码哈希。
func (s *AuthServiceImpl) hashEmailCode(email, code string) string {
	mac := hmac.New(sha256.New, s.codeHMACKey)
	mac.Write([]byte("register:"))
	mac.Write([]byte(email))
	mac.Write([]byte(":"))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// 验证注册请求。emailVerificationRequired 为 true 时必须提供 6 位数字邮箱验证码；为 false 时不校验验证码（仍可忽略客户端误传）。
func validateRegisterRequest(req *models.RegisterRequest, emailVerificationRequired bool) error {
	trimmedUsername := strings.TrimSpace(req.Username)
	if trimmedUsername == "" {
		return apperr.New(apperr.ErrBadRequest, "用户名不能为空")
	}
	if !usernameRegisterPattern.MatchString(trimmedUsername) {
		return apperr.New(apperr.ErrBadRequest, "用户名须为 3-32 位，仅允许英文字母、数字与下划线（与数据库约束一致）")
	}
	trimmedEmail := strings.TrimSpace(req.Email)
	if trimmedEmail == "" {
		return apperr.New(apperr.ErrBadRequest, "邮箱不能为空")
	}
	if !isValidEmail(trimmedEmail) {
		return apperr.New(apperr.ErrBadRequest, "邮箱格式不正确")
	}
	if err := utils.ValidatePasswordStrength(req.Password); err != nil {
		return err
	}
	if emailVerificationRequired {
		code := strings.TrimSpace(req.EmailCode)
		if len(code) != 6 {
			return apperr.New(apperr.ErrBadRequest, "请输入 6 位邮箱验证码")
		}
		for _, ch := range code {
			if ch < '0' || ch > '9' {
				return apperr.New(apperr.ErrBadRequest, "邮箱验证码应为 6 位数字")
			}
		}
	}
	return nil
}

func validateSendPasswordResetCodeRequest(req *models.SendPasswordResetCodeRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return apperr.New(apperr.ErrBadRequest, "邮箱不能为空")
	}
	if !isValidEmail(req.Email) {
		return apperr.New(apperr.ErrBadRequest, "邮箱格式不正确")
	}
	return nil
}

func validateResetPasswordWithCodeRequest(req *models.ResetPasswordWithCodeRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return apperr.New(apperr.ErrBadRequest, "邮箱不能为空")
	}
	if !isValidEmail(req.Email) {
		return apperr.New(apperr.ErrBadRequest, "邮箱格式不正确")
	}
	code := strings.TrimSpace(req.EmailCode)
	if len(code) != 6 {
		return apperr.New(apperr.ErrBadRequest, "请输入 6 位邮箱验证码")
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return apperr.New(apperr.ErrBadRequest, "邮箱验证码应为 6 位数字")
		}
	}
	if err := utils.ValidatePasswordStrength(req.NewPassword); err != nil {
		return err
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
	identifier := resolveLoginIdentifier(req)
	if identifier == "" {
		return apperr.New(apperr.ErrBadRequest, "用户名或邮箱不能为空")
	}
	if strings.TrimSpace(req.Password) == "" {
		return apperr.New(apperr.ErrBadRequest, "密码不能为空")
	}
	return nil
}
