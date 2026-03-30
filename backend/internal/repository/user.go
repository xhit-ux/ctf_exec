package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/go-sql-driver/mysql"
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
			err = apperr.New(apperr.ErrNotFound, "目标用户不存在")
			return err
		}
		err = fmt.Errorf("查询目标用户角色失败: %w", scanErr)
		return err
	}

	if oldRole == newRole {
		err = apperr.New(apperr.ErrBadRequest, "角色未发生变化")
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

// UpdateUserActiveStatus 管理端更新用户激活状态并写入审计日志。
func (r *UserRepository) UpdateUserActiveStatus(operatorID, targetUserID string, isActive bool, reason, ipAddress string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var oldActive bool
	var targetRole string
	query := `SELECT is_active, role FROM users WHERE id = ? FOR UPDATE`
	if scanErr := tx.QueryRow(query, targetUserID).Scan(&oldActive, &targetRole); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			err = apperr.New(apperr.ErrNotFound, "目标用户不存在")
			return err
		}
		err = fmt.Errorf("查询目标用户状态失败: %w", scanErr)
		return err
	}

	if models.NormalizeRole(targetRole) == models.RoleRoot {
		err = apperr.New(apperr.ErrForbidden, "禁止修改 root 账号状态")
		return err
	}

	if oldActive == isActive {
		err = apperr.New(apperr.ErrBadRequest, "状态未发生变化")
		return err
	}

	if _, execErr := tx.Exec(`UPDATE users SET is_active = ?, updated_at = NOW() WHERE id = ?`, isActive, targetUserID); execErr != nil {
		err = fmt.Errorf("更新用户状态失败: %w", execErr)
		return err
	}

	details := map[string]interface{}{
		"operator_id":   operatorID,
		"old_is_active": oldActive,
		"new_is_active": isActive,
		"reason":        reason,
	}
	detailsJSON, marshalErr := json.Marshal(details)
	if marshalErr != nil {
		err = fmt.Errorf("构造状态变更审计日志详情失败: %w", marshalErr)
		return err
	}

	insertLogQuery := `
		INSERT INTO user_activity_logs (id, user_id, action_type, target_id, details, ip_address, created_at)
		VALUES (?, ?, 'user_status_change', ?, ?, ?, ?)
	`
	if _, execErr := tx.Exec(insertLogQuery, uuid.New().String(), operatorID, targetUserID, string(detailsJSON), ipAddress, time.Now()); execErr != nil {
		err = fmt.Errorf("写入状态变更审计日志失败: %w", execErr)
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
func (r *UserRepository) CreateUser(req *models.RegisterRequest, passwordHash, registrationIP string) (*models.User, error) {
	userID := uuid.New().String()
	now := time.Now()
	safeRegistrationIP := normalizeIPAddress(registrationIP)

	query := `
		INSERT INTO users (
			id, username, email, password_hash, role, display_name, 
			is_active, is_verified, registration_ip, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		safeRegistrationIP,
		now,
		now,
	)

	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, apperr.New(apperr.ErrConflict, "用户名已被使用")
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
		       COALESCE(avatar_url, ''), COALESCE(bio, ''),
		       COALESCE(website_url, ''), COALESCE(organization, ''), COALESCE(country, ''),
		       score, solved_count, is_active, is_verified, rank_num,
		       last_login_at, created_at, updated_at,
		       COALESCE(preferences, '{}')
		FROM users WHERE username = ? AND is_active = 1
	`

	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.Role, &user.DisplayName, &user.AvatarURL, &user.Bio,
		&user.WebsiteURL, &user.Organization, &user.Country,
		&user.Score, &user.SolvedCount, &user.IsActive, &user.IsVerified,
		&user.RankNum, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		&user.Preferences,
	)

	if err == sql.ErrNoRows {
		return nil, apperr.New(apperr.ErrNotFound, "用户不存在")
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
		       COALESCE(avatar_url, ''), COALESCE(bio, ''),
		       COALESCE(website_url, ''), COALESCE(organization, ''), COALESCE(country, ''),
		       score, solved_count, is_active, is_verified, rank_num,
		       last_login_at, created_at, updated_at,
		       COALESCE(preferences, '{}')
		FROM users WHERE id = ? AND is_active = 1
	`

	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.Role, &user.DisplayName, &user.AvatarURL, &user.Bio,
		&user.WebsiteURL, &user.Organization, &user.Country,
		&user.Score, &user.SolvedCount, &user.IsActive, &user.IsVerified,
		&user.RankNum, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		&user.Preferences,
	)

	if err == sql.ErrNoRows {
		return nil, apperr.New(apperr.ErrNotFound, "用户不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return user, nil
}

// GetLeaderboard 获取排行榜
func (r *UserRepository) GetLeaderboard(limit int) ([]*models.User, error) {
	users, _, err := r.GetLeaderboardPage(0, limit)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// GetLeaderboardPage 分页获取排行榜
func (r *UserRepository) GetLeaderboardPage(offset int, limit int) ([]*models.User, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}

	var total int
	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM leaderboard_global lg
		JOIN users u ON u.id = lg.ref_id
		WHERE lg.board_type = 'user' AND u.role <> 'root'
	`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计排行榜总人数失败: %w", err)
	}

	var users []*models.User

	query := `
		SELECT lg.ref_id, lg.username, COALESCE(lg.display_name, ''), COALESCE(lg.avatar_url, ''), lg.score, lg.solved_count
		FROM leaderboard_global lg
		JOIN users u ON u.id = lg.ref_id
		WHERE lg.board_type = 'user' AND u.role <> 'root'
		ORDER BY lg.rank_num ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询排行榜失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Score, &user.SolvedCount,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描用户行失败: %w", err)
		}
		users = append(users, user)
	}

	return users, total, nil
}

// GetLeaderboardUserByUsername 通过用户名获取排行榜中的用户和排名
func (r *UserRepository) GetLeaderboardUserByUsername(username string) (*models.User, int, error) {
	queryUser := `
		SELECT lg.ref_id, lg.username, COALESCE(lg.display_name, ''), COALESCE(lg.avatar_url, ''), lg.score, lg.solved_count,
			(
				SELECT COUNT(*)
				FROM leaderboard_global lg2
				JOIN users u2 ON u2.id = lg2.ref_id
				WHERE lg2.board_type = 'user' AND u2.role <> 'root' AND lg2.rank_num <= lg.rank_num
			) AS rank_filtered
		FROM leaderboard_global lg
		JOIN users u ON u.id = lg.ref_id
		WHERE lg.board_type = 'user' AND lg.username = ? AND u.role <> 'root'
		LIMIT 1
	`

	user := &models.User{}
	var rank int
	err := r.db.QueryRow(queryUser, username).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Score, &user.SolvedCount, &rank,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, apperr.New(apperr.ErrNotFound, "用户不存在")
		}
		return nil, 0, fmt.Errorf("查询用户失败: %w", err)
	}

	return user, rank, nil
}

// RefreshGlobalUserLeaderboardSnapshot 刷新用户全站排行榜快照
func (r *UserRepository) RefreshGlobalUserLeaderboardSnapshot(snapshotAt time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启刷新用户榜单事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM leaderboard_global WHERE board_type = 'user'`); err != nil {
		return fmt.Errorf("清理用户榜单快照失败: %w", err)
	}

	insertSQL := `
		INSERT INTO leaderboard_global (
			board_type, rank_num, ref_id, username, display_name, avatar_url,
			score, solved_count, total_score, avg_score, member_count, snapshot_at
		)
		SELECT
			'user' AS board_type,
			ranked.rank_num,
			ranked.id,
			ranked.username,
			ranked.display_name,
			ranked.avatar_url,
			ranked.score,
			ranked.solved_count,
			0,
			0,
			0,
			?
		FROM (
			SELECT
				u.id,
				u.username,
				COALESCE(u.display_name, '') AS display_name,
				COALESCE(u.avatar_url, '') AS avatar_url,
				u.score,
				u.solved_count,
				(@rn := @rn + 1) AS rank_num
			FROM users u
			JOIN (SELECT @rn := 0) r
			WHERE u.is_active = 1 AND u.role <> 'root'
			ORDER BY u.score DESC, u.solved_count DESC, u.created_at ASC, u.id ASC
		) ranked
	`

	if _, err = tx.Exec(insertSQL, snapshotAt); err != nil {
		return fmt.Errorf("写入用户榜单快照失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交用户榜单快照事务失败: %w", err)
	}

	return nil
}

