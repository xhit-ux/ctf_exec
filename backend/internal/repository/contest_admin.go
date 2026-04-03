package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/google/uuid"
)

// ContestAdminRepository 定义赛事管理员数据访问能力。
type ContestAdminRepository interface {
	IsContestAdmin(userID, contestID string) (bool, error)
	GetContestAdminRole(userID, contestID string) (string, error)
	AddContestAdmin(userID, contestID, role, grantedBy string) error
	RemoveContestAdminByUsername(username, contestID string) error
	CountContestAdmins(contestID string) (int, error)
	ListContestAdmins(contestID string) ([]*models.ContestAdmin, error)
}

// ContestAdminRepositoryImpl 赛事管理员仓储实现。
type ContestAdminRepositoryImpl struct {
	db *sql.DB
}

func NewContestAdminRepository(db *sql.DB) *ContestAdminRepositoryImpl {
	return &ContestAdminRepositoryImpl{db: db}
}

func (r *ContestAdminRepositoryImpl) IsContestAdmin(userID, contestID string) (bool, error) {
	var count int
	query := `
		SELECT COUNT(1)
		FROM contest_admins
		WHERE contest_id = ? AND user_id = ? AND role = 'admin'
	`
	if err := r.db.QueryRow(query, contestID, userID).Scan(&count); err != nil {
		return false, fmt.Errorf("查询赛事管理员失败: %w", err)
	}

	return count > 0, nil
}

func (r *ContestAdminRepositoryImpl) GetContestAdminRole(userID, contestID string) (string, error) {
	var role string
	err := r.db.QueryRow(`
		SELECT role
		FROM contest_admins
		WHERE contest_id = ? AND user_id = ?
		LIMIT 1
	`, contestID, userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("查询赛事管理员角色失败: %w", err)
	}

	return strings.ToLower(strings.TrimSpace(role)), nil
}

func (r *ContestAdminRepositoryImpl) AddContestAdmin(userID, contestID, role, grantedBy string) error {
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	if normalizedRole == "" {
		normalizedRole = models.ContestAdminRoleAdmin
	}
	if normalizedRole != models.ContestAdminRoleAdmin && normalizedRole != models.ContestAdminRoleObserver {
		return apperr.New(apperr.ErrBadRequest, "无效的赛事管理员角色")
	}

	query := `
		INSERT INTO contest_admins (id, contest_id, user_id, role, granted_by)
		VALUES (?, ?, ?, ?, ?)
	`
	if _, err := r.db.Exec(query, uuid.New().String(), contestID, userID, normalizedRole, grantedBy); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return apperr.New(apperr.ErrConflict, "用户已是该赛事管理员")
		}
		return fmt.Errorf("添加赛事管理员失败: %w", err)
	}

	return nil
}

func (r *ContestAdminRepositoryImpl) RemoveContestAdminByUsername(username, contestID string) error {
	result, err := r.db.Exec(`
		DELETE ca
		FROM contest_admins ca
		JOIN users u ON u.id = ca.user_id
		WHERE ca.contest_id = ? AND u.username = ?
	`, contestID, strings.TrimSpace(username))
	if err != nil {
		return fmt.Errorf("移除赛事管理员失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "赛事管理员关系不存在")
	}

	return nil
}

func (r *ContestAdminRepositoryImpl) CountContestAdmins(contestID string) (int, error) {
	var count int
	if err := r.db.QueryRow(`SELECT COUNT(1) FROM contest_admins WHERE contest_id = ?`, contestID).Scan(&count); err != nil {
		return 0, fmt.Errorf("统计赛事管理员失败: %w", err)
	}

	return count, nil
}

func (r *ContestAdminRepositoryImpl) ListContestAdmins(contestID string) ([]*models.ContestAdmin, error) {
	query := `
		SELECT ca.id, ca.contest_id, ca.user_id, COALESCE(u.username, ''), ca.role, ca.granted_by, ca.granted_at
		FROM contest_admins ca
		LEFT JOIN users u ON u.id = ca.user_id
		WHERE ca.contest_id = ?
		ORDER BY granted_at ASC
	`
	rows, err := r.db.Query(query, contestID)
	if err != nil {
		return nil, fmt.Errorf("查询赛事管理员列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.ContestAdmin, 0)
	for rows.Next() {
		item := &models.ContestAdmin{}
		if err := rows.Scan(&item.ID, &item.ContestID, &item.UserID, &item.Username, &item.Role, &item.GrantedBy, &item.GrantedAt); err != nil {
			return nil, fmt.Errorf("扫描赛事管理员失败: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}
