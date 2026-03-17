package models

import "time"

// Contest 赛事模型。
type Contest struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CoverImage  string     `json:"cover_image"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	ScoringType string     `json:"scoring_type"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateContestRequest 创建赛事请求。
type CreateContestRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CoverImage  string     `json:"cover_image"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	ScoringType string     `json:"scoring_type"`
}

// UpdateContestRequest 更新赛事请求。
type UpdateContestRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CoverImage  string     `json:"cover_image"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	ScoringType string     `json:"scoring_type"`
}

// ContestChallengeBindRequest 赛事题目编排请求。
type ContestChallengeBindRequest struct {
	ChallengeID   int  `json:"challenge_id"`
	ScoreOverride *int `json:"score_override"`
}

// JoinContestRequest 报名赛事请求。
type JoinContestRequest struct {
	TeamID *string `json:"team_id"`
}

// LeaveContestRequest 退赛请求。
type LeaveContestRequest struct {
	TeamID *string `json:"team_id"`
}

// ContestChallengeView 赛事题目展示结构。
type ContestChallengeView struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	Difficulty    string `json:"difficulty"`
	Points        int    `json:"points"`
	ScoreOverride *int   `json:"score_override,omitempty"`
	IsSolved      bool   `json:"is_solved"`
}

// ContestScoreboardEntry 赛事榜条目，兼容个人/团队榜。
type ContestScoreboardEntry struct {
	Rank          int        `json:"rank"`
	UserID        *string    `json:"user_id,omitempty"`
	TeamID        *string    `json:"team_id,omitempty"`
	Username      *string    `json:"username,omitempty"`
	DisplayName   *string    `json:"display_name,omitempty"`
	Name          *string    `json:"name,omitempty"`
	Score         int        `json:"score"`
	SolvedCount   int        `json:"solved_count"`
	LastSolveTime *time.Time `json:"last_solve_time,omitempty"`
}

// ContestParticipantScope 表示赛事提交计分主体（个人或团队）。
type ContestParticipantScope struct {
	UserID *string
	TeamID *string
}
