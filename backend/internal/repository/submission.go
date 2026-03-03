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
		INSERT INTO submissions (id, user_id, problem_id, flag, is_correct, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query, submissionID, userID, req.ProblemID, req.Flag, req.IsCorrect, now)
	if err != nil {
		return nil, fmt.Errorf("创建提交记录失败: %w", err)
	}

	return &models.Submission{
		ID:        submissionID,
		UserID:    userID,
		ProblemID: req.ProblemID,
		Flag:      req.Flag,
		IsCorrect: req.IsCorrect,
		CreatedAt: now,
	}, nil
}

// GetSubmissionsByUserID 获取用户的提交记录
func (r *SubmissionRepository) GetSubmissionsByUserID(userID string) ([]*models.Submission, error) {
	var submissions []*models.Submission
	query := `
		SELECT s.id, s.user_id, s.problem_id, s.flag, s.is_correct, s.created_at, p.title
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
		err := rows.Scan(&s.ID, &s.UserID, &s.ProblemID, &s.Flag, &s.IsCorrect, &s.CreatedAt, &s.ProblemTitle)
		if err != nil {
			return nil, fmt.Errorf("扫描提交记录失败: %w", err)
		}
		submissions = append(submissions, s)
	}

	return submissions, nil
}
