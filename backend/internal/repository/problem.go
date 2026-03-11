package repository

import (
	"database/sql"
	"encoding/json"
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
		SELECT p.id, p.title, p.category, p.points, p.difficulty, p.description,
		       IFNULL(CAST(JSON_ARRAYAGG(ct.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN challenge_tags ct ON p.id = ct.challenge_id
		GROUP BY p.id, p.title, p.category, p.points, p.difficulty, p.description
		ORDER BY p.id ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询题目列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p := &models.Problem{}
		var tagsJSON string
		if err := rows.Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &p.Description, &tagsJSON); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		// 解析 tags JSON 为 string 数组
		if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
			p.Tags = []string{} // 失败时默认为空数组
		}
		problems = append(problems, p)
	}

	return problems, nil
}

// GetProblemByID 获取单个题目详情
func (r *ProblemRepository) GetProblemByID(id int) (*models.Problem, error) {
	p := &models.Problem{}
	query := `
		SELECT p.id, p.title, p.category, p.points, p.difficulty, p.description,
		       IFNULL(CAST(JSON_ARRAYAGG(ct.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN challenge_tags ct ON p.id = ct.challenge_id
		WHERE p.id = ?
		GROUP BY p.id, p.title, p.category, p.points, p.difficulty, p.description
	`

	var tagsJSON string
	err := r.db.QueryRow(query, id).Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &p.Description, &tagsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("题目不存在")
		}
		return nil, fmt.Errorf("查询题目失败: %w", err)
	}
	// 解析 tags JSON 为 string 数组
	if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
		p.Tags = []string{} // 失败时默认为空数组
	}
	return p, nil
}

// GetFlagHashByProblemID 获取题目的 Flag Hash (用于验证)
func (r *ProblemRepository) GetFlagHashByProblemID(id int) (string, error) {
	var flagHash string
	query := `SELECT flag_hash FROM problems WHERE id = ?`
	err := r.db.QueryRow(query, id).Scan(&flagHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("题目不存在")
		}
		return "", fmt.Errorf("查询 Flag Hash 失败: %w", err)
	}
	return flagHash, nil
}

// GetProblemsWithStatus 获取题目列表，并包含用户是否已解决的状态
func (r *ProblemRepository) GetProblemsWithStatus(userID string) ([]*models.Problem, error) {
	var problems []*models.Problem
	query := `
		SELECT p.id, p.title, p.category, p.points, p.difficulty, p.description,
		       IFNULL(CAST(JSON_ARRAYAGG(ct.tag) AS CHAR), '[]') as tags,
		       IF(COUNT(s.id) > 0, 1, 0) as is_solved
		FROM problems p
		LEFT JOIN challenge_tags ct ON p.id = ct.challenge_id
		LEFT JOIN submissions s ON p.id = s.problem_id AND s.user_id = ? AND s.is_correct = 1
		GROUP BY p.id, p.title, p.category, p.points, p.difficulty, p.description
		ORDER BY p.id ASC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询题目列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p := &models.Problem{}
		var tagsJSON string
		var isSolved int
		if err := rows.Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &p.Description, &tagsJSON, &isSolved); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		// 解析 tags JSON 为 string 数组
		if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
			p.Tags = []string{} // 失败时默认为空数组
		}
		p.IsSolved = (isSolved == 1)
		problems = append(problems, p)
	}

	return problems, nil
}
