package models

import "time"

// User 用户模型
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // 不序列化到 JSON
	Role         string     `json:"role"`
	DisplayName  string     `json:"display_name"`
	AvatarURL    string     `json:"avatar_url"`
	Bio          string     `json:"bio"`
	WebsiteURL   string     `json:"website_url"`
	Organization string     `json:"organization"`
	Country      string     `json:"country"`
	Score        int        `json:"score"`
	SolvedCount  int        `json:"solved_count"`
	RankNum      *int       `json:"rank_num"`
	IsActive     bool       `json:"is_active"`
	IsVerified   bool       `json:"is_verified"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Preferences  string     `json:"preferences,omitempty"` // JSON 字符串存储用户偏好
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	EmailCode  string `json:"email_code"`
	BotToken   string `json:"bot_token"`
	RememberMe bool   `json:"remember_me"`
}

// SendRegisterCodeRequest 发送注册邮箱验证码请求
type SendRegisterCodeRequest struct {
	Email    string `json:"email"`
	BotToken string `json:"bot_token"`
}

// RegistrationHints 注册页公开策略（不含密钥），供前端与公开接口使用。
type RegistrationHints struct {
	RegistrationEnabled       bool `json:"registration_enabled"`
	EmailVerificationRequired bool `json:"email_verification_required"`
	BotProtectionEnabled      bool `json:"bot_protection_enabled"`
}

// SendPasswordResetCodeRequest 发送密码重置邮箱验证码
type SendPasswordResetCodeRequest struct {
	Email    string `json:"email"`
	BotToken string `json:"bot_token"`
}

// ResetPasswordWithCodeRequest 使用邮箱验证码重置密码
type ResetPasswordWithCodeRequest struct {
	Email       string `json:"email"`
	EmailCode   string `json:"email_code"`
	NewPassword string `json:"new_password"`
}

// LoginRequest 登陆请求（支持用户名或邮箱）
type LoginRequest struct {
	Identifier string `json:"identifier"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Token                string `json:"token"`
	RefreshToken         string `json:"refresh_token"`
	AccessExpiresInSecs  int64  `json:"access_expires_in_secs"`
	RefreshExpiresInSecs int64  `json:"refresh_expires_in_secs"`
	TokenType            string `json:"token_type"`
	User                 *User  `json:"user"`
	// SessionID 当前会话的 access token jti，前端用于 WS 鉴权或登出。
	SessionID string `json:"sid,omitempty"`
	// RememberMe 实际生效的 remember_me 状态（admin 角色会被强制为 false）。
	RememberMe bool `json:"remember_me"`
}

// RefreshTokenRequest 刷新 access token 请求。
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest 登出请求（可选携带 refresh token 以撤销当前设备刷新会话）。
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// UpdateProfileRequest 更新个人资料请求
type UpdateProfileRequest struct {
	DisplayName  string       `json:"display_name"`
	Email        string       `json:"email"`
	Bio          string       `json:"bio"`
	WebsiteURL   string       `json:"website_url"`
	Organization string       `json:"organization"`
	Country      string       `json:"country"`
	EmailPublic  bool         `json:"email_public"` // 是否公开邮箱
	GithubURL    string       `json:"github_url"`
	SocialLinks  []SocialLink `json:"social_links"`
}

type SocialLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type ProfileStats struct {
	FollowersCount       int  `json:"followers_count"`
	FollowingCount       int  `json:"following_count"`
	ContestsParticipated int  `json:"contests_participated"`
	RankNum              *int `json:"rank_num"`
}

type UserProfileResponse struct {
	User
	Stats        ProfileStats `json:"stats"`
	IsOwnProfile bool         `json:"is_own_profile"`
	IsFollowing  *bool        `json:"is_following"`
	IsFollowedBy *bool        `json:"is_followed_by"`
	IsOnline     bool         `json:"is_online"`
	LastActiveAt *time.Time   `json:"last_active_at"`
}

type FollowUser struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	AvatarURL    string `json:"avatar_url"`
	Bio          string `json:"bio"`
	IsFollowing  *bool  `json:"is_following"`
	IsFollowedBy *bool  `json:"is_followed_by"`
}

// UpdateUserRoleRequest root 修改用户角色请求
type UpdateUserRoleRequest struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

// UpdateUserActiveRequest 管理端更新用户激活状态。
type UpdateUserActiveRequest struct {
	IsActive bool   `json:"is_active"`
	Reason   string `json:"reason"`
}

// AdminUserListItem root 管理页用户列表项。
type AdminUserListItem struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	IsActive    bool      `json:"is_active"`
	IsVerified  bool      `json:"is_verified"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AdminRoleChangeLogItem root 角色变更审计日志项。
type AdminRoleChangeLogItem struct {
	ID               string    `json:"id"`
	OperatorUserID   string    `json:"operator_user_id"`
	OperatorUsername string    `json:"operator_username"`
	TargetUserID     string    `json:"target_user_id"`
	TargetUsername   string    `json:"target_username"`
	OldRole          string    `json:"old_role"`
	NewRole          string    `json:"new_role"`
	Reason           string    `json:"reason"`
	IP               string    `json:"ip"`
	CreatedAt        time.Time `json:"created_at"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
