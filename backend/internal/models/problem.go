package models

// Problem 题目模型 (用于API返回，不包含Flag)
type Problem struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Points      int      `json:"points"`
	Difficulty  string   `json:"difficulty"`
	Tags        []string `json:"tags"` // 标签数组
	Description string   `json:"description"`
	IsSolved    bool     `json:"is_solved"`
}

// ProblemDetail 题目详情 (内部包括Flag，用于验证)
type ProblemDetail struct {
	Problem
	FlagHash string `json:"-"` // 不序列化
}

// CreateProblemRequest 创建题目请求
type CreateProblemRequest struct {
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Points      int      `json:"points"`
	Difficulty  string   `json:"difficulty"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Flag        string   `json:"flag"`
}

// UpdateProblemRequest 更新题目请求
type UpdateProblemRequest struct {
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Points      int      `json:"points"`
	Difficulty  string   `json:"difficulty"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Flag        string   `json:"flag"`
}
