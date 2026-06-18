package models

// TaskInstance 容器实例创建任务 DB 行（task_instances 表）。
// 生命周期：pending → queued → provisioning → scheduling → pulling → starting → ready
// 失败/超时可在任意阶段触发。
type TaskInstance struct {
	ID           string  `json:"id"`
	UserID       string  `json:"user_id"`
	ProblemID    string  `json:"problem_id"`
	InstanceID   *string `json:"instance_id,omitempty"`
	PodName      *string `json:"pod_name,omitempty"`
	Namespace    *string `json:"namespace,omitempty"`
	AccessURL    *string `json:"access_url,omitempty"`
	NodePort     *int    `json:"node_port,omitempty"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message,omitempty"`
	K8sReason    *string `json:"k8s_reason,omitempty"`
	RetryCount   int     `json:"retry_count"`
	MaxRetry     int     `json:"max_retry"`
	ClaimedBy    *string `json:"claimed_by,omitempty"`
	ClaimedAt    *string `json:"claimed_at,omitempty"`
	StartedAt    *string `json:"started_at,omitempty"`
	FinishedAt   *string `json:"finished_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// TaskEventLog 任务生命周期事件 DB 行（task_event_logs 表）。
type TaskEventLog struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Stage     string `json:"stage"` // VALIDATING|CREATING_NS|CREATING_POD|CREATING_SVC|CREATING_INGRESS|WATCHING|DONE
	Level     string `json:"level"` // info|warn|error
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// --- Response DTOs ---

// TaskAcceptedResponse POST /api/problems/:id/instance/start 的初始响应。
// 任务已接受，前端应轮询 GET /api/tasks/:id/status 获取实际进度。
type TaskAcceptedResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"` // 初始为 "pending"
	Message string `json:"message"`
}

// TaskStatusResponse GET /api/tasks/:id/status 的响应。
// 前端轮询此接口直到 status 为 ready / failed / timeout。
type TaskStatusResponse struct {
	TaskID     string              `json:"task_id"`
	Status     string              `json:"status"`
	AccessURL  *string             `json:"access_url,omitempty"`
	ErrorMsg   *string             `json:"error_message,omitempty"`
	K8sReason  *string             `json:"k8s_reason,omitempty"`
	StartedAt  *string             `json:"started_at,omitempty"`
	FinishedAt *string             `json:"finished_at,omitempty"`
	Events     []TaskEventLogEntry `json:"events,omitempty"` // 最近 20 条事件
}

// TaskEventLogEntry 任务事件条目（API 响应用）。
type TaskEventLogEntry struct {
	Stage     string `json:"stage"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// --- 状态常量 ---

const (
	TaskStatusPending      = "pending"
	TaskStatusQueued       = "queued"
	TaskStatusProvisioning = "provisioning"
	TaskStatusScheduling   = "scheduling"
	TaskStatusPulling      = "pulling"
	TaskStatusStarting     = "starting"
	TaskStatusReady        = "ready"
	TaskStatusFailed       = "failed"
	TaskStatusTimeout      = "timeout"
)

// IsTaskTerminal 判断任务是否已进入终态。
func IsTaskTerminal(status string) bool {
	switch status {
	case TaskStatusReady, TaskStatusFailed, TaskStatusTimeout:
		return true
	default:
		return false
	}
}

// IsTaskFailed 判断任务是否失败。
func IsTaskFailed(status string) bool {
	return status == TaskStatusFailed || status == TaskStatusTimeout
}
