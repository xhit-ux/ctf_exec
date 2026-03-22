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
	normalizedActorRole := models.NormalizeRole(actorRole)
	normalizedTargetRole := models.NormalizeRole(targetRole)

	if normalizedActorRole != models.RoleRoot {
		return apperr.New(apperr.ErrForbidden, "仅 root 可修改全局角色")
	}

	if !models.IsValidRole(normalizedTargetRole) {
		return apperr.New(apperr.ErrBadRequest, "非法角色变更")
	}

	// root 账号保持唯一：禁止将其他用户提升为 root。
	if actorUserID != targetUserID && normalizedTargetRole == models.RoleRoot {
		return apperr.New(apperr.ErrForbidden, "root 账号必须保持唯一，禁止提权为 root")
	}

	// root 不能把自己降级成非 root，避免系统丢失最高权限账户。
	if actorUserID == targetUserID && normalizedTargetRole != models.RoleRoot {
		return apperr.New(apperr.ErrBadRequest, "禁止 root 自降级")
	}

	return nil
}

// IsTeamLeader 判断当前用户是否为队长。
func IsTeamLeader(actorUserID, leaderUserID string) bool {
	if actorUserID == "" || leaderUserID == "" {
		return false
	}

	return actorUserID == leaderUserID
}
