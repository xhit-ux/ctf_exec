package handler

import (
	"net/http"
	"strings"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// TaskHandler 任务状态查询 handler。
type TaskHandler struct {
	taskRepo *repository.TaskRepository
}

// NewTaskHandler 构造函数。
func NewTaskHandler(taskRepo *repository.TaskRepository) *TaskHandler {
	return &TaskHandler{taskRepo: taskRepo}
}

// GetTaskStatusGin 获取任务状态（前端轮询）。
// GET /api/tasks/:id/status
func (h *TaskHandler) GetTaskStatusGin(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务ID"})
		return
	}

	task, err := h.taskRepo.GetTaskByID(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	events, _ := h.taskRepo.GetTaskEvents(taskID)
	eventEntries := make([]models.TaskEventLogEntry, 0, len(events))
	for _, e := range events {
		eventEntries = append(eventEntries, models.TaskEventLogEntry{
			Stage:     e.Stage,
			Level:     e.Level,
			Message:   e.Message,
			CreatedAt: e.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, models.TaskStatusResponse{
		TaskID:     task.ID,
		Status:     task.Status,
		AccessURL:  task.AccessURL,
		ErrorMsg:   task.ErrorMessage,
		K8sReason:  task.K8sReason,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
		Events:     eventEntries,
	})
}