// UpdateLastLogin 更新最后登陆时间和登录 IP
func (r *UserRepository) UpdateLastLogin(userID, lastLoginIP string) error {
	query := `UPDATE users SET last_login_at = NOW(), last_login_ip = ? WHERE id = ?`
	_, err := r.db.Exec(query, normalizeIPAddress(lastLoginIP), userID)
	if err != nil {
		return fmt.Errorf("更新登陆时间失败: %w", err)
	}
	return nil
}

// UpdateLastActive 更新用户活跃时间，不影响登录时间语义字段。
func (r *UserRepository) UpdateLastActive(userID, ipAddress string) error {
	query := `UPDATE users SET updated_at = NOW() WHERE id = ?`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("更新活跃时间失败: %w", err)
	}
	_ = ipAddress
	return nil
}

// LogLogin 写入用户登录审计日志。
func (r *UserRepository) LogLogin(userID, ipAddress string) error {
	query := `
		INSERT INTO user_activity_logs (id, user_id, action_type, target_id, details, ip_address, created_at)
		VALUES (?, ?, 'login', NULL, NULL, ?, ?)
	`
	_, err := r.db.Exec(query, uuid.New().String(), userID, normalizeIPAddress(ipAddress), time.Now())
	if err != nil {
		return fmt.Errorf("写入登录审计日志失败: %w", err)
	}

	return nil
}

