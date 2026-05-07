package models

import (
	"encoding/json"
	"time"
)

// Submission 提交记录模型，对齐 v2.1 Appendix A 的 submissions 表结构。
type Submission struct {
	ID           string          `json:"id"`
	UserID       *string         `json:"user_id,omitempty"`
	TeamID       *string         `json:"team_id,omitempty"`
	ContestID    *string         `json:"contest_id,omitempty"`
	ProblemID    string          `json:"problem_id"`
	InstanceID   *string         `json:"instance_id,omitempty"`
	Flag         string          `json:"flag,omitempty"`
	IsCorrect    bool            `json:"is_correct"`
	ScoreAwarded int             `json:"score_awarded"`
	IPAddress    string          `json:"ip_address,omitempty"`
	ProblemTitle string          `json:"problem_title,omitempty"`
	Meta         json.RawMessage `json:"meta,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// CreateSubmissionRequest 创建提交请求
type CreateSubmissionRequest struct {
	ProblemID string  `json:"problem_id"`
	Flag      string  `json:"flag"`
	TeamID    *string `json:"team_id,omitempty"`
	ContestID *string `json:"contest_id,omitempty"`
	IsCorrect bool    `json:"is_correct"` // 由服务层计算
	IPAddress string  `json:"-"`          // 由后端根据请求上下文注入
}
