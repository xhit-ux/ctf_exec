package authz

import (
	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
)

// CanAccessRole 判断当前角色是否允许访问目标角色资源。
func CanAccessRole(currentRole string, allowedRoles ...string) bool {
	if len(allowedRoles) == 0 {
		return false
	}

	normalizedCurrent := models.NormalizeRole(currentRole)
	for _, role := range allowedRoles {
		if normalizedCurrent == models.NormalizeRole(role) {
			return true
		}
	}

	return false
}

// CanPatchUserRole 判断角色变更操作是否被允许。
func CanPatchUserRole(actorUserID, targetUserID, actorRole, targetRole string) error {
	normalizedTargetRole := models.NormalizeRole(targetRole)
	if !models.IsValidRole(normalizedTargetRole) {
		return apperr.New(apperr.ErrBadRequest, "非法角色变更")
	}

	// root 不能把自己降级成非 root，避免系统丢失最高权限账户。
	if actorUserID == targetUserID && normalizedTargetRole != models.RoleRoot {
		return apperr.New(apperr.ErrBadRequest, "禁止 root 自降级")
	}

	_ = actorRole // 预留给后续扩展更细粒度 RBAC 策略。
	return nil
}

// IsTeamLeader 判断当前用户是否为队长。
func IsTeamLeader(actorUserID, leaderUserID string) bool {
	if actorUserID == "" || leaderUserID == "" {
		return false
	}

	return actorUserID == leaderUserID
}
