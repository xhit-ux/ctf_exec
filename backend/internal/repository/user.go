package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/google/uuid"
)

// UserRepository 用户数据访问层
type UserRepository struct {
	db *sql.DB
}

// UpdateUserRole root 更新用户角色并写入审计日志。
func (r *UserRepository) UpdateUserRole(operatorID, targetUserID, newRole, reason, ipAddress string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var oldRole string
	selectQuery := `SELECT role FROM users WHERE id = ? AND is_active = 1 FOR UPDATE`
	if scanErr := tx.QueryRow(selectQuery, targetUserID).Scan(&oldRole); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			err = fmt.Errorf("目标用户不存在")
			return err
		}
		err = fmt.Errorf("查询目标用户角色失败: %w", scanErr)
		return err
	}

	if oldRole == newRole {
		err = fmt.Errorf("角色未发生变化")
		return err
	}

	updateQuery := `UPDATE users SET role = ?, updated_at = NOW() WHERE id = ?`
	if _, execErr := tx.Exec(updateQuery, newRole, targetUserID); execErr != nil {
		err = fmt.Errorf("更新用户角色失败: %w", execErr)
		return err
	}

	details := map[string]string{
		"operator_id": operatorID,
		"old_role":    oldRole,
		"new_role":    newRole,
		"reason":      reason,
	}
	detailsJSON, marshalErr := json.Marshal(details)
	if marshalErr != nil {
		err = fmt.Errorf("构造审计日志详情失败: %w", marshalErr)
		return err
	}

	insertLogQuery := `
		INSERT INTO user_activity_logs (id, user_id, action_type, target_id, details, ip_address, created_at)
		VALUES (?, ?, 'role_change', ?, ?, ?, ?)
	`
	if _, execErr := tx.Exec(insertLogQuery, uuid.New().String(), operatorID, targetUserID, string(detailsJSON), ipAddress, time.Now()); execErr != nil {
		err = fmt.Errorf("写入角色变更审计日志失败: %w", execErr)
		return err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("提交事务失败: %w", commitErr)
	}

	return nil
}

// NewUserRepository 创建用户仓库实例
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser 创建新用户
func (r *UserRepository) CreateUser(req *models.RegisterRequest, passwordHash string) (*models.User, error) {
	userID := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO users (
			id, username, email, password_hash, role, display_name, 
			is_active, is_verified, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(
		query,
		userID,
		req.Username,
		req.Email,
		passwordHash,
		"user",       // 默认角色
		req.Username, // 初始显示名称同用户名
		1,            // is_active
		1,            // is_verified
		now,
		now,
	)

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "Duplicate entry") {
			return nil, fmt.Errorf("用户名已被使用")
		}
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return &models.User{
		ID:          userID,
		Username:    req.Username,
		Email:       req.Email,
		Role:        "user",
		DisplayName: req.Username,
		IsActive:    true,
		IsVerified:  true,
		Score:       0,
		SolvedCount: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetUserByUsername 通过用户名获取用户
func (r *UserRepository) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, role, display_name, 
		       COALESCE(avatar_url, ''), COALESCE(bio, ''), score, solved_count, is_active, is_verified, 
		       last_login_at, created_at, updated_at,
		       COALESCE(preferences, '{}')
		FROM users WHERE username = ? AND is_active = 1
	`

	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.Role, &user.DisplayName, &user.AvatarURL, &user.Bio,
		&user.Score, &user.SolvedCount, &user.IsActive, &user.IsVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		&user.Preferences,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return user, nil
}

// GetUserByID 通过ID获取用户
func (r *UserRepository) GetUserByID(id string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, role, display_name, 
		       COALESCE(avatar_url, ''), COALESCE(bio, ''), score, solved_count, is_active, is_verified, 
		       last_login_at, created_at, updated_at,
		       COALESCE(preferences, '{}')
		FROM users WHERE id = ? AND is_active = 1
	`

	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.Role, &user.DisplayName, &user.AvatarURL, &user.Bio,
		&user.Score, &user.SolvedCount, &user.IsActive, &user.IsVerified,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		&user.Preferences,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return user, nil
}

// GetLeaderboard 获取排行榜
func (r *UserRepository) GetLeaderboard(limit int) ([]*models.User, error) {
	var users []*models.User
	query := `
		SELECT id, username, md5(email), role, display_name, 
		       COALESCE(avatar_url, ''), COALESCE(bio, ''), score, solved_count, is_active, is_verified, 
		       created_at
		FROM users 
		WHERE is_active = 1
		ORDER BY score DESC, solved_count DESC, created_at ASC
		LIMIT ?
	`
	// Note: I removed password_hash, updated_at etc. to keep it lighter,
	// AND used md5(email) to pretend we map email to gravatar-ish id if needed,
	// but actually the struct expectation requires matching fields.
	// Let's stick to filling the struct.

	query = `
		SELECT id, username, email, role, display_name, 
		       COALESCE(avatar_url, ''), COALESCE(bio, ''), score, solved_count, is_active, is_verified, 
		       created_at, updated_at
		FROM users 
		WHERE is_active = 1
		ORDER BY score DESC, solved_count DESC, created_at ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询排行榜失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		user := &models.User{}
		var email string
		// We select email but maybe we shouldn't expose it publically?
		// For now, I'll select it but the frontend might hide it.
		err := rows.Scan(
			&user.ID, &user.Username, &email, &user.Role, &user.DisplayName,
			&user.AvatarURL, &user.Bio, &user.Score, &user.SolvedCount, &user.IsActive, &user.IsVerified,
			&user.CreatedAt, &user.UpdatedAt,
		)
		user.Email = email // Assign
		if err != nil {
			return nil, fmt.Errorf("扫描用户行失败: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// UpdateLastLogin 更新最后登陆时间
func (r *UserRepository) UpdateLastLogin(userID string) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = ?`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("更新登陆时间失败: %w", err)
	}
	return nil
}

