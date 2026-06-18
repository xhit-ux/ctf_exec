package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ctf_exec/backend/internal/config"
	"github.com/ctf_exec/backend/internal/k8s"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/notify"
	"github.com/ctf_exec/backend/internal/rabbitmq"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	corev1 "k8s.io/api/core/v1"
)

// ProvisionerWorker 容器实例创建 Worker。
// 从 RabbitMQ 消费 → 创建 k8s 资源 → Watch Pod 直到 Ready → 更新任务状态。
type ProvisionerWorker struct {
	cfg         config.Config
	mq          *rabbitmq.Client
	k8sClient   *k8s.Client
	taskRepo    *TaskRepository
	problemRepo *ProblemRepository
	workerID    string
	concurrency int
	timeout     time.Duration
	wg          sync.WaitGroup
}

// NewProvisionerWorker 创建 ProvisionerWorker。
func NewProvisionerWorker(
	cfg config.Config,
	mq *rabbitmq.Client,
	k8sClient *k8s.Client,
	taskRepo *TaskRepository,
	problemRepo *ProblemRepository,
) *ProvisionerWorker {
	host, _ := os.Hostname()
	return &ProvisionerWorker{
		cfg:         cfg,
		mq:          mq,
		k8sClient:   k8sClient,
		taskRepo:    taskRepo,
		problemRepo: problemRepo,
		workerID:    fmt.Sprintf("%s:%d", host, os.Getpid()),
		concurrency: cfg.RabbitMQ.WorkerConcurrency,
		timeout:     time.Duration(cfg.RabbitMQ.TaskTimeoutSec) * time.Second,
	}
}

// Start 启动 Worker 消费循环（阻塞）。
func (pw *ProvisionerWorker) Start(ctx context.Context) error {
	log.Printf("[Provisioner:%s] 启动 concurrency=%d timeout=%v", pw.workerID, pw.concurrency, pw.timeout)

	deliveries, err := pw.mq.ConsumeTasks(ctx, pw.cfg.RabbitMQ.QueueChallenge, pw.concurrency)
	if err != nil {
		return fmt.Errorf("Provisioner 消费启动失败: %w", err)
	}

	for i := 0; i < pw.concurrency; i++ {
		pw.wg.Add(1)
		go pw.workerLoop(ctx, deliveries, i)
	}

	pw.wg.Wait()
	return nil
}

func (pw *ProvisionerWorker) workerLoop(ctx context.Context, deliveries <-chan amqp.Delivery, idx int) {
	defer pw.wg.Done()
	log.Printf("[Provisioner:%s:%d] 就绪", pw.workerID, idx)

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			pw.processDelivery(ctx, delivery, idx)
		}
	}
}

type taskMessage struct {
	TaskID    string `json:"task_id"`
	UserID    string `json:"user_id"`
	ProblemID string `json:"problem_id"`
}

func (pw *ProvisionerWorker) processDelivery(ctx context.Context, delivery amqp.Delivery, idx int) {
	var msg taskMessage
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		log.Printf("[Provisioner:%s:%d] 消息解析失败: %v", pw.workerID, idx, err)
		_ = delivery.Nack(false, false)
		return
	}

	log.Printf("[Provisioner:%s:%d] 处理任务 %s", pw.workerID, idx, msg.TaskID)

	pw.taskRepo.TransitionTask(msg.TaskID, models.TaskStatusProvisioning,
		"PROVISIONING_START", "info", "Worker 开始创建 k8s 资源", nil)
	pw.pushTaskWS(msg.TaskID, msg.UserID, models.TaskStatusProvisioning, "Worker 开始创建 k8s 资源", nil, nil, nil)

	accessURL, instID, err := pw.doProvision(ctx, msg.TaskID, msg.UserID, msg.ProblemID)

	if err != nil {
		log.Printf("[Provisioner:%s:%d] 失败 task=%s: %v", pw.workerID, idx, msg.TaskID, err)
		errStr := err.Error()
		pw.taskRepo.TransitionTask(msg.TaskID, models.TaskStatusFailed,
			"FAILED", "error", errStr,
			map[string]interface{}{"error_message": errStr})
		pw.pushTaskWS(msg.TaskID, msg.UserID, models.TaskStatusFailed, errStr, nil, &errStr, nil)
		_ = delivery.Ack(false)
		return
	}

	pw.taskRepo.TransitionTask(msg.TaskID, models.TaskStatusReady,
		"DONE", "info", "实例就绪: "+accessURL,
		map[string]interface{}{"access_url": accessURL, "instance_id": instID})
	pw.pushTaskWS(msg.TaskID, msg.UserID, models.TaskStatusReady, "实例就绪: "+accessURL, &accessURL, nil, nil)
	_ = delivery.Ack(false)
	log.Printf("[Provisioner:%s:%d] 完成 task=%s url=%s", pw.workerID, idx, msg.TaskID, accessURL)
}

