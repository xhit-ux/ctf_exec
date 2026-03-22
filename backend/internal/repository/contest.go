package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// ContestRepository 赛事数据访问层。
type ContestRepository struct {
	db *sql.DB
}

func NewContestRepository(db *sql.DB) *ContestRepository {
	return &ContestRepository{db: db}
}

func (r *ContestRepository) CreateContest(req *models.CreateContestRequest) (*models.Contest, error) {
	contestID := uuid.New().String()
	now := time.Now()
	scoringType := req.ScoringType
	if scoringType == "" {
		scoringType = "jeopardy"
	}

	query := `
		INSERT INTO contests (
			id, title, description, cover_image, start_time, end_time, scoring_type, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(
		query,
		contestID,
		req.Title,
		req.Description,
		req.CoverImage,
		req.StartTime,
		req.EndTime,
		scoringType,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建赛事失败: %w", err)
	}

	return r.GetContestByID(contestID)
}

func (r *ContestRepository) GetContestByID(contestID string) (*models.Contest, error) {
	contest := &models.Contest{}
	query := `
		SELECT id, title, COALESCE(description, ''), COALESCE(cover_image, ''), start_time, end_time, scoring_type, created_at, updated_at
		FROM contests
		WHERE id = ?
	`

	err := r.db.QueryRow(query, contestID).Scan(
		&contest.ID,
		&contest.Title,
		&contest.Description,
		&contest.CoverImage,
		&contest.StartTime,
		&contest.EndTime,
		&contest.ScoringType,
		&contest.CreatedAt,
		&contest.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "赛事不存在")
		}
		return nil, fmt.Errorf("查询赛事失败: %w", err)
	}

	return contest, nil
}

func (r *ContestRepository) ListContests(limit int) ([]*models.Contest, error) {
	query := `
		SELECT id, title, COALESCE(description, ''), COALESCE(cover_image, ''), start_time, end_time, scoring_type, created_at, updated_at
		FROM contests
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询赛事列表失败: %w", err)
	}
	defer rows.Close()

	contests := make([]*models.Contest, 0)
	for rows.Next() {
		contest := &models.Contest{}
		if err := rows.Scan(
			&contest.ID,
			&contest.Title,
			&contest.Description,
			&contest.CoverImage,
			&contest.StartTime,
			&contest.EndTime,
			&contest.ScoringType,
			&contest.CreatedAt,
			&contest.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描赛事失败: %w", err)
		}
		contests = append(contests, contest)
	}

	return contests, nil
}

func (r *ContestRepository) UpdateContest(contestID string, req *models.UpdateContestRequest) error {
	query := `
		UPDATE contests
		SET title = ?, description = ?, cover_image = ?, start_time = ?, end_time = ?, scoring_type = ?, updated_at = NOW()
		WHERE id = ?
	`

	result, err := r.db.Exec(
		query,
		req.Title,
		req.Description,
		req.CoverImage,
		req.StartTime,
		req.EndTime,
		req.ScoringType,
		contestID,
	)
	if err != nil {
		return fmt.Errorf("更新赛事失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if rowsAffected == 0 {
		return apperr.New(apperr.ErrNotFound, "赛事不存在")
	}

	return nil
}

func (r *ContestRepository) DeleteContest(contestID string) error {
	result, err := r.db.Exec(`DELETE FROM contests WHERE id = ?`, contestID)
	if err != nil {
		return fmt.Errorf("删除赛事失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if rowsAffected == 0 {
		return apperr.New(apperr.ErrNotFound, "赛事不存在")
	}

	return nil
}

func (r *ContestRepository) AddChallengeToContest(contestID string, challengeID int, scoreOverride *int) error {
	query := `
		INSERT INTO contest_challenges (contest_id, challenge_id, score_override)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE score_override = VALUES(score_override)
	`

	if _, err := r.db.Exec(query, contestID, challengeID, scoreOverride); err != nil {
		return fmt.Errorf("添加赛事题目失败: %w", err)
	}

	return nil
}

func (r *ContestRepository) RemoveChallengeFromContest(contestID string, challengeID int) error {
	result, err := r.db.Exec(`DELETE FROM contest_challenges WHERE contest_id = ? AND challenge_id = ?`, contestID, challengeID)
	if err != nil {
		return fmt.Errorf("移除赛事题目失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if rowsAffected == 0 {
		return apperr.New(apperr.ErrNotFound, "赛事题目关联不存在")
	}

	return nil
}

// JoinContest 写入报名记录（个人或团队）。
func (r *ContestRepository) JoinContest(contestID, userID string, teamID *string) error {
	participantID := uuid.New().String()
	query := `INSERT INTO contest_participants (id, contest_id, user_id, team_id, registered_at) VALUES (?, ?, ?, ?, ?)`

	var userIDVal interface{}
	var teamIDVal interface{}
	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		teamIDVal = strings.TrimSpace(*teamID)
		userIDVal = nil
	} else {
		userIDVal = userID
		teamIDVal = nil
	}

	_, err := r.db.Exec(query, participantID, contestID, userIDVal, teamIDVal, time.Now())
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return apperr.New(apperr.ErrConflict, "已报名该赛事")
		}
		return fmt.Errorf("报名失败: %w", err)
	}

	return nil
}

