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
	Score        int        `json:"score"`
	SolvedCount  int        `json:"solved_count"`
	IsActive     bool       `json:"is_active"`
	IsVerified   bool       `json:"is_verified"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Preferences  string     `json:"preferences,omitempty"` // JSON 字符串存储用户偏好
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	EmailCode string `json:"email_code"`
	BotToken  string `json:"bot_token"`
}

// SendRegisterCodeRequest 发送注册邮箱验证码请求
type SendRegisterCodeRequest struct {
	Email    string `json:"email"`
	BotToken string `json:"bot_token"`
}

// LoginRequest 登陆请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// UpdateProfileRequest 更新个人资料请求
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Bio         string `json:"bio"`
	EmailPublic bool   `json:"email_public"` // 是否公开邮箱
}

// UpdateUserRoleRequest root 修改用户角色请求
type UpdateUserRoleRequest struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
