package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/utils"
)

// ProblemRepository 题库数据访问层
type ProblemRepository struct {
	db *sql.DB
}

// GetProblemSubmissionMeta 获取提交校验所需的题目元数据。
func (r *ProblemRepository) GetProblemSubmissionMeta(id int) (*models.ProblemSubmissionMeta, error) {
	meta := &models.ProblemSubmissionMeta{}
	query := `
		SELECT id, title, points, scope, status, contest_id, visible_from, visible_to
		FROM problems
		WHERE id = ?
	`

	var contestID sql.NullString
	var visibleFrom sql.NullTime
	var visibleTo sql.NullTime
	err := r.db.QueryRow(query, id).Scan(
		&meta.ID,
		&meta.Title,
		&meta.Points,
		&meta.Scope,
		&meta.Status,
		&contestID,
		&visibleFrom,
		&visibleTo,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询题目元数据失败: %w", err)
	}

	if contestID.Valid {
		value := strings.TrimSpace(contestID.String)
		if value != "" {
			meta.ContestID = &value
		}
	}
	if visibleFrom.Valid {
		value := visibleFrom.Time
		meta.VisibleFrom = &value
	}
	if visibleTo.Valid {
		value := visibleTo.Time
		meta.VisibleTo = &value
	}

	return meta, nil
}

func isProblemVisible(now time.Time, from, to *time.Time) bool {
	if from != nil && now.Before(*from) {
		return false
	}
	if to != nil && now.After(*to) {
		return false
	}
	return true
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
		WHERE p.scope = 'platform'
		  AND p.status = 'published'
		  AND (p.visible_from IS NULL OR p.visible_from <= NOW())
		  AND (p.visible_to IS NULL OR p.visible_to >= NOW())
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
		  AND p.scope = 'platform'
		  AND p.status = 'published'
		  AND (p.visible_from IS NULL OR p.visible_from <= NOW())
		  AND (p.visible_to IS NULL OR p.visible_to >= NOW())
		GROUP BY p.id, p.title, p.category, p.points, p.difficulty, p.description
	`

	var tagsJSON string
	err := r.db.QueryRow(query, id).Scan(&p.ID, &p.Title, &p.Category, &p.Points, &p.Difficulty, &p.Description, &tagsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
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
			return "", apperr.New(apperr.ErrNotFound, "题目不存在")
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
		WHERE p.scope = 'platform'
		  AND p.status = 'published'
		  AND (p.visible_from IS NULL OR p.visible_from <= NOW())
		  AND (p.visible_to IS NULL OR p.visible_to >= NOW())
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

// CreateProblem 创建题目（管理端）。
func (r *ProblemRepository) CreateProblem(req *models.CreateProblemRequest) (*models.Problem, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	insertProblemQuery := `
		INSERT INTO problems (title, category, points, difficulty, description, flag_hash)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, execErr := tx.Exec(
		insertProblemQuery,
		req.Title,
		req.Category,
		req.Points,
		req.Difficulty,
		req.Description,
		utils.HashFlag(req.Flag),
	)
	if execErr != nil {
		err = fmt.Errorf("创建题目失败: %w", execErr)
		return nil, err
	}

	problemID, idErr := result.LastInsertId()
	if idErr != nil {
		err = fmt.Errorf("获取题目ID失败: %w", idErr)
		return nil, err
	}

	if tagErr := r.syncProblemTagsTx(tx, int(problemID), req.Tags); tagErr != nil {
		err = tagErr
		return nil, err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return nil, fmt.Errorf("提交事务失败: %w", commitErr)
	}

	return r.GetProblemByID(int(problemID))
}

// UpdateProblem 更新题目（管理端）。
func (r *ProblemRepository) UpdateProblem(problemID int, req *models.UpdateProblemRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var existsID int
	checkQuery := `SELECT id FROM problems WHERE id = ?`
	if scanErr := tx.QueryRow(checkQuery, problemID).Scan(&existsID); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			err = apperr.New(apperr.ErrNotFound, "题目不存在")
			return err
		}
		err = fmt.Errorf("查询题目失败: %w", scanErr)
		return err
	}

	updateQuery := `
		UPDATE problems
		SET title = ?, category = ?, points = ?, difficulty = ?, description = ?, updated_at = NOW()
		WHERE id = ?
	`
	if _, execErr := tx.Exec(
		updateQuery,
		req.Title,
		req.Category,
		req.Points,
		req.Difficulty,
		req.Description,
		problemID,
	); execErr != nil {
		err = fmt.Errorf("更新题目失败: %w", execErr)
		return err
	}

	if strings.TrimSpace(req.Flag) != "" {
		updateFlagQuery := `UPDATE problems SET flag_hash = ?, updated_at = NOW() WHERE id = ?`
		if _, execErr := tx.Exec(updateFlagQuery, utils.HashFlag(req.Flag), problemID); execErr != nil {
			err = fmt.Errorf("更新题目 flag 失败: %w", execErr)
			return err
		}
	}

	if tagErr := r.syncProblemTagsTx(tx, problemID, req.Tags); tagErr != nil {
		err = tagErr
		return err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("提交事务失败: %w", commitErr)
	}

	return nil
}

// DeleteProblem 删除题目（管理端）。
func (r *ProblemRepository) DeleteProblem(problemID int) error {
	result, err := r.db.Exec(`DELETE FROM problems WHERE id = ?`, problemID)
	if err != nil {
		return fmt.Errorf("删除题目失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return apperr.New(apperr.ErrNotFound, "题目不存在")
	}

	return nil
}

func (r *ProblemRepository) syncProblemTagsTx(tx *sql.Tx, problemID int, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM challenge_tags WHERE challenge_id = ?`, problemID); err != nil {
		return fmt.Errorf("清理题目标签失败: %w", err)
	}

	if len(tags) == 0 {
		return nil
	}

	insertTagQuery := `INSERT INTO challenge_tags (challenge_id, tag) VALUES (?, ?)`
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		normalizedTag := strings.TrimSpace(tag)
		if normalizedTag == "" {
			continue
		}
		if _, ok := seen[normalizedTag]; ok {
			continue
		}
		seen[normalizedTag] = struct{}{}

		if _, err := tx.Exec(insertTagQuery, problemID, normalizedTag); err != nil {
			return fmt.Errorf("写入题目标签失败: %w", err)
		}
	}

	return nil
}
