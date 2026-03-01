package models

import "time"

// Submission 提交记录模型
type Submission struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ProblemID    int       `json:"problem_id"`
	Flag         string    `json:"flag"`
	IsCorrect    bool      `json:"is_correct"`
	ProblemTitle string    `json:"problem_title"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateSubmissionRequest 创建提交请求
type CreateSubmissionRequest struct {
	ProblemID int    `json:"problem_id"`
	Flag      string `json:"flag"`
	IsCorrect bool   `json:"is_correct"` // 暂时由前端传递验证结果
}