// UserExistsByUsername 检查用户名是否存在
func (r *UserRepository) UserExistsByUsername(username string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE username = ? AND is_active = 1`
	err := r.db.QueryRow(query, username).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查用户存在失败: %w", err)
	}
	return count > 0, nil
}

// UserExistsByEmail 检查邮箱是否存在
func (r *UserRepository) UserExistsByEmail(email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = ? AND is_active = 1`
	err := r.db.QueryRow(query, email).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查邮箱存在失败: %w", err)
	}
	return count > 0, nil
}

// UpdateAvatarURL 更新用户头像 URL
func (r *UserRepository) UpdateAvatarURL(userID, avatarURL string) error {
	query := `UPDATE users SET avatar_url = ?, updated_at = NOW() WHERE id = ?`
	result, err := r.db.Exec(query, avatarURL, userID)
	if err != nil {
		return fmt.Errorf("更新头像失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("用户不存在")
	}

	return nil
}

// UpdateProfile 更新用户信息
func (r *UserRepository) UpdateProfile(userID string, req *models.UpdateProfileRequest) error {
	// 使用 JSON_SET 来设置 email_public 字段
	// 使用 CAST(? AS JSON) 确保布尔值被正确存储为 JSON 布尔值
	// Go的bool类型在MySQL中可能被转换为整数1/0，所以需要转换为字符串'true'/'false'
	var emailPublicStr string
	if req.EmailPublic {
		emailPublicStr = "true"
	} else {
		emailPublicStr = "false"
	}

	query := `
		UPDATE users 
		SET display_name = ?, email = ?, bio = ?, 
			preferences = JSON_SET(
				COALESCE(preferences, '{}'),
				'$.email_public', CAST(? AS JSON)
			),
			updated_at = NOW() 
		WHERE id = ? AND is_active = 1
	`
	params := []interface{}{req.DisplayName, req.Email, req.Bio, emailPublicStr, userID}

	result, err := r.db.Exec(query, params...)
	if err != nil {
		return fmt.Errorf("更新用户信息失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("用户不存在或无权更新")
	}

	return nil
}

// GetAllUsers 获取所有用户（用于排行榜同步）
func (r *UserRepository) GetAllUsers() ([]*models.User, error) {
	var users []*models.User
	query := `
		SELECT id, username, email, role, display_name, 
		       COALESCE(avatar_url, ''), COALESCE(bio, ''), score, solved_count, is_active, is_verified, 
		       created_at, updated_at
		FROM users 
		WHERE is_active = 1
		ORDER BY score DESC, solved_count DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询所有用户失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.Role, &user.DisplayName,
			&user.AvatarURL, &user.Bio, &user.Score, &user.SolvedCount, &user.IsActive, &user.IsVerified,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描用户行失败: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}
