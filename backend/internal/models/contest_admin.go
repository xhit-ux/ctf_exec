package models

import "time"

const (
	ContestAdminRoleAdmin    = "admin"
	ContestAdminRoleObserver = "observer"
)

// ContestAdmin 表示赛事级管理员关系。
type ContestAdmin struct {
	ID        string    `json:"id"`
	ContestID string    `json:"contest_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	GrantedBy *string   `json:"granted_by,omitempty"`
	GrantedAt time.Time `json:"granted_at"`
}

// AddContestAdminRequest 添加赛事管理员请求。
type AddContestAdminRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
