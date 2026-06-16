package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/logger"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
)

type problemContainerRuntimeSpec struct {
	ProblemType     string
	DeployMode      string
	UseKata         bool
	ImageName       string
	ImageTag        string
	ChallengePort   int
	CPULimit        string
	MemoryLimit     string
	Env             json.RawMessage
	IngressHostTpl  string
	GlobalIngressTP string
}

func resolveGlobalContainerTTL(ttl int) int {
	if ttl > 0 {
		return ttl
	}
	return 3600
}

func enrichChallengeInstance(instance *models.ChallengeInstance) {
	if instance == nil {
		return
	}
	instance.RemainingSeconds = calcRemaining(instance.ExpiredAt)
}

func (r *ProblemRepository) normalizeInstanceTTL(instance *models.ChallengeInstance) {
	if instance == nil {
		return
	}
	status := strings.ToLower(strings.TrimSpace(instance.Status))
	if status != "running" && status != "pending" {
		return
	}
	globalTTL := resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)
	if instance.StartedAt == nil || instance.ExpiredAt == nil {
		return
	}
	started, err1 := parseMySQLDateTime(*instance.StartedAt)
	expired, err2 := parseMySQLDateTime(*instance.ExpiredAt)
	if err1 != nil || err2 != nil {
		return
	}
	maxExpired := started.Add(time.Duration(globalTTL) * time.Second)
	if !expired.After(maxExpired) {
		return
	}
	newExpiredStr := maxExpired.Format("2006-01-02 15:04:05")
	_, err := r.db.Exec(`
		UPDATE challenge_instances
		SET expired_at = ?
		WHERE user_id = ? AND problem_id = ? AND status IN ('running', 'pending')
	`, newExpiredStr, strings.TrimSpace(instance.UserID), strings.TrimSpace(instance.ProblemID))
	if err != nil {
		log.Printf("[ttl-normalize] 修正实例过期时间失败: %s/%s err=%v", instance.UserID, instance.ProblemID, err)
		return
	}
	instance.ExpiredAt = &newExpiredStr
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

func (r *ProblemRepository) sharedCacheKey(problemID string) string {
	return "ctf:shared:instance:" + strings.TrimSpace(problemID)
}

func (r *ProblemRepository) sharedCacheGet(problemID string) *models.ChallengeInstance {
	if r.redis == nil {
		return nil
	}
	key := r.sharedCacheKey(problemID)
	data, err := r.redis.Get(context.Background(), key).Result()
	if err != nil {
		return nil
	}
	var inst models.ChallengeInstance
	if json.Unmarshal([]byte(data), &inst) != nil {
		return nil
	}
	if inst.Status != "running" && inst.Status != "pending" {
		return nil
	}
	return &inst
}

func (r *ProblemRepository) sharedCacheSet(inst *models.ChallengeInstance, ttlSeconds int) {
	if r.redis == nil || inst == nil {
		return
	}
	key := r.sharedCacheKey(inst.ProblemID)
	data, _ := json.Marshal(inst)
	r.redis.Set(context.Background(), key, string(data), time.Duration(ttlSeconds)*time.Second)
}

func (r *ProblemRepository) sharedCacheDel(problemID string) {
	if r.redis == nil {
		return
	}
	r.redis.Del(context.Background(), r.sharedCacheKey(problemID))
}

func (r *ProblemRepository) getProblemDeployMode(problemID string) (string, error) {
	var deployMode string
	err := r.db.QueryRow(`SELECT deploy_mode FROM problems WHERE id = ?`, strings.TrimSpace(problemID)).Scan(&deployMode)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", apperr.New(apperr.ErrNotFound, "题目不存在")
		}
		return "", fmt.Errorf("查询 deploy_mode 失败: %w", err)
	}
	return strings.ToLower(strings.TrimSpace(deployMode)), nil
}

