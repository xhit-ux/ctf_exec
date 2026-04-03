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
		SELECT id, title, points, scope, status, flag_hash, IFNULL(flag_rule, ''), contest_id, visible_from, visible_to
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
		&meta.FlagHash,
		&meta.FlagRule,
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

func (r *ProblemRepository) archiveEndedContestProblems() error {
	query := `
		UPDATE problems p
		JOIN contests c ON c.id = p.contest_id
		SET p.scope = 'platform',
		    p.contest_id = NULL,
		    p.visible_from = COALESCE(p.visible_from, c.end_time),
		    p.visible_to = NULL,
		    p.updated_at = NOW()
		WHERE p.scope = 'contest'
		  AND p.status = 'published'
		  AND c.end_time IS NOT NULL
		  AND c.end_time <= NOW()
	`

	if _, err := r.db.Exec(query); err != nil {
		return fmt.Errorf("归档已结束赛事题目失败: %w", err)
	}

	return nil
}

// NewProblemRepository 创建题库仓库实例
func NewProblemRepository(db *sql.DB) *ProblemRepository {
	return &ProblemRepository{db: db}
}

// GetProblems 获取所有题目列表
func (r *ProblemRepository) GetProblems() ([]*models.Problem, error) {
	if err := r.archiveEndedContestProblems(); err != nil {
		return nil, err
	}

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
	if err := r.archiveEndedContestProblems(); err != nil {
		return nil, err
	}

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
	if err := r.archiveEndedContestProblems(); err != nil {
		return "", err
	}

	var flagHash string
	query := `
		SELECT p.flag_hash
		FROM problems p
		LEFT JOIN contests c ON c.id = p.contest_id
		WHERE p.id = ?
		  AND p.status = 'published'
		  AND (
			(p.scope = 'platform' AND (p.visible_from IS NULL OR p.visible_from <= NOW()) AND (p.visible_to IS NULL OR p.visible_to >= NOW()))
			OR
			(p.scope = 'contest' AND c.start_time IS NOT NULL AND c.end_time IS NOT NULL AND NOW() BETWEEN c.start_time AND c.end_time)
		  )
	`
	err := r.db.QueryRow(query, id).Scan(&flagHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", apperr.New(apperr.ErrNotFound, "题目不存在或当前不可提交")
		}
		return "", fmt.Errorf("查询 Flag Hash 失败: %w", err)
	}
	return flagHash, nil
}

// GetProblemsWithStatus 获取题目列表，并包含用户是否已解决的状态
func (r *ProblemRepository) GetProblemsWithStatus(userID string) ([]*models.Problem, error) {
	if err := r.archiveEndedContestProblems(); err != nil {
		return nil, err
	}

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
		INSERT INTO problems (
			title, category, type, points, difficulty, description,
			flag_hash, flag_rule, flag_example, docker_image, run_command,
			created_by, scope, contest_id, status, visible_from, visible_to
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = "platform"
	}

	var contestID interface{}
	if scope == "contest" && strings.TrimSpace(req.ContestID) != "" {
		contestID = strings.TrimSpace(req.ContestID)
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "published"
	}

	typeName := strings.ToLower(strings.TrimSpace(req.Type))
	if typeName == "" {
		typeName = "static"
	}

	flagRule := strings.TrimSpace(req.FlagRule)
	flagHash := buildProblemFlagHash(req.Flag, flagRule)

	result, execErr := tx.Exec(
		insertProblemQuery,
		req.Title,
		req.Category,
		typeName,
		req.Points,
		req.Difficulty,
		req.Description,
		flagHash,
		flagRule,
		strings.TrimSpace(req.FlagExample),
		strings.TrimSpace(req.DockerImage),
		strings.TrimSpace(req.RunCommand),
		req.CreatedBy,
		scope,
		contestID,
		status,
		req.VisibleFrom,
		req.VisibleTo,
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

	return r.getProblemByIDAnyScope(int(problemID))
}

func (r *ProblemRepository) getProblemByIDAnyScope(id int) (*models.Problem, error) {
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
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询题目失败: %w", err)
	}

	if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
		p.Tags = []string{}
	}

	return p, nil
}

