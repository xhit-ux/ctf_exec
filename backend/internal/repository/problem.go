package repository

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
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
	"sort"
	"strconv"
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

// CleanupStaleDraftProblems 清理长期未发布的草稿题目。
func (r *ProblemRepository) CleanupStaleDraftProblems(ctx context.Context, staleAfter time.Duration) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("problem repository unavailable")
	}
	if staleAfter < 0 {
		staleAfter = 0
	}

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM problems
		WHERE status = 'draft'
		  AND COALESCE(updated_at, created_at) < DATE_SUB(NOW(), INTERVAL ? SECOND)
	`, int(staleAfter.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("cleanup stale draft problems failed: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read stale draft cleanup affected rows failed: %w", err)
	}

	return affected, nil
}

// StartDraftCleanupScheduler 启动草稿题目清理任务。
func (r *ProblemRepository) StartDraftCleanupScheduler(interval, staleAfter time.Duration) {
	if r == nil || r.db == nil {
		log.Println("[draft-cleanup] problem repository unavailable, scheduler skipped")
		return
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	go func() {
		deleted, err := r.CleanupStaleDraftProblems(context.Background(), staleAfter)
		if err != nil {
			log.Printf("[draft-cleanup] initial cleanup failed: %v", err)
		} else {
			log.Printf("[draft-cleanup] initial cleanup completed, deleted=%d", deleted)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			deleted, runErr := r.CleanupStaleDraftProblems(context.Background(), staleAfter)
			if runErr != nil {
				log.Printf("[draft-cleanup] scheduled cleanup failed: %v", runErr)
				continue
			}
			if deleted > 0 {
				log.Printf("[draft-cleanup] scheduled cleanup completed, deleted=%d", deleted)
			}
		}
	}()
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
	attachments, err := r.ListProblemAttachments(p.ID)
	if err != nil {
		return nil, err
	}
	p.Attachments = attachments
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

func (r *ProblemRepository) ListProblemAttachments(problemID string) ([]*models.Attachment, error) {
	rows, err := r.db.Query(`
		SELECT id, problem_id, file_name, bucket_name, object_key, file_size, sha256, created_at
		FROM attachments
		WHERE problem_id = ?
		ORDER BY created_at DESC
	`, strings.TrimSpace(problemID))
	if err != nil {
		return nil, fmt.Errorf("查询附件失败: %w", err)
	}
	defer rows.Close()

	items := make([]*models.Attachment, 0)
	for rows.Next() {
		item := &models.Attachment{}
		var sha sql.NullString
		if scanErr := rows.Scan(
			&item.ID,
			&item.ProblemID,
			&item.FileName,
			&item.BucketName,
			&item.ObjectKey,
			&item.FileSize,
			&sha,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("读取附件失败: %w", scanErr)
		}
		if sha.Valid {
			v := strings.TrimSpace(sha.String)
			if v != "" {
				item.Sha256 = &v
			}
		}
		if item.FilePath == "" {
			item.FilePath = item.ObjectKey
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取附件失败: %w", err)
	}
	return items, nil
}

func (r *ProblemRepository) ReplaceProblemAttachment(problemID, fileName, bucketName, objectKey string, fileSize int64, sha256 string) (*models.Attachment, error) {
	cleanID := strings.TrimSpace(problemID)
	cleanName := strings.TrimSpace(fileName)
	cleanBucket := strings.TrimSpace(bucketName)
	cleanKey := strings.TrimSpace(objectKey)
	cleanSHA := strings.TrimSpace(sha256)

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM attachments WHERE problem_id = ?`, cleanID); err != nil {
		return nil, fmt.Errorf("清理旧附件失败: %w", err)
	}

	newID := uuid.New().String()
	now := time.Now()
	if _, err = tx.Exec(`
		INSERT INTO attachments (id, problem_id, file_name, bucket_name, object_key, file_size, sha256, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, newID, cleanID, cleanName, cleanBucket, cleanKey, fileSize, nullIfEmpty(cleanSHA), now); err != nil {
		return nil, fmt.Errorf("写入附件失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	item := &models.Attachment{
		ID:         newID,
		ProblemID:  cleanID,
		FileName:   cleanName,
		BucketName: cleanBucket,
		ObjectKey:  cleanKey,
		FileSize:   fileSize,
		CreatedAt:  now,
	}
	if cleanSHA != "" {
		item.Sha256 = &cleanSHA
	}
	return item, nil
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
			buildTask, getBuildErr := r.GetLatestImageBuildTask(problemID)
			if getBuildErr != nil {
				return getBuildErr
			}
			if buildTask != nil {
				return apperr.New(apperr.ErrBadRequest, "容器配置缺失，无法发布")
			}
			// 无容器配置且没有镜像构建记录时，自动降级为 static。
			_, _ = r.db.Exec(`UPDATE problems SET type = 'static' WHERE id = ?`, strings.TrimSpace(problemID))
			detail.Type = "static"
		} else {
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

type problemContainerRuntimeSpec struct {
	ProblemType     string
	UseKata         bool
	ImageName       string
	ImageTag        string
	ChallengePort   int
	CPULimit        string
	MemoryLimit     string
	Env             json.RawMessage
	TTLSeconds      int
	IngressHostTpl  string
	GlobalIngressTP string
}

func (r *ProblemRepository) GetChallengeInstance(problemID, userID string) (*models.ChallengeInstance, error) {
	instance := &models.ChallengeInstance{}
	var podName sql.NullString
	var namespace sql.NullString
	var accessURL sql.NullString
	var nodePort sql.NullInt64
	var startedAt sql.NullString
	var expiredAt sql.NullString
	var destroyedAt sql.NullString

	err := r.db.QueryRow(`
		SELECT id, user_id, problem_id, pod_name, namespace, access_url, node_port, status,
		       IFNULL(DATE_FORMAT(started_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(expired_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(destroyed_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM challenge_instances
		WHERE user_id = ? AND problem_id = ?
		LIMIT 1
	`, strings.TrimSpace(userID), strings.TrimSpace(problemID)).Scan(
		&instance.ID,
		&instance.UserID,
		&instance.ProblemID,
		&podName,
		&namespace,
		&accessURL,
		&nodePort,
		&instance.Status,
		&startedAt,
		&expiredAt,
		&destroyedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询实例状态失败: %w", err)
	}

	if podName.Valid {
		v := strings.TrimSpace(podName.String)
		if v != "" {
			instance.PodName = &v
		}
	}
	if namespace.Valid {
		v := strings.TrimSpace(namespace.String)
		if v != "" {
			instance.Namespace = &v
		}
	}
	if accessURL.Valid {
		v := strings.TrimSpace(accessURL.String)
		if v != "" {
			instance.AccessURL = &v
		}
	}
	if nodePort.Valid && nodePort.Int64 > 0 {
		v := int(nodePort.Int64)
		instance.NodePort = &v
	}
	if startedAt.Valid {
		v := strings.TrimSpace(startedAt.String)
		if v != "" {
			instance.StartedAt = &v
		}
	}
	if expiredAt.Valid {
		v := strings.TrimSpace(expiredAt.String)
		if v != "" {
			instance.ExpiredAt = &v
		}
	}
	if destroyedAt.Valid {
		v := strings.TrimSpace(destroyedAt.String)
		if v != "" {
			instance.DestroyedAt = &v
		}
	}

	return instance, nil
}

func (r *ProblemRepository) StartChallengeInstance(problemID, userID string) (*models.ChallengeInstance, error) {
	cleanProblemID := strings.TrimSpace(problemID)
	cleanUserID := strings.TrimSpace(userID)
	if cleanProblemID == "" || cleanUserID == "" {
		return nil, apperr.New(apperr.ErrBadRequest, "problem_id/user_id 不能为空")
	}

	spec, err := r.getProblemContainerRuntimeSpec(cleanProblemID)
	if err != nil {
		return nil, err
	}

	existing, err := r.GetChallengeInstance(cleanProblemID, cleanUserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		switch strings.ToLower(strings.TrimSpace(existing.Status)) {
		case "running":
			return existing, nil
		case "pending":
			return existing, nil
		case "stopping":
			return nil, apperr.New(apperr.ErrConflict, "实例正在销毁中，请稍后重试")
		}
	}

	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		return nil, fmt.Errorf("加载配置失败: %w", cfgErr)
	}

	instanceID := uuid.New().String()
	if existing != nil && strings.TrimSpace(existing.ID) != "" {
		instanceID = strings.TrimSpace(existing.ID)
	}
	namespace := buildChallengeNamespace(cfg.Container.NamespacePrefix, cleanUserID)
	podName := buildChallengePodName(cleanProblemID, cleanUserID)
	serviceName := podName + "-svc"
	ingressName := podName + "-ing"
	host := renderIngressHost(spec.IngressHostTpl, cfg.Ingress.DomainTemplate, instanceID, cleanProblemID, cleanUserID, podName)

	ttlSeconds := spec.TTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = cfg.Container.DefaultContainerTTL
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}

	if err := r.upsertChallengeInstancePending(instanceID, cleanUserID, cleanProblemID, podName, namespace, ttlSeconds); err != nil {
		return nil, err
	}

	if err := ensureChallengeNamespace(cfg, namespace); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("创建命名空间失败: %w", err)
	}

	podManifest, err := buildChallengePodManifest(namespace, podName, spec, cfg)
	if err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, err
	}
	if _, err := applyManifest(cfg, podManifest); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("创建 Pod 失败: %w", err)
	}

	svcManifest := buildChallengeServiceManifest(namespace, serviceName, podName, spec)
	if _, err := applyManifest(cfg, svcManifest); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("创建 Service 失败: %w", err)
	}

	accessURL := ""
	var nodePort *int
	if spec.ProblemType == "container_web" {
		ingManifest := buildChallengeIngressManifest(namespace, ingressName, serviceName, host, cfg.Ingress.Class, cfg.Ingress.TLSEnabled, cfg.Ingress.TLSSecret)
		if _, err := applyManifest(cfg, ingManifest); err != nil {
			_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
			return nil, fmt.Errorf("创建 Ingress 失败: %w", err)
		}

		scheme := "http"
		if cfg.Ingress.TLSEnabled {
			scheme = "https"
		}
		if host != "" {
			accessURL = fmt.Sprintf("%s://%s", scheme, host)
		} else {
			accessURL = fmt.Sprintf("http://%s.%s.svc.cluster.local", serviceName, namespace)
		}
	} else {
		value, fetchErr := getServiceNodePort(cfg, namespace, serviceName)
		if fetchErr != nil {
			_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
			return nil, fmt.Errorf("读取 Service NodePort 失败: %w", fetchErr)
		}
		nodePort = &value
		nodeHost := resolveNodeHost()
		accessURL = fmt.Sprintf("%s:%d", nodeHost, value)
	}

	if err := r.markChallengeInstanceRunning(cleanUserID, cleanProblemID, accessURL, nodePort); err != nil {
		return nil, err
	}

	return r.GetChallengeInstance(cleanProblemID, cleanUserID)
}

func (r *ProblemRepository) DestroyChallengeInstance(problemID, userID string) error {
	cleanProblemID := strings.TrimSpace(problemID)
	cleanUserID := strings.TrimSpace(userID)
	if cleanProblemID == "" || cleanUserID == "" {
		return apperr.New(apperr.ErrBadRequest, "problem_id/user_id 不能为空")
	}

	instance, err := r.GetChallengeInstance(cleanProblemID, cleanUserID)
	if err != nil {
		return err
	}
	if instance == nil {
		return apperr.New(apperr.ErrNotFound, "实例不存在")
	}
	if strings.EqualFold(strings.TrimSpace(instance.Status), "destroyed") {
		return nil
	}

	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		return fmt.Errorf("加载配置失败: %w", cfgErr)
	}

	namespace := buildChallengeNamespace(cfg.Container.NamespacePrefix, cleanUserID)
	if instance.Namespace != nil && strings.TrimSpace(*instance.Namespace) != "" {
		namespace = strings.TrimSpace(*instance.Namespace)
	}
	podName := buildChallengePodName(cleanProblemID, cleanUserID)
	if instance.PodName != nil && strings.TrimSpace(*instance.PodName) != "" {
		podName = strings.TrimSpace(*instance.PodName)
	}
	serviceName := podName + "-svc"
	ingressName := podName + "-ing"

	if _, err := r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'stopping'
		WHERE user_id = ? AND problem_id = ?
	`, cleanUserID, cleanProblemID); err != nil {
		return fmt.Errorf("更新实例状态失败: %w", err)
	}

	if err := deleteResourceIgnoreNotFound(cfg, namespace, "ingress", ingressName); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return fmt.Errorf("删除 Ingress 失败: %w", err)
	}
	if err := deleteResourceIgnoreNotFound(cfg, namespace, "service", serviceName); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return fmt.Errorf("删除 Service 失败: %w", err)
	}
	if err := deleteResourceIgnoreNotFound(cfg, namespace, "pod", podName); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return fmt.Errorf("删除 Pod 失败: %w", err)
	}

	_, err = r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'destroyed', access_url = NULL, node_port = NULL,
		    destroyed_at = NOW(), expired_at = NULL
		WHERE user_id = ? AND problem_id = ?
	`, cleanUserID, cleanProblemID)
	if err != nil {
		return fmt.Errorf("更新实例销毁状态失败: %w", err)
	}

	return nil
}

func (r *ProblemRepository) getProblemContainerRuntimeSpec(problemID string) (*problemContainerRuntimeSpec, error) {
	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		return nil, fmt.Errorf("加载配置失败: %w", cfgErr)
	}

	spec := &problemContainerRuntimeSpec{GlobalIngressTP: strings.TrimSpace(cfg.Ingress.DomainTemplate)}
	err := r.db.QueryRow(`
		SELECT p.type, p.use_kata,
		       IFNULL(pc.image_name, ''), IFNULL(pc.image_tag, ''), IFNULL(pc.challenge_port, 0),
		       IFNULL(pc.cpu_limit, ''), IFNULL(pc.memory_limit, ''), pc.env,
		       IFNULL(pc.ttl_seconds, 0), IFNULL(pc.ingress_host_tpl, '')
		FROM problems p
		LEFT JOIN problem_containers pc ON pc.problem_id = p.id
		WHERE p.id = ? AND p.status = 'ready'
	`, strings.TrimSpace(problemID)).Scan(
		&spec.ProblemType,
		&spec.UseKata,
		&spec.ImageName,
		&spec.ImageTag,
		&spec.ChallengePort,
		&spec.CPULimit,
		&spec.MemoryLimit,
		&spec.Env,
		&spec.TTLSeconds,
		&spec.IngressHostTpl,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在或未发布")
		}
		return nil, fmt.Errorf("读取题目容器配置失败: %w", err)
	}

	spec.ProblemType = strings.ToLower(strings.TrimSpace(spec.ProblemType))
	if spec.ProblemType != "container_web" && spec.ProblemType != "container_tcp" {
		return nil, apperr.New(apperr.ErrBadRequest, "该题目不是容器题")
	}
	if strings.TrimSpace(spec.ImageName) == "" || strings.TrimSpace(spec.ImageTag) == "" {
		return nil, apperr.New(apperr.ErrBadRequest, "题目镜像配置不完整，请先完成发布构建")
	}
	if spec.ChallengePort <= 0 {
		return nil, apperr.New(apperr.ErrBadRequest, "challenge_port 配置不合法")
	}

	return spec, nil
}

func (r *ProblemRepository) upsertChallengeInstancePending(instanceID, userID, problemID, podName, namespace string, ttlSeconds int) error {
	_, err := r.db.Exec(`
		INSERT INTO challenge_instances (
			id, user_id, problem_id, pod_name, namespace,
			status, started_at, expired_at, destroyed_at, access_url, node_port
		) VALUES (?, ?, ?, ?, ?, 'pending', NOW(), DATE_ADD(NOW(), INTERVAL ? SECOND), NULL, NULL, NULL)
		ON DUPLICATE KEY UPDATE
			id = VALUES(id),
			pod_name = VALUES(pod_name),
			namespace = VALUES(namespace),
			status = 'pending',
			started_at = NOW(),
			expired_at = DATE_ADD(NOW(), INTERVAL ? SECOND),
			destroyed_at = NULL,
			access_url = NULL,
			node_port = NULL
	`, strings.TrimSpace(instanceID), strings.TrimSpace(userID), strings.TrimSpace(problemID), strings.TrimSpace(podName), strings.TrimSpace(namespace), ttlSeconds, ttlSeconds)
	if err != nil {
		return fmt.Errorf("写入实例状态失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) markChallengeInstanceFailed(userID, problemID string) error {
	_, err := r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'failed'
		WHERE user_id = ? AND problem_id = ?
	`, strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("更新实例失败状态失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) markChallengeInstanceRunning(userID, problemID, accessURL string, nodePort *int) error {
	var nodePortValue any
	if nodePort != nil && *nodePort > 0 {
		nodePortValue = *nodePort
	}
	_, err := r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'running', access_url = ?, node_port = ?
		WHERE user_id = ? AND problem_id = ?
	`, strings.TrimSpace(accessURL), nodePortValue, strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("更新实例运行状态失败: %w", err)
	}
	return nil
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
	if detail.Container == nil {
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

func kubectlArgs(cfg *config.Config, args ...string) []string {
	result := make([]string, 0, len(args)+2)
	if cfg != nil && strings.TrimSpace(cfg.Container.KubeconfigPath) != "" {
		result = append(result, "--kubeconfig", strings.TrimSpace(cfg.Container.KubeconfigPath))
	}
	result = append(result, args...)
	return result
}

func runKubectl(cfg *config.Config, args ...string) (string, error) {
	return runCommand("kubectl", kubectlArgs(cfg, args...)...)
}

func ensureChallengeNamespace(cfg *config.Config, namespace string) error {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return fmt.Errorf("namespace 不能为空")
	}
	if _, err := runKubectl(cfg, "get", "namespace", ns); err == nil {
		return nil
	}
	if output, err := runKubectl(cfg, "create", "namespace", ns); err != nil {
		return fmt.Errorf("%s", sanitizeCommandOutput(output, err))
	}
	return nil
}

func applyManifest(cfg *config.Config, manifest string) (string, error) {
	tmpFile, err := os.CreateTemp("", "ctf-k8s-*.yaml")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(manifest); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	return runKubectl(cfg, "apply", "-f", tmpPath)
}

func deleteResourceIgnoreNotFound(cfg *config.Config, namespace, kind, name string) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(kind) == "" || strings.TrimSpace(name) == "" {
		return nil
	}
	if output, err := runKubectl(cfg, "delete", kind, name, "-n", namespace, "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("%s", sanitizeCommandOutput(output, err))
	}
	return nil
}

func getServiceNodePort(cfg *config.Config, namespace, serviceName string) (int, error) {
	output, err := runKubectl(cfg, "get", "service", serviceName, "-n", namespace, "-o", "jsonpath={.spec.ports[0].nodePort}")
	if err != nil {
		return 0, fmt.Errorf("%s", sanitizeCommandOutput(output, err))
	}
	port, parseErr := strconv.Atoi(strings.TrimSpace(output))
	if parseErr != nil || port <= 0 {
		return 0, fmt.Errorf("无效的 nodePort: %s", strings.TrimSpace(output))
	}
	return port, nil
}

func renderIngressHost(problemTpl, defaultTpl, instanceID, problemID, userID, podName string) string {
	tpl := strings.TrimSpace(problemTpl)
	if tpl == "" {
		tpl = strings.TrimSpace(defaultTpl)
	}
	if tpl == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"{id}", shortToken(instanceID),
		"{instance_id}", shortToken(instanceID),
		"{problem_id}", shortToken(problemID),
		"{user_id}", shortToken(userID),
		"{pod_name}", sanitizeK8sName(podName, 63),
	)
	return strings.TrimSpace(replacer.Replace(tpl))
}

func buildChallengeNamespace(prefix, userID string) string {
	cleanPrefix := sanitizeK8sName(prefix, 30)
	if cleanPrefix == "" {
		cleanPrefix = "ctf"
	}
	name := cleanPrefix + "-" + shortToken(userID)
	return sanitizeK8sName(name, 63)
}

func buildChallengePodName(problemID, userID string) string {
	name := fmt.Sprintf("chal-%s-%s", shortToken(problemID), shortToken(userID))
	return sanitizeK8sName(name, 63)
}

func shortToken(input string) string {
	clean := strings.ToLower(strings.TrimSpace(input))
	clean = strings.ReplaceAll(clean, "-", "")
	b := strings.Builder{}
	for _, ch := range clean {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		}
	}
	out := b.String()
	if out == "" {
		return "x0000000"
	}
	if len(out) > 8 {
		return out[:8]
	}
	for len(out) < 8 {
		out += "0"
	}
	return out
}

func sanitizeK8sName(input string, maxLen int) string {
	v := strings.ToLower(strings.TrimSpace(input))
	b := strings.Builder{}
	lastDash := false
	for _, ch := range v {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "challenge"
	}
	if maxLen > 0 && len(out) > maxLen {
		out = strings.Trim(out[:maxLen], "-")
		if out == "" {
			out = "challenge"
		}
	}
	return out
}

func parseContainerEnv(raw json.RawMessage) [][2]string {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}

	keyValues := make([][2]string, 0)
	obj := map[string]any{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			keyValues = append(keyValues, [2]string{key, fmt.Sprintf("%v", obj[k])})
		}
		return keyValues
	}

	arr := make([]map[string]any, 0)
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			name := strings.TrimSpace(fmt.Sprintf("%v", item["name"]))
			if name == "" {
				continue
			}
			value := ""
			if v, ok := item["value"]; ok {
				value = fmt.Sprintf("%v", v)
			}
			keyValues = append(keyValues, [2]string{name, value})
		}
	}

	return keyValues
}

func buildChallengePodManifest(namespace, podName string, spec *problemContainerRuntimeSpec, cfg *config.Config) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("容器配置为空")
	}
	imageRef := strings.TrimSpace(spec.ImageName) + ":" + strings.TrimSpace(spec.ImageTag)

	envItems := parseContainerEnv(spec.Env)
	envSection := ""
	if len(envItems) > 0 {
		b := strings.Builder{}
		b.WriteString("        env:\n")
		for _, pair := range envItems {
			b.WriteString(fmt.Sprintf("          - name: %s\n", pair[0]))
			b.WriteString(fmt.Sprintf("            value: %q\n", pair[1]))
		}
		envSection = b.String()
	}

	resourceSection := ""
	if strings.TrimSpace(spec.CPULimit) != "" || strings.TrimSpace(spec.MemoryLimit) != "" {
		b := strings.Builder{}
		b.WriteString("        resources:\n")
		b.WriteString("          limits:\n")
		if strings.TrimSpace(spec.CPULimit) != "" {
			b.WriteString(fmt.Sprintf("            cpu: %q\n", strings.TrimSpace(spec.CPULimit)))
		}
		if strings.TrimSpace(spec.MemoryLimit) != "" {
			b.WriteString(fmt.Sprintf("            memory: %q\n", strings.TrimSpace(spec.MemoryLimit)))
		}
		resourceSection = b.String()
	}

	runtimeClassSection := ""
	if spec.UseKata && cfg != nil && strings.TrimSpace(cfg.Container.KataRuntimeClass) != "" {
		runtimeClassSection = fmt.Sprintf("  runtimeClassName: %s\n", strings.TrimSpace(cfg.Container.KataRuntimeClass))
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
%s  restartPolicy: Never
  containers:
    - name: challenge
      image: %s
      imagePullPolicy: Always
      ports:
        - containerPort: %d
%s%s`, podName, namespace, podName, runtimeClassSection, imageRef, spec.ChallengePort, envSection, resourceSection)

	return manifest, nil
}