// LogActivity 写入用户活跃审计日志。
func (r *UserRepository) LogActivity(userID, actionType, ipAddress string) error {
	query := `
		INSERT INTO user_activity_logs (id, user_id, action_type, target_id, details, ip_address, created_at)
		VALUES (?, ?, ?, NULL, NULL, ?, ?)
	`
	_, err := r.db.Exec(query, uuid.New().String(), userID, actionType, normalizeIPAddress(ipAddress), time.Now())
	if err != nil {
		return fmt.Errorf("写入活跃日志失败: %w", err)
	}

	return nil
}

// GetLastActiveAt 返回用户最近一次活跃时间，没有记录时返回 nil。
func (r *UserRepository) GetLastActiveAt(userID string) (*time.Time, error) {
	query := `
		SELECT created_at
		FROM user_activity_logs
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	var t time.Time
	err := r.db.QueryRow(query, userID).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询最近活跃时间失败: %w", err)
	}

	return &t, nil
}

func normalizeIPAddress(raw string) string {
	ip := strings.TrimSpace(raw)
	if ip == "" {
		return ""
	}

	// users.last_login_ip / registration_ip 均为 VARCHAR(45)
	if len(ip) > 45 {
		return ip[:45]
	}

	return ip
}

// IncreaseUserScoreAndSolvedCount 增加用户全站积分与解题数。
func (r *UserRepository) IncreaseUserScoreAndSolvedCount(userID string, deltaScore int) error {
	if deltaScore <= 0 {
		return nil
	}

	query := `
		UPDATE users
		SET score = score + ?, solved_count = solved_count + 1, updated_at = NOW()
		WHERE id = ? AND is_active = 1
	`
	result, err := r.db.Exec(query, deltaScore, userID)
	if err != nil {
		return fmt.Errorf("更新用户积分失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取用户积分更新结果失败: %w", err)
	}
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "用户不存在")
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
		return apperr.New(apperr.ErrNotFound, "用户不存在")
	}

	return nil
}

// UpdateProfile 更新用户信息
func (r *UserRepository) UpdateProfile(userID string, req *models.UpdateProfileRequest) error {
	websiteURL := strings.TrimSpace(req.WebsiteURL)
	if websiteURL != "" {
		if _, err := url.ParseRequestURI(websiteURL); err != nil {
			return apperr.New(apperr.ErrBadRequest, "个人网站 URL 格式不正确")
		}
	}

	githubURL := strings.TrimSpace(req.GithubURL)
	if githubURL != "" {
		if _, err := url.ParseRequestURI(githubURL); err != nil {
			return apperr.New(apperr.ErrBadRequest, "GitHub URL 格式不正确")
		}
	}

	if len(req.Bio) > 500 {
		return apperr.New(apperr.ErrBadRequest, "个人简介不能超过 500 字符")
	}

	if len(req.Organization) > 100 {
		return apperr.New(apperr.ErrBadRequest, "组织/学校不能超过 100 字符")
	}

	if country := strings.TrimSpace(req.Country); len(country) > 2 {
		return apperr.New(apperr.ErrBadRequest, "国家/地区应为 2 位国家代码")
	}

	if len(req.SocialLinks) > 5 {
		return apperr.New(apperr.ErrBadRequest, "社交链接最多 5 项")
	}

	for _, link := range req.SocialLinks {
		if strings.TrimSpace(link.Label) == "" || strings.TrimSpace(link.URL) == "" {
			return apperr.New(apperr.ErrBadRequest, "社交链接的标签和 URL 不能为空")
		}
		if _, err := url.ParseRequestURI(strings.TrimSpace(link.URL)); err != nil {
			return apperr.New(apperr.ErrBadRequest, "社交链接 URL 格式不正确")
		}
	}

	socialLinksJSON, err := json.Marshal(req.SocialLinks)
	if err != nil {
		return fmt.Errorf("序列化社交链接失败: %w", err)
	}

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
		SET display_name = ?, email = ?, bio = ?, website_url = ?, organization = ?, country = ?,
			preferences = JSON_SET(
				COALESCE(preferences, '{}'),
				'$.email_public', CAST(? AS JSON),
				'$.github_url', ?,
				'$.social_links', CAST(? AS JSON)
			),
			updated_at = NOW() 
		WHERE id = ? AND is_active = 1
	`
	params := []interface{}{
		req.DisplayName,
		req.Email,
		req.Bio,
		websiteURL,
		strings.TrimSpace(req.Organization),
		strings.ToUpper(strings.TrimSpace(req.Country)),
		emailPublicStr,
		githubURL,
		string(socialLinksJSON),
		userID,
	}

	result, err := r.db.Exec(query, params...)
	if err != nil {
		return fmt.Errorf("更新用户信息失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return apperr.New(apperr.ErrNotFound, "用户不存在或无权更新")
	}

	return nil
}

// FollowUser 创建关注关系。
func (r *UserRepository) FollowUser(followerID, followeeID string) error {
	if followerID == followeeID {
		return apperr.New(apperr.ErrBadRequest, "不能关注自己")
	}

	query := `INSERT INTO user_follows (id, follower_id, followee_id, created_at) VALUES (?, ?, ?, ?)`
	_, err := r.db.Exec(query, uuid.New().String(), followerID, followeeID, time.Now())
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) {
			if mysqlErr.Number == 1062 {
				return apperr.New(apperr.ErrConflict, "已关注该用户")
			}
			if mysqlErr.Number == 1452 {
				return apperr.New(apperr.ErrNotFound, "目标用户不存在")
			}
		}
		return fmt.Errorf("创建关注关系失败: %w", err)
	}

	return nil
}

