package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
)

// EnsureDefaultRootUser 启动时确保存在 root 用户，不存在时按默认密码创建。
func EnsureDefaultRootUser(userRepo *repository.UserRepository, rootDefaultPassword string) error {
	password := strings.TrimSpace(rootDefaultPassword)
	if password == "" {
		return fmt.Errorf("ROOT_DEFAULT_PASSWORD 未配置，无法初始化默认 root 用户")
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("默认 root 密码哈希失败: %w", err)
	}

	created, err := userRepo.EnsureRootUser(hashedPassword)
	if err != nil {
		return err
	}

	if created {
		log.Println("✓ 默认 root 用户已创建")
		return nil
	}

	log.Println("✓ root 用户已存在，跳过默认初始化")
	return nil
}