// LeaveContest 删除报名记录。
func (r *ContestRepository) LeaveContest(contestID, userID string, teamID *string) error {
	var result sql.Result
	var err error

	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		result, err = r.db.Exec(`DELETE FROM contest_participants WHERE contest_id = ? AND team_id = ?`, contestID, strings.TrimSpace(*teamID))
	} else {
		result, err = r.db.Exec(`DELETE FROM contest_participants WHERE contest_id = ? AND user_id = ?`, contestID, userID)
	}
	if err != nil {
		return fmt.Errorf("退赛失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return apperr.New(apperr.ErrNotFound, "未找到报名记录")
	}

	return nil
}

// IsParticipant 校验用户或团队是否已报名。
func (r *ContestRepository) IsParticipant(contestID, userID string, teamID *string) (bool, error) {
	var cnt int
	var err error

	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		err = r.db.QueryRow(`SELECT COUNT(1) FROM contest_participants WHERE contest_id = ? AND team_id = ?`, contestID, strings.TrimSpace(*teamID)).Scan(&cnt)
	} else {
		err = r.db.QueryRow(`SELECT COUNT(1) FROM contest_participants WHERE contest_id = ? AND user_id = ?`, contestID, userID).Scan(&cnt)
	}
	if err != nil {
		return false, fmt.Errorf("查询报名状态失败: %w", err)
	}

	return cnt > 0, nil
}

