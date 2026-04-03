package models

import "time"

// Problem 题目模型 (用于API返回，不包含Flag)
type Problem struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Type        string   `json:"type,omitempty"`
	Points      int      `json:"points"`
	Difficulty  string   `json:"difficulty"`
	Tags        []string `json:"tags"` // 标签数组
	Description string   `json:"description"`
	IsSolved    bool     `json:"is_solved"`
	Scope       string   `json:"scope,omitempty"`
	ContestID   *string  `json:"contest_id,omitempty"`
	Status      string   `json:"status,omitempty"`
	FlagRule    string   `json:"flag_rule,omitempty"`
	FlagExample string   `json:"flag_example,omitempty"`
	DockerImage string   `json:"docker_image,omitempty"`
	RunCommand  string   `json:"run_command,omitempty"`
	CreatedBy   string   `json:"created_by,omitempty"`
	VisibleFrom *string  `json:"visible_from,omitempty"`
	VisibleTo   *string  `json:"visible_to,omitempty"`
}

// ProblemDetail 题目详情 (内部包括Flag，用于验证)
type ProblemDetail struct {
	Problem
	FlagHash string `json:"-"` // 不序列化
}

// CreateProblemRequest 创建题目请求
type CreateProblemRequest struct {
	Title       string     `json:"title"`
	Category    string     `json:"category"`
	Type        string     `json:"type"`
	Points      int        `json:"points"`
	Difficulty  string     `json:"difficulty"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Flag        string     `json:"flag"`
	FlagRule    string     `json:"flag_rule"`
	FlagExample string     `json:"flag_example"`
	DockerImage string     `json:"docker_image"`
	RunCommand  string     `json:"run_command"`
	Scope       string     `json:"scope"`
	ContestID   string     `json:"contest_id"`
	Status      string     `json:"status"`
	VisibleFrom *time.Time `json:"visible_from"`
	VisibleTo   *time.Time `json:"visible_to"`
	CreatedBy   string     `json:"-"`
}

// UpdateProblemRequest 更新题目请求
type UpdateProblemRequest struct {
	Title       string     `json:"title"`
	Category    string     `json:"category"`
	Type        string     `json:"type"`
	Points      int        `json:"points"`
	Difficulty  string     `json:"difficulty"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Flag        string     `json:"flag"`
	FlagRule    string     `json:"flag_rule"`
	FlagExample string     `json:"flag_example"`
	DockerImage string     `json:"docker_image"`
	RunCommand  string     `json:"run_command"`
	Status      string     `json:"status"`
	VisibleFrom *time.Time `json:"visible_from"`
	VisibleTo   *time.Time `json:"visible_to"`
}

// ProblemSubmissionMeta 提交场景下的题目元数据。
type ProblemSubmissionMeta struct {
	ID          int
	Title       string
	Points      int
	Scope       string
	Status      string
	FlagHash    string
	FlagRule    string
	ContestID   *string
	VisibleFrom *time.Time
	VisibleTo   *time.Time
}
