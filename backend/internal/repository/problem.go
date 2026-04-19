package repository

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/google/uuid"
)

type ProblemRepository struct {
	db *sql.DB
}

func NewProblemRepository(db *sql.DB) *ProblemRepository {
	return &ProblemRepository{db: db}
}

func (r *ProblemRepository) GetProblemSubmissionMeta(id string) (*models.ProblemSubmissionMeta, error) {
	meta := &models.ProblemSubmissionMeta{}
	query := `
		SELECT p.id, p.title, p.base_score, p.status,
		       IFNULL(pf.flag_hash, ''), IFNULL(pf.flag_rule, 'exact'), IFNULL(pf.flag_type, 'static')
		FROM problems p
		JOIN problem_flags pf ON pf.problem_id = p.id
		WHERE p.id = ?
	`
	if err := r.db.QueryRow(query, strings.TrimSpace(id)).Scan(
		&meta.ID,
		&meta.Title,
		&meta.Points,
		&meta.Status,
		&meta.FlagHash,
		&meta.FlagRule,
		&meta.FlagType,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询题目元数据失败: %w", err)
	}
	return meta, nil
}

func (r *ProblemRepository) GetFlagHashByProblemID(id string) (string, error) {
	var hash string
	err := r.db.QueryRow(`
		SELECT pf.flag_hash
		FROM problems p
		JOIN problem_flags pf ON pf.problem_id = p.id
		WHERE p.id = ? AND p.status = 'ready'
	`, strings.TrimSpace(id)).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", apperr.New(apperr.ErrNotFound, "题目不存在或当前不可提交")
		}
		return "", fmt.Errorf("查询 flag hash 失败: %w", err)
	}
	return hash, nil
}

func (r *ProblemRepository) GetProblems() ([]*models.Problem, error) {
	query := `
		SELECT p.id, p.title, p.category, p.type, p.description, p.base_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		WHERE p.status = 'ready'
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.base_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status
		ORDER BY p.created_at DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询题目列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.Problem, 0)
	for rows.Next() {
		p := &models.Problem{}
		var tagsJSON string
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Category,
			&p.Type,
			&p.Description,
			&p.BaseScore,
			&p.Difficulty,
			&p.DeployMode,
			&p.UseKata,
			&p.Status,
			&tagsJSON,
		); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
			p.Tags = []string{}
		}
		items = append(items, p)
	}
	return items, nil
}

func (r *ProblemRepository) GetProblemsWithStatus(userID string) ([]*models.Problem, error) {
	query := `
		SELECT p.id, p.title, p.category, p.type, p.description, p.base_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags,
		       IF(COUNT(s.id) > 0, 1, 0) as is_solved
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		LEFT JOIN submissions s ON p.id = s.problem_id AND s.user_id = ? AND s.is_correct = 1
		WHERE p.status = 'ready'
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.base_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status
		ORDER BY p.created_at DESC
	`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询题目列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.Problem, 0)
	for rows.Next() {
		p := &models.Problem{}
		var tagsJSON string
		var isSolved int
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Category,
			&p.Type,
			&p.Description,
			&p.BaseScore,
			&p.Difficulty,
			&p.DeployMode,
			&p.UseKata,
			&p.Status,
			&tagsJSON,
			&isSolved,
		); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
			p.Tags = []string{}
		}
		p.IsSolved = isSolved == 1
		items = append(items, p)
	}
	return items, nil
}

func (r *ProblemRepository) GetProblemByID(id string) (*models.Problem, error) {
	query := `
		SELECT p.id, p.title, p.category, p.type, p.description, p.base_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		WHERE p.id = ? AND p.status = 'ready'
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.base_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status
	`
	p := &models.Problem{}
	var tagsJSON string
	if err := r.db.QueryRow(query, strings.TrimSpace(id)).Scan(
		&p.ID,
		&p.Title,
		&p.Category,
		&p.Type,
		&p.Description,
		&p.BaseScore,
		&p.Difficulty,
		&p.DeployMode,
		&p.UseKata,
		&p.Status,
		&tagsJSON,
	); err != nil {
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

	id := uuid.New().String()
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "draft"
	}
	typeName := strings.ToLower(strings.TrimSpace(req.Type))
	if typeName == "" {
		typeName = "static"
	}
	deployMode := strings.ToLower(strings.TrimSpace(req.DeployMode))
	if deployMode == "" {
		deployMode = "shared"
	}

	_, err = tx.Exec(`
		INSERT INTO problems (
			id, title, category, type, description, base_score, difficulty,
			deploy_mode, use_kata, status, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, strings.TrimSpace(req.Title), strings.TrimSpace(req.Category), typeName,
		nullIfEmpty(req.Description), req.BaseScore, strings.TrimSpace(req.Difficulty),
		deployMode, req.UseKata, status, nullIfEmpty(req.CreatedBy))
	if err != nil {
		return nil, fmt.Errorf("创建题目失败: %w", err)
	}

	if err = r.upsertFlagTx(tx, id, &req.Flag); err != nil {
		return nil, err
	}
	if req.Container != nil {
		if err = r.upsertContainerTx(tx, id, req.Container); err != nil {
			return nil, err
		}
	}
	if err = r.syncProblemTagsTx(tx, id, req.Tags); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}
	return r.GetAdminProblemByID(id)
}

