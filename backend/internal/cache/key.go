package cache

import "fmt"

// Redis键常量定义
const (
	// 全站用户排行榜
	KeyGlobalUserLeaderboard = "leaderboard:global:user"
	
	// 全站团队排行榜
	KeyGlobalTeamLeaderboard = "leaderboard:global:team"
	
	// 赛事用户排行榜模板
	KeyContestUserLeaderboardTemplate = "leaderboard:contest:%s:user"
	
	// 赛事团队排行榜模板
	KeyContestTeamLeaderboardTemplate = "leaderboard:contest:%s:team"
	
	// 用户提交频率限制
	KeySubmissionRateLimitTemplate = "submission:rate_limit:%s"
	
	// 用户最近提交时间集合
	KeyUserRecentSubmissionsTemplate = "user:recent_submissions:%s"
	
	// 题目解决状态
	KeyProblemSolvedStatusTemplate = "problem:solved:%s:%d"
)

// GetContestUserLeaderboardKey 获取赛事用户排行榜键
func GetContestUserLeaderboardKey(contestID string) string {
	return fmt.Sprintf(KeyContestUserLeaderboardTemplate, contestID)
}

// GetContestTeamLeaderboardKey 获取赛事团队排行榜键
func GetContestTeamLeaderboardKey(contestID string) string {
	return fmt.Sprintf(KeyContestTeamLeaderboardTemplate, contestID)
}

// GetSubmissionRateLimitKey 获取用户提交频率限制键
func GetSubmissionRateLimitKey(userID string) string {
	return fmt.Sprintf(KeySubmissionRateLimitTemplate, userID)
}

// GetUserRecentSubmissionsKey 获取用户最近提交键
func GetUserRecentSubmissionsKey(userID string) string {
	return fmt.Sprintf(KeyUserRecentSubmissionsTemplate, userID)
}

// GetProblemSolvedStatusKey 获取题目解决状态键
func GetProblemSolvedStatusKey(userID string, problemID int) string {
	return fmt.Sprintf(KeyProblemSolvedStatusTemplate, userID, problemID)
}