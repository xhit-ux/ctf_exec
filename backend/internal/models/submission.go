package models

import "time"

// Submission 提交记录模型
type Submission struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	TeamID       *string   `json:"team_id,omitempty"`
	ContestID    *string   `json:"contest_id,omitempty"`
	ProblemID    int       `json:"problem_id"`
	Flag         string    `json:"flag"`
	IsCorrect    bool      `json:"is_correct"`
	ProblemTitle string    `json:"problem_title"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateSubmissionRequest 创建提交请求
type CreateSubmissionRequest struct {
	ProblemID int     `json:"problem_id"`
	Flag      string  `json:"flag"`
	TeamID    *string `json:"team_id,omitempty"`
	ContestID *string `json:"contest_id,omitempty"`
	IsCorrect bool    `json:"is_correct"` // 由服务层计算
}
