package models

import "strings"

const (
	RoleRoot  = "root"
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// NormalizeRole 统一角色字符串格式。
func NormalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

// IsValidRole 校验角色是否在系统支持范围内。
func IsValidRole(role string) bool {
	switch NormalizeRole(role) {
	case RoleRoot, RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}