func (r *ProblemRepository) UpdateProblem(problemID string, req *models.UpdateProblemRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var exists string
	if scanErr := tx.QueryRow(`SELECT id FROM problems WHERE id = ?`, strings.TrimSpace(problemID)).Scan(&exists); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return fmt.Errorf("查询题目失败: %w", scanErr)
	}

	typeName := strings.ToLower(strings.TrimSpace(req.Type))
	if typeName == "" {
		typeName = "static"
	}
	deployMode := strings.ToLower(strings.TrimSpace(req.DeployMode))
	if deployMode == "" {
		deployMode = "shared"
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "draft"
	}

	_, err = tx.Exec(`
		UPDATE problems
		SET title = ?, category = ?, type = ?, description = ?, base_score = ?, difficulty = ?,
		    deploy_mode = ?, use_kata = ?, status = ?, updated_at = NOW()
		WHERE id = ?
	`, strings.TrimSpace(req.Title), strings.TrimSpace(req.Category), typeName,
		nullIfEmpty(req.Description), req.BaseScore, strings.TrimSpace(req.Difficulty),
		deployMode, req.UseKata, status, strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("更新题目失败: %w", err)
	}

	if err = r.upsertFlagTx(tx, strings.TrimSpace(problemID), &req.Flag); err != nil {
		return err
	}
	if req.Container != nil {
		if err = r.upsertContainerTx(tx, strings.TrimSpace(problemID), req.Container); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(`DELETE FROM problem_containers WHERE problem_id = ?`, strings.TrimSpace(problemID)); err != nil {
			return fmt.Errorf("清理容器配置失败: %w", err)
		}
	}
	if err = r.syncProblemTagsTx(tx, strings.TrimSpace(problemID), req.Tags); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) DeleteProblem(problemID string) error {
	cleanID := strings.TrimSpace(problemID)
	result, err := r.db.Exec(`DELETE FROM problems WHERE id = ?`, cleanID)
	if err != nil {
		return fmt.Errorf("删除题目失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "题目不存在")
	}

	// best-effort 清理题目镜像上传暂存目录（发布失败回滚场景）
	uploadDir := filepath.Join("..", "static", "images", "problems", cleanID)
	if rmErr := os.RemoveAll(uploadDir); rmErr != nil {
		return fmt.Errorf("删除题目成功但清理镜像暂存失败: %w", rmErr)
	}
	return nil
}

func (r *ProblemRepository) GetProblemAccessMeta(problemID string) (scope string, contestID, createdBy string, err error) {
	var createdByNS sql.NullString
	err = r.db.QueryRow(`SELECT created_by FROM problems WHERE id = ?`, strings.TrimSpace(problemID)).Scan(&createdByNS)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return "", "", "", fmt.Errorf("查询题目元信息失败: %w", err)
	}
	if createdByNS.Valid {
		createdBy = strings.TrimSpace(createdByNS.String)
	}
	return "platform", "", createdBy, nil
}

func (r *ProblemRepository) GetAdminProblemByID(problemID string) (*models.Problem, error) {
	p := &models.Problem{}
	query := `
		SELECT p.id, p.title, p.category, p.type, p.description, p.base_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		WHERE p.id = ?
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.base_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by
	`
	var tagsJSON string
	var createdBy sql.NullString
	if err := r.db.QueryRow(query, strings.TrimSpace(problemID)).Scan(
		&p.ID,
		&p.Title,
		&p.Category,
		&p.Type,
		&p.Description,
		&p.BaseScore,
		&p.Difficulty,
		&p.DeployMode,
		&p.UseKata,
		&p.Status,
		&createdBy,
		&tagsJSON,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询管理端题目失败: %w", err)
	}
	if createdBy.Valid {
		v := strings.TrimSpace(createdBy.String)
		if v != "" {
			p.CreatedBy = &v
		}
	}
	if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
		p.Tags = []string{}
	}

	flag, _ := r.getProblemFlag(problemID)
	container, _ := r.getProblemContainer(problemID)
	p.Flag = flag
	p.Container = container
	return p, nil
}

func (r *ProblemRepository) ListAdminProblems(keyword, _, status string) ([]*models.Problem, error) {
	args := make([]interface{}, 0, 2)
	conds := make([]string, 0, 2)
	if strings.TrimSpace(keyword) != "" {
		conds = append(conds, "p.title LIKE ?")
		args = append(args, "%"+strings.TrimSpace(keyword)+"%")
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
		SELECT p.id, p.title, p.category, p.type, p.description, p.base_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		%s
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.base_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by
		ORDER BY p.updated_at DESC
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
		var createdBy sql.NullString
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Category,
			&p.Type,
			&p.Description,
			&p.BaseScore,
			&p.Difficulty,
			&p.DeployMode,
			&p.UseKata,
			&p.Status,
			&createdBy,
			&tagsJSON,
		); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		if createdBy.Valid {
			v := strings.TrimSpace(createdBy.String)
			if v != "" {
				p.CreatedBy = &v
			}
		}
		if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
			p.Tags = []string{}
		}
		items = append(items, p)
	}
	return items, nil
}

