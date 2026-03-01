package models

import "time"

type Team struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	InviteCode  string            `json:"invite_code,omitempty"`
	LeaderID    string            `json:"leader_id"`
	Score       int               `json:"score"`
	SolvedCount int               `json:"solved_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Members     []*TeamMemberUser `json:"members,omitempty"`
}

type TeamMemberUser struct {
	User
	RoleInTeam string `json:"role_in_team"`
}

type TeamMember struct {
	TeamID     string    `json:"team_id"`
	UserID     string    `json:"user_id"`
	RoleInTeam string    `json:"role_in_team"` // leader, member
	JoinedAt   time.Time `json:"joined_at"`
}

type CreateTeamRequest struct {
	Name string `json:"name"`
}

type JoinTeamRequest struct {
	InviteCode string `json:"invite_code"`
}

type TeamLeaderboardEntry struct {
	Rank        int     `json:"rank"`
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TotalScore  int     `json:"total_score"`
	AvgScore    float64 `json:"avg_score"`
	MemberCount int     `json:"member_count"`
}

type UpdateTeamRequest struct {
	Name string `json:"name"`
}

type KickMemberRequest struct {
	UserID string `json:"user_id"`
}
