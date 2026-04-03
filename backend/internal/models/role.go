package models

import "strings"

const (
	RoleRoot         = "root"
	RoleSiteAdmin    = "site_admin"
	RoleContestAdmin = "contest_admin"
	RoleUser         = "user"
)

// NormalizeRole 统一角色字符串格式。
func NormalizeRole(role string) string {
	normalized := strings.ToLower(strings.TrimSpace(role))
	if normalized == "admin" {
		// 兼容历史 token / 数据中的旧角色名。
		return RoleSiteAdmin
	}

	return normalized
}

// IsValidRole 校验角色是否在系统支持范围内。
func IsValidRole(role string) bool {
	switch NormalizeRole(role) {
	case RoleRoot, RoleSiteAdmin, RoleContestAdmin, RoleUser:
		return true
	default:
		return false
	}
}
