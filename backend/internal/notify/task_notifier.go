// Package notify 提供任务状态变更的全局发布-订阅枢纽。
// 独立包，不依赖 repository / middleware / ws，避免循环引用。
package notify

import "sync"

// TaskStatusEvent 任务状态变更事件。
type TaskStatusEvent struct {
	Type         string  `json:"type"`
	TaskID       string  `json:"task_id"`
	UserID       string  `json:"user_id"`
	Status       string  `json:"status"`
	AccessURL    *string `json:"access_url,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
	K8sReason    *string `json:"k8s_reason,omitempty"`
	Message      string  `json:"message"`
}

// TaskNotifier 全局单例。
type TaskNotifier struct {
	mu          sync.RWMutex
	subscribers map[string][]chan TaskStatusEvent
}

var globalNotifier = &TaskNotifier{
	subscribers: make(map[string][]chan TaskStatusEvent),
}

// GetNotifier 返回全局单例。
func GetNotifier() *TaskNotifier {
	return globalNotifier
}

// Subscribe 为用户注册事件 channel（缓冲 64）。
func (n *TaskNotifier) Subscribe(userID string) <-chan TaskStatusEvent {
	ch := make(chan TaskStatusEvent, 64)
	n.mu.Lock()
	n.subscribers[userID] = append(n.subscribers[userID], ch)
	n.mu.Unlock()
	return ch
}

// Unsubscribe 取消订阅并关闭 channel。
func (n *TaskNotifier) Unsubscribe(userID string, ch <-chan TaskStatusEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	chs := n.subscribers[userID]
	for i, c := range chs {
		if c == ch {
			n.subscribers[userID] = append(chs[:i], chs[i+1:]...)
			close(c)
			return
		}
	}
}

// Publish 向指定用户推送事件（非阻塞）。
func (n *TaskNotifier) Publish(event TaskStatusEvent) {
	n.mu.RLock()
	chs := n.subscribers[event.UserID]
	n.mu.RUnlock()
	for _, ch := range chs {
		select {
		case ch <- event:
		default:
		}
	}
}