func (r *ProblemRepository) getSharedRunningInstance(problemID string) (*models.ChallengeInstance, error) {
	sharedPodName := fmt.Sprintf("chal-%s", shortToken(problemID))
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
		WHERE problem_id = ? AND status IN ('running', 'pending')
		  AND (namespace = 'ctf-shared' OR pod_name = ?)
		ORDER BY started_at DESC
		LIMIT 1
	`, strings.TrimSpace(problemID), sharedPodName).Scan(
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
		return nil, fmt.Errorf("查询共享实例失败: %w", err)
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
	r.normalizeInstanceTTL(instance)
	return instance, nil
}

func (r *ProblemRepository) finalizeChallengeInstance(instance *models.ChallengeInstance) *models.ChallengeInstance {
	if instance == nil {
		return nil
	}
	r.normalizeInstanceTTL(instance)
	enrichChallengeInstance(instance)
	return instance
}

func (r *ProblemRepository) GetChallengeInstanceForUser(problemID, userID string) (*models.ChallengeInstance, error) {
	instance, err := r.GetChallengeInstance(problemID, userID)
	if err != nil {
		return nil, err
	}
	if instance != nil {
		return r.finalizeChallengeInstance(instance), nil
	}
	deployMode, modeErr := r.getProblemDeployMode(problemID)
	if modeErr != nil || deployMode != "shared" {
		return nil, modeErr
	}
	unlock, locked := r.lockSharedInstance(problemID)
	if !locked {
		return nil, apperr.New(apperr.ErrConflict, "共享实例操作进行中，请稍后重试")
	}
	defer unlock()

	globalTTL := resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)
	if cached := r.sharedCacheGet(problemID); cached != nil {
		cached.UserID = strings.TrimSpace(userID)
		_ = r.touchSharedLease(strings.TrimSpace(userID), strings.TrimSpace(problemID), cached)
		if lease, leaseErr := r.GetChallengeInstance(problemID, userID); leaseErr == nil && lease != nil {
			r.sharedCacheSet(lease, globalTTL)
			return r.finalizeChallengeInstance(lease), nil
		}
		return r.finalizeChallengeInstance(cached), nil
	}
	instance, err = r.getSharedRunningInstance(problemID)
	if err != nil || instance == nil {
		return instance, err
	}
	instance.UserID = strings.TrimSpace(userID)
	_ = r.touchSharedLease(strings.TrimSpace(userID), strings.TrimSpace(problemID), instance)
	if lease, leaseErr := r.GetChallengeInstance(problemID, userID); leaseErr == nil && lease != nil {
		r.sharedCacheSet(lease, globalTTL)
		return r.finalizeChallengeInstance(lease), nil
	}
	return r.finalizeChallengeInstance(instance), nil
}

func (r *ProblemRepository) touchSharedLease(userID, problemID string, instance *models.ChallengeInstance) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(problemID) == "" || instance == nil {
		return nil
	}
	ttl := resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)
	namespace := ""
	podName := ""
	accessURL := ""
	var nodePort any
	if instance.Namespace != nil {
		namespace = strings.TrimSpace(*instance.Namespace)
	}
	if instance.PodName != nil {
		podName = strings.TrimSpace(*instance.PodName)
	}
	if instance.AccessURL != nil {
		accessURL = strings.TrimSpace(*instance.AccessURL)
	}
	if instance.NodePort != nil && *instance.NodePort > 0 {
		nodePort = *instance.NodePort
	}
	_, err := r.db.Exec(`
		INSERT INTO challenge_instances (
			id, user_id, problem_id, pod_name, namespace, flag_value,
			status, started_at, expired_at, destroyed_at, access_url, node_port
		) VALUES (?, ?, ?, ?, ?, NULL, 'running', NOW(), DATE_ADD(NOW(), INTERVAL ? SECOND), NULL, NULLIF(?, ''), ?)
		ON DUPLICATE KEY UPDATE
			pod_name = VALUES(pod_name),
			namespace = VALUES(namespace),
			status = 'running',
			expired_at = DATE_ADD(NOW(), INTERVAL ? SECOND),
			destroyed_at = NULL,
			access_url = VALUES(access_url),
			node_port = VALUES(node_port)
	`, uuid.New().String(), strings.TrimSpace(userID), strings.TrimSpace(problemID), nullIfEmpty(podName), nullIfEmpty(namespace), ttl, accessURL, nodePort, ttl)
	if err != nil {
		return fmt.Errorf("更新共享实例租约失败: %w", err)
	}
	return nil
}

const (
	maxRetries       = 3
	backoffBase      = 500 * time.Millisecond
	sharedLockTTL    = 30 * time.Second
	sharedLockKeyFmt = "ctf:lock:shared:instance:%s"
)

func (r *ProblemRepository) lockSharedInstance(problemID string) (func(), bool) {
	if r.redis == nil {
		return func() {}, true
	}
	key := fmt.Sprintf(sharedLockKeyFmt, strings.TrimSpace(problemID))
	ok, err := r.redis.SetNX(context.Background(), key, "1", sharedLockTTL).Result()
	if err != nil || !ok {
		return func() {}, false
	}
	unlock := func() { r.redis.Del(context.Background(), key) }
	return unlock, true
}

func (r *ProblemRepository) StartChallengeInstance(ctx context.Context, problemID, userID string) (*models.ChallengeInstance, error) {
	cleanProblemID := strings.TrimSpace(problemID)
	cleanUserID := strings.TrimSpace(userID)
	if cleanProblemID == "" || cleanUserID == "" {
		return nil, apperr.New(apperr.ErrBadRequest, "problem_id/user_id 不能为空")
	}

	spec, err := r.getProblemContainerRuntimeSpec(cleanProblemID)
	if err != nil {
		return nil, err
	}

	isShared := spec.DeployMode == "shared"
	flagValue, flagEnvVars, _ := r.resolveAndInjectFlag(cleanProblemID, cleanUserID)
	globalTTL := resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)

	if isShared {
		unlock, locked := r.lockSharedInstance(cleanProblemID)
		if !locked {
			return nil, apperr.New(apperr.ErrConflict, "共享实例操作进行中，请稍后重试")
		}
		defer unlock()

		if cached := r.sharedCacheGet(cleanProblemID); cached != nil {
			cached.UserID = cleanUserID
			_ = r.touchSharedLease(cleanUserID, cleanProblemID, cached)
			if lease, leaseErr := r.GetChallengeInstance(cleanProblemID, cleanUserID); leaseErr == nil && lease != nil {
				r.sharedCacheSet(lease, globalTTL)
				return r.finalizeChallengeInstance(lease), nil
			}
			return r.finalizeChallengeInstance(cached), nil
		}
		existingShared, sharedErr := r.getSharedRunningInstance(cleanProblemID)
		if sharedErr != nil {
			return nil, sharedErr
		}
		if existingShared != nil {
			existingShared.UserID = cleanUserID
			_ = r.touchSharedLease(cleanUserID, cleanProblemID, existingShared)
			if lease, leaseErr := r.GetChallengeInstance(cleanProblemID, cleanUserID); leaseErr == nil && lease != nil {
				r.sharedCacheSet(lease, globalTTL)
				return r.finalizeChallengeInstance(lease), nil
			}
			return r.finalizeChallengeInstance(existingShared), nil
		}
	} else {
		existing, err := r.GetChallengeInstance(cleanProblemID, cleanUserID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			switch strings.ToLower(strings.TrimSpace(existing.Status)) {
			case "running":
				return r.finalizeChallengeInstance(existing), nil
			case "pending":
				return r.finalizeChallengeInstance(existing), nil
			case "stopping":
				return nil, apperr.New(apperr.ErrConflict, "实例正在销毁中，请稍后重试")
			}
		}
	}

	if !isShared {
		runningCount, _ := r.countRunningInstances(cleanUserID)
		if runningCount >= r.cfg.Container.MaxConcurrentContainers {
			return nil, apperr.New(apperr.ErrTooManyRequests,
				fmt.Sprintf("已达到最大并发容器数 (%d)", r.cfg.Container.MaxConcurrentContainers))
		}
	}

	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		return nil, fmt.Errorf("加载配置失败: %w", cfgErr)
	}

	instanceID := uuid.New().String()

	namespace := buildChallengeNamespace(cfg.Container.NamespacePrefix, cleanUserID)
	podName := buildChallengePodName(cleanProblemID, cleanUserID)
	if isShared {
		namespace = "ctf-shared"
		podName = fmt.Sprintf("chal-%s", shortToken(cleanProblemID))
	}
	serviceName := podName + "-svc"
	ingressName := podName + "-ing"
	hostUserID := cleanUserID
	if isShared {
		hostUserID = ""
	}
	host := renderIngressHost(spec.IngressHostTpl, cfg.Ingress.DomainTemplate, instanceID, cleanProblemID, hostUserID, podName)

	ttlSeconds := resolveGlobalContainerTTL(cfg.Container.DefaultContainerTTL)

	if err := r.upsertChallengeInstancePending(instanceID, cleanUserID, cleanProblemID, podName, namespace, flagValue, ttlSeconds); err != nil {
		return nil, err
	}

	if r.k8s == nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("K8s client 未初始化")
	}

	if err := r.k8s.EnsureNamespace(ctx, namespace); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("创建命名空间失败: %w", err)
	}

	pod, podErr := buildChallengePod(namespace, podName, spec, cfg, flagEnvVars)
	if podErr != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("构建 Pod 定义失败: %w", podErr)
	}
	if _, err := r.k8s.CreatePod(ctx, namespace, pod); err != nil {
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("创建 Pod 失败: %w", err)
	}

	svc := buildChallengeService(namespace, serviceName, podName, spec)
	if _, err := r.k8s.CreateService(ctx, namespace, svc); err != nil {
		_ = r.k8s.DeletePod(ctx, namespace, podName)
		_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
		return nil, fmt.Errorf("创建 Service 失败: %w", err)
	}

	accessURL := ""
	var nodePort *int
	if spec.ProblemType == "container_web" {
		ing := buildChallengeIngress(namespace, ingressName, serviceName, host, cfg.Ingress.Class, cfg.Ingress.TLSEnabled, cfg.Ingress.TLSSecret)
		if _, err := r.k8s.CreateIngress(ctx, namespace, ing); err != nil {
			_ = r.k8s.DeleteService(ctx, namespace, serviceName)
			_ = r.k8s.DeletePod(ctx, namespace, podName)
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
		np, npErr := r.k8s.GetServiceNodePort(ctx, namespace, serviceName)
		if npErr != nil {
			_ = r.k8s.DeleteService(ctx, namespace, serviceName)
			_ = r.k8s.DeletePod(ctx, namespace, podName)
			_ = r.markChallengeInstanceFailed(cleanUserID, cleanProblemID)
			return nil, fmt.Errorf("读取 Service NodePort 失败: %w", npErr)
		}
		nodePort = &np
		nodeHost := r.resolveNodeHost()
		accessURL = fmt.Sprintf("%s:%d", nodeHost, np)
	}

	if err := r.markChallengeInstanceRunning(cleanUserID, cleanProblemID, accessURL, nodePort); err != nil {
		return nil, err
	}

	inst, _ := r.GetChallengeInstanceForUser(cleanProblemID, cleanUserID)
	if isShared && inst != nil {
		r.cacheSharedInstanceOnCreate(cleanProblemID, cleanUserID, ttlSeconds)
	}
	return inst, nil
}

func (r *ProblemRepository) cacheSharedInstanceOnCreate(problemID, userID string, ttlSeconds int) {
	inst, _ := r.GetChallengeInstance(problemID, userID)
	if inst != nil {
		r.sharedCacheSet(inst, ttlSeconds)
	}
}

func (r *ProblemRepository) DestroyChallengeInstance(problemID, userID string) error {
	cleanProblemID := strings.TrimSpace(problemID)
	cleanUserID := strings.TrimSpace(userID)
	if cleanProblemID == "" || cleanUserID == "" {
		return apperr.New(apperr.ErrBadRequest, "problem_id/user_id 不能为空")
	}
	deployMode, modeErr := r.getProblemDeployMode(cleanProblemID)
	if modeErr != nil {
		return modeErr
	}

	instance, err := r.GetChallengeInstance(cleanProblemID, cleanUserID)
	if err != nil {
		return err
	}
	if instance == nil {
		if deployMode == "shared" {
			return nil
		}
		return apperr.New(apperr.ErrNotFound, "实例不存在")
	}
	if strings.EqualFold(strings.TrimSpace(instance.Status), "destroyed") {
		return nil
	}
	if deployMode == "shared" {
		// shared 模式仅释放当前用户占用映射，不销毁共享底层资源。
		if err := r.markChallengeInstanceDestroyed(cleanUserID, cleanProblemID); err != nil {
			return err
		}
		activeCount, countErr := r.countActiveSharedLeases(cleanProblemID)
		if countErr != nil {
			return countErr
		}
		if activeCount == 0 {
			ns, pod := r.resolveInstanceNames(instance, cleanProblemID, cleanUserID)
			if err := r.destroySharedK8sResources(ns, pod, cleanProblemID); err != nil {
				return err
			}
		}
		return nil
	}

	ns, pod := r.resolveInstanceNames(instance, cleanProblemID, cleanUserID)
	svcName := pod + "-svc"
	ingName := pod + "-ing"

	if _, err := r.db.Exec(`UPDATE challenge_instances SET status = 'stopping' WHERE user_id = ? AND problem_id = ?`, cleanUserID, cleanProblemID); err != nil {
		return fmt.Errorf("更新实例状态失败: %w", err)
	}

	r.sharedCacheDel(cleanProblemID)
	return r.destroyK8sResources(ns, pod, svcName, ingName, cleanUserID, cleanProblemID)
}

func (r *ProblemRepository) resolveInstanceNames(instance *models.ChallengeInstance, problemID, userID string) (namespace, podName string) {
	cfg, _ := config.LoadConfig()
	if cfg != nil {
		namespace = buildChallengeNamespace(cfg.Container.NamespacePrefix, userID)
		podName = buildChallengePodName(problemID, userID)
	}
	if instance.Namespace != nil && strings.TrimSpace(*instance.Namespace) != "" {
		namespace = strings.TrimSpace(*instance.Namespace)
	}
	if instance.PodName != nil && strings.TrimSpace(*instance.PodName) != "" {
		podName = strings.TrimSpace(*instance.PodName)
	}
	return
}

func (r *ProblemRepository) destroyK8sResources(namespace, podName, serviceName, ingressName, userID, problemID string) error {
	ctx := context.Background()
	var failed []string
	if err := retryDelete(func() error { return r.k8s.DeleteIngress(ctx, namespace, ingressName) }); err != nil {
		failed = append(failed, "ingress:"+ingressName)
	}
	if err := retryDelete(func() error { return r.k8s.DeleteService(ctx, namespace, serviceName) }); err != nil {
		failed = append(failed, "service:"+serviceName)
	}
	if err := retryDelete(func() error { return r.k8s.DeletePod(ctx, namespace, podName) }); err != nil {
		failed = append(failed, "pod:"+podName)
	}
	if len(failed) > 0 {
		logger.Warn("K8s 资源删除失败(已重试3次)", "ns", namespace, "failed", strings.Join(failed, ","))
	}
	_ = r.cleanupNamespaceIfEmpty(ctx, namespace)

	_, err := r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'destroyed', access_url = NULL, node_port = NULL, destroyed_at = NOW(), expired_at = NULL
		WHERE user_id = ? AND problem_id = ?
	`, strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("更新实例销毁状态失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) destroySharedK8sResources(namespace, podName, problemID string) error {
	r.sharedCacheDel(problemID)

	ctx := context.Background()
	serviceName := podName + "-svc"
	ingressName := podName + "-ing"

	var failed []string
	if err := retryDelete(func() error { return r.k8s.DeleteIngress(ctx, namespace, ingressName) }); err != nil {
		failed = append(failed, "ingress:"+ingressName)
	}
	if err := retryDelete(func() error { return r.k8s.DeleteService(ctx, namespace, serviceName) }); err != nil {
		failed = append(failed, "service:"+serviceName)
	}
	if err := retryDelete(func() error { return r.k8s.DeletePod(ctx, namespace, podName) }); err != nil {
		failed = append(failed, "pod:"+podName)
	}
	if len(failed) > 0 {
		logger.Warn("共享 K8s 资源删除失败(已重试3次)", "ns", namespace, "failed", strings.Join(failed, ","))
	}
	_ = r.cleanupNamespaceIfEmpty(ctx, namespace)

	if _, err := r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'destroyed', access_url = NULL, node_port = NULL, destroyed_at = NOW(), expired_at = NULL
		WHERE problem_id = ? AND status IN ('running', 'pending', 'stopping')
	`, strings.TrimSpace(problemID)); err != nil {
		return fmt.Errorf("更新共享实例销毁状态失败: %w", err)
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
		SELECT p.type, p.deploy_mode, p.use_kata,
		       IFNULL(pc.image_name, ''), IFNULL(pc.image_tag, ''), IFNULL(pc.challenge_port, 0),
		       IFNULL(pc.cpu_limit, ''), IFNULL(pc.memory_limit, ''), pc.env,
		       IFNULL(pc.ingress_host_tpl, '')
		FROM problems p
		LEFT JOIN problem_containers pc ON pc.problem_id = p.id
		WHERE p.id = ? AND p.status = 'ready'
	`, strings.TrimSpace(problemID)).Scan(
		&spec.ProblemType,
		&spec.DeployMode,
		&spec.UseKata,
		&spec.ImageName,
		&spec.ImageTag,
		&spec.ChallengePort,
		&spec.CPULimit,
		&spec.MemoryLimit,
		&spec.Env,
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

func (r *ProblemRepository) ExtendChallengeInstance(ctx context.Context, problemID, userID string) (*models.ChallengeInstance, error) {
	instance, err := r.GetChallengeInstance(problemID, userID)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.Status != "running" {
		return nil, apperr.New(apperr.ErrBadRequest, "没有运行中的实例")
	}
	if instance.ExpiredAt == nil {
		return nil, apperr.New(apperr.ErrBadRequest, "实例无过期时间")
	}
	expiredAt, parseErr := parseMySQLDateTime(*instance.ExpiredAt)
	if parseErr != nil {
		return nil, fmt.Errorf("解析过期时间失败: %w", parseErr)
	}
	remaining := time.Until(expiredAt)
	if remaining > 10*time.Minute {
		return nil, apperr.New(apperr.ErrBadRequest,
			fmt.Sprintf("仅剩 10m0s 可延期，当前剩余 %s", remaining.Round(time.Second)))
	}
	ttl := resolveGlobalContainerTTL(r.cfg.Container.DefaultContainerTTL)
	newExpiredAt := expiredAt.Add(time.Duration(ttl/2) * time.Second)
	_, dbErr := r.db.Exec(`
		UPDATE challenge_instances SET expired_at = ?
		WHERE user_id = ? AND problem_id = ? AND status = 'running'
	`, newExpiredAt.Format("2006-01-02 15:04:05"), strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if dbErr != nil {
		return nil, fmt.Errorf("延期失败: %w", dbErr)
	}
	inst, err := r.GetChallengeInstance(problemID, userID)
	if err != nil {
		return nil, err
	}
	return r.finalizeChallengeInstance(inst), nil
}

func (r *ProblemRepository) resolveAndInjectFlag(problemID, userID string) (string, []corev1.EnvVar, error) {
	flag, fErr := r.getProblemFlag(problemID)
	if fErr != nil || flag == nil {
		return "", nil, nil
	}
	var flagValue string
	if flag.FlagType == "dynamic" {
		flagValue = utils.GenerateDynamicFlag(r.cfg.Flag.Template)
	}
	var envVars []corev1.EnvVar
	if flag.FlagInject == "env" && flagValue != "" {
		envKey := flag.FlagEnvKey
		if envKey == "" {
			envKey = "EXEC_FLAG"
		}
		envVars = []corev1.EnvVar{{Name: envKey, Value: flagValue}}
	}
	return flagValue, envVars, nil
}

func (r *ProblemRepository) upsertChallengeInstancePending(instanceID, userID, problemID, podName, namespace, flagValue string, ttlSeconds int) error {
	_, err := r.db.Exec(`
		INSERT INTO challenge_instances (
			id, user_id, problem_id, pod_name, namespace, flag_value,
			status, started_at, expired_at, destroyed_at, access_url, node_port
		) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), 'pending', NOW(), DATE_ADD(NOW(), INTERVAL ? SECOND), NULL, NULL, NULL)
		ON DUPLICATE KEY UPDATE
			id = VALUES(id), pod_name = VALUES(pod_name), namespace = VALUES(namespace),
			flag_value = COALESCE(NULLIF(VALUES(flag_value), ''), flag_value),
			status = 'pending', started_at = NOW(),
			expired_at = DATE_ADD(NOW(), INTERVAL ? SECOND),
			destroyed_at = NULL, access_url = NULL, node_port = NULL
	`, strings.TrimSpace(instanceID), strings.TrimSpace(userID), strings.TrimSpace(problemID), strings.TrimSpace(podName), strings.TrimSpace(namespace), flagValue, ttlSeconds, ttlSeconds)
	if err != nil {
		return fmt.Errorf("写入实例状态失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) markChallengeInstanceFailed(userID, problemID string) error {
	_, err := r.db.Exec(`UPDATE challenge_instances SET status = 'failed' WHERE user_id = ? AND problem_id = ?`, strings.TrimSpace(userID), strings.TrimSpace(problemID))
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
	_, err := r.db.Exec(`UPDATE challenge_instances SET status = 'running', access_url = ?, node_port = ? WHERE user_id = ? AND problem_id = ?`,
		strings.TrimSpace(accessURL), nodePortValue, strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("更新实例运行状态失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) activeNodePorts() map[int]bool {
	rows, err := r.db.Query(`SELECT node_port FROM challenge_instances WHERE status IN ('running','pending') AND node_port IS NOT NULL`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	used := make(map[int]bool)
	for rows.Next() {
		var p int
		if rows.Scan(&p) == nil && p > 0 {
			used[p] = true
		}
	}
	return used
}

func (r *ProblemRepository) StartTTLCleanupScheduler(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			r.cleanupExpiredInstances()
		}
	}()
}

type expiredInstance struct {
	UserID, ProblemID  string
	DeployMode         string
	Namespace, PodName sql.NullString
}

func (r *ProblemRepository) cleanupExpiredInstances() {
	rows, err := r.db.Query(`
		SELECT ci.user_id, ci.problem_id, p.deploy_mode, ci.namespace, ci.pod_name
		FROM challenge_instances ci
		JOIN problems p ON p.id = ci.problem_id
		WHERE ci.status IN ('running', 'pending') AND ci.expired_at IS NOT NULL AND ci.expired_at < NOW()
	`)
	if err != nil {
		log.Printf("[ttl-cleanup] 查询过期实例失败: %v", err)
		return
	}
	defer rows.Close()

	var cleaned int
	for rows.Next() {
		var inst expiredInstance
		if err := rows.Scan(&inst.UserID, &inst.ProblemID, &inst.DeployMode, &inst.Namespace, &inst.PodName); err != nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(inst.DeployMode), "shared") {
			unlock, locked := r.lockSharedInstance(inst.ProblemID)
			if !locked {
				continue
			}
			if err := r.markChallengeInstanceDestroyed(inst.UserID, inst.ProblemID); err != nil {
				unlock()
				log.Printf("[ttl-cleanup] 标记共享实例过期失败: %s/%s err=%v", inst.UserID, inst.ProblemID, err)
				continue
			}
			activeCount := 0
			if countErr := r.db.QueryRow(`
				SELECT COUNT(*)
				FROM challenge_instances
				WHERE problem_id = ?
				  AND status IN ('running', 'pending')
				  AND (expired_at IS NULL OR expired_at >= NOW())
			`, strings.TrimSpace(inst.ProblemID)).Scan(&activeCount); countErr != nil {
				unlock()
				log.Printf("[ttl-cleanup] 查询共享实例活跃数失败: %s err=%v", inst.ProblemID, countErr)
				continue
			}
			if activeCount > 0 {
				unlock()
				cleaned++
				continue
			}
			ns, pod := r.resolveExpiredNames(&inst)
			if err := r.destroySharedK8sResources(ns, pod, inst.ProblemID); err != nil {
				unlock()
				log.Printf("[ttl-cleanup] 销毁共享实例资源失败: %s err=%v", inst.ProblemID, err)
				continue
			}
			unlock()
			cleaned++
			continue
		}
		ns, pod := r.resolveExpiredNames(&inst)
		svcName := pod + "-svc"
		ingName := pod + "-ing"
		if err := r.destroyK8sResources(ns, pod, svcName, ingName, inst.UserID, inst.ProblemID); err != nil {
			log.Printf("[ttl-cleanup] 销毁过期实例 %s/%s 失败: %v", inst.UserID, inst.ProblemID, err)
			continue
		}
		cleaned++
	}
	if cleaned > 0 {
		log.Printf("[ttl-cleanup] 清理了 %d 个过期实例 (含 K8s 资源)", cleaned)
	}
}

func (r *ProblemRepository) markChallengeInstanceDestroyed(userID, problemID string) error {
	_, err := r.db.Exec(`
		UPDATE challenge_instances
		SET status = 'destroyed', access_url = NULL, node_port = NULL, destroyed_at = NOW(), expired_at = NULL
		WHERE user_id = ? AND problem_id = ?
	`, strings.TrimSpace(userID), strings.TrimSpace(problemID))
	if err != nil {
		return fmt.Errorf("更新实例销毁状态失败: %w", err)
	}
	return nil
}

func (r *ProblemRepository) resolveExpiredNames(inst *expiredInstance) (namespace, podName string) {
	cfg, _ := config.LoadConfig()
	if cfg != nil {
		namespace = buildChallengeNamespace(cfg.Container.NamespacePrefix, inst.UserID)
		podName = buildChallengePodName(inst.ProblemID, inst.UserID)
	}
	if inst.Namespace.Valid && strings.TrimSpace(inst.Namespace.String) != "" {
		namespace = strings.TrimSpace(inst.Namespace.String)
	}
	if inst.PodName.Valid && strings.TrimSpace(inst.PodName.String) != "" {
		podName = strings.TrimSpace(inst.PodName.String)
	}
	return
}

func (r *ProblemRepository) countRunningInstances(userID string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM challenge_instances WHERE user_id = ? AND status IN ('running', 'pending')`, strings.TrimSpace(userID)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ProblemRepository) countActiveSharedLeases(problemID string) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM challenge_instances
		WHERE problem_id = ?
		  AND status IN ('running', 'pending')
		  AND (expired_at IS NULL OR expired_at >= NOW())
	`, strings.TrimSpace(problemID)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("查询共享实例活跃租约失败: %w", err)
	}
	return count, nil
}

func retryBackoff(attempt int) time.Duration {
	d := backoffBase
	for i := 1; i < attempt; i++ {
		d *= 2
	}
	if d > 10*time.Second {
		d = 10 * time.Second
	}
	return d
}

func retryDelete(op func() error) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(retryBackoff(i))
		}
		lastErr = op()
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (r *ProblemRepository) cleanupNamespaceIfEmpty(ctx context.Context, namespace string) error {
	if r.k8s == nil {
		return nil
	}
	// 跳过 shared 命名空间和系统命名空间
	if namespace == "ctf-shared" {
		return nil
	}
	_ = r.k8s.DeleteNamespaceIfEmpty(ctx, namespace)
	return nil
}
