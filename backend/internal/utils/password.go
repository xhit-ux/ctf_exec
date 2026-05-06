package utils

import (
	"fmt"

	"github.com/ctf_exec/backend/internal/apperr"
	"golang.org/x/crypto/bcrypt"
)

// 密码强度策略：仅作用于「新设密码」（注册 / 重置 / 改密），登录时不重新校验，避免影响存量弱密码用户。
const (
	PasswordMinLength = 8
	PasswordMaxLength = 128
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

// ValidatePasswordStrength 校验「新设密码」是否满足强度要求：
//   - 长度 [PasswordMinLength, PasswordMaxLength]
//   - 至少包含一个字母与一个数字（特殊字符不强制，但允许）
//
// 仅在注册 / 密码重置 / 修改密码时调用；登录时不再调用，避免影响历史弱密码用户。
func ValidatePasswordStrength(password string) error {
	if len(password) < PasswordMinLength {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("密码长度至少 %d 个字符", PasswordMinLength))
	}
	if len(password) > PasswordMaxLength {
		return apperr.New(apperr.ErrBadRequest, fmt.Sprintf("密码长度不能超过 %d 个字符", PasswordMaxLength))
	}

	hasLetter := false
	hasDigit := false
	for _, ch := range password {
		switch {
		case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z'):
			hasLetter = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return apperr.New(apperr.ErrBadRequest, "密码须同时包含字母与数字")
	}
	return nil
}
