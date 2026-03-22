package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// GenerateInviteCode 生成随机邀请码
func (r *TeamRepository) GenerateInviteCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seed := rand.NewSource(time.Now().UnixNano())
	random := rand.New(seed)

	code := make([]byte, 8)
	for i := range code {
		code[i] = charset[random.Intn(len(charset))]
	}
	return string(code)
}

// CreateTeam 创建队伍
func (r *TeamRepository) CreateTeam(userID string, name string) (*models.Team, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. 检查用户是否已在队伍中
	var memberCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM team_members WHERE user_id = ?", userID).Scan(&memberCount)
	if err != nil {
		return nil, err
	}
	if memberCount > 0 {
		return nil, apperr.New(apperr.ErrConflict, "你已经在队伍中了")
	}

	// 2. 创建队伍
	teamID := uuid.New().String()
	inviteCode := r.GenerateInviteCode()
	now := time.Now()

	_, err = tx.Exec(`
		INSERT INTO teams (id, name, invite_code, leader_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, teamID, name, inviteCode, userID, now, now)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, apperr.New(apperr.ErrConflict, "队伍名称已存在")
		}
		return nil, fmt.Errorf("创建队伍失败: %w", err)
	}

	// 3. 添加成员
	_, err = tx.Exec(`
		INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
		VALUES (?, ?, ?, ?)
	`, teamID, userID, "leader", now)
	if err != nil {
		return nil, fmt.Errorf("添加成员失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &models.Team{
		ID:         teamID,
		Name:       name,
		InviteCode: inviteCode,
		LeaderID:   userID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// GetTeamByID 获取队伍详情 (包含动态计算的积分和解题数)
func (r *TeamRepository) GetTeamByID(teamID string) (*models.Team, error) {
	team := &models.Team{}
	err := r.db.QueryRow(`
		SELECT t.id, t.name, t.invite_code, t.leader_id, 
		       COALESCE(SUM(u.score), 0) as total_score, 
		       COALESCE(SUM(u.solved_count), 0) as total_solved, 
		       t.created_at, t.updated_at
		FROM teams t
		LEFT JOIN team_members tm ON t.id = tm.team_id
		LEFT JOIN users u ON tm.user_id = u.id
		WHERE t.id = ?
		GROUP BY t.id
	`, teamID).Scan(
		&team.ID, &team.Name, &team.InviteCode, &team.LeaderID,
		&team.Score, &team.SolvedCount, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "队伍不存在")
		}
		return nil, err
	}
	return team, nil
}

// GetTeamMembers 获取队伍成员
func (r *TeamRepository) GetTeamMembers(teamID string) ([]*models.TeamMemberUser, error) {
	rows, err := r.db.Query(`
		SELECT 
			u.id, u.username, u.email, u.password_hash, u.role, u.display_name,
			COALESCE(u.avatar_url, ''), COALESCE(u.bio, ''), u.score, u.solved_count,
			u.is_active, u.is_verified, u.last_login_at, u.created_at, u.updated_at,
			COALESCE(u.preferences, '{}'), tm.role_in_team
		FROM users u
		JOIN team_members tm ON u.id = tm.user_id
		WHERE tm.team_id = ?
		ORDER BY tm.joined_at ASC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]*models.TeamMemberUser, 0)
	for rows.Next() {
		member := &models.TeamMemberUser{}
		err := rows.Scan(
			&member.ID, &member.Username, &member.Email, &member.PasswordHash, &member.Role, &member.DisplayName,
			&member.AvatarURL, &member.Bio, &member.Score, &member.SolvedCount,
			&member.IsActive, &member.IsVerified, &member.LastLoginAt, &member.CreatedAt, &member.UpdatedAt,
			&member.Preferences, &member.RoleInTeam,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, nil
}

// JoinTeam 加入队伍 (通过邀请码)
func (r *TeamRepository) JoinTeam(userID string, inviteCode string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 获取队伍信息
	var teamID string
	err = tx.QueryRow("SELECT id FROM teams WHERE invite_code = ?", inviteCode).Scan(&teamID)
	if err != nil {
		if err == sql.ErrNoRows {
			return apperr.New(apperr.ErrBadRequest, "无效的邀请码")
		}
		return err
	}

	// 2. 检查人数限制 (最多5人)
	var memberCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM team_members WHERE team_id = ?", teamID).Scan(&memberCount)
	if err != nil {
		return err
	}
	if memberCount >= 5 {
		return apperr.New(apperr.ErrConflict, "队伍已满 (上限 5 人)")
	}

	// 3. 检查用户是否已在队伍中
	var userInTeamCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM team_members WHERE user_id = ?", userID).Scan(&userInTeamCount)
	if err != nil {
		return err
	}
	if userInTeamCount > 0 {
		return apperr.New(apperr.ErrConflict, "你已经在队伍中了，请先退出当前队伍")
	}

	// 4. 加入队伍
	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO team_members (team_id, user_id, role_in_team, joined_at)
		VALUES (?, ?, ?, ?)
	`, teamID, userID, "member", now)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetTeamRanking 获取团队排行榜 (按人均积分)
func (r *TeamRepository) GetTeamRanking(limit int) ([]*models.TeamLeaderboardEntry, error) {
	ranking, _, err := r.GetTeamRankingPage(0, limit)
	if err != nil {
		return nil, err
	}

	return ranking, nil
}

// GetTeamRankingPage 分页获取团队排行榜（按人均积分）
func (r *TeamRepository) GetTeamRankingPage(offset int, limit int) ([]*models.TeamLeaderboardEntry, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM leaderboard_global WHERE board_type = 'team'`).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			COALESCE(ref_id, ''),
			COALESCE(team_name, ''),
			total_score,
			avg_score,
			member_count,
			rank_num
		FROM leaderboard_global
		WHERE board_type = 'team'
		ORDER BY rank_num ASC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ranking := make([]*models.TeamLeaderboardEntry, 0)
	for rows.Next() {
		entry := &models.TeamLeaderboardEntry{}
		err := rows.Scan(&entry.ID, &entry.Name, &entry.TotalScore, &entry.AvgScore, &entry.MemberCount, &entry.Rank)
		if err != nil {
			return nil, 0, err
		}
		ranking = append(ranking, entry)
	}
	return ranking, total, nil
}

