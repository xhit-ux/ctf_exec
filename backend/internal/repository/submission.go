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

	// 如果提交正确，更新用户分数和解题数 (简单处理，不检查是否重复提交正确)
	// TODO: 真正的逻辑应该检查是否已经解决过该题目，防止重复加分
	if req.IsCorrect {
		// 这里假设题目分值固定，比如 100 分，或者以后从题目表中获取
		// 暂时简化：更新用户解题数，分数需要在 service 层处理更合适，或者这里硬编码一下
		// 为了演示，我们先只增加 solved_count，分数逻辑比较复杂暂不处理，或者前端传？
		// 最好是在 Handler 层或者 Service 层做，Repository 只负责原子操作。
		// 但为了简单，可以在这里 Update.

		// 检查该用户该题目是否已经正确提交过
		var count int
		checkQuery := `SELECT COUNT(*) FROM submissions WHERE user_id = ? AND problem_id = ? AND is_correct = 1 AND id != ?`
		_ = r.db.QueryRow(checkQuery, userID, req.ProblemID, submissionID).Scan(&count)

		if count == 0 {
			// 第一次解出，加分
			// 假设默认 100 分
			updateUser := `UPDATE users SET solved_count = solved_count + 1, score = score + 100 WHERE id = ?`
			_, _ = r.db.Exec(updateUser, userID)
		}
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
