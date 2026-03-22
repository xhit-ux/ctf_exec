package service

import (
	"fmt"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
)

// UserService 用户服务接口
type UserService interface {
	GetLeaderboard(limit int) ([]*models.User, error)
	GetCurrentUser(userID string) (*models.User, error)
	UpdateProfile(userID string, req *models.UpdateProfileRequest) error
	GetPublicProfile(userID string) (*models.User, error)
}

// UserServiceImpl 用户服务实现
type UserServiceImpl struct {
	userRepo *repository.UserRepository
}

// NewUserService 创建用户服务实例
func NewUserService(userRepo *repository.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: userRepo}
}

// GetLeaderboard 获取排行榜
func (s *UserServiceImpl) GetLeaderboard(limit int) ([]*models.User, error) {
	if limit <= 0 {
		limit = 50 // 默认值
	}
	if limit > 100 {
		limit = 100 // 限制最大值
	}

	users, err := s.userRepo.GetLeaderboard(limit)
	if err != nil {
		return nil, fmt.Errorf("获取排行榜失败: %w", err)
	}

	return users, nil
}

// GetCurrentUser 获取当前用户信息
func (s *UserServiceImpl) GetCurrentUser(userID string) (*models.User, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	return user, nil
}

// UpdateProfile 更新用户资料
func (s *UserServiceImpl) UpdateProfile(userID string, req *models.UpdateProfileRequest) error {
	// 检查邮箱是否已被其他用户使用
	if req.Email != "" {
		exists, err := s.userRepo.UserExistsByEmail(req.Email)
		if err != nil {
			return fmt.Errorf("检查邮箱失败: %w", err)
		}
		if exists {
			// 需要检查是否是当前用户自己的邮箱
			user, err := s.userRepo.GetUserByID(userID)
			if err != nil {
				return fmt.Errorf("获取用户信息失败: %w", err)
			}
			if user.Email != req.Email {
				return apperr.New(apperr.ErrConflict, "邮箱已被其他用户使用")
			}
		}
	}

	// 更新用户资料
	err := s.userRepo.UpdateProfile(userID, req)
	if err != nil {
		return fmt.Errorf("更新用户资料失败: %w", err)
	}

	return nil
}

// GetPublicProfile 获取公开用户资料
func (s *UserServiceImpl) GetPublicProfile(userID string) (*models.User, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %w", err)
	}

	// 隐藏敏感信息
	user.Email = ""
	user.PasswordHash = ""
	user.IsActive = false   // 不公开
	user.IsVerified = false // 不公开

	return user, nil
}