// UnfollowUser 删除关注关系。
func (r *UserRepository) UnfollowUser(followerID, followeeID string) error {
	query := `DELETE FROM user_follows WHERE follower_id = ? AND followee_id = ?`
	result, err := r.db.Exec(query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("取消关注失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取取消关注结果失败: %w", err)
	}
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "尚未关注该用户")
	}

	return nil
}

// IsFollowing 判断 follower 是否关注了 followee。
func (r *UserRepository) IsFollowing(followerID, followeeID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM user_follows WHERE follower_id = ? AND followee_id = ?`
	if err := r.db.QueryRow(query, followerID, followeeID).Scan(&count); err != nil {
		return false, fmt.Errorf("查询关注状态失败: %w", err)
	}

	return count > 0, nil
}

// GetFollowCounts 获取用户粉丝数和关注数。
func (r *UserRepository) GetFollowCounts(userID string) (followersCount int, followingCount int, err error) {
	// 粉丝数
	if err = r.db.QueryRow(`
		SELECT COUNT(*) 
		FROM user_follows uf
		JOIN users u ON u.id = uf.follower_id AND u.is_active = 1
		WHERE uf.followee_id = ?`, userID).Scan(&followersCount); err != nil {
		return 0, 0, fmt.Errorf("查询粉丝数失败: %w", err)
	}

	// 关注数
	if err = r.db.QueryRow(`
		SELECT COUNT(*) 
		FROM user_follows uf
		JOIN users u ON u.id = uf.followee_id AND u.is_active = 1
		WHERE uf.follower_id = ?`, userID).Scan(&followingCount); err != nil {
		return 0, 0, fmt.Errorf("查询关注数失败: %w", err)
	}

	return followersCount, followingCount, nil
}

// GetContestsParticipatedCount 获取用户参与赛事数量。
func (r *UserRepository) GetContestsParticipatedCount(userID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(DISTINCT cp.contest_id)
		FROM contest_participants cp
		WHERE cp.user_id = ?
			OR cp.team_id IN (
				SELECT tm.team_id
				FROM team_members tm
				WHERE tm.user_id = ?
			)
	`
	if err := r.db.QueryRow(query, userID, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("查询参与赛事数量失败: %w", err)
	}
	return count, nil
}