// ArchiveContestProblems 将指定赛事的 contest 题目归档到平台域。
func (r *ProblemRepository) ArchiveContestProblems(contestID string) (int64, error) {
	query := `
		UPDATE problems p
		JOIN contests c ON c.id = p.contest_id
		SET p.scope = 'platform',
		    p.contest_id = NULL,
		    p.visible_from = COALESCE(p.visible_from, c.end_time),
		    p.visible_to = NULL,
		    p.updated_at = NOW()
		WHERE p.scope = 'contest'
		  AND p.status = 'published'
		  AND p.contest_id = ?
	`

	result, err := r.db.Exec(query, strings.TrimSpace(contestID))
	if err != nil {
		return 0, fmt.Errorf("归档赛事题目失败: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("获取归档影响行数失败: %w", err)
	}

	return rows, nil
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

	typeName := strings.ToLower(strings.TrimSpace(req.Type))
	if typeName == "" {
		typeName = "static"
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "published"
	}

	flagRule := strings.TrimSpace(req.FlagRule)

	updateQuery := `
		UPDATE problems
		SET title = ?, category = ?, type = ?, points = ?, difficulty = ?, description = ?,
			flag_rule = ?, flag_example = ?, docker_image = ?, run_command = ?,
			status = ?, visible_from = ?, visible_to = ?, updated_at = NOW()
		WHERE id = ?
	`
	if _, execErr := tx.Exec(
		updateQuery,
		req.Title,
		req.Category,
		typeName,
		req.Points,
		req.Difficulty,
		req.Description,
		flagRule,
		strings.TrimSpace(req.FlagExample),
		strings.TrimSpace(req.DockerImage),
		strings.TrimSpace(req.RunCommand),
		status,
		req.VisibleFrom,
		req.VisibleTo,
		problemID,
	); execErr != nil {
		err = fmt.Errorf("更新题目失败: %w", execErr)
		return err
	}

	if strings.TrimSpace(req.Flag) != "" {
		updateFlagQuery := `UPDATE problems SET flag_hash = ?, updated_at = NOW() WHERE id = ?`
		if _, execErr := tx.Exec(updateFlagQuery, buildProblemFlagHash(req.Flag, flagRule), problemID); execErr != nil {
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

func buildProblemFlagHash(flag, rule string) string {
	normalizedRule := strings.ToLower(strings.TrimSpace(rule))
	normalizedFlag := flag

	if normalizedRule == "case_insensitive" {
		normalizedFlag = strings.ToLower(normalizedFlag)
	}

	if strings.TrimSpace(normalizedFlag) == "" {
		normalizedFlag = normalizedRule
	}

	return utils.HashFlag(normalizedFlag)
}

func (r *ProblemRepository) GetProblemAccessMeta(problemID int) (scope string, contestID, createdBy string, err error) {
	err = r.db.QueryRow(`
		SELECT scope, IFNULL(contest_id, ''), IFNULL(created_by, '')
		FROM problems
		WHERE id = ?
	`, problemID).Scan(&scope, &contestID, &createdBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return "", "", "", fmt.Errorf("查询题目权限元信息失败: %w", err)
	}

	return strings.TrimSpace(scope), strings.TrimSpace(contestID), strings.TrimSpace(createdBy), nil
}

func (r *ProblemRepository) GetAdminProblemByID(problemID int) (*models.Problem, error) {
	p := &models.Problem{}
	query := `
		SELECT p.id, p.title, p.category, p.type, p.points, p.difficulty, p.description,
		       p.scope, p.contest_id, p.status, IFNULL(p.flag_rule, ''), IFNULL(p.flag_example, ''),
		       IFNULL(p.docker_image, ''), IFNULL(p.run_command, ''), IFNULL(p.created_by, ''),
		       p.visible_from, p.visible_to,
		       IFNULL(CAST(JSON_ARRAYAGG(ct.tag) AS CHAR), '[]') AS tags
		FROM problems p
		LEFT JOIN challenge_tags ct ON p.id = ct.challenge_id
		WHERE p.id = ?
		GROUP BY p.id, p.title, p.category, p.type, p.points, p.difficulty, p.description,
		         p.scope, p.contest_id, p.status, p.flag_rule, p.flag_example,
		         p.docker_image, p.run_command, p.created_by, p.visible_from, p.visible_to
	`

	var tagsJSON string
	var contestID sql.NullString
	var visibleFrom sql.NullTime
	var visibleTo sql.NullTime
	err := r.db.QueryRow(query, problemID).Scan(
		&p.ID,
		&p.Title,
		&p.Category,
		&p.Type,
		&p.Points,
		&p.Difficulty,
		&p.Description,
		&p.Scope,
		&contestID,
		&p.Status,
		&p.FlagRule,
		&p.FlagExample,
		&p.DockerImage,
		&p.RunCommand,
		&p.CreatedBy,
		&visibleFrom,
		&visibleTo,
		&tagsJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询管理端题目详情失败: %w", err)
	}

	if contestID.Valid {
		v := strings.TrimSpace(contestID.String)
		if v != "" {
			p.ContestID = &v
		}
	}
	if visibleFrom.Valid {
		v := visibleFrom.Time.Format(time.RFC3339)
		p.VisibleFrom = &v
	}
	if visibleTo.Valid {
		v := visibleTo.Time.Format(time.RFC3339)
		p.VisibleTo = &v
	}

	if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
		p.Tags = []string{}
	}

	return p, nil
}

func (r *ProblemRepository) ListAdminProblems(keyword, scope, status string) ([]*models.Problem, error) {
	args := make([]interface{}, 0, 3)
	conds := make([]string, 0, 3)

	if strings.TrimSpace(keyword) != "" {
		conds = append(conds, "p.title LIKE ?")
		args = append(args, "%"+strings.TrimSpace(keyword)+"%")
	}
	if strings.TrimSpace(scope) != "" {
		conds = append(conds, "p.scope = ?")
		args = append(args, strings.TrimSpace(scope))
	}
	if strings.TrimSpace(status) != "" {
		conds = append(conds, "p.status = ?")
		args = append(args, strings.TrimSpace(status))
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.title, p.category, p.type, p.points, p.difficulty, p.description,
		       p.scope, p.contest_id, p.status, IFNULL(p.flag_rule, ''), IFNULL(p.flag_example, ''),
		       IFNULL(p.docker_image, ''), IFNULL(p.run_command, ''), IFNULL(p.created_by, ''),
		       p.visible_from, p.visible_to,
		       IFNULL(CAST(JSON_ARRAYAGG(ct.tag) AS CHAR), '[]') AS tags
		FROM problems p
		LEFT JOIN challenge_tags ct ON p.id = ct.challenge_id
		%s
		GROUP BY p.id, p.title, p.category, p.type, p.points, p.difficulty, p.description,
		         p.scope, p.contest_id, p.status, p.flag_rule, p.flag_example,
		         p.docker_image, p.run_command, p.created_by, p.visible_from, p.visible_to
		ORDER BY p.updated_at DESC, p.id DESC
	`, where)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询管理端题目列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.Problem, 0)
	for rows.Next() {
		p := &models.Problem{}
		var tagsJSON string
		var contestID sql.NullString
		var visibleFrom sql.NullTime
		var visibleTo sql.NullTime
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Category,
			&p.Type,
			&p.Points,
			&p.Difficulty,
			&p.Description,
			&p.Scope,
			&contestID,
			&p.Status,
			&p.FlagRule,
			&p.FlagExample,
			&p.DockerImage,
			&p.RunCommand,
			&p.CreatedBy,
			&visibleFrom,
			&visibleTo,
			&tagsJSON,
		); err != nil {
			return nil, fmt.Errorf("扫描管理端题目列表失败: %w", err)
		}

		if contestID.Valid {
			v := strings.TrimSpace(contestID.String)
			if v != "" {
				p.ContestID = &v
			}
		}
		if visibleFrom.Valid {
			v := visibleFrom.Time.Format(time.RFC3339)
			p.VisibleFrom = &v
		}
		if visibleTo.Valid {
			v := visibleTo.Time.Format(time.RFC3339)
			p.VisibleTo = &v
		}

		if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
			p.Tags = []string{}
		}

		items = append(items, p)
	}

	return items, nil
}
