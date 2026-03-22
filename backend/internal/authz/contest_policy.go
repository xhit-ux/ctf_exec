package authz

import (
	"time"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
)

// CanManageContest 判断用户是否有权管理指定赛事。
func CanManageContest(user *models.User, contestID string, repo repository.ContestAdminRepository) (bool, error) {
	if user == nil {
		return false, nil
	}

	role := models.NormalizeRole(user.Role)
	if role == models.RoleRoot || role == models.RoleSiteAdmin {
		return true, nil
	}

	if repo == nil {
		return false, nil
	}

	return repo.IsContestAdmin(user.ID, contestID)
}

// CanEditProblemInContest 判断是否可以在赛事中管理题目。
func CanEditProblemInContest(user *models.User, contest *models.Contest, contestID string, repo repository.ContestAdminRepository) (bool, error) {
	if contest == nil {
		return false, nil
	}

	if contest.EndTime != nil && time.Now().After(*contest.EndTime) {
		return false, nil
	}

	return CanManageContest(user, contestID, repo)
}

// CanOperateUser 判断操作者是否可以对目标用户执行敏感操作。
func CanOperateUser(actor, target *models.User) bool {
	if actor == nil || target == nil {
		return false
	}

	targetRole := models.NormalizeRole(target.Role)
	if targetRole == models.RoleRoot {
		return false
	}

	actorRole := models.NormalizeRole(actor.Role)
	if actorRole == models.RoleRoot {
		return true
	}

	return actorRole == models.RoleSiteAdmin && targetRole == models.RoleUser
}