func boolPtr(v bool) *bool {
	b := v
	return &b
}

// GetFollowers 获取某用户粉丝列表。
func (r *UserRepository) GetFollowers(targetUserID string, viewerUserID string, offset int, limit int, keyword string) ([]*models.FollowUser, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}

	trimmed := strings.TrimSpace(keyword)
	likeKeyword := "%" + trimmed + "%"

	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM user_follows uf
		JOIN users u ON u.id = uf.follower_id AND u.is_active = 1
		WHERE uf.followee_id = ?
			AND (? = '' OR u.username LIKE ? OR u.display_name LIKE ?)
	`
	if err := r.db.QueryRow(countQuery, targetUserID, trimmed, likeKeyword, likeKeyword).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计粉丝总数失败: %w", err)
	}

	query := `
		SELECT
			u.id,
			u.username,
			COALESCE(u.display_name, ''),
			COALESCE(u.avatar_url, ''),
			COALESCE(u.bio, '')
		FROM user_follows uf
		JOIN users u ON u.id = uf.follower_id AND u.is_active = 1
		WHERE uf.followee_id = ?
			AND (? = '' OR u.username LIKE ? OR u.display_name LIKE ?)
		ORDER BY uf.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, targetUserID, trimmed, likeKeyword, likeKeyword, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询粉丝列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.FollowUser, 0)
	for rows.Next() {
		item := &models.FollowUser{}
		if scanErr := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.AvatarURL, &item.Bio); scanErr != nil {
			return nil, 0, fmt.Errorf("扫描粉丝列表失败: %w", scanErr)
		}

		if viewerUserID != "" {
			isFollowing, followErr := r.IsFollowing(viewerUserID, item.ID)
			if followErr != nil {
				return nil, 0, followErr
			}
			isFollowedBy, followedErr := r.IsFollowing(item.ID, viewerUserID)
			if followedErr != nil {
				return nil, 0, followedErr
			}
			item.IsFollowing = boolPtr(isFollowing)
			item.IsFollowedBy = boolPtr(isFollowedBy)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历粉丝列表失败: %w", err)
	}

	return items, total, nil
}

