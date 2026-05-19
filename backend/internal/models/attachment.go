package models

import "time"

// Attachment 题目附件（一题一包）。
// v2.1：DB 层已加 UNIQUE(problem_id) 强制单附件。
// 下载链接通过 GET /api/problems/{id}/attachment 重定向获取。
type Attachment struct {
	ID          string    `json:"id"`
	ProblemID   string    `json:"problem_id"`
	FileName    string    `json:"file_name"`
	BucketName  string    `json:"bucket_name,omitempty"`
	ObjectKey   string    `json:"object_key,omitempty"`
	FilePath    string    `json:"file_path,omitempty"`
	FileSize    int64     `json:"file_size"`
	Sha256      *string   `json:"sha256,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url,omitempty"`
}
