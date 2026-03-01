package service

import (
	"fmt"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

// AuthService 认证服务接口
type AuthService interface {
	Register(req *models.RegisterRequest) (*models.AuthResponse, error)
	Login(req *models.LoginRequest) (*models.AuthResponse, error)
}

// AuthServiceImpl 认证服务实现
type AuthServiceImpl struct {
	userRepo *repository.UserRepository
}

// NewAuthService 创建认证服务实例
func NewAuthService(userRepo *repository.UserRepository) *AuthServiceImpl {
	return &AuthServiceImpl{userRepo: userRepo}
}

// Register 处理用户注册
func (s *AuthServiceImpl) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	// 验证输入
	if err := validateRegisterRequest(req); err != nil {
		return nil, fmt.Errorf("请求验证失败: %w", err)
	}

	// 检查用户名是否存在
	exists, err := s.userRepo.UserExistsByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("检查用户名失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("用户名已被使用")
	}

	// 检查邮箱是否存在
	if req.Email != "" {
		exists, err := s.userRepo.UserExistsByEmail(req.Email)
		if err != nil {
			return nil, fmt.Errorf("检查邮箱失败: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("邮箱已被使用")
		}
	}

	// 加密密码
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user, err := s.userRepo.CreateUser(req, hashedPassword)
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
func (s *AuthServiceImpl) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	// 验证输入
	if err := validateLoginRequest(req); err != nil {
		return nil, fmt.Errorf("请求验证失败: %w", err)
	}

	// 查询用户
	user, err := s.userRepo.GetUserByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 验证密码
	if err := utils.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 更新最后登陆时间
	_ = s.userRepo.UpdateLastLogin(user.ID)

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

// 验证注册请求
func validateRegisterRequest(req *models.RegisterRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return fmt.Errorf("用户名长度应在 3-32 个字符之间")
	}
	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	if len(req.Password) < 6 {
		return fmt.Errorf("密码长度至少 6 个字符")
	}
	if len(req.Password) > 128 {
		return fmt.Errorf("密码长度不能超过 128 个字符")
	}
	return nil
}

// 验证登录请求
func validateLoginRequest(req *models.LoginRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("密码不能为空")
	}
	return nil
}