// IsTeamMember 校验用户是否属于某团队。
func (r *ContestRepository) IsTeamMember(teamID, userID string) (bool, error) {
	var cnt int
	err := r.db.QueryRow(`SELECT COUNT(1) FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("查询团队成员关系失败: %w", err)
	}
	return cnt > 0, nil
}

// ListContestChallengesForUser 返回赛事题目及用户在该赛事下的解题状态。
func (r *ContestRepository) ListContestChallengesForUser(contestID, userID string) ([]*models.ContestChallengeView, error) {
	query := `
		SELECT p.id, p.title, p.category, p.difficulty, p.points, cc.score_override,
		       IF(COUNT(s.id) > 0, 1, 0) AS is_solved
		FROM contest_challenges cc
		JOIN problems p ON p.id = cc.challenge_id
		LEFT JOIN submissions s
		  ON s.problem_id = p.id AND s.user_id = ? AND s.contest_id = ? AND s.is_correct = 1
		WHERE cc.contest_id = ?
		GROUP BY p.id, p.title, p.category, p.difficulty, p.points, cc.score_override
		ORDER BY p.id ASC
	`

	rows, err := r.db.Query(query, userID, contestID, contestID)
	if err != nil {
		return nil, fmt.Errorf("查询赛事题目失败: %w", err)
	}
	defer rows.Close()

	list := make([]*models.ContestChallengeView, 0)
	for rows.Next() {
		item := &models.ContestChallengeView{}
		var solved int
		if err := rows.Scan(&item.ID, &item.Title, &item.Category, &item.Difficulty, &item.Points, &item.ScoreOverride, &solved); err != nil {
			return nil, fmt.Errorf("扫描赛事题目失败: %w", err)
		}
		item.IsSolved = solved == 1
		list = append(list, item)
	}

	return list, nil
}

// IsProblemInContest 校验题目是否编排在赛事中。
func (r *ContestRepository) IsProblemInContest(contestID string, problemID int) (bool, error) {
	var cnt int
	err := r.db.QueryRow(`SELECT COUNT(1) FROM contest_challenges WHERE contest_id = ? AND challenge_id = ?`, contestID, problemID).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("校验赛事题目归属失败: %w", err)
	}
	return cnt > 0, nil
}

// GetContestChallengeScore 获取赛事题目实际分值。
func (r *ContestRepository) GetContestChallengeScore(contestID string, problemID int) (int, error) {
	var override sql.NullInt64
	var points int
	query := `
		SELECT cc.score_override, p.points
		FROM contest_challenges cc
		JOIN problems p ON p.id = cc.challenge_id
		WHERE cc.contest_id = ? AND cc.challenge_id = ?
	`
	err := r.db.QueryRow(query, contestID, problemID).Scan(&override, &points)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, apperr.New(apperr.ErrNotFound, "赛事题目不存在")
		}
		return 0, fmt.Errorf("获取赛事题目分值失败: %w", err)
	}

	if override.Valid {
		return int(override.Int64), nil
	}
	return points, nil
}

// HasSolvedInContest 校验主体在赛事中是否已正确解过该题。
func (r *ContestRepository) HasSolvedInContest(contestID string, problemID int, userID string, teamID *string) (bool, error) {
	var cnt int
	var err error

	if teamID != nil && strings.TrimSpace(*teamID) != "" {
		err = r.db.QueryRow(`SELECT COUNT(1) FROM submissions WHERE contest_id = ? AND problem_id = ? AND team_id = ? AND is_correct = 1`, contestID, problemID, strings.TrimSpace(*teamID)).Scan(&cnt)
	} else {
		err = r.db.QueryRow(`SELECT COUNT(1) FROM submissions WHERE contest_id = ? AND problem_id = ? AND user_id = ? AND is_correct = 1`, contestID, problemID, userID).Scan(&cnt)
	}
	if err != nil {
		return false, fmt.Errorf("查询赛事解题状态失败: %w", err)
	}

	return cnt > 0, nil
}

// EnsureContestUserScore 初始化个人赛分数行。
func (r *ContestRepository) EnsureContestUserScore(contestID, userID string) error {
	query := `
		INSERT INTO contest_scores (id, contest_id, user_id, score, solved_count, updated_at)
		VALUES (?, ?, ?, 0, 0, NOW())
		ON DUPLICATE KEY UPDATE updated_at = NOW()
	`
	_, err := r.db.Exec(query, uuid.New().String(), contestID, userID)
	if err != nil {
		return fmt.Errorf("初始化赛事个人分数失败: %w", err)
	}
	return nil
}

// EnsureContestTeamScore 初始化团队赛分数行。
func (r *ContestRepository) EnsureContestTeamScore(contestID, teamID string) error {
	query := `
		INSERT INTO contest_scores (id, contest_id, team_id, score, solved_count, updated_at)
		VALUES (?, ?, ?, 0, 0, NOW())
		ON DUPLICATE KEY UPDATE updated_at = NOW()
	`
	_, err := r.db.Exec(query, uuid.New().String(), contestID, teamID)
	if err != nil {
		return fmt.Errorf("初始化赛事团队分数失败: %w", err)
	}
	return nil
}

// UpsertContestUserScore 正确提交后更新个人赛分数并返回最新分数。
func (r *ContestRepository) UpsertContestUserScore(contestID, userID string, delta int, solveTime time.Time) (int, error) {
	query := `
		INSERT INTO contest_scores (id, contest_id, user_id, score, solved_count, last_solve_time, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, NOW())
		ON DUPLICATE KEY UPDATE
			score = score + VALUES(score),
			solved_count = solved_count + 1,
			last_solve_time = VALUES(last_solve_time),
			updated_at = NOW()
	`
	if _, err := r.db.Exec(query, uuid.New().String(), contestID, userID, delta, solveTime); err != nil {
		return 0, fmt.Errorf("更新赛事个人分数失败: %w", err)
	}

	var latest int
	if err := r.db.QueryRow(`SELECT score FROM contest_scores WHERE contest_id = ? AND user_id = ?`, contestID, userID).Scan(&latest); err != nil {
		return 0, fmt.Errorf("查询赛事个人最新分数失败: %w", err)
	}

	return latest, nil
}

// UpsertContestTeamScore 正确提交后更新团队赛分数并返回最新分数。
func (r *ContestRepository) UpsertContestTeamScore(contestID, teamID string, delta int, solveTime time.Time) (int, error) {
	query := `
		INSERT INTO contest_scores (id, contest_id, team_id, score, solved_count, last_solve_time, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, NOW())
		ON DUPLICATE KEY UPDATE
			score = score + VALUES(score),
			solved_count = solved_count + 1,
			last_solve_time = VALUES(last_solve_time),
			updated_at = NOW()
	`
	if _, err := r.db.Exec(query, uuid.New().String(), contestID, teamID, delta, solveTime); err != nil {
		return 0, fmt.Errorf("更新赛事团队分数失败: %w", err)
	}

	var latest int
	if err := r.db.QueryRow(`SELECT score FROM contest_scores WHERE contest_id = ? AND team_id = ?`, contestID, teamID).Scan(&latest); err != nil {
		return 0, fmt.Errorf("查询赛事团队最新分数失败: %w", err)
	}

	return latest, nil
}

// GetContestUserScoreboardFromDB 从数据库查询赛事个人榜。
func (r *ContestRepository) GetContestUserScoreboardFromDB(contestID string, limit int) ([]*models.ContestScoreboardEntry, error) {
	query := `
		SELECT cs.user_id, u.username, u.display_name, cs.score, cs.solved_count, cs.last_solve_time
		FROM contest_scores cs
		JOIN users u ON u.id = cs.user_id
		WHERE cs.contest_id = ? AND cs.user_id IS NOT NULL
		ORDER BY cs.score DESC, cs.last_solve_time ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, contestID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询赛事个人榜失败: %w", err)
	}
	defer rows.Close()

	result := make([]*models.ContestScoreboardEntry, 0)
	rank := 1
	for rows.Next() {
		entry := &models.ContestScoreboardEntry{Rank: rank}
		var userID string
		var username string
		var displayName string
		if err := rows.Scan(&userID, &username, &displayName, &entry.Score, &entry.SolvedCount, &entry.LastSolveTime); err != nil {
			return nil, fmt.Errorf("扫描赛事个人榜失败: %w", err)
		}
		entry.UserID = &userID
		entry.Username = &username
		entry.DisplayName = &displayName
		result = append(result, entry)
		rank++
	}

	return result, nil
}

// GetContestTeamScoreboardFromDB 从数据库查询赛事团队榜。
func (r *ContestRepository) GetContestTeamScoreboardFromDB(contestID string, limit int) ([]*models.ContestScoreboardEntry, error) {
	query := `
		SELECT cs.team_id, t.name, cs.score, cs.solved_count, cs.last_solve_time
		FROM contest_scores cs
		JOIN teams t ON t.id = cs.team_id
		WHERE cs.contest_id = ? AND cs.team_id IS NOT NULL
		ORDER BY cs.score DESC, cs.last_solve_time ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, contestID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询赛事团队榜失败: %w", err)
	}
	defer rows.Close()

	result := make([]*models.ContestScoreboardEntry, 0)
	rank := 1
	for rows.Next() {
		entry := &models.ContestScoreboardEntry{Rank: rank}
		var teamID string
		var teamName string
		if err := rows.Scan(&teamID, &teamName, &entry.Score, &entry.SolvedCount, &entry.LastSolveTime); err != nil {
			return nil, fmt.Errorf("扫描赛事团队榜失败: %w", err)
		}
		entry.TeamID = &teamID
		entry.Name = &teamName
		result = append(result, entry)
		rank++
	}

	return result, nil
}
