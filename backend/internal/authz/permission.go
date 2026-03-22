package authz

import (
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
)

// PermissionChecker 聚合权限判断，避免在 handler/service 中散落判断逻辑。
type PermissionChecker struct {
	ContestAdminRepo repository.ContestAdminRepository
}

func NewPermissionChecker(contestAdminRepo repository.ContestAdminRepository) *PermissionChecker {
	return &PermissionChecker{ContestAdminRepo: contestAdminRepo}
}

// CanManageContest 统一赛事管理权限入口。
func (p *PermissionChecker) CanManageContest(user *models.User, contestID string) (bool, error) {
	return CanManageContest(user, contestID, p.ContestAdminRepo)
}