func (r *ProblemRepository) upsertFlagTx(tx *sql.Tx, problemID string, req *models.CreateFlagRequest) error {
	if req == nil {
		return fmt.Errorf("flag 配置不能为空")
	}
	flagRule := strings.TrimSpace(req.FlagRule)
	if flagRule == "" {
		flagRule = "exact"
	}
	flagType := strings.ToLower(strings.TrimSpace(req.FlagType))
	if flagType == "" {
		flagType = "static"
	}
	flagInject := strings.ToLower(strings.TrimSpace(req.FlagInject))
	if flagInject == "" {
		flagInject = "env"
	}
	flagEnvKey := strings.TrimSpace(req.FlagEnvKey)
	if flagEnvKey == "" {
		flagEnvKey = "FLAG"
	}

	var flagHash interface{}
	if flagType == "static" {
		plain := strings.TrimSpace(req.FlagPlain)
		if plain == "" {
			return fmt.Errorf("静态 flag 不能为空")
		}
		if strings.EqualFold(flagRule, "case_insensitive") {
			plain = strings.ToLower(plain)
		}
		h := utils.HashFlag(plain)
		flagHash = h
	} else {
		flagHash = nil
	}

	_, err := tx.Exec(`
		INSERT INTO problem_flags (
			problem_id, flag_rule, flag_hash, flag_example, flag_type,
			flag_inject, flag_env_key, flag_sidecar_pattern
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			flag_rule = VALUES(flag_rule),
			flag_hash = VALUES(flag_hash),
			flag_example = VALUES(flag_example),
			flag_type = VALUES(flag_type),
			flag_inject = VALUES(flag_inject),
			flag_env_key = VALUES(flag_env_key),
			flag_sidecar_pattern = VALUES(flag_sidecar_pattern)
	`, problemID, flagRule, flagHash, req.FlagExample, flagType, flagInject, flagEnvKey, req.FlagSidecarPattern)
	if err != nil {
		return fmt.Errorf("写入 flag 配置失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) upsertContainerTx(tx *sql.Tx, problemID string, req *models.CreateContainerRequest) error {
	if req == nil {
		return nil
	}
	imageTag := strings.TrimSpace(req.ImageTag)
	if imageTag == "" {
		imageTag = "latest"
	}
	cpu := strings.TrimSpace(req.CPULimit)
	if cpu == "" {
		cpu = "500m"
	}
	memory := strings.TrimSpace(req.MemoryLimit)
	if memory == "" {
		memory = "512Mi"
	}
	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = 3600
	}
	_, err := tx.Exec(`
		INSERT INTO problem_containers (
			problem_id, image_name, image_tag, challenge_port,
			sidecar_image, sidecar_port, cpu_limit, memory_limit,
			env, ttl_seconds, ingress_host_tpl
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			image_name = VALUES(image_name),
			image_tag = VALUES(image_tag),
			challenge_port = VALUES(challenge_port),
			sidecar_image = VALUES(sidecar_image),
			sidecar_port = VALUES(sidecar_port),
			cpu_limit = VALUES(cpu_limit),
			memory_limit = VALUES(memory_limit),
			env = VALUES(env),
			ttl_seconds = VALUES(ttl_seconds),
			ingress_host_tpl = VALUES(ingress_host_tpl)
	`,
		problemID,
		strings.TrimSpace(req.ImageName),
		imageTag,
		req.ChallengePort,
		strings.TrimSpace(req.SidecarImage),
		req.SidecarPort,
		cpu,
		memory,
		nullJSON(req.Env),
		ttl,
		req.IngressHostTpl,
	)
	if err != nil {
		return fmt.Errorf("写入容器配置失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) syncProblemTagsTx(tx *sql.Tx, problemID string, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM problem_tags WHERE problem_id = ?`, problemID); err != nil {
		return fmt.Errorf("清理题目标签失败: %w", err)
	}
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		if _, err := tx.Exec(`INSERT INTO problem_tags (problem_id, tag) VALUES (?, ?)`, problemID, t); err != nil {
			return fmt.Errorf("写入题目标签失败: %w", err)
		}
	}
	return nil
}

func (r *ProblemRepository) getProblemFlag(problemID string) (*models.ProblemFlag, error) {
	flag := &models.ProblemFlag{}
	var hash sql.NullString
	var example sql.NullString
	var pattern sql.NullString
	query := `
		SELECT problem_id, flag_rule, flag_hash, flag_example, flag_type, flag_inject, flag_env_key, flag_sidecar_pattern
		FROM problem_flags
		WHERE problem_id = ?
	`
	err := r.db.QueryRow(query, strings.TrimSpace(problemID)).Scan(
		&flag.ProblemID,
		&flag.FlagRule,
		&hash,
		&example,
		&flag.FlagType,
		&flag.FlagInject,
		&flag.FlagEnvKey,
		&pattern,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if hash.Valid {
		v := hash.String
		flag.FlagHash = &v
	}
	if example.Valid {
		v := example.String
		flag.FlagExample = &v
	}
	if pattern.Valid {
		v := pattern.String
		flag.FlagSidecarPattern = &v
	}
	return flag, nil
}

func (r *ProblemRepository) getProblemContainer(problemID string) (*models.ProblemContainer, error) {
	container := &models.ProblemContainer{}
	var env sql.NullString
	var ingress sql.NullString
	query := `
		SELECT problem_id, image_name, image_tag, challenge_port, sidecar_image, sidecar_port,
		       cpu_limit, memory_limit, env, ttl_seconds, ingress_host_tpl
		FROM problem_containers
		WHERE problem_id = ?
	`
	err := r.db.QueryRow(query, strings.TrimSpace(problemID)).Scan(
		&container.ProblemID,
		&container.ImageName,
		&container.ImageTag,
		&container.ChallengePort,
		&container.SidecarImage,
		&container.SidecarPort,
		&container.CPULimit,
		&container.MemoryLimit,
		&env,
		&container.TTLSeconds,
		&ingress,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if env.Valid {
		container.Env = json.RawMessage(env.String)
	}
	if ingress.Valid {
		v := ingress.String
		container.IngressHostTpl = &v
	}
	return container, nil
}

func nullIfEmpty(v string) interface{} {
	t := strings.TrimSpace(v)
	if t == "" {
		return nil
	}
	return t
}

func nullJSON(v json.RawMessage) interface{} {
	trimmed := strings.TrimSpace(string(v))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	return trimmed
}

func (r *ProblemRepository) CreateProblemDraft(req *models.CreateProblemDraftRequest, createdBy string) (string, error) {
	id := uuid.New().String()
	typeName := strings.ToLower(strings.TrimSpace(req.Type))
	if typeName == "" {
		typeName = "static"
	}
	_, err := r.db.Exec(`
		INSERT INTO problems (id, title, category, type, description, base_score, difficulty, deploy_mode, use_kata, status, created_by)
		VALUES (?, '', ?, ?, NULL, 0, 'easy', 'shared', 0, 'draft', ?)
	`, id, strings.TrimSpace(req.Category), typeName, nullIfEmpty(createdBy))
	if err != nil {
		return "", fmt.Errorf("创建草稿失败: %w", err)
	}
	return id, nil
}

func (r *ProblemRepository) UpdateProblemBasic(problemID string, req *models.UpdateProblemBasicRequest) error {
	_, err := r.db.Exec(`
		UPDATE problems
		SET title = ?, category = ?, type = ?, description = ?, base_score = ?, difficulty = ?,
		    deploy_mode = ?, use_kata = ?, updated_at = NOW()
		WHERE id = ?
	`, strings.TrimSpace(req.Title), strings.TrimSpace(req.Category), strings.TrimSpace(req.Type),
		nullIfEmpty(req.Description), req.BaseScore, strings.TrimSpace(req.Difficulty),
		strings.TrimSpace(req.DeployMode), req.UseKata, strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("保存基础信息失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) UpsertProblemFlag(problemID string, req *models.CreateFlagRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = r.upsertFlagTx(tx, strings.TrimSpace(problemID), req); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) UpsertProblemContainer(problemID string, req *models.CreateContainerRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if req == nil {
		if _, err = tx.Exec(`DELETE FROM problem_containers WHERE problem_id = ?`, strings.TrimSpace(problemID)); err != nil {
			return fmt.Errorf("删除容器配置失败: %w", err)
		}
	} else {
		if err = r.upsertContainerTx(tx, strings.TrimSpace(problemID), req); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) PublishProblem(problemID string) error {
	detail, err := r.GetAdminProblemByID(problemID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(detail.Title) == "" || detail.Description == nil || strings.TrimSpace(*detail.Description) == "" {
		return apperr.New(apperr.ErrBadRequest, "基础信息不完整，无法发布")
	}
	if detail.Flag == nil {
		return apperr.New(apperr.ErrBadRequest, "Flag 未配置，无法发布")
	}
	if detail.Type == "container_web" || detail.Type == "container_tcp" {
		if detail.Container == nil {
			return apperr.New(apperr.ErrBadRequest, "容器配置缺失，无法发布")
		}
		if strings.TrimSpace(detail.Container.ImageName) == "" || strings.TrimSpace(detail.Container.SidecarImage) == "" {
			return apperr.New(apperr.ErrBadRequest, "容器镜像配置不完整，无法发布")
		}
		buildTask, getBuildErr := r.GetLatestImageBuildTask(problemID)
		if getBuildErr != nil {
			return getBuildErr
		}
		if buildTask == nil || strings.ToLower(strings.TrimSpace(buildTask.Status)) != "success" {
			return apperr.New(apperr.ErrBadRequest, "容器镜像尚未构建成功，无法发布")
		}
		if detail.Type == "container_web" {
			hasIngress := detail.Container.IngressHostTpl != nil && strings.TrimSpace(*detail.Container.IngressHostTpl) != ""
			if !hasIngress {
				cfg, cfgErr := config.LoadConfig()
				if cfgErr == nil {
					tpl := strings.TrimSpace(cfg.Ingress.DomainTemplate)
					if tpl != "" {
						_, _ = r.db.Exec(`
							UPDATE problem_containers
							SET ingress_host_tpl = ?
							WHERE problem_id = ? AND (ingress_host_tpl IS NULL OR ingress_host_tpl = '')
						`, tpl, strings.TrimSpace(problemID))
						hasIngress = true
					}
				}
			}
			if !hasIngress {
				return apperr.New(apperr.ErrBadRequest, "container_web 缺少 ingress_host_tpl，无法发布")
			}
		}
	}

	_, err = r.db.Exec(`UPDATE problems SET status = 'ready', updated_at = NOW() WHERE id = ?`, strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("发布题目失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) CreateImageBuildTask(problemID, filePath string, fileSize int64, imageRepo, imageTag string) (*models.ImageBuildTask, error) {
	taskID := uuid.New().String()
	cleanProblemID := strings.TrimSpace(problemID)
	repoVal := strings.TrimSpace(imageRepo)
	tagVal := strings.TrimSpace(imageTag)
	if tagVal == "" {
		tagVal = time.Now().Format("2006-01-02")
	}
	if repoVal == "" {
		repoVal = fmt.Sprintf("ctf-exec/%s", cleanProblemID)
	}
	repoVal = normalizeRegistryRepo(repoVal)

	_, err := r.db.Exec(`
		INSERT INTO image_build_tasks (
			id, problem_id, file_path, file_size,
			status, current_step, image_repo, image_tag,
			retry_count, max_retry, started_at
		) VALUES (?, ?, ?, ?, 'pending', 'uploaded', ?, ?, 0, 3, NOW())
	`, taskID, cleanProblemID, strings.TrimSpace(filePath), fileSize, repoVal, tagVal)
	if err != nil {
		return nil, fmt.Errorf("创建构建任务失败: %w", err)
	}

	_, _ = r.db.Exec(`
		UPDATE problem_containers
		SET image_name = ?, image_tag = ?
		WHERE problem_id = ?
	`, repoVal, tagVal, cleanProblemID)

	return r.GetLatestImageBuildTask(cleanProblemID)
}

func (r *ProblemRepository) GetLatestImageBuildTask(problemID string) (*models.ImageBuildTask, error) {
	task := &models.ImageBuildTask{}
	var imageRepo sql.NullString
	var imageTag sql.NullString
	var errorLog sql.NullString
	var workerID sql.NullString
	var finishedAt sql.NullString

	err := r.db.QueryRow(`
		SELECT id, problem_id, file_path, file_size, status, current_step,
		       image_repo, image_tag, error_log, retry_count, max_retry,
		       worker_id, DATE_FORMAT(started_at, '%Y-%m-%d %H:%i:%s'),
		       IFNULL(DATE_FORMAT(finished_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM image_build_tasks
		WHERE problem_id = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, strings.TrimSpace(problemID)).Scan(
		&task.ID,
		&task.ProblemID,
		&task.FilePath,
		&task.FileSize,
		&task.Status,
		&task.CurrentStep,
		&imageRepo,
		&imageTag,
		&errorLog,
		&task.RetryCount,
		&task.MaxRetry,
		&workerID,
		&task.StartedAt,
		&finishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询构建任务失败: %w", err)
	}

	if imageRepo.Valid {
		v := strings.TrimSpace(imageRepo.String)
		task.ImageRepo = &v
	}
	if imageTag.Valid {
		v := strings.TrimSpace(imageTag.String)
		task.ImageTag = &v
	}
	if errorLog.Valid {
		v := errorLog.String
		task.ErrorLog = &v
	}
	if workerID.Valid {
		v := workerID.String
		task.WorkerID = &v
	}
	if strings.TrimSpace(finishedAt.String) != "" {
		v := finishedAt.String
		task.FinishedAt = &v
	}

	return task, nil
}

func (r *ProblemRepository) RunImageBuildPipeline(task *models.ImageBuildTask, uploadType string) {
	if task == nil {
		log.Printf("[build-pipeline] skip: nil task")
		return
	}
	taskID := strings.TrimSpace(task.ID)
	if taskID == "" {
		log.Printf("[build-pipeline] skip: empty task id")
		return
	}

	workerID := fmt.Sprintf("%s-%d", runtime.GOOS, os.Getpid())
	imageRepo := ""
	imageTag := "latest"
	if task.ImageRepo != nil {
		imageRepo = strings.TrimSpace(*task.ImageRepo)
	}
	if task.ImageTag != nil && strings.TrimSpace(*task.ImageTag) != "" {
		imageTag = strings.TrimSpace(*task.ImageTag)
	}
	if imageTag == "latest" || imageTag == "" {
		imageTag = time.Now().Format("2006-01-02")
	}
	if imageRepo == "" {
		imageRepo = fmt.Sprintf("ctf-exec/%s", strings.TrimSpace(task.ProblemID))
	}
	imageRepo = normalizeRegistryRepo(imageRepo)
	imageRef := fmt.Sprintf("%s:%s", imageRepo, imageTag)
	uploadType = strings.TrimSpace(uploadType)
	if uploadType == "" || strings.EqualFold(uploadType, "auto") {
		detectedType, detectErr := detectUploadTypeFromArchive(task.FilePath)
		if detectErr != nil {
			_ = r.updateImageBuildTask(taskID, "failed", "preparing", detectErr.Error(), workerID, true)
			log.Printf("[build-pipeline] task=%s step=preparing detect_upload_type_failed err=%v", taskID, detectErr)
			return
		}
		uploadType = detectedType
	}
	uploadType = normalizeUploadType(uploadType)

	log.Printf("[build-pipeline] task=%s problem=%s start upload_type=%s image=%s", taskID, strings.TrimSpace(task.ProblemID), uploadType, imageRef)

	if err := r.updateImageBuildTask(taskID, "running", "preparing", "", workerID, false); err != nil {
		log.Printf("[build-pipeline] task=%s step=preparing update_status_failed err=%v", taskID, err)
		return
	}
	log.Printf("[build-pipeline] task=%s step=preparing ok", taskID)

	workDir := filepath.Join(os.TempDir(), "ctf_exec", "build_ctx", taskID)
	defer os.RemoveAll(workDir)

	if uploadType == "docker_save" {
		if err := r.updateImageBuildTask(taskID, "running", "loading", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=loading update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=loading cmd=docker load -i %s", taskID, strings.TrimSpace(task.FilePath))
		loadOut, loadErr := runCommand("docker", "load", "-i", task.FilePath)
		if loadErr != nil {
			log.Printf("[build-pipeline] task=%s step=loading failed err=%v output=%s", taskID, loadErr, strings.TrimSpace(loadOut))
			_ = r.updateImageBuildTask(taskID, "failed", "loading", sanitizeCommandOutput(loadOut, loadErr), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=loading ok output=%s", taskID, strings.TrimSpace(loadOut))
		loadedRef, parseErr := parseLoadedImageRef(loadOut)
		if parseErr != nil {
			log.Printf("[build-pipeline] task=%s step=loading parse_loaded_ref_failed err=%v", taskID, parseErr)
			_ = r.updateImageBuildTask(taskID, "failed", "loading", parseErr.Error(), workerID, true)
			return
		}

		if err := r.updateImageBuildTask(taskID, "running", "tagging", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=tagging update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=tagging cmd=docker tag %s %s", taskID, loadedRef, imageRef)
		if output, err := runCommand("docker", "tag", loadedRef, imageRef); err != nil {
			log.Printf("[build-pipeline] task=%s step=tagging failed err=%v output=%s", taskID, err, strings.TrimSpace(output))
			_ = r.updateImageBuildTask(taskID, "failed", "tagging", sanitizeCommandOutput(output, err), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=tagging ok", taskID)

		if err := r.updateImageBuildTask(taskID, "running", "pushing", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=pushing update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=pushing cmd=docker push %s", taskID, imageRef)
		if output, err := runCommand("docker", "push", imageRef); err != nil {
			log.Printf("[build-pipeline] task=%s step=pushing failed err=%v output=%s", taskID, err, strings.TrimSpace(output))
			_ = r.updateImageBuildTask(taskID, "failed", "pushing", sanitizeCommandOutput(output, err), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=pushing ok", taskID)
	} else {
		if mkErr := os.MkdirAll(workDir, 0o755); mkErr != nil {
			log.Printf("[build-pipeline] task=%s step=extracting mkdir_failed err=%v", taskID, mkErr)
			_ = r.updateImageBuildTask(taskID, "failed", "extracting", mkErr.Error(), workerID, true)
			return
		}
		if err := r.updateImageBuildTask(taskID, "running", "extracting", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=extracting update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=extracting from=%s to=%s", taskID, strings.TrimSpace(task.FilePath), workDir)
		if err := extractArchive(task.FilePath, workDir); err != nil {
			log.Printf("[build-pipeline] task=%s step=extracting failed err=%v", taskID, err)
			_ = r.updateImageBuildTask(taskID, "failed", "extracting", err.Error(), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=extracting ok", taskID)

		composeFile, findErr := findComposeFile(workDir)
		if findErr != nil {
			log.Printf("[build-pipeline] task=%s step=extracting compose_file_not_found err=%v", taskID, findErr)
			_ = r.updateImageBuildTask(taskID, "failed", "extracting", findErr.Error(), workerID, true)
			return
		}
		composeDir := filepath.Dir(composeFile)
		composeEnv := []string{
			"IMAGE_REPO=" + imageRepo,
			"IMAGE_TAG=" + imageTag,
		}

		if err := r.updateImageBuildTask(taskID, "running", "building", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=building update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=building cmd=docker compose -f %s build", taskID, composeFile)
		if output, err := runCommandWithEnv(composeDir, composeEnv, "docker", "compose", "-f", composeFile, "build"); err != nil {
			log.Printf("[build-pipeline] task=%s step=building failed err=%v output=%s", taskID, err, strings.TrimSpace(output))
			_ = r.updateImageBuildTask(taskID, "failed", "building", sanitizeCommandOutput(output, err), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=building ok", taskID)

		if err := r.updateImageBuildTask(taskID, "running", "pushing", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=pushing update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=pushing cmd=docker compose -f %s push", taskID, composeFile)
		if output, err := runCommandWithEnv(composeDir, composeEnv, "docker", "compose", "-f", composeFile, "push"); err != nil {
			log.Printf("[build-pipeline] task=%s step=pushing failed err=%v output=%s", taskID, err, strings.TrimSpace(output))
			_ = r.updateImageBuildTask(taskID, "failed", "pushing", sanitizeCommandOutput(output, err), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=pushing ok", taskID)

		if err := r.updateImageBuildTask(taskID, "running", "verifying_registry", "", workerID, false); err != nil {
			log.Printf("[build-pipeline] task=%s step=verifying_registry update_status_failed err=%v", taskID, err)
			return
		}
		log.Printf("[build-pipeline] task=%s step=verifying_registry check=registry-http image=%s", taskID, imageRef)
		if err := verifyImageInRegistry(imageRepo, imageTag); err != nil {
			log.Printf("[build-pipeline] task=%s step=verifying_registry failed err=%v", taskID, err)
			_ = r.updateImageBuildTask(taskID, "failed", "verifying_registry", err.Error(), workerID, true)
			return
		}
		log.Printf("[build-pipeline] task=%s step=verifying_registry ok", taskID)

		_ = r.updateImageBuildTask(taskID, "success", "completed", "", workerID, true)
		log.Printf("[build-pipeline] task=%s completed success image=%s", taskID, imageRef)
		return
	}

	if err := r.updateImageBuildTask(taskID, "running", "verifying_registry", "", workerID, false); err != nil {
		log.Printf("[build-pipeline] task=%s step=verifying_registry update_status_failed err=%v", taskID, err)
		return
	}
	log.Printf("[build-pipeline] task=%s step=verifying_registry check=registry-http image=%s", taskID, imageRef)
	if err := verifyImageInRegistry(imageRepo, imageTag); err != nil {
		log.Printf("[build-pipeline] task=%s step=verifying_registry failed err=%v", taskID, err)
		_ = r.updateImageBuildTask(taskID, "failed", "verifying_registry", err.Error(), workerID, true)
		return
	}
	log.Printf("[build-pipeline] task=%s step=verifying_registry ok", taskID)

	_ = r.updateImageBuildTask(taskID, "success", "completed", "", workerID, true)
	log.Printf("[build-pipeline] task=%s completed success image=%s", taskID, imageRef)
}

func (r *ProblemRepository) StartImageBuildPipelineForProblem(problemID string) (bool, error) {
	detail, err := r.GetAdminProblemByID(problemID)
	if err != nil {
		return false, err
	}
	if detail.Type != "container_web" && detail.Type != "container_tcp" {
		return false, nil
	}

	task, err := r.GetLatestImageBuildTask(problemID)
	if err != nil {
		return false, err
	}
	if task == nil {
		return false, apperr.New(apperr.ErrBadRequest, "请先上传镜像包")
	}

	status := strings.ToLower(strings.TrimSpace(task.Status))
	if status == "success" {
		return false, nil
	}
	if status == "running" {
		return false, apperr.New(apperr.ErrBadRequest, "镜像构建进行中，请稍后重试发布")
	}

	_, err = r.db.Exec(`
		UPDATE image_build_tasks
		SET status = 'running', current_step = 'queued', error_log = NULL, finished_at = NULL, started_at = NOW(), worker_id = NULL
		WHERE id = ?
	`, strings.TrimSpace(task.ID))
	if err != nil {
		return false, fmt.Errorf("启动构建任务失败: %w", err)
	}

	task.Status = "running"
	task.CurrentStep = "queued"
	task.ErrorLog = nil
	go r.RunImageBuildPipeline(task, "auto")
	log.Printf("[build-pipeline] task=%s problem=%s queued by publish", strings.TrimSpace(task.ID), strings.TrimSpace(problemID))
	return true, nil
}

func (r *ProblemRepository) GetProblemChallengePort(problemID string) (int, error) {
	var port int
	err := r.db.QueryRow(`SELECT challenge_port FROM problem_containers WHERE problem_id = ?`, strings.TrimSpace(problemID)).Scan(&port)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, apperr.New(apperr.ErrBadRequest, "请先配置容器端口")
		}
		return 0, fmt.Errorf("查询 challenge_port 失败: %w", err)
	}
	return port, nil
}

func (r *ProblemRepository) updateImageBuildTask(taskID, status, currentStep, errorLog, workerID string, finished bool) error {
	var finishedAt interface{}
	if finished {
		finishedAt = time.Now()
	}
	_, err := r.db.Exec(`
		UPDATE image_build_tasks
		SET status = ?, current_step = ?, error_log = NULLIF(?, ''), worker_id = ?, finished_at = ?
		WHERE id = ?
	`, strings.TrimSpace(status), strings.TrimSpace(currentStep), strings.TrimSpace(errorLog), strings.TrimSpace(workerID), finishedAt, strings.TrimSpace(taskID))
	if err != nil {
		return fmt.Errorf("更新构建任务状态失败: %w", err)
	}
	return nil
}

func extractTarArchive(tarPath, dst string) error {
	file, err := os.Open(strings.TrimSpace(tarPath))
	if err != nil {
		return fmt.Errorf("打开 tar 文件失败: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file
	header := make([]byte, 2)
	if _, err = file.Read(header); err != nil {
		return fmt.Errorf("读取 tar 头失败: %w", err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("重置 tar 文件游标失败: %w", err)
	}
	if header[0] == 0x1f && header[1] == 0x8b {
		gzReader, gzErr := gzip.NewReader(file)
		if gzErr != nil {
			return fmt.Errorf("解析 gzip 失败: %w", gzErr)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)
	for {
		hdr, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return fmt.Errorf("读取 tar 条目失败: %w", nextErr)
		}

		target := filepath.Join(dst, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("tar 包含非法路径: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("创建父目录失败: %w", err)
			}
			out, createErr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if createErr != nil {
				return fmt.Errorf("创建文件失败: %w", createErr)
			}
			if _, err = io.Copy(out, tarReader); err != nil {
				_ = out.Close()
				return fmt.Errorf("写入文件失败: %w", err)
			}
			if closeErr := out.Close(); closeErr != nil {
				return fmt.Errorf("关闭文件失败: %w", closeErr)
			}
		}
	}
	return nil
}

func extractZipArchive(zipPath, dst string) error {
	zr, err := zip.OpenReader(strings.TrimSpace(zipPath))
	if err != nil {
		return fmt.Errorf("打开 zip 文件失败: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		target := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("zip 包含非法路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}
		src, openErr := f.Open()
		if openErr != nil {
			return fmt.Errorf("打开 zip 条目失败: %w", openErr)
		}
		out, createErr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr != nil {
			_ = src.Close()
			return fmt.Errorf("创建文件失败: %w", createErr)
		}
		if _, err = io.Copy(out, src); err != nil {
			_ = out.Close()
			_ = src.Close()
			return fmt.Errorf("写入文件失败: %w", err)
		}
		if closeErr := out.Close(); closeErr != nil {
			_ = src.Close()
			return fmt.Errorf("关闭文件失败: %w", closeErr)
		}
		if closeErr := src.Close(); closeErr != nil {
			return fmt.Errorf("关闭zip条目失败: %w", closeErr)
		}
	}
	return nil
}

func extractArchive(filePath, dst string) error {
	path := strings.TrimSpace(filePath)
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".zip") {
		return extractZipArchive(path, dst)
	}
	return extractTarArchive(path, dst)
}

func detectUploadTypeFromArchive(filePath string) (string, error) {
	path := strings.TrimSpace(filePath)
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return "", fmt.Errorf("识别上传类型失败: %w", err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			name := strings.ToLower(filepath.Base(f.Name))
			if name == "docker-compose.yml" || name == "docker-compose.yaml" || name == "compose.yml" || name == "compose.yaml" {
				return "compose_bundle", nil
			}
		}
		return "docker_save", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("识别上传类型失败: %w", err)
	}
	defer f.Close()

	var reader io.Reader = f
	header := make([]byte, 2)
	if _, err = f.Read(header); err != nil {
		return "", fmt.Errorf("识别上传类型失败: %w", err)
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("识别上传类型失败: %w", err)
	}
	if header[0] == 0x1f && header[1] == 0x8b {
		gzReader, gzErr := gzip.NewReader(f)
		if gzErr != nil {
			return "", fmt.Errorf("识别上传类型失败: %w", gzErr)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tr := tar.NewReader(reader)
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return "", fmt.Errorf("识别上传类型失败: %w", nextErr)
		}
		name := strings.ToLower(filepath.Base(hdr.Name))
		if name == "docker-compose.yml" || name == "docker-compose.yaml" || name == "compose.yml" || name == "compose.yaml" {
			return "compose_bundle", nil
		}
	}

	return "docker_save", nil
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runCommandWithEnv(cwd string, envPairs []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = strings.TrimSpace(cwd)
	}
	cmd.Env = append(os.Environ(), envPairs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func parseLoadedImageRef(output string) (string, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Loaded image:") {
			ref := strings.TrimSpace(strings.TrimPrefix(trimmed, "Loaded image:"))
			if ref != "" {
				return ref, nil
			}
		}
	}
	return "", fmt.Errorf("docker load 输出未解析到镜像名")
}

func normalizeUploadType(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "compose_bundle" {
		return "compose_bundle"
	}
	return "docker_save"
}

func normalizeRegistryRepo(repo string) string {
	trimmedRepo := strings.Trim(strings.TrimSpace(repo), "/")
	if trimmedRepo == "" {
		return trimmedRepo
	}
	if hasExplicitRegistryHost(trimmedRepo) {
		return trimmedRepo
	}
	host := strings.Trim(strings.TrimSpace(os.Getenv("REGISTRY_HOST")), "/")
	if host == "" {
		return trimmedRepo
	}
	return host + "/" + trimmedRepo
}

func hasExplicitRegistryHost(repo string) bool {
	parts := strings.Split(repo, "/")
	if len(parts) < 2 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(parts[0]))
	return strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost"
}

func findComposeFile(root string) (string, error) {
	candidates := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	for _, name := range candidates {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("未找到 compose 文件")
}

func chooseHostPortInRange(minPort, maxPort int) (int, error) {
	if minPort <= 0 {
		minPort = 20000
	}
	if maxPort <= 0 {
		maxPort = 30000
	}
	if minPort > maxPort {
		minPort, maxPort = maxPort, minPort
	}
	if minPort < 1024 {
		minPort = 1024
	}
	if maxPort > 65535 {
		maxPort = 65535
	}
	if minPort > maxPort {
		return 0, fmt.Errorf("端口范围不合法")
	}

	pool := make([]int, 0, maxPort-minPort+1)
	for p := minPort; p <= maxPort; p++ {
		pool = append(pool, p)
	}
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	rnd.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	for _, candidate := range pool {
		if isPortFree(candidate) {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("端口范围内无可用端口")
}

func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func sanitizeCommandOutput(output string, execErr error) string {
	result := strings.TrimSpace(output)
	if execErr != nil {
		if result == "" {
			return execErr.Error()
		}
		return result + "\n" + execErr.Error()
	}
	return result
}

func verifyImageInRegistry(imageRepo, imageTag string) error {
	host, repo, parseErr := parseRegistryHostAndRepo(imageRepo)
	if parseErr != nil {
		return parseErr
	}
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = "latest"
	}

	manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", strings.Trim(repo, "/"), tag)
	urls := []string{
		"http://" + strings.Trim(host, "/") + manifestPath,
		"https://" + strings.Trim(host, "/") + manifestPath,
	}

	client := &http.Client{Timeout: 8 * time.Second}
	var lastErr error
	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("registry verify failed url=%s status=%d body=%s", u, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	imageRef := strings.Trim(strings.TrimSpace(imageRepo), "/") + ":" + tag
	if output, err := runCommand("docker", "manifest", "inspect", imageRef); err == nil {
		_ = output
		return nil
	} else {
		fallbackErr := fmt.Errorf("docker manifest inspect fallback failed: %s", sanitizeCommandOutput(output, err))
		if lastErr != nil {
			return fmt.Errorf("%v; %v", lastErr, fallbackErr)
		}
		return fallbackErr
	}
}

func parseRegistryHostAndRepo(imageRepo string) (string, string, error) {
	repo := strings.Trim(strings.TrimSpace(imageRepo), "/")
	if repo == "" {
		return "", "", fmt.Errorf("image_repo 为空")
	}
	parts := strings.Split(repo, "/")
	if len(parts) >= 2 && hasExplicitRegistryHost(repo) {
		return parts[0], strings.Join(parts[1:], "/"), nil
	}
	host := strings.Trim(strings.TrimSpace(os.Getenv("REGISTRY_HOST")), "/")
	if host == "" {
		return "", "", fmt.Errorf("未配置 REGISTRY_HOST，无法校验仓库镜像")
	}
	return host, repo, nil
}