// RefreshGlobalTeamLeaderboardSnapshot 刷新团队全站排行榜快照
func (r *TeamRepository) RefreshGlobalTeamLeaderboardSnapshot(snapshotAt time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启刷新团队榜单事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM leaderboard_global WHERE board_type = 'team'`); err != nil {
		return fmt.Errorf("清理团队榜单快照失败: %w", err)
	}

	insertSQL := `
		INSERT INTO leaderboard_global (
			board_type, rank_num, ref_id, team_name, username, display_name, avatar_url,
			score, solved_count, total_score, avg_score, member_count, snapshot_at
		)
		SELECT
			'team' AS board_type,
			ranked.rank_num,
			ranked.id,
			ranked.name,
			NULL,
			NULL,
			NULL,
			0,
			0,
			ranked.total_score,
			ranked.avg_score,
			ranked.member_count,
			?
		FROM (
			SELECT
				t.id,
				t.name,
				IFNULL(SUM(u.score), 0) AS total_score,
				IFNULL(AVG(u.score), 0) AS avg_score,
				COUNT(u.id) AS member_count,
				(@rn := @rn + 1) AS rank_num
			FROM teams t
			JOIN team_members tm ON t.id = tm.team_id
			JOIN users u ON tm.user_id = u.id AND u.is_active = 1
			JOIN (SELECT @rn := 0) r
			GROUP BY t.id, t.name
			ORDER BY avg_score DESC, total_score DESC, t.id ASC
		) ranked
	`

	if _, err = tx.Exec(insertSQL, snapshotAt); err != nil {
		return fmt.Errorf("写入团队榜单快照失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交团队榜单快照事务失败: %w", err)
	}

	return nil
}

// UpdateTeam 更新队伍信息
func (r *TeamRepository) UpdateTeam(teamID string, name string) error {
	_, err := r.db.Exec("UPDATE teams SET name = ?, updated_at = ? WHERE id = ?", name, time.Now(), teamID)
	return err
}

// KickMember 踢出成员
func (r *TeamRepository) KickMember(teamID string, userID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 检查被踢用户是否在队伍中
	var memberTeamID string
	var role string
	err = tx.QueryRow("SELECT team_id, role_in_team FROM team_members WHERE user_id = ?", userID).Scan(&memberTeamID, &role)
	if err != nil {
		return apperr.New(apperr.ErrNotFound, "成员不存在或不在队伍中")
	}
	if memberTeamID != teamID {
		return apperr.New(apperr.ErrForbidden, "该成员不属于你的队伍")
	}
	if role == "leader" {
		return apperr.New(apperr.ErrBadRequest, "不能踢出队长")
	}

	// 2. 删除成员记录
	_, err = tx.Exec("DELETE FROM team_members WHERE user_id = ?", userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// LeaveTeam 退出队伍
func (r *TeamRepository) LeaveTeam(userID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 获取队伍信息和角色
	var teamID string
	var role string
	err = tx.QueryRow("SELECT team_id, role_in_team FROM team_members WHERE user_id = ?", userID).Scan(&teamID, &role)
	if err != nil {
		return apperr.New(apperr.ErrNotFound, "你不在任何队伍中")
	}

	if role == "leader" {
		return apperr.New(apperr.ErrBadRequest, "队长不能直接退出，请先解散队伍或转让队长")
	}

	// 2. 删除成员记录
	_, err = tx.Exec("DELETE FROM team_members WHERE user_id = ?", userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DissolveTeam 解散队伍
func (r *TeamRepository) DissolveTeam(userID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 验证是否是队长
	var teamID string
	var role string
	err = tx.QueryRow("SELECT team_id, role_in_team FROM team_members WHERE user_id = ?", userID).Scan(&teamID, &role)
	if err != nil || role != "leader" {
		return apperr.New(apperr.ErrForbidden, "仅队长可以解散队伍")
	}

	// 2. 删除所有成员记录
	_, err = tx.Exec("DELETE FROM team_members WHERE team_id = ?", teamID)
	if err != nil {
		return err
	}

	// 3. 删除队伍记录
	_, err = tx.Exec("DELETE FROM teams WHERE id = ?", teamID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetTeamByUserID 获取用户所属的队伍
func (r *TeamRepository) GetTeamByUserID(userID string) (*models.Team, error) {
	var teamID sql.NullString
	err := r.db.QueryRow("SELECT team_id FROM team_members WHERE user_id = ?", userID).Scan(&teamID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "不在队伍中")
		}
		return nil, err
	}
	if !teamID.Valid {
		return nil, apperr.New(apperr.ErrNotFound, "不在队伍中")
	}

	team, err := r.GetTeamByID(teamID.String)
	if err != nil {
		return nil, err
	}

	members, err := r.GetTeamMembers(team.ID)
	if err == nil {
		team.Members = members
	}

	return team, nil
}

// GetAllTeams 获取所有团队（用于排行榜同步）
func (r *TeamRepository) GetAllTeams() ([]*models.Team, error) {
	var teams []*models.Team
	query := `
		SELECT t.id, t.name, t.invite_code, t.leader_id, 
		       COALESCE(SUM(u.score), 0) as total_score, 
		       COALESCE(SUM(u.solved_count), 0) as total_solved, 
		       t.created_at, t.updated_at
		FROM teams t
		LEFT JOIN team_members tm ON t.id = tm.team_id
		LEFT JOIN users u ON tm.user_id = u.id
		GROUP BY t.id
		ORDER BY total_score DESC, total_solved DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询所有团队失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		team := &models.Team{}
		err := rows.Scan(
			&team.ID, &team.Name, &team.InviteCode, &team.LeaderID,
			&team.Score, &team.SolvedCount, &team.CreatedAt, &team.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描团队行失败: %w", err)
		}
		teams = append(teams, team)
	}

	return teams, nil
}