// GetFollowing 获取某用户关注列表。
func (r *UserRepository) GetFollowing(targetUserID string, viewerUserID string, offset int, limit int, keyword string) ([]*models.FollowUser, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}

	trimmed := strings.TrimSpace(keyword)
	likeKeyword := "%" + trimmed + "%"

	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM user_follows uf
		JOIN users u ON u.id = uf.followee_id AND u.is_active = 1
		WHERE uf.follower_id = ?
			AND (? = '' OR u.username LIKE ? OR u.display_name LIKE ?)
	`
	if err := r.db.QueryRow(countQuery, targetUserID, trimmed, likeKeyword, likeKeyword).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计关注总数失败: %w", err)
	}

	query := `
		SELECT
			u.id,
			u.username,
			COALESCE(u.display_name, ''),
			COALESCE(u.avatar_url, ''),
			COALESCE(u.bio, '')
		FROM user_follows uf
		JOIN users u ON u.id = uf.followee_id AND u.is_active = 1
		WHERE uf.follower_id = ?
			AND (? = '' OR u.username LIKE ? OR u.display_name LIKE ?)
		ORDER BY uf.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, targetUserID, trimmed, likeKeyword, likeKeyword, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询关注列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.FollowUser, 0)
	for rows.Next() {
		item := &models.FollowUser{}
		if scanErr := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.AvatarURL, &item.Bio); scanErr != nil {
			return nil, 0, fmt.Errorf("扫描关注列表失败: %w", scanErr)
		}

		if viewerUserID != "" {
			isFollowing, followErr := r.IsFollowing(viewerUserID, item.ID)
			if followErr != nil {
				return nil, 0, followErr
			}
			isFollowedBy, followedErr := r.IsFollowing(item.ID, viewerUserID)
			if followedErr != nil {
				return nil, 0, followedErr
			}
			item.IsFollowing = boolPtr(isFollowing)
			item.IsFollowedBy = boolPtr(isFollowedBy)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历关注列表失败: %w", err)
	}

	return items, total, nil
}

// GetMutualFollowing 获取 viewer 与 target 的共同关注列表。
func (r *UserRepository) GetMutualFollowing(targetUserID, viewerUserID string, offset int, limit int, keyword string) ([]*models.FollowUser, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}

	trimmed := strings.TrimSpace(keyword)
	likeKeyword := "%" + trimmed + "%"

	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM user_follows f1
		JOIN user_follows f2 ON f1.followee_id = f2.followee_id
		JOIN users u ON u.id = f1.followee_id AND u.is_active = 1
		WHERE f1.follower_id = ?
			AND f2.follower_id = ?
			AND (? = '' OR u.username LIKE ? OR u.display_name LIKE ?)
	`
	if err := r.db.QueryRow(countQuery, targetUserID, viewerUserID, trimmed, likeKeyword, likeKeyword).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计共同关注总数失败: %w", err)
	}

	query := `
		SELECT
			u.id,
			u.username,
			COALESCE(u.display_name, ''),
			COALESCE(u.avatar_url, ''),
			COALESCE(u.bio, '')
		FROM user_follows f1
		JOIN user_follows f2 ON f1.followee_id = f2.followee_id
		JOIN users u ON u.id = f1.followee_id AND u.is_active = 1
		WHERE f1.follower_id = ?
			AND f2.follower_id = ?
			AND (? = '' OR u.username LIKE ? OR u.display_name LIKE ?)
		ORDER BY u.username ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, targetUserID, viewerUserID, trimmed, likeKeyword, likeKeyword, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询共同关注列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.FollowUser, 0)
	for rows.Next() {
		item := &models.FollowUser{}
		if scanErr := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.AvatarURL, &item.Bio); scanErr != nil {
			return nil, 0, fmt.Errorf("扫描共同关注列表失败: %w", scanErr)
		}

		isFollowing, followErr := r.IsFollowing(viewerUserID, item.ID)
		if followErr != nil {
			return nil, 0, followErr
		}
		isFollowedBy, followedErr := r.IsFollowing(item.ID, viewerUserID)
		if followedErr != nil {
			return nil, 0, followedErr
		}
		item.IsFollowing = boolPtr(isFollowing)
		item.IsFollowedBy = boolPtr(isFollowedBy)

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历共同关注列表失败: %w", err)
	}

	return items, total, nil
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

