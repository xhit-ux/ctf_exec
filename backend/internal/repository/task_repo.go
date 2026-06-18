package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/google/uuid"
)

// TaskRepository 任务追踪数据访问层。
type TaskRepository struct {
	db *sql.DB
}

// NewTaskRepository 构造函数。
func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// CreateTask 创建任务记录（状态 = pending）。
func (r *TaskRepository) CreateTask(userID, problemID string) (*models.TaskInstance, error) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.Exec(`
		INSERT INTO task_instances (id, user_id, problem_id, status, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?)
	`, id, userID, problemID, now, now)
	if err != nil {
		return nil, fmt.Errorf("创建任务记录失败: %w", err)
	}
	return r.GetTaskByID(id)
}

// ClaimTask Worker 认领一个 pending 任务（并发安全 — SELECT FOR UPDATE SKIP LOCKED）。
// 成功返回任务，无待处理任务返回 nil, nil。
func (r *TaskRepository) ClaimTask(ctx context.Context, workerID string) (*models.TaskInstance, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Format("2006-01-02 15:04:05")

	row := tx.QueryRowContext(ctx, `
		SELECT id FROM task_instances
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询待处理任务失败: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE task_instances
		SET status = 'queued', claimed_by = ?, claimed_at = ?, started_at = ?, updated_at = ?
		WHERE id = ?
	`, workerID, now, now, now, id)
	if err != nil {
		return nil, fmt.Errorf("认领任务失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交认领事务失败: %w", err)
	}
	return r.GetTaskByID(id)
}

// GetTaskByID 按 ID 查询任务。
func (r *TaskRepository) GetTaskByID(id string) (*models.TaskInstance, error) {
	return r.scanTask(r.db.QueryRow(`
		SELECT id, user_id, problem_id,
		       instance_id, pod_name, namespace, access_url, node_port,
		       status, error_message, k8s_reason,
		       retry_count, max_retry,
		       claimed_by,
		       IFNULL(DATE_FORMAT(claimed_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(started_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(finished_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM task_instances WHERE id = ?
	`, id))
}

// GetTaskByUserAndProblem 查询用户对某题目的最近一个任务。
func (r *TaskRepository) GetTaskByUserAndProblem(userID, problemID string) (*models.TaskInstance, error) {
	return r.scanTask(r.db.QueryRow(`
		SELECT id, user_id, problem_id,
		       instance_id, pod_name, namespace, access_url, node_port,
		       status, error_message, k8s_reason,
		       retry_count, max_retry,
		       claimed_by,
		       IFNULL(DATE_FORMAT(claimed_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(started_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(finished_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), ''),
		       IFNULL(DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM task_instances
		WHERE user_id = ? AND problem_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, problemID))
}

func (r *TaskRepository) scanTask(row *sql.Row) (*models.TaskInstance, error) {
	t := &models.TaskInstance{}
	var instanceID, podName, namespace, accessURL sql.NullString
	var nodePort sql.NullInt64
	var errorMsg, k8sReason sql.NullString
	var claimedBy sql.NullString
	var claimedAt, startedAt, finishedAt sql.NullString

	err := row.Scan(
		&t.ID, &t.UserID, &t.ProblemID,
		&instanceID, &podName, &namespace, &accessURL, &nodePort,
		&t.Status, &errorMsg, &k8sReason,
		&t.RetryCount, &t.MaxRetry,
		&claimedBy,
		&claimedAt, &startedAt, &finishedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询任务失败: %w", err)
	}
	t.InstanceID = nullStr(instanceID)
	t.PodName = nullStr(podName)
	t.Namespace = nullStr(namespace)
	t.AccessURL = nullStr(accessURL)
	if nodePort.Valid {
		np := int(nodePort.Int64)
		t.NodePort = &np
	}
	t.ErrorMessage = nullStr(errorMsg)
	t.K8sReason = nullStr(k8sReason)
	t.ClaimedBy = nullStr(claimedBy)
	t.ClaimedAt = nullStr(claimedAt)
	t.StartedAt = nullStr(startedAt)
	t.FinishedAt = nullStr(finishedAt)
	return t, nil
}

// --- 状态转换 ---

// TransitionTask 原子更新任务状态 + 写入事件日志（单事务）。
// extraFields 支持: instance_id, pod_name, namespace, access_url, node_port, error_message, k8s_reason
func (r *TaskRepository) TransitionTask(taskID, newStatus, stage, level, message string, extraFields map[string]interface{}) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Format("2006-01-02 15:04:05")

	setClauses := []string{"status = ?", "updated_at = ?"}
	args := []interface{}{newStatus, now}

	allowed := map[string]bool{
		"instance_id": true, "pod_name": true, "namespace": true,
		"access_url": true, "node_port": true,
		"error_message": true, "k8s_reason": true,
	}
	for col, val := range extraFields {
		if allowed[col] {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
	}

	if models.IsTaskTerminal(newStatus) {
		setClauses = append(setClauses, "finished_at = ?")
		args = append(args, now)
	}

	query := "UPDATE task_instances SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	args = append(args, taskID)

	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	eventID := uuid.New().String()
	if _, err := tx.Exec(`
		INSERT INTO task_event_logs (id, task_id, stage, level, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, eventID, taskID, stage, level, message, now); err != nil {
		return fmt.Errorf("写入事件日志失败: %w", err)
	}

	return tx.Commit()
}

// WriteEventLog 单独写入事件日志（不改变任务 status）。
func (r *TaskRepository) WriteEventLog(taskID, stage, level, message string) error {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.Exec(`
		INSERT INTO task_event_logs (id, task_id, stage, level, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, taskID, stage, level, message, now)
	return err
}

// GetTaskEvents 获取任务事件日志（按时间正序，最多 50 条）。
func (r *TaskRepository) GetTaskEvents(taskID string) ([]models.TaskEventLog, error) {
	rows, err := r.db.Query(`
		SELECT id, task_id, stage, level, message,
		       IFNULL(DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), '')
		FROM task_event_logs
		WHERE task_id = ?
		ORDER BY created_at ASC
		LIMIT 50
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("查询事件日志失败: %w", err)
	}
	defer rows.Close()

	var events []models.TaskEventLog
	for rows.Next() {
		var e models.TaskEventLog
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Stage, &e.Level, &e.Message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描事件日志失败: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

// MarkStaleTasksTimeout 标记超时任务（定时调用）。
func (r *TaskRepository) MarkStaleTasksTimeout(timeoutSeconds int) (int64, error) {
	cutoff := time.Now().Add(-time.Duration(timeoutSeconds) * time.Second).Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := r.db.Exec(`
		UPDATE task_instances
		SET status = 'timeout',
		    error_message = CONCAT('任务超时，超过 ', ?, ' 秒未完成'),
		    finished_at = ?, updated_at = ?
		WHERE status IN ('pending','queued','provisioning','scheduling','pulling','starting')
		  AND created_at < ?
	`, timeoutSeconds, now, now, cutoff)
	if err != nil {
		return 0, fmt.Errorf("标记超时任务失败: %w", err)
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		log.Printf("[TaskRepo] 已标记 %d 个超时任务", n)
	}
	return n, nil
}

// --- 辅助 ---

func nullStr(ns sql.NullString) *string {
	if ns.Valid {
		v := ns.String
		return &v
	}
	return nil
}

func workerIDStr() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}