// doProvision 执行完整的 k8s 资源创建 + Pod 就绪等待。
func (pw *ProvisionerWorker) doProvision(ctx context.Context, taskID, userID, problemID string) (string, string, error) {
	spec, err := pw.problemRepo.getProblemContainerRuntimeSpec(problemID)
	if err != nil {
		return "", "", fmt.Errorf("获取容器配置失败: %w", err)
	}

	namespace := buildChallengeNamespace(pw.cfg.Container.NamespacePrefix, userID)
	podName := buildChallengePodName(problemID, userID)
	serviceName := podName + "-svc"
	ingressName := podName + "-ing"
	instanceID := uuid.New().String()

	// 先写入 challenge_instances 记录（pending），刷新页面可看到状态
	flagStr, flagEnvVars, _ := pw.problemRepo.resolveAndInjectFlag(problemID, userID)
	ttl := resolveGlobalContainerTTL(pw.cfg.Container.DefaultContainerTTL)
	_ = pw.problemRepo.upsertChallengeInstancePending(instanceID, userID, problemID, podName, namespace, flagStr, ttl)

	// Create Namespace
	pw.taskRepo.WriteEventLog(taskID, "CREATING_NS", "info", "创建 Namespace: "+namespace)
	if err := pw.k8sClient.EnsureNamespace(ctx, namespace); err != nil {
		return "", "", fmt.Errorf("创建 Namespace 失败: %w", err)
	}

	// Create Pod
	pw.taskRepo.WriteEventLog(taskID, "CREATING_POD", "info", "创建 Pod: "+podName)
	pod, podErr := buildChallengePod(namespace, podName, spec, &pw.cfg, flagEnvVars)
	if podErr != nil {
		return "", "", fmt.Errorf("构建 Pod 定义失败: %w", podErr)
	}
	if _, err := pw.k8sClient.CreatePod(ctx, namespace, pod); err != nil {
		return "", "", fmt.Errorf("创建 Pod 失败: %w", err)
	}

	// Create Service
	pw.taskRepo.WriteEventLog(taskID, "CREATING_SVC", "info", "创建 Service: "+serviceName)
	svc := buildChallengeService(namespace, serviceName, podName, spec)
	if _, err := pw.k8sClient.CreateService(ctx, namespace, svc); err != nil {
		pw.k8sClient.DeletePod(ctx, namespace, podName)
		return "", "", fmt.Errorf("创建 Service 失败: %w", err)
	}

	// Create Ingress / NodePort
	accessURL := ""
	if spec.ProblemType == "container_web" {
		pw.taskRepo.WriteEventLog(taskID, "CREATING_INGRESS", "info", "创建 Ingress")
		host := renderIngressHost(spec.IngressHostTpl, pw.cfg.Ingress.DomainTemplate, instanceID, problemID, userID, podName)
		ing := buildChallengeIngress(namespace, ingressName, serviceName, host,
			pw.cfg.Ingress.Class, pw.cfg.Ingress.TLSEnabled, pw.cfg.Ingress.TLSSecret)
		if _, err := pw.k8sClient.CreateIngress(ctx, namespace, ing); err != nil {
			pw.k8sClient.DeleteService(ctx, namespace, serviceName)
			pw.k8sClient.DeletePod(ctx, namespace, podName)
			return "", "", fmt.Errorf("创建 Ingress 失败: %w", err)
		}
		scheme := "http"
		if pw.cfg.Ingress.TLSEnabled {
			scheme = "https"
		}
		if host != "" {
			accessURL = fmt.Sprintf("%s://%s", scheme, host)
		}
	} else {
		np, npErr := pw.k8sClient.GetServiceNodePort(ctx, namespace, serviceName)
		if npErr != nil {
			return "", "", fmt.Errorf("读取 NodePort 失败: %w", npErr)
		}
		accessURL = fmt.Sprintf("%s:%d", pw.problemRepo.resolveNodeHost(), np)
	}

	// Watch Pod until ready
	pw.taskRepo.WriteEventLog(taskID, "WATCHING", "info", "等待 Pod 就绪...")
	if err := pw.watchUntilReady(ctx, namespace, podName, taskID, userID); err != nil {
		// 失败时抓取 k8s Events 写入日志（在删除资源之前）
		k8sEvents, evErr := pw.k8sClient.GetPodEvents(ctx, namespace, podName)
		if evErr == nil && len(k8sEvents) > 0 {
			eventSummary := strings.Join(k8sEvents, "; ")
			pw.taskRepo.WriteEventLog(taskID, "FAILED", "error", "K8s Events: "+eventSummary)
			err = fmt.Errorf("%w\nK8s Events: %s", err, eventSummary)
		}
		pw.k8sClient.DeleteIngress(ctx, namespace, ingressName)
		pw.k8sClient.DeleteService(ctx, namespace, serviceName)
		pw.k8sClient.DeletePod(ctx, namespace, podName)
		return "", "", fmt.Errorf("Pod 就绪等待失败: %w", err)
	}

	// Write challenge_instances
	_ = pw.problemRepo.markChallengeInstanceRunning(userID, problemID, accessURL, nil)

	return accessURL, instanceID, nil
}

