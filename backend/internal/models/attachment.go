package models

import "time"

// Attachment represents a problem attachment stored in object storage or local filesystem.
type Attachment struct {
	ID         string    `json:"id"`
	ProblemID  string    `json:"problem_id"`
	FileName   string    `json:"file_name"`
	BucketName string    `json:"bucket_name"`
	ObjectKey  string    `json:"object_key"`
	FilePath   string    `json:"file_path,omitempty"`
	FileSize   int64     `json:"file_size"`
	Sha256     *string   `json:"sha256,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
