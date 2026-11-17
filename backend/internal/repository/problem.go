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
	_ "sort"    // unused; in split files
	_ "strconv" // unused; in split files
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/cache"
	"github.com/ctf_exec/backend/internal/config"
	k8s "github.com/ctf_exec/backend/internal/k8s"
	"github.com/ctf_exec/backend/internal/logger"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/google/uuid"
)

type ProblemRepository struct {
	db          *sql.DB
	k8s         *k8s.Client
	cfg         *config.Config
	redis       *cache.RedisClient
	minioClient MinioClient
}

// MinioClient 最小接口，避免循环导入；nil 表示 MinIO 未启用。
type MinioClient interface {
	Remove(ctx context.Context, category, objectName string) error
}

func NewProblemRepository(db *sql.DB, k8sClient *k8s.Client, cfg *config.Config, redisClient *cache.RedisClient, minioClient MinioClient) *ProblemRepository {
	return &ProblemRepository{db: db, k8s: k8sClient, cfg: cfg, redis: redisClient, minioClient: minioClient}
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
	var hash sql.NullString
	var pattern sql.NullString
	query := `
		SELECT p.id, p.title, p.platform_score, p.status, p.deploy_mode,
		       IFNULL(pf.flag_type, 'static'),
		       IFNULL(pf.flag_rule, 'exact'),
		       pf.flag_hash,
		       pf.flag_pattern
		FROM problems p
		JOIN problem_flags pf ON pf.problem_id = p.id
		WHERE p.id = ?
	`
	if err := r.db.QueryRow(query, strings.TrimSpace(id)).Scan(
		&meta.ID,
		&meta.Title,
		&meta.PlatformScore,
		&meta.Status,
		&meta.DeployMode,
		&meta.FlagType,
		&meta.FlagRule,
		&hash,
		&pattern,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询题目元数据失败: %w", err)
	}
	if hash.Valid {
		meta.FlagHash = strings.TrimSpace(hash.String)
	}
	if pattern.Valid {
		meta.FlagPattern = strings.TrimSpace(pattern.String)
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
		SELECT p.id, p.title, p.category, p.type, p.description, p.platform_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		WHERE p.status = 'ready'
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.platform_score,
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
			&p.PlatformScore,
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
		SELECT p.id, p.title, p.category, p.type, p.description, p.platform_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags,
		       IF(COUNT(s.id) > 0, 1, 0) as is_solved
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		LEFT JOIN submissions s ON p.id = s.problem_id AND s.user_id = ? AND s.is_correct = 1
		WHERE p.status = 'ready'
		GROUP BY p.id, p.title, p.category, p.type, p.description, p.platform_score,
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
			&p.PlatformScore,
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
	return r.getProblemByID(id, "")
}

// GetProblemByIDForUser 按用户视角返回详情（含 instance/connection_info/hints/prerequisites/is_solved/is_locked）。
// userID 为空表示匿名访问，等价于 GetProblemByID。
func (r *ProblemRepository) GetProblemByIDForUser(id, userID string) (*models.Problem, error) {
	return r.getProblemByID(id, strings.TrimSpace(userID))
}

func (r *ProblemRepository) getProblemByID(id, userID string) (*models.Problem, error) {
	query := `
		SELECT p.id, p.title, p.author, p.category, p.type, p.description, p.platform_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_at, p.updated_at,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		WHERE p.id = ? AND p.status = 'ready'
		GROUP BY p.id, p.title, p.author, p.category, p.type, p.description, p.platform_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_at, p.updated_at
	`
	p := &models.Problem{}
	var tagsJSON string
	var author sql.NullString
	if err := r.db.QueryRow(query, strings.TrimSpace(id)).Scan(
		&p.ID,
		&p.Title,
		&author,
		&p.Category,
		&p.Type,
		&p.Description,
		&p.PlatformScore,
		&p.Difficulty,
		&p.DeployMode,
		&p.UseKata,
		&p.Status,
		&p.CreatedAt,
		&p.UpdatedAt,
		&tagsJSON,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询题目失败: %w", err)
	}
	if author.Valid {
		v := strings.TrimSpace(author.String)
		if v != "" {
			p.Author = &v
		}
	}
	if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
		p.Tags = []string{}
	}

	// 单附件。
	attachment, err := r.GetProblemAttachment(p.ID)
	if err != nil {
		return nil, err
	}
	p.Attachment = attachment

	// 提示与前置依赖（platform scope）。
	if hints, hintErr := r.ListProblemHints(p.ID, userID, nil); hintErr == nil {
		// 未解锁的 content 抹掉。
		for _, h := range hints {
			if !h.IsUnlocked {
				h.Content = ""
			}
		}
		p.Hints = hints
	}
	if prereqs, prErr := r.ListProblemPrerequisites(p.ID, userID); prErr == nil {
		p.Prerequisites = prereqs
	}

	// 容器实例 + connection_info（仅容器题）。
	if userID != "" && (p.Type == "container_web" || p.Type == "container_tcp") {
		inst, instErr := r.GetChallengeInstanceForUser(p.ID, userID)
		if instErr == nil && inst != nil {
			p.Instance = &models.InstanceView{
				Status:    inst.Status,
				AccessURL: inst.AccessURL,
				StartedAt: inst.StartedAt,
				ExpiresAt: inst.ExpiredAt, RemainingSeconds: calcRemaining(inst.ExpiredAt),
			}
			if container, _ := r.getProblemContainer(p.ID); container != nil {
				p.Connection = models.BuildConnectionInfo(p.Type, inst, container)
			}
		}
	}

	// is_solved / is_locked（仅有 userID 时计算）。
	if userID != "" {
		var solved int
		_ = r.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE problem_id = ? AND user_id = ? AND is_correct = 1`,
			p.ID, userID).Scan(&solved)
		p.IsSolved = solved > 0

		if all, lockErr := r.AreAllPrerequisitesSolvedByUser(p.ID, userID); lockErr == nil {
			p.IsLocked = !all
		}
	}

	// flag_example：仅用于题面提示 flag 格式
	if flagExample := r.getFlagExample(p.ID); flagExample != nil {
		p.Flag = &models.ProblemFlag{FlagExample: flagExample}
	}

	return p, nil
}

// 注：v3.0 后取消了"一键创建/更新整道题目"的接口。
// 题目录入由 5 步向导驱动：
//   CreateProblemDraft → UpdateProblemBasic → UpsertProblemFlag
//   → (UpsertProblemContainer | SaveProblemImage) → SaveProblemAttachment
//   → PublishProblem。

func (r *ProblemRepository) DeleteProblem(problemID string) error {
	cleanID := strings.TrimSpace(problemID)
	logger.Info("开始删除题目", "problem_id", cleanID)

	r.cleanupProblemK8sResources(cleanID)
	r.cleanupRegistryImages(cleanID)
	r.cleanupAttachments(cleanID)
	r.cleanupUploadedFiles(cleanID)
	r.cleanupBuildTempDirs(cleanID)

	result, err := r.db.Exec(`DELETE FROM problems WHERE id = ?`, cleanID)
	if err != nil {
		logger.Error("删除题目失败", "problem_id", cleanID, "err", err)
		return fmt.Errorf("删除题目失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if rows == 0 {
		return apperr.New(apperr.ErrNotFound, "题目不存在")
	}
	logger.Info("题目已删除", "problem_id", cleanID)
	return nil
}

func (r *ProblemRepository) cleanupRegistryImages(problemID string) {
	rows, err := r.db.Query(`
		SELECT image_repo, image_tag FROM image_build_tasks
		WHERE problem_id = ? AND status = 'success' AND image_repo IS NOT NULL
	`, strings.TrimSpace(problemID))
	if err != nil {
		logger.Warn("查询镜像构建记录失败", "problem_id", problemID, "err", err)
		return
	}
	defer rows.Close()

	registryHost := strings.TrimSpace(r.cfg.Registry.Host)
	if registryHost == "" {
		return
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}
	httpClient := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	for rows.Next() {
		var repo, tag sql.NullString
		if scanErr := rows.Scan(&repo, &tag); scanErr != nil {
			continue
		}
		imageRepo := "ctf-exec/" + strings.TrimSpace(problemID)
		if repo.Valid && strings.TrimSpace(repo.String) != "" {
			imageRepo = strings.TrimSpace(repo.String)
		}
		imageRepo = r.stripRegistryHost(imageRepo)
		imageTag := "latest"
		if tag.Valid && strings.TrimSpace(tag.String) != "" {
			imageTag = strings.TrimSpace(tag.String)
		}
		r.deleteRegistryManifest(httpClient, imageRepo, imageTag)

		imageRef := fmt.Sprintf("%s/%s:%s", strings.Trim(r.cfg.Registry.Host, "/"), imageRepo, imageTag)
		if output, err := runCommand("docker", "rmi", imageRef); err != nil {
			logger.Warn("本地镜像清理失败(非致命)", "image", imageRef, "err", strings.TrimSpace(sanitizeCommandOutput(output, err)))
		}
	}
}

func (r *ProblemRepository) stripRegistryHost(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost") {
		return parts[1]
	}
	return repo
}

const registryManifestMediaTypes = "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json"

func (r *ProblemRepository) deleteRegistryManifest(httpClient *http.Client, repo, tag string) {
	registryHost := strings.Trim(strings.TrimSpace(r.cfg.Registry.Host), "/")
	url := fmt.Sprintf("http://%s/v2/%s/manifests/%s", registryHost, repo, tag)

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		logger.Warn("创建 registry 请求失败", "repo", repo, "tag", tag, "err", err)
		return
	}
	req.Header.Set("Accept", registryManifestMediaTypes)
	if r.cfg.Registry.Username != "" {
		req.SetBasicAuth(r.cfg.Registry.Username, r.cfg.Registry.Password)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Warn("获取镜像 digest 失败", "repo", repo, "tag", tag, "err", err)
		return
	}
	resp.Body.Close()

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		logger.Warn("未找到镜像 digest", "repo", repo, "tag", tag, "http_status", resp.StatusCode)
		return
	}

	delURL := fmt.Sprintf("http://%s/v2/%s/manifests/%s", registryHost, repo, digest)
	delReq, _ := http.NewRequest("DELETE", delURL, nil)
	if r.cfg.Registry.Username != "" {
		delReq.SetBasicAuth(r.cfg.Registry.Username, r.cfg.Registry.Password)
	}

	delResp, delErr := httpClient.Do(delReq)
	if delErr != nil {
		logger.Warn("删除 registry 镜像失败", "repo", repo, "tag", tag, "digest", digest, "err", delErr)
		return
	}
	delResp.Body.Close()

	if delResp.StatusCode == http.StatusAccepted || delResp.StatusCode == http.StatusOK {
		logger.Info("已删除 registry 镜像", "repo", repo, "tag", tag, "digest", digest)
	} else {
		logger.Warn("删除镜像返回异常状态", "repo", repo, "tag", tag, "digest", digest, "http_status", delResp.StatusCode)
	}
}

func (r *ProblemRepository) cleanupAttachments(problemID string) {
	rows, err := r.db.Query(`
		SELECT bucket_name, object_key FROM attachments WHERE problem_id = ?
	`, strings.TrimSpace(problemID))
	if err != nil {
		logger.Warn("查询附件失败", "problem_id", problemID, "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var bucket, key sql.NullString
		if scanErr := rows.Scan(&bucket, &key); scanErr != nil {
			continue
		}
		cleanBucket := ""
		cleanKey := ""
		if bucket.Valid {
			cleanBucket = strings.TrimSpace(bucket.String)
		}
		if key.Valid {
			cleanKey = strings.TrimSpace(key.String)
		}
		if cleanKey == "" {
			continue
		}

		if cleanBucket != "" && cleanBucket != "static" && r.minioClient != nil {
			category, objName := splitMinioObjectKey(cleanKey)
			if rmErr := r.minioClient.Remove(context.Background(), category, objName); rmErr != nil {
				logger.Warn("MinIO 附件删除失败", "key", cleanKey, "err", rmErr)
			}
		} else {
			if rmErr := os.Remove(cleanKey); rmErr != nil && !os.IsNotExist(rmErr) {
				logger.Warn("静态附件删除失败", "path", cleanKey, "err", rmErr)
			}
		}
	}
}

func splitMinioObjectKey(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "misc", key
}

func (r *ProblemRepository) cleanupUploadedFiles(problemID string) {
	uploadDir := filepath.Join("..", "static", "images", "problems", strings.TrimSpace(problemID))
	if rmErr := os.RemoveAll(uploadDir); rmErr != nil {
		logger.Warn("清理上传目录失败", "path", uploadDir, "err", rmErr)
	}
}

func (r *ProblemRepository) cleanupBuildTempDirs(problemID string) {
	rows, err := r.db.Query(`SELECT id FROM image_build_tasks WHERE problem_id = ?`, strings.TrimSpace(problemID))
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var taskID string
		if scanErr := rows.Scan(&taskID); scanErr != nil {
			continue
		}
		buildDir := filepath.Join(os.TempDir(), "ctf_exec", "build_ctx", strings.TrimSpace(taskID))
		_ = os.RemoveAll(buildDir)
	}
}

func (r *ProblemRepository) cleanupProblemK8sResources(problemID string) {
	rows, err := r.db.Query(`
		SELECT user_id, problem_id, namespace, pod_name
		FROM challenge_instances
		WHERE problem_id = ? AND status IN ('running', 'pending')
	`, strings.TrimSpace(problemID))
	if err != nil {
		logger.Warn("查询运行实例失败", "problem_id", problemID, "err", err)
		return
	}
	defer rows.Close()

	type cleanupTarget struct {
		userID, problemID string
		namespace, pod    sql.NullString
	}
	for rows.Next() {
		var t cleanupTarget
		if scanErr := rows.Scan(&t.userID, &t.problemID, &t.namespace, &t.pod); scanErr != nil {
			continue
		}
		ns := buildChallengeNamespace(r.cfg.Container.NamespacePrefix, t.userID)
		podName := buildChallengePodName(t.problemID, t.userID)
		if t.namespace.Valid && strings.TrimSpace(t.namespace.String) != "" {
			ns = strings.TrimSpace(t.namespace.String)
		}
		if t.pod.Valid && strings.TrimSpace(t.pod.String) != "" {
			podName = strings.TrimSpace(t.pod.String)
		}
		svcName := podName + "-svc"
		ingName := podName + "-ing"
		if err := r.destroyK8sResources(ns, podName, svcName, ingName, t.userID, t.problemID); err != nil {
			logger.Warn("清理 K8s 资源失败", "problem_id", t.problemID, "user_id", t.userID, "err", err)
		}
	}
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
		SELECT p.id, p.title, p.author, p.category, p.type, p.description, p.platform_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by, p.source_problem_id,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		WHERE p.id = ?
		GROUP BY p.id, p.title, p.author, p.category, p.type, p.description, p.platform_score,
		         p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by, p.source_problem_id
	`
	var tagsJSON string
	var author sql.NullString
	var createdBy sql.NullString
	var sourceProblemID sql.NullString
	if err := r.db.QueryRow(query, strings.TrimSpace(problemID)).Scan(
		&p.ID,
		&p.Title,
		&author,
		&p.Category,
		&p.Type,
		&p.Description,
		&p.PlatformScore,
		&p.Difficulty,
		&p.DeployMode,
		&p.UseKata,
		&p.Status,
		&createdBy,
		&sourceProblemID,
		&tagsJSON,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return nil, fmt.Errorf("查询管理端题目失败: %w", err)
	}
	if author.Valid {
		v := strings.TrimSpace(author.String)
		if v != "" {
			p.Author = &v
		}
	}
	if createdBy.Valid {
		v := strings.TrimSpace(createdBy.String)
		if v != "" {
			p.CreatedBy = &v
		}
	}
	if sourceProblemID.Valid {
		v := strings.TrimSpace(sourceProblemID.String)
		if v != "" {
			p.SourceProblemID = &v
		}
	}
	if err := json.Unmarshal([]byte(tagsJSON), &p.Tags); err != nil {
		p.Tags = []string{}
	}

	flag, _ := r.getProblemFlag(problemID)
	container, _ := r.getProblemContainer(problemID)
	attachment, _ := r.GetProblemAttachment(problemID)
	p.Flag = flag
	p.Container = container
	p.Attachment = attachment
	return p, nil
}

func (r *ProblemRepository) ListAdminProblems(keyword, _, status, createdBy string) ([]*models.Problem, error) {
	args := make([]interface{}, 0, 3)
	conds := make([]string, 0, 3)
	if strings.TrimSpace(keyword) != "" {
		conds = append(conds, "p.title LIKE ?")
		args = append(args, "%"+strings.TrimSpace(keyword)+"%")
	}
	if strings.TrimSpace(status) != "" {
		conds = append(conds, "p.status = ?")
		args = append(args, strings.TrimSpace(status))
	}
	if strings.TrimSpace(createdBy) != "" {
		conds = append(conds, "p.created_by = ?")
		args = append(args, strings.TrimSpace(createdBy))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.title, p.author, p.category, p.type, p.description, p.platform_score,
		       p.difficulty, p.deploy_mode, p.use_kata, p.status, p.created_by,
		       IFNULL(CAST(JSON_ARRAYAGG(pt.tag) AS CHAR), '[]') as tags
		FROM problems p
		LEFT JOIN problem_tags pt ON p.id = pt.problem_id
		%s
		GROUP BY p.id, p.title, p.author, p.category, p.type, p.description, p.platform_score,
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
		var author sql.NullString
		var createdBy sql.NullString
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&author,
			&p.Category,
			&p.Type,
			&p.Description,
			&p.PlatformScore,
			&p.Difficulty,
			&p.DeployMode,
			&p.UseKata,
			&p.Status,
			&createdBy,
			&tagsJSON,
		); err != nil {
			return nil, fmt.Errorf("扫描题目失败: %w", err)
		}
		if author.Valid {
			v := strings.TrimSpace(author.String)
			if v != "" {
				p.Author = &v
			}
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

// upsertFlagTx 写入 problem_flags。
// 严格遵循 flag_mode_explain.md 的合法组合矩阵：
//   - static + (exact|case_insensitive)  → 仅写 flag_hash，flag_pattern=NULL
//   - static + (regex|contains)          → 仅写 flag_pattern，flag_hash=NULL
//   - dynamic + (exact|case_insensitive) → flag_hash 与 flag_pattern 均 NULL，
//     运行期由 challenge_instances.flag_value 即席校验
//   - dynamic + (regex|contains)         → 业务上不支持，handler 已拦截
func (r *ProblemRepository) upsertFlagTx(tx *sql.Tx, problemID string, req *models.SaveProblemFlagRequest) error {
	if req == nil {
		return fmt.Errorf("flag 配置不能为空")
	}

	flagType := strings.ToLower(strings.TrimSpace(req.FlagType))
	if flagType != "static" && flagType != "dynamic" {
		return fmt.Errorf("非法 flag_type: %s", req.FlagType)
	}
	flagRule := utils.NormalizeFlagRule(req.FlagRule)
	if flagRule == "" {
		return fmt.Errorf("非法 flag_rule: %s", req.FlagRule)
	}
	flagInject := strings.ToLower(strings.TrimSpace(req.FlagInject))
	if flagInject == "" {
		flagInject = "env"
	}
	if flagInject != "env" && flagInject != "sidecar" {
		return fmt.Errorf("非法 flag_inject: %s", req.FlagInject)
	}
	flagEnvKey := strings.TrimSpace(req.FlagEnvKey)
	if flagEnvKey == "" {
		flagEnvKey = "EXEC_FLAG"
	}

	var flagHash interface{}
	var flagPattern interface{}

	switch flagType {
	case "static":
		switch flagRule {
		case utils.FlagRuleExact, utils.FlagRuleCaseInsensitive:
			h, hashErr := utils.PrepareStaticFlagHash(flagRule, req.FlagValue)
			if hashErr != nil {
				return hashErr
			}
			flagHash = h
			// env 注入需要明文 flag，一并存入 flag_pattern
			if strings.TrimSpace(req.FlagInject) == "env" && req.FlagValue != "" {
				flagPattern = strings.TrimSpace(req.FlagValue)
			}
		case utils.FlagRuleRegex, utils.FlagRuleContains:
			pattern := strings.TrimSpace(req.FlagValue)
			if pattern == "" {
				return fmt.Errorf("static + %s 必须提供 pattern", flagRule)
			}
			if err := utils.ValidateFlagPattern(flagRule, pattern); err != nil {
				return err
			}
			flagPattern = pattern
		}
	case "dynamic":
		if flagRule == utils.FlagRuleRegex || flagRule == utils.FlagRuleContains {
			return fmt.Errorf("dynamic flag 不支持 regex/contains 规则")
		}
	}

	var sidecarPattern interface{}
	if req.FlagSidecarPattern != nil {
		v := strings.TrimSpace(*req.FlagSidecarPattern)
		if v != "" {
			sidecarPattern = v
		}
	}
	var example interface{}
	if req.FlagExample != nil {
		v := strings.TrimSpace(*req.FlagExample)
		if v != "" {
			example = v
		}
	}

	_, err := tx.Exec(`
		INSERT INTO problem_flags (
			problem_id, flag_type, flag_rule, flag_hash, flag_pattern, flag_example,
			flag_inject, flag_env_key, flag_sidecar_pattern
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			flag_type = VALUES(flag_type),
			flag_rule = VALUES(flag_rule),
			flag_hash = VALUES(flag_hash),
			flag_pattern = VALUES(flag_pattern),
			flag_example = VALUES(flag_example),
			flag_inject = VALUES(flag_inject),
			flag_env_key = VALUES(flag_env_key),
			flag_sidecar_pattern = VALUES(flag_sidecar_pattern)
	`, problemID, flagType, flagRule, flagHash, flagPattern, example, flagInject, flagEnvKey, sidecarPattern)
	if err != nil {
		return fmt.Errorf("写入 flag 配置失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) upsertContainerTx(tx *sql.Tx, problemID string, req *models.SaveProblemContainerRequest) error {
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
	ttl := resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)
	var sidecarImage interface{}
	if req.SidecarImage != nil {
		v := strings.TrimSpace(*req.SidecarImage)
		if v != "" {
			sidecarImage = v
		}
	}
	var sidecarPort interface{}
	if req.SidecarPort != nil && *req.SidecarPort > 0 {
		sidecarPort = *req.SidecarPort
	}
	var ingressTpl interface{}
	if req.IngressHostTpl != nil {
		v := strings.TrimSpace(*req.IngressHostTpl)
		if v != "" {
			ingressTpl = v
		}
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
		sidecarImage,
		sidecarPort,
		cpu,
		memory,
		nullJSON(req.Env),
		ttl,
		ingressTpl,
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

// getFlagExample 只查询 flag_example（轻量，用于题面展示），无数据返回 nil。
func (r *ProblemRepository) getFlagExample(problemID string) *string {
	var v sql.NullString
	_ = r.db.QueryRow(`SELECT flag_example FROM problem_flags WHERE problem_id = ?`,
		strings.TrimSpace(problemID)).Scan(&v)
	if v.Valid && strings.TrimSpace(v.String) != "" {
		s := v.String
		return &s
	}
	return nil
}

func (r *ProblemRepository) getProblemFlag(problemID string) (*models.ProblemFlag, error) {
	flag := &models.ProblemFlag{}
	var hash sql.NullString
	var pattern sql.NullString
	var example sql.NullString
	var sidecarPattern sql.NullString
	query := `
		SELECT problem_id, flag_type, flag_rule, flag_hash, flag_pattern, flag_example,
		       flag_inject, flag_env_key, flag_sidecar_pattern
		FROM problem_flags
		WHERE problem_id = ?
	`
	err := r.db.QueryRow(query, strings.TrimSpace(problemID)).Scan(
		&flag.ProblemID,
		&flag.FlagType,
		&flag.FlagRule,
		&hash,
		&pattern,
		&example,
		&flag.FlagInject,
		&flag.FlagEnvKey,
		&sidecarPattern,
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
	if pattern.Valid {
		v := pattern.String
		flag.FlagPattern = &v
	}
	if example.Valid {
		v := example.String
		flag.FlagExample = &v
	}
	if sidecarPattern.Valid {
		v := sidecarPattern.String
		flag.FlagSidecarPattern = &v
	}
	return flag, nil
}

func (r *ProblemRepository) getProblemContainer(problemID string) (*models.ProblemContainer, error) {
	container := &models.ProblemContainer{}
	var sidecarImage sql.NullString
	var sidecarPort sql.NullInt64
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
		&sidecarImage,
		&sidecarPort,
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
	if sidecarImage.Valid {
		v := strings.TrimSpace(sidecarImage.String)
		if v != "" {
			container.SidecarImage = &v
		}
	}
	if sidecarPort.Valid && sidecarPort.Int64 > 0 {
		v := int(sidecarPort.Int64)
		container.SidecarPort = &v
	}
	if env.Valid {
		container.Env = json.RawMessage(env.String)
	}
	if container.TTLSeconds <= 0 {
		container.TTLSeconds = resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)
	}
	if ingress.Valid {
		v := ingress.String
		container.IngressHostTpl = &v
	}
	return container, nil
}

// GetActiveInstanceFlagValue 取该用户在该题目下的当前 running/pending 实例的 flag_value（明文）。
// 不存在时返回 ("", nil)。
func (r *ProblemRepository) GetActiveInstanceFlagValue(userID, problemID string) (string, error) {
	var flag sql.NullString
	err := r.db.QueryRow(`
		SELECT flag_value
		FROM challenge_instances
		WHERE user_id = ? AND problem_id = ?
		  AND status IN ('running', 'pending')
		LIMIT 1
	`, strings.TrimSpace(userID), strings.TrimSpace(problemID)).Scan(&flag)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("查询实例 flag 失败: %w", err)
	}
	if !flag.Valid {
		return "", nil
	}
	return strings.TrimSpace(flag.String), nil
}

// GetProblemAttachment 返回该题目唯一的附件（v2.1：一题一包）。
// 不存在时返回 (nil, nil)。
func (r *ProblemRepository) GetProblemAttachment(problemID string) (*models.Attachment, error) {
	item := &models.Attachment{}
	var sha sql.NullString
	err := r.db.QueryRow(`
		SELECT id, problem_id, file_name, bucket_name, object_key, file_size, sha256, created_at
		FROM attachments
		WHERE problem_id = ?
		LIMIT 1
	`, strings.TrimSpace(problemID)).Scan(
		&item.ID,
		&item.ProblemID,
		&item.FileName,
		&item.BucketName,
		&item.ObjectKey,
		&item.FileSize,
		&sha,
		&item.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询附件失败: %w", err)
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
	return item, nil
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

const mysqlDateTimeLayout = "2006-01-02 15:04:05"

// parseMySQLDateTime 解析 MySQL DATE_FORMAT 输出的本地时间字符串。
func parseMySQLDateTime(value string) (time.Time, error) {
	return time.ParseInLocation(mysqlDateTimeLayout, strings.TrimSpace(value), time.Local)
}

func calcRemaining(expiredAt *string) *int {
	if expiredAt == nil {
		return nil
	}
	t, err := parseMySQLDateTime(*expiredAt)
	if err != nil {
		return nil
	}
	sec := int(time.Until(t).Seconds())
	if sec <= 0 {
		return nil
	}
	return &sec
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
	cleanCategory := strings.TrimSpace(req.Category)
	cleanType := strings.ToLower(strings.TrimSpace(req.Type))
	if cleanType == "" {
		cleanType = "static"
	}
	cleanSourceID := ""
	if req != nil && req.SourceProblemID != nil {
		cleanSourceID = strings.TrimSpace(*req.SourceProblemID)
	}

	if cleanSourceID == "" {
		id := uuid.New().String()
		// 草稿默认 platform_score=1（满足 chk_platform_score >= 1），等待 Step1 覆盖。
		_, err := r.db.Exec(`
			INSERT INTO problems (id, title, category, type, description, platform_score, difficulty, deploy_mode, use_kata, status, created_by, source_problem_id)
			VALUES (?, '', ?, ?, NULL, 1, 'easy', 'shared', 1, 'draft', ?, NULL)
		`, id, cleanCategory, cleanType, nullIfEmpty(createdBy))
		if err != nil {
			return "", fmt.Errorf("创建草稿失败: %w", err)
		}
		return id, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var sourceTitle, sourceCategory, sourceType, sourceDeployMode, sourceDifficulty string
	var sourceDescription sql.NullString
	var sourceAuthor sql.NullString
	var platformScore int
	var useKata bool
	if scanErr := tx.QueryRow(`
		SELECT title, category, type, description, author, platform_score, difficulty, deploy_mode, use_kata
		FROM problems
		WHERE id = ?
	`, cleanSourceID).Scan(&sourceTitle, &sourceCategory, &sourceType, &sourceDescription, &sourceAuthor, &platformScore, &sourceDifficulty, &sourceDeployMode, &useKata); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return "", apperr.New(apperr.ErrNotFound, "源题目不存在")
		}
		return "", fmt.Errorf("读取源题目失败: %w", scanErr)
	}

	id := uuid.New().String()
	_, err = tx.Exec(`
		INSERT INTO problems (
			id, title, category, type, description, author, platform_score, difficulty,
			deploy_mode, use_kata, status, created_by, source_problem_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?)
	`,
		id,
		sourceTitle,
		sourceCategory,
		sourceType,
		nullIfEmpty(sourceDescription.String),
		nullIfEmpty(sourceAuthor.String),
		platformScore,
		sourceDifficulty,
		sourceDeployMode,
		useKata,
		nullIfEmpty(createdBy),
		cleanSourceID,
	)
	if err != nil {
		return "", fmt.Errorf("创建草稿失败: %w", err)
	}

	if _, err = tx.Exec(`
		INSERT INTO problem_flags (problem_id, flag_type, flag_rule, flag_hash, flag_pattern, flag_example, flag_inject, flag_env_key, flag_sidecar_pattern)
		SELECT ?, flag_type, flag_rule, flag_hash, flag_pattern, flag_example, flag_inject, flag_env_key, flag_sidecar_pattern
		FROM problem_flags WHERE problem_id = ?
	`, id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制 flag 配置失败: %w", err)
	}
	if _, err = tx.Exec(`
		INSERT INTO attachments (id, problem_id, file_name, bucket_name, object_key, file_size, sha256, created_at)
		SELECT ?, ?, file_name, bucket_name, object_key, file_size, sha256, created_at
		FROM attachments WHERE problem_id = ?
	`, uuid.New().String(), id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制附件失败: %w", err)
	}
	if _, err = tx.Exec(`
		INSERT INTO problem_containers (problem_id, image_name, image_tag, challenge_port, sidecar_image, sidecar_port, cpu_limit, memory_limit, env, ttl_seconds, ingress_host_tpl)
		SELECT ?, image_name, image_tag, challenge_port, sidecar_image, sidecar_port, cpu_limit, memory_limit, env, ttl_seconds, ingress_host_tpl
		FROM problem_containers WHERE problem_id = ?
	`, id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制容器配置失败: %w", err)
	}
	if _, err = tx.Exec(`
		INSERT INTO problem_tags (problem_id, tag)
		SELECT ?, tag FROM problem_tags WHERE problem_id = ?
	`, id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制题目标签失败: %w", err)
	}
	if _, err = tx.Exec(`
		INSERT INTO problem_hints (id, problem_id, content, cost, sort_order, is_visible, created_at)
		SELECT ?, ?, content, cost, sort_order, is_visible, created_at
		FROM problem_hints WHERE problem_id = ?
	`, uuid.New().String(), id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制提示失败: %w", err)
	}
	if _, err = tx.Exec(`
		INSERT INTO problem_prerequisites (problem_id, prerequisite_id)
		SELECT ?, prerequisite_id FROM problem_prerequisites WHERE problem_id = ?
	`, id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制前置题目失败: %w", err)
	}
	if _, err = tx.Exec(`
		INSERT INTO image_build_tasks (
			id, problem_id, file_path, file_size, status, current_step, image_repo, image_tag,
			error_log, retry_count, max_retry, worker_id, claimed_at, started_at, finished_at
		)
		SELECT ?, ?, file_path, file_size, status, current_step, image_repo, image_tag,
			error_log, retry_count, max_retry, worker_id, claimed_at, started_at, finished_at
		FROM image_build_tasks WHERE problem_id = ? ORDER BY started_at DESC LIMIT 1
	`, uuid.New().String(), id, cleanSourceID); err != nil {
		return "", fmt.Errorf("复制镜像构建任务失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("提交事务失败: %w", err)
	}
	return id, nil
}

func (r *ProblemRepository) UpdateProblemBasic(problemID string, req *models.UpdateProblemBasicRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.Exec(`
		UPDATE problems
		SET title = ?, author = ?, category = ?, type = ?, description = ?,
		    platform_score = ?, difficulty = ?, deploy_mode = ?, use_kata = ?,
		    updated_at = NOW()
		WHERE id = ?
	`, strings.TrimSpace(req.Title), nullIfEmpty(req.Author),
		strings.TrimSpace(req.Category), strings.TrimSpace(req.Type),
		nullIfEmpty(req.Description), req.PlatformScore, strings.TrimSpace(req.Difficulty),
		strings.TrimSpace(req.DeployMode), req.UseKata, strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("保存基础信息失败: %w", err)
	}

	if err = r.syncProblemTagsTx(tx, strings.TrimSpace(problemID), req.Tags); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) UpsertProblemFlag(problemID string, req *models.SaveProblemFlagRequest) error {
	if req != nil && strings.EqualFold(strings.TrimSpace(req.FlagType), "dynamic") {
		deployMode, modeErr := r.getProblemDeployMode(problemID)
		if modeErr != nil {
			return modeErr
		}
		if deployMode == "shared" {
			return apperr.New(apperr.ErrBadRequest, "shared 部署模式不支持 dynamic flag，请改为 per_user")
		}
	}
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

func (r *ProblemRepository) UpsertProblemContainer(problemID string, req *models.SaveProblemContainerRequest) error {
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

// PublishProblem 校验并发布题目。校验清单见 v3.0 Part 7.4：
//  1. title / description / category / difficulty 完整
//  2. platform_score 落在难度区间
//  3. flag 已配置（且 dynamic 不允许 regex/contains）
//  4. container 题：镜像配置完整 + 镜像构建成功
//  5. container_web：ingress_host_tpl 存在（缺失时自动从全局模板填充）
func (r *ProblemRepository) PublishProblem(problemID string) error {
	detail, err := r.GetAdminProblemByID(problemID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(detail.Title) == "" {
		return apperr.New(apperr.ErrBadRequest, "title 不能为空")
	}
	if len(strings.TrimSpace(detail.Title)) > 128 {
		return apperr.New(apperr.ErrBadRequest, "title 不能超过 128 个字符")
	}
	if detail.Description == nil || strings.TrimSpace(*detail.Description) == "" {
		return apperr.New(apperr.ErrBadRequest, "description 不能为空")
	}
	if strings.TrimSpace(detail.Category) == "" {
		return apperr.New(apperr.ErrBadRequest, "category 不能为空")
	}
	if strings.TrimSpace(detail.Difficulty) == "" {
		return apperr.New(apperr.ErrBadRequest, "difficulty 不能为空")
	}
	if dr, ok := models.GetDifficultyRange(detail.Difficulty); ok {
		if detail.PlatformScore < dr.Min || detail.PlatformScore > dr.Max {
			return apperr.New(apperr.ErrBadRequest,
				fmt.Sprintf("difficulty=%s 要求 platform_score 在 [%d, %d] 范围内，当前=%d",
					detail.Difficulty, dr.Min, dr.Max, detail.PlatformScore))
		}
	}
	if detail.Flag == nil {
		return apperr.New(apperr.ErrBadRequest, "Flag 未配置，无法发布")
	}
	if detail.Flag.FlagType == "dynamic" &&
		(detail.Flag.FlagRule == utils.FlagRuleRegex || detail.Flag.FlagRule == utils.FlagRuleContains) {
		return apperr.New(apperr.ErrBadRequest, "dynamic flag 不支持 regex/contains 规则")
	}
	if detail.Flag.FlagType == "dynamic" && strings.EqualFold(strings.TrimSpace(detail.DeployMode), "shared") {
		return apperr.New(apperr.ErrBadRequest, "shared 部署模式不支持 dynamic flag，请改为 per_user")
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
			if strings.TrimSpace(detail.Container.ImageName) == "" {
				return apperr.New(apperr.ErrBadRequest, "image_name 不能为空，无法发布")
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

	if detail.SourceProblemID != nil && strings.TrimSpace(*detail.SourceProblemID) != "" {
		return r.publishDraftToSource(detail, strings.TrimSpace(*detail.SourceProblemID))
	}

	_, err = r.db.Exec(`UPDATE problems SET status = 'ready', updated_at = NOW() WHERE id = ?`, strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("发布题目失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) SetProblemStatus(problemID, status string) error {
	cleanID := strings.TrimSpace(problemID)
	cleanStatus := strings.TrimSpace(status)
	if cleanStatus != "ready" && cleanStatus != "hidden" {
		return apperr.New(apperr.ErrBadRequest, "status 只能为 ready 或 hidden")
	}
	res, err := r.db.Exec(`UPDATE problems SET status = ?, updated_at = NOW() WHERE id = ?`, cleanStatus, cleanID)
	if err != nil {
		logger.Error("更新题目状态失败", "problem_id", cleanID, "status", cleanStatus, "err", err)
		return fmt.Errorf("更新题目状态失败: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return apperr.New(apperr.ErrNotFound, "题目不存在")
	}
	logger.Info("题目状态已更新", "problem_id", cleanID, "status", cleanStatus)
	return nil
}

func (r *ProblemRepository) publishDraftToSource(detail *models.Problem, sourceID string) error {
	cleanDraftID := strings.TrimSpace(detail.ID)
	cleanSourceID := strings.TrimSpace(sourceID)
	if cleanDraftID == "" || cleanSourceID == "" {
		return apperr.New(apperr.ErrBadRequest, "无效的草稿或源题目ID")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var sourceExists int
	if err = tx.QueryRow(`SELECT COUNT(1) FROM problems WHERE id = ?`, cleanSourceID).Scan(&sourceExists); err != nil {
		return fmt.Errorf("校验源题目失败: %w", err)
	}
	if sourceExists == 0 {
		return apperr.New(apperr.ErrNotFound, "源题目不存在")
	}

	if _, err = tx.Exec(`
		UPDATE problems
		SET title = ?, author = ?, category = ?, type = ?, description = ?, platform_score = ?,
		    difficulty = ?, deploy_mode = ?, use_kata = ?, status = 'ready', updated_at = NOW()
		WHERE id = ?
	`, detail.Title, detail.Author, detail.Category, detail.Type, detail.Description, detail.PlatformScore,
		detail.Difficulty, detail.DeployMode, detail.UseKata, cleanSourceID); err != nil {
		return fmt.Errorf("回写源题目失败: %w", err)
	}

	cleanupStatements := []string{
		`DELETE FROM attachments WHERE problem_id = ?`,
		`DELETE FROM problem_flags WHERE problem_id = ?`,
		`DELETE FROM problem_containers WHERE problem_id = ?`,
		`DELETE FROM problem_tags WHERE problem_id = ?`,
		`DELETE FROM problem_hints WHERE problem_id = ?`,
		`DELETE FROM problem_prerequisites WHERE problem_id = ?`,
		`DELETE FROM image_build_tasks WHERE problem_id = ?`,
		`DELETE FROM challenge_instances WHERE problem_id = ?`,
		`DELETE FROM flag_mappings WHERE problem_id = ?`,
	}
	for _, stmt := range cleanupStatements {
		if _, err = tx.Exec(stmt, cleanSourceID); err != nil {
			return fmt.Errorf("清理源题目旧数据失败: %w", err)
		}
	}

	moveStatements := []string{
		`UPDATE attachments SET problem_id = ? WHERE problem_id = ?`,
		`UPDATE problem_flags SET problem_id = ? WHERE problem_id = ?`,
		`UPDATE problem_containers SET problem_id = ? WHERE problem_id = ?`,
		`UPDATE problem_tags SET problem_id = ? WHERE problem_id = ?`,
		`UPDATE problem_hints SET problem_id = ? WHERE problem_id = ?`,
		`UPDATE problem_prerequisites SET problem_id = ? WHERE problem_id = ?`,
		`UPDATE image_build_tasks SET problem_id = ? WHERE problem_id = ?`,
	}
	for _, stmt := range moveStatements {
		if _, err = tx.Exec(stmt, cleanSourceID, cleanDraftID); err != nil {
			return fmt.Errorf("迁移草稿数据失败: %w", err)
		}
	}

	if _, err = tx.Exec(`DELETE FROM problems WHERE id = ?`, cleanDraftID); err != nil {
		return fmt.Errorf("删除草稿失败: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
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
	repoVal = r.normalizeRegistryRepo(repoVal)

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
	imageRepo = r.normalizeRegistryRepo(imageRepo)
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
		host := r.cfg.Registry.Host
		if r.cfg.Registry.Username != "" && r.cfg.Registry.Password != "" {
			loginCmd := exec.Command("docker", "login", host,
				"-u", r.cfg.Registry.Username, "--password-stdin")
			loginCmd.Stdin = strings.NewReader(r.cfg.Registry.Password)
			if loginOut, loginErr := loginCmd.CombinedOutput(); loginErr != nil {
				log.Printf("[build-pipeline] task=%s step=login failed err=%v output=%s", taskID, loginErr, strings.TrimSpace(string(loginOut)))
				_ = r.updateImageBuildTask(taskID, "failed", "logging_in", sanitizeCommandOutput(string(loginOut), loginErr), workerID, true)
				return
			}
		}
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
		host := r.cfg.Registry.Host
		if r.cfg.Registry.Username != "" && r.cfg.Registry.Password != "" {
			loginCmd := exec.Command("docker", "login", host,
				"-u", r.cfg.Registry.Username, "--password-stdin")
			loginCmd.Stdin = strings.NewReader(r.cfg.Registry.Password)
			if loginOut, loginErr := loginCmd.CombinedOutput(); loginErr != nil {
				log.Printf("[build-pipeline] task=%s step=login failed err=%v output=%s", taskID, loginErr, strings.TrimSpace(string(loginOut)))
				_ = r.updateImageBuildTask(taskID, "failed", "logging_in", sanitizeCommandOutput(string(loginOut), loginErr), workerID, true)
				return
			}
		}
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
		if err := r.verifyImageInRegistry(imageRepo, imageTag); err != nil {
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
	if err := r.verifyImageInRegistry(imageRepo, imageTag); err != nil {
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

func (r *ProblemRepository) normalizeRegistryRepo(repo string) string {
	trimmedRepo := strings.Trim(strings.TrimSpace(repo), "/")
	if trimmedRepo == "" {
		return trimmedRepo
	}
	if hasExplicitRegistryHost(trimmedRepo) {
		return trimmedRepo
	}
	host := strings.Trim(strings.TrimSpace(r.cfg.Registry.Host), "/")
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

func (r *ProblemRepository) chooseHostPortInRange(minPort, maxPort int) (int, error) {
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
	usedPorts := r.activeNodePorts()
	rnd.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	for _, candidate := range pool {
		if !usedPorts[candidate] && isPortFree(candidate) {
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

func (r *ProblemRepository) verifyImageInRegistry(imageRepo, imageTag string) error {
	host, repo, parseErr := r.parseRegistryHostAndRepo(imageRepo)
	if parseErr != nil {
		return parseErr
	}
	tag := strings.TrimSpace(imageTag)
	if tag == "" {
		tag = "latest"
	}

	manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", strings.Trim(repo, "/"), tag)
	hasAuth := r.cfg.Registry.Username != "" && r.cfg.Registry.Password != ""
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
		req.Header.Set("Accept", registryManifestMediaTypes)
		if hasAuth {
			req.SetBasicAuth(r.cfg.Registry.Username, r.cfg.Registry.Password)
		}

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

func (r *ProblemRepository) parseRegistryHostAndRepo(imageRepo string) (string, string, error) {
	repo := strings.Trim(strings.TrimSpace(imageRepo), "/")
	if repo == "" {
		return "", "", fmt.Errorf("image_repo 为空")
	}
	parts := strings.Split(repo, "/")
	if len(parts) >= 2 && hasExplicitRegistryHost(repo) {
		return parts[0], strings.Join(parts[1:], "/"), nil
	}
	host := strings.Trim(strings.TrimSpace(r.cfg.Registry.Host), "/")
	if host == "" {
		return "", "", fmt.Errorf("未配置 REGISTRY_HOST，无法校验仓库镜像")
	}
	return host, repo, nil
}