func buildChallengeServiceManifest(namespace, serviceName, podName string, spec *problemContainerRuntimeSpec) string {
	serviceType := "ClusterIP"
	servicePort := spec.ChallengePort
	if spec != nil && spec.ProblemType == "container_tcp" {
		serviceType = "NodePort"
	} else {
		servicePort = 80
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  type: %s
  selector:
    app: %s
  ports:
    - protocol: TCP
      port: %d
      targetPort: %d
`, serviceName, namespace, serviceType, podName, servicePort, spec.ChallengePort)
}

func buildChallengeIngressManifest(namespace, ingressName, serviceName, host, ingressClass string, tlsEnabled bool, tlsSecret string) string {
	hostBlock := ""
	if strings.TrimSpace(host) != "" {
		hostBlock = fmt.Sprintf("    - host: %s\n", strings.TrimSpace(host))
	} else {
		hostBlock = "    -\n"
	}

	ingressClassSection := ""
	if strings.TrimSpace(ingressClass) != "" {
		ingressClassSection = fmt.Sprintf("  ingressClassName: %s\n", strings.TrimSpace(ingressClass))
	}

	tlsSection := ""
	if tlsEnabled && strings.TrimSpace(host) != "" {
		tlsSection = "  tls:\n"
		tlsSection += fmt.Sprintf("    - hosts:\n        - %s\n", strings.TrimSpace(host))
		if strings.TrimSpace(tlsSecret) != "" {
			tlsSection += fmt.Sprintf("      secretName: %s\n", strings.TrimSpace(tlsSecret))
		}
	}

	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
spec:
%s%s  rules:
%s      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s
                port:
                  number: 80
`, ingressName, namespace, ingressClassSection, tlsSection, hostBlock, serviceName)
}

func resolveNodeHost() string {
	host := strings.TrimSpace(os.Getenv("K3S_NODE_HOST"))
	if host != "" {
		return host
	}
	name, err := os.Hostname()
	if err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "127.0.0.1"
}