// ListAdminUsers root 分页查询用户列表。
func (r *UserRepository) ListAdminUsers(offset int, limit int, keyword string, excludeRoot bool, excludeUserID string) ([]*models.AdminUserListItem, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}

	trimmedKeyword := "%" + keyword + "%"

	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM users
		WHERE (? = '' OR username LIKE ? OR display_name LIKE ? OR email LIKE ?)
			AND (? = 0 OR role <> 'root')
			AND (? = '' OR id <> ?)
	`
	if err := r.db.QueryRow(countQuery, keyword, trimmedKeyword, trimmedKeyword, trimmedKeyword, excludeRoot, excludeUserID, excludeUserID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计用户总数失败: %w", err)
	}

	query := `
		SELECT id, username, COALESCE(display_name, ''), COALESCE(email, ''), role, is_active, is_verified, created_at, updated_at
		FROM users
		WHERE (? = '' OR username LIKE ? OR display_name LIKE ? OR email LIKE ?)
			AND (? = 0 OR role <> 'root')
			AND (? = '' OR id <> ?)
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, keyword, trimmedKeyword, trimmedKeyword, trimmedKeyword, excludeRoot, excludeUserID, excludeUserID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.AdminUserListItem, 0)
	for rows.Next() {
		item := &models.AdminUserListItem{}
		if scanErr := rows.Scan(
			&item.ID,
			&item.Username,
			&item.DisplayName,
			&item.Email,
			&item.Role,
			&item.IsActive,
			&item.IsVerified,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			return nil, 0, fmt.Errorf("扫描用户列表失败: %w", scanErr)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历用户列表失败: %w", err)
	}

	return items, total, nil
}

// ListRoleChangeLogs root 分页查询角色变更日志。
func (r *UserRepository) ListRoleChangeLogs(offset int, limit int) ([]*models.AdminRoleChangeLogItem, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM user_activity_logs WHERE action_type = 'role_change'`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计角色变更日志失败: %w", err)
	}

	query := `
		SELECT
			l.id,
			COALESCE(l.user_id, ''),
			COALESCE(op.username, ''),
			COALESCE(l.target_id, ''),
			COALESCE(tg.username, ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(l.details, '$.old_role')), ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(l.details, '$.new_role')), ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(l.details, '$.reason')), ''),
			COALESCE(l.ip_address, ''),
			l.created_at
		FROM user_activity_logs l
		LEFT JOIN users op ON op.id = l.user_id
		LEFT JOIN users tg ON tg.id = l.target_id
		WHERE l.action_type = 'role_change'
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询角色变更日志失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.AdminRoleChangeLogItem, 0)
	for rows.Next() {
		item := &models.AdminRoleChangeLogItem{}
		if scanErr := rows.Scan(
			&item.ID,
			&item.OperatorUserID,
			&item.OperatorUsername,
			&item.TargetUserID,
			&item.TargetUsername,
			&item.OldRole,
			&item.NewRole,
			&item.Reason,
			&item.IP,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, 0, fmt.Errorf("扫描角色变更日志失败: %w", scanErr)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历角色变更日志失败: %w", err)
	}

	return items, total, nil
}
