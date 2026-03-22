package utils

import (
	"fmt"

	"github.com/ctf_exec/backend/internal/apperr"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword 加密密码
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("密码加密失败: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func VerifyPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return apperr.New(apperr.ErrUnauthorized, "密码错误")
	}
	return nil
}