func (pw *ProvisionerWorker) watchUntilReady(ctx context.Context, namespace, podName, taskID, userID string) error {
	deadline := time.After(pw.timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastPhase, lastReason := "", ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("Pod 未在 %v 内就绪 (last: %s/%s)", pw.timeout, lastPhase, lastReason)
		case <-ticker.C:
			pod, err := pw.k8sClient.GetPod(ctx, namespace, podName)
			if err != nil {
				continue
			}

			phase, reason := k8s.PodPhase(pod)
			if phase != lastPhase || reason != lastReason {
				lastPhase, lastReason = phase, reason
				pw.transitionOnWatch(taskID, userID, phase, reason)
			}

			// 终端失败：Pod phase 直接失败
			if phase == "Failed" || phase == "Unknown" {
				msg := getPodFailureMessage(pod)
				return fmt.Errorf("k8s: Pod %s — %s", phase, msg)
			}

			// 终端失败：已知不可恢复的容器等待原因
			switch reason {
			case "CreateContainerConfigError", "ErrImagePull", "ImagePullBackOff",
				"CrashLoopBackOff", "InvalidImageName", "Error":
				msg := getPodFailureMessage(pod)
				return fmt.Errorf("k8s: %s — %s", reason, msg)
			}

			if podIsReady(pod) {
				return nil
			}
		}
	}
}

func (pw *ProvisionerWorker) transitionOnWatch(taskID, userID, phase, reason string) {
	status := mapPodPhaseToStatus(phase, reason)
	message := fmt.Sprintf("Pod: phase=%s reason=%s", phase, reason)
	pw.taskRepo.TransitionTask(taskID, status, "WATCHING", "info", message,
		map[string]interface{}{"k8s_reason": reason})

	// 推送 WebSocket
	errMsg := (*string)(nil)
	if status == models.TaskStatusFailed {
		errMsg = &reason
	}
	pw.pushTaskWS(taskID, userID, status, message, nil, errMsg, &reason)
}

// pushTaskWS 推送任务状态变更到 WebSocket。
func (pw *ProvisionerWorker) pushTaskWS(taskID, userID, status, message string, accessURL, errMsg, k8sReason *string) {
	notify.GetNotifier().Publish(notify.TaskStatusEvent{
		Type:         "task_status",
		TaskID:       taskID,
		UserID:       userID,
		Status:       status,
		AccessURL:    accessURL,
		ErrorMessage: errMsg,
		K8sReason:    k8sReason,
		Message:      message,
	})
}

func mapPodPhaseToStatus(phase, reason string) string {
	switch reason {
	case "ContainerCreating":
		return models.TaskStatusScheduling
	case "Pulling":
		return models.TaskStatusPulling
	case "ErrImagePull", "ImagePullBackOff", "CreateContainerConfigError",
		"CrashLoopBackOff", "InvalidImageName":
		return models.TaskStatusFailed
	}
	if phase == "Running" {
		return models.TaskStatusStarting
	}
	return models.TaskStatusProvisioning
}

func podIsReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// getPodFailureMessage 从 Pod 状态中提取失败原因，优先容器状态消息。
func getPodFailureMessage(pod *corev1.Pod) string {
	// 容器级别：Waiting / Terminated 消息
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			msg := cs.State.Waiting.Message
			if msg != "" {
				return reason + ": " + msg
			}
			return reason
		}
		if cs.State.Terminated != nil {
			reason := cs.State.Terminated.Reason
			exitCode := cs.State.Terminated.ExitCode
			msg := cs.State.Terminated.Message
			detail := reason
			if exitCode != 0 {
				detail = fmt.Sprintf("%s(exit=%d)", reason, exitCode)
			}
			if msg != "" {
				return detail + ": " + msg
			}
			return detail
		}
	}
	// Pod 级别：Reason / Message
	if pod.Status.Reason != "" {
		return string(pod.Status.Reason)
	}
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	return string(pod.Status.Phase)
}
