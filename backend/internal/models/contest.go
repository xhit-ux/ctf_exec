package models

import "time"

// Contest 赛事模型。
type Contest struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CoverImage  string     `json:"cover_image"`
	Visibility  string     `json:"visibility"`
	CreatedBy   *string    `json:"created_by,omitempty"`
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
	Visibility  string     `json:"visibility"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	ScoringType string     `json:"scoring_type"`
}

// UpdateContestRequest 更新赛事请求。
type UpdateContestRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CoverImage  string     `json:"cover_image"`
	Visibility  string     `json:"visibility"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	ScoringType string     `json:"scoring_type"`
}

// ContestChallenge 赛事题目实例（contest_challenges 表行）。
// v2.1 模板-实例分离的关键模型：每场赛事可独立定义计分参数。
type ContestChallenge struct {
	ID             string    `json:"id"`
	ContestID      string    `json:"contest_id"`
	ProblemID      string    `json:"problem_id"`
	ContestScore   int       `json:"contest_score"`
	IsDynamicScore bool      `json:"is_dynamic_score"`
	InitialScore   *int      `json:"initial_score,omitempty"`
	MinScore       *int      `json:"min_score,omitempty"`
	DecayConstant  *int      `json:"decay_constant,omitempty"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ContestChallengeBindRequest 赛事题目编排（绑定）请求。
// v2.1：用 5 个独立字段替代旧 score_override 单一字段。
type ContestChallengeBindRequest struct {
	ChallengeID    string `json:"challenge_id"`
	ContestScore   *int   `json:"contest_score"`
	IsDynamicScore bool   `json:"is_dynamic_score"`
	InitialScore   *int   `json:"initial_score"`
	MinScore       *int   `json:"min_score"`
	DecayConstant  *int   `json:"decay_constant"`
	SortOrder      *int   `json:"sort_order"`
}

// UpdateContestChallengeRequest 更新已绑定的赛事题目计分配置。
type UpdateContestChallengeRequest struct {
	ContestScore   *int  `json:"contest_score"`
	IsDynamicScore *bool `json:"is_dynamic_score"`
	InitialScore   *int  `json:"initial_score"`
	MinScore       *int  `json:"min_score"`
	DecayConstant  *int  `json:"decay_constant"`
	SortOrder      *int  `json:"sort_order"`
}

// JoinContestRequest 报名赛事请求。
type JoinContestRequest struct {
	TeamID *string `json:"team_id"`
}

// LeaveContestRequest 退赛请求。
type LeaveContestRequest struct {
	TeamID *string `json:"team_id"`
}

// ContestChallengeView 赛事题目展示结构（user 视角）。
type ContestChallengeView struct {
	ID             string `json:"id"`
	ProblemID      string `json:"problem_id"`
	Title          string `json:"title"`
	Category       string `json:"category"`
	Difficulty     string `json:"difficulty"`
	CurrentScore   int    `json:"current_score"`
	IsDynamicScore bool   `json:"is_dynamic_score"`
	InitialScore   *int   `json:"initial_score,omitempty"`
	MinScore       *int   `json:"min_score,omitempty"`
	DecayConstant  *int   `json:"decay_constant,omitempty"`
	SolveCount     int    `json:"solve_count"`
	IsSolved       bool   `json:"is_solved"`
	IsLocked       bool   `json:"is_locked,omitempty"`
	SortOrder      int    `json:"sort_order"`
}

// ContestChallengeAdminView 赛事管理员视角的题目实例视图。
type ContestChallengeAdminView struct {
	ID             string `json:"id"`
	ProblemID      string `json:"problem_id"`
	Title          string `json:"title"`
	Category       string `json:"category"`
	Difficulty     string `json:"difficulty"`
	PlatformScore  int    `json:"platform_score"`
	ContestScore   int    `json:"contest_score"`
	IsDynamicScore bool   `json:"is_dynamic_score"`
	InitialScore   *int   `json:"initial_score,omitempty"`
	MinScore       *int   `json:"min_score,omitempty"`
	DecayConstant  *int   `json:"decay_constant,omitempty"`
	CurrentScore   int    `json:"current_score"`
	SolveCount     int    `json:"solve_count"`
	SortOrder      int    `json:"sort_order"`
	Status         string `json:"status"`
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

// ContestParticipantScope 赛事提交计分主体（个人或团队）。
type ContestParticipantScope struct {
	UserID *string
	TeamID *string
}

// ContestParticipantItem 赛事参赛者条目。
type ContestParticipantItem struct {
	ID           string    `json:"id"`
	ContestID    string    `json:"contest_id"`
	UserID       *string   `json:"user_id,omitempty"`
	TeamID       *string   `json:"team_id,omitempty"`
	Username     *string   `json:"username,omitempty"`
	DisplayName  *string   `json:"display_name,omitempty"`
	TeamName     *string   `json:"team_name,omitempty"`
	Status       string    `json:"status"`
	RegisteredAt time.Time `json:"registered_at"`
}

// ContestSubmissionItem 赛事提交记录条目。
type ContestSubmissionItem struct {
	ID           string    `json:"id"`
	UserID       *string   `json:"user_id,omitempty"`
	TeamID       *string   `json:"team_id,omitempty"`
	ProblemID    string    `json:"problem_id"`
	ContestID    *string   `json:"contest_id,omitempty"`
	IsCorrect    bool      `json:"is_correct"`
	IPAddress    *string   `json:"ip_address,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Username     *string   `json:"username,omitempty"`
	DisplayName  *string   `json:"display_name,omitempty"`
	TeamName     *string   `json:"team_name,omitempty"`
	ProblemTitle *string   `json:"problem_title,omitempty"`
}
