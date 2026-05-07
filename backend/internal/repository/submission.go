package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/google/uuid"
)

// SubmissionRepository 提交记录数据访问层
type SubmissionRepository struct {
	db *sql.DB
}

// NewSubmissionRepository 创建提交记录仓库实例
func NewSubmissionRepository(db *sql.DB) *SubmissionRepository {
	return &SubmissionRepository{db: db}
}

// CreateSubmission 创建提交记录
func (r *SubmissionRepository) CreateSubmission(userID string, req *models.CreateSubmissionRequest) (*models.Submission, error) {
	submissionID := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO submissions (id, user_id, team_id, problem_id, contest_id, flag, is_correct, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, submissionID, userID, req.TeamID, req.ProblemID, req.ContestID, req.Flag, req.IsCorrect, normalizeIPAddress(req.IPAddress), now)
	if err != nil {
		return nil, fmt.Errorf("创建提交记录失败: %w", err)
	}

	uid := userID
	return &models.Submission{
		ID:        submissionID,
		UserID:    &uid,
		TeamID:    req.TeamID,
		ContestID: req.ContestID,
		ProblemID: req.ProblemID,
		Flag:      req.Flag,
		IsCorrect: req.IsCorrect,
		IPAddress: normalizeIPAddress(req.IPAddress),
		CreatedAt: now,
	}, nil
}

// GetSubmissionsByUserID 获取用户的提交记录
func (r *SubmissionRepository) GetSubmissionsByUserID(userID string) ([]*models.Submission, error) {
	var submissions []*models.Submission
	query := `
		SELECT s.id, s.user_id, s.team_id, s.problem_id, s.contest_id, s.flag, s.is_correct, s.ip_address, s.created_at, p.title
		FROM submissions s
		LEFT JOIN problems p ON s.problem_id = p.id
		WHERE s.user_id = ?
		ORDER BY s.created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询提交记录失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		s := &models.Submission{}
		var userIDVal sql.NullString
		var teamID sql.NullString
		var contestID sql.NullString
		var ipAddress sql.NullString
		err := rows.Scan(&s.ID, &userIDVal, &teamID, &s.ProblemID, &contestID, &s.Flag, &s.IsCorrect, &ipAddress, &s.CreatedAt, &s.ProblemTitle)
		if err != nil {
			return nil, fmt.Errorf("扫描提交记录失败: %w", err)
		}
		if userIDVal.Valid {
			v := userIDVal.String
			s.UserID = &v
		}
		if teamID.Valid {
			v := teamID.String
			s.TeamID = &v
		}
		if contestID.Valid {
			v := contestID.String
			s.ContestID = &v
		}
		if ipAddress.Valid {
			s.IPAddress = ipAddress.String
		}
		submissions = append(submissions, s)
	}

	return submissions, nil
}

// HasCorrectSubmissionInContest 校验用户在赛事中是否已正确解过该题。
func (r *SubmissionRepository) HasCorrectSubmissionInContest(userID, contestID string, problemID string) (bool, error) {
	var cnt int
	err := r.db.QueryRow(`
		SELECT COUNT(1)
		FROM submissions
		WHERE user_id = ? AND contest_id = ? AND problem_id = ? AND is_correct = 1
	`, userID, contestID, problemID).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("查询赛事解题记录失败: %w", err)
	}

	return cnt > 0, nil
}

// HasCorrectSubmissionByUserProblem 校验用户是否已正确解出该题（全局维度）。
func (r *SubmissionRepository) HasCorrectSubmissionByUserProblem(userID string, problemID string) (bool, error) {
	var cnt int
	err := r.db.QueryRow(`
		SELECT COUNT(1)
		FROM submissions
		WHERE user_id = ? AND problem_id = ? AND is_correct = 1
	`, userID, problemID).Scan(&cnt)
	if err != nil {
		return false, fmt.Errorf("查询用户解题记录失败: %w", err)
	}

	return cnt > 0, nil
}
