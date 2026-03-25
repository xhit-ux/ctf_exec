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

	// 邮箱验证码
	KeyEmailVerifyCodeTemplate = "auth:email_verify:code:%s"

	// 邮箱验证码发送冷却
	KeyEmailVerifyCooldownTemplate = "auth:email_verify:cooldown:%s"

	// 邮箱验证码发送日限额
	KeyEmailVerifyDailyLimitTemplate = "auth:email_verify:daily:%s:%s"

	// 机器人验证限流
	KeyBotVerifyRateLimitTemplate = "auth:bot_verify:ip:%s"

	// JWT 会话: jti -> user_id
	KeyJWTSessionByJTITemplate = "auth:jwt:session:jti:%s"

	// 用户会话索引: user_id -> set(jti)
	KeyJWTUserSessionSetTemplate = "auth:jwt:user_sessions:%s"

	// Refresh 会话: jti -> user_id
	KeyJWTRefreshSessionByJTITemplate = "auth:jwt:refresh:jti:%s"

	// Refresh 用户会话索引: user_id -> set(jti)
	KeyJWTRefreshUserSessionSetTemplate = "auth:jwt:refresh:user_sessions:%s"
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

// GetEmailVerifyCodeKey 获取邮箱验证码键
func GetEmailVerifyCodeKey(email string) string {
	return fmt.Sprintf(KeyEmailVerifyCodeTemplate, email)
}

// GetEmailVerifyCooldownKey 获取验证码发送冷却键
func GetEmailVerifyCooldownKey(email string) string {
	return fmt.Sprintf(KeyEmailVerifyCooldownTemplate, email)
}

// GetEmailVerifyDailyLimitKey 获取验证码发送日限额键
func GetEmailVerifyDailyLimitKey(email, day string) string {
	return fmt.Sprintf(KeyEmailVerifyDailyLimitTemplate, email, day)
}

// GetBotVerifyRateLimitKey 获取机器人验证限流键
func GetBotVerifyRateLimitKey(clientIP string) string {
	return fmt.Sprintf(KeyBotVerifyRateLimitTemplate, clientIP)
}

// GetJWTSessionByJTIKey 获取 jti 会话键。
func GetJWTSessionByJTIKey(jti string) string {
	return fmt.Sprintf(KeyJWTSessionByJTITemplate, jti)
}

// GetJWTUserSessionSetKey 获取用户会话集合键。
func GetJWTUserSessionSetKey(userID string) string {
	return fmt.Sprintf(KeyJWTUserSessionSetTemplate, userID)
}

// GetJWTRefreshSessionByJTIKey 获取 refresh jti 会话键。
func GetJWTRefreshSessionByJTIKey(jti string) string {
	return fmt.Sprintf(KeyJWTRefreshSessionByJTITemplate, jti)
}

// GetJWTRefreshUserSessionSetKey 获取用户 refresh 会话集合键。
func GetJWTRefreshUserSessionSetKey(userID string) string {
	return fmt.Sprintf(KeyJWTRefreshUserSessionSetTemplate, userID)
}
