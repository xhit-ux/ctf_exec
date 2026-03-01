package repository

import (
	"database/sql"
	"fmt"

	"github.com/ctf_exec/backend/internal/models"
)

// ProblemRepository 题库数据访问层
type ProblemRepository struct {
	db *sql.DB
}

// NewProblemRepository 创建题库仓库实例
func NewProblemRepository(db *sql.DB) *ProblemRepository {
	return &ProblemRepository{db: db}
}

// GetProblems 获取所有题目列表
func (r *ProblemRepository) GetProblems() ([]*models.Problem, error) {
	var problems []*models.Problem
	query := `
		SELECT id, title, category, points, difficulty, tags, description
		FROM problems
		ORDER BY id ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询题目列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p := &models.Problem{}
		var tags []byte
		if err := rows.Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &tags, &p.Description); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		p.Tags = tags
		problems = append(problems, p)
	}

	return problems, nil
}

// GetProblemByID 获取单个题目详情
func (r *ProblemRepository) GetProblemByID(id int) (*models.Problem, error) {
	p := &models.Problem{}
	query := `
		SELECT id, title, category, points, difficulty, tags, description
		FROM problems
		WHERE id = ?
	`

	var tags []byte
	err := r.db.QueryRow(query, id).Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &tags, &p.Description)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("题目不存在")
		}
		return nil, fmt.Errorf("查询题目失败: %w", err)
	}
	p.Tags = tags
	return p, nil
}

// GetFlagByProblemID 获取题目的 Flag (用于验证)
func (r *ProblemRepository) GetFlagByProblemID(id int) (string, error) {
	var flag string
	query := `SELECT flag FROM problems WHERE id = ?`
	err := r.db.QueryRow(query, id).Scan(&flag)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("题目不存在")
		}
		return "", fmt.Errorf("查询 Flag 失败: %w", err)
	}
	return flag, nil
}

// GetProblemsWithStatus 获取题目列表，并包含用户是否已解决的状态
func (r *ProblemRepository) GetProblemsWithStatus(userID string) ([]*models.Problem, error) {
	var problems []*models.Problem
	query := `
		SELECT p.id, p.title, p.category, p.points, p.difficulty, p.tags, p.description,
		       IF(s.id IS NULL, 0, 1) as is_solved
		FROM problems p
		LEFT JOIN submissions s ON p.id = s.problem_id AND s.user_id = ? AND s.is_correct = 1
		ORDER BY p.id ASC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询题目列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p := &models.Problem{}
		var tags []byte
		var isSolved int
		if err := rows.Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &tags, &p.Description, &isSolved); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		p.Tags = tags
		p.IsSolved = (isSolved == 1)
		problems = append(problems, p)
	}

	return problems, nil
}
