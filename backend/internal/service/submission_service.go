package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/models"
	"github.com/ctf_exec/backend/internal/repository"
	"github.com/ctf_exec/backend/internal/utils"
	"github.com/redis/go-redis/v9"
)

// SubmissionService 提交服务接口
type SubmissionService interface {
	SubmitFlag(userID string, problemID string, flag string, contestID *string, clientIP string) (*models.Submission, error)
	GetMySubmissions(userID string) ([]*models.Submission, error)
	CheckRateLimit(userID string) (bool, int64, error)
}

// SubmissionServiceImpl 提交服务实现
type SubmissionServiceImpl struct {
	submissionRepo *repository.SubmissionRepository
	problemRepo    *repository.ProblemRepository
	userRepo       *repository.UserRepository
	contestSvc     ContestService
	redisClient    *redis.Client
	rateLimit      int
	windowSeconds  int
}

// NewSubmissionService 创建提交服务实例
func NewSubmissionService(
	submissionRepo *repository.SubmissionRepository,
	problemRepo *repository.ProblemRepository,
	userRepo *repository.UserRepository,
	contestSvc ContestService,
	redisClient *redis.Client,
	rateLimit int,
	windowSeconds int,
) *SubmissionServiceImpl {
	return &SubmissionServiceImpl{
		submissionRepo: submissionRepo,
		problemRepo:    problemRepo,
		userRepo:       userRepo,
		contestSvc:     contestSvc,
		redisClient:    redisClient,
		rateLimit:      rateLimit,
		windowSeconds:  windowSeconds,
	}
}

// SubmitFlag 提交 flag
func (s *SubmissionServiceImpl) SubmitFlag(userID string, problemID string, flag string, contestID *string, clientIP string) (*models.Submission, error) {
	if err := validateSubmissionInput(problemID, flag); err != nil {
		return nil, err
	}

	// 检查提交频率限制
	allowed, remaining, err := s.CheckRateLimit(userID)
	if err != nil {
		return nil, apperr.Wrap(apperr.ErrInternal, "检查提交限制失败", err)
	}
	if !allowed {
		return nil, apperr.NewWithRetryAfter(apperr.ErrTooManyRequests, fmt.Sprintf("提交过于频繁，请稍后再试 (剩余时间: %d秒)", remaining), int(remaining))
	}

	// 获取题目元数据并校验提交可见性策略。
	problemMeta, err := s.problemRepo.GetProblemSubmissionMeta(problemID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, apperr.New(apperr.ErrBadRequest, "题目不存在")
		}
		return nil, apperr.Wrap(apperr.ErrInternal, "查询题目失败", err)
	}

	if err := validateProblemSubmissionPolicy(problemMeta, contestID); err != nil {
		return nil, err
	}

	// 按 flag_type/flag_rule 执行判题。
	// dynamic：取该用户当前实例的 flag_value 即席比对；
	// static + (exact|case_insensitive)：使用 flag_hash；
	// static + (regex|contains)：使用 flag_pattern。
	isCorrect, evalErr := s.evaluateFlagForSubmission(userID, problemMeta, flag)
	if evalErr != nil {
		return nil, evalErr
	}

	var contestScope *models.ContestParticipantScope
	var contestScore int
	if contestID != nil && *contestID != "" {
		if s.contestSvc == nil {
			return nil, apperr.New(apperr.ErrBadRequest, "赛事服务不可用")
		}
		scope, score, err := s.contestSvc.ValidateContestSubmission(*contestID, userID, problemID)
		if err != nil {
			return nil, err
		}
		contestScope = scope
		contestScore = score
	}

	alreadySolved, err := s.submissionRepo.HasCorrectSubmissionByUserProblem(userID, problemID)
	if err != nil {
		return nil, apperr.Wrap(apperr.ErrInternal, "查询历史解题记录失败", err)
	}

	// 非赛事场景：检查是否已经正确提交过该题目
	if isCorrect && alreadySolved && (contestID == nil || *contestID == "") {
		return nil, apperr.New(apperr.ErrConflict, "你已经解决过这个题目了")
	}

	// 创建提交请求
	req := &models.CreateSubmissionRequest{
		ProblemID: problemID,
		Flag:      flag,
		TeamID:    nil,
		ContestID: contestID,
		IsCorrect: isCorrect,
		IPAddress: clientIP,
	}
	if contestScope != nil {
		req.TeamID = contestScope.TeamID
	}

	// 保存提交记录
	submission, err := s.submissionRepo.CreateSubmission(userID, req)
	if err != nil {
		return nil, apperr.Wrap(apperr.ErrInternal, "保存提交记录失败", err)
	}

	// 正确提交后仅更新对应维度的分数。
	if isCorrect {
		if !alreadySolved && s.userRepo != nil && (contestID == nil || *contestID == "") {
			err = s.userRepo.IncreaseUserScoreAndSolvedCount(userID, problemMeta.PlatformScore)
			if err != nil {
				log.Printf("[submission] 更新全站用户积分失败 user_id=%s points=%d err=%v", userID, problemMeta.PlatformScore, err)
			}
		}

		if contestID != nil && *contestID != "" {
			err = s.contestSvc.ApplyContestSolve(*contestID, contestScope, contestScore, time.Now())
			if err != nil {
				log.Printf("[submission] 更新赛事榜失败 user_id=%s contest_id=%s problem_id=%s err=%v", userID, *contestID, problemID, err)
			}
		}
	}

	// 记录提交频率
	err = s.recordSubmission(userID)
	if err != nil {
		log.Printf("[submission] 记录提交频率失败 user_id=%s err=%v", userID, err)
	}

	// 设置提交标题
	submission.ProblemTitle = problemMeta.Title

	return submission, nil
}

// evaluateFlagForSubmission 是 SubmitFlag 的判题入口。
// 路由 4 条评估链路（flag_mode_explain.md Part 4）：
//
//  1. static  + exact|case_insensitive  → utils.EvaluateFlag(rule, hash, "", submitted)
//  2. static  + regex|contains          → utils.EvaluateFlag(rule, "", pattern, submitted)
//  3. dynamic + exact|case_insensitive  → 取 challenge_instances.flag_value 即席比对
//  4. dynamic + regex|contains          → 不允许（DB CHECK + handler/repo 已拦截）
func (s *SubmissionServiceImpl) evaluateFlagForSubmission(userID string, meta *models.ProblemSubmissionMeta, submitted string) (bool, error) {
	flagType := strings.ToLower(strings.TrimSpace(meta.FlagType))
	rule := utils.NormalizeFlagRule(meta.FlagRule)
	if rule == "" {
		return false, apperr.New(apperr.ErrInternal, "题目 flag_rule 不合法")
	}

	if flagType == "dynamic" {
		if rule == utils.FlagRuleRegex || rule == utils.FlagRuleContains {
			return false, apperr.New(apperr.ErrInternal, "dynamic flag 不支持 regex/contains 规则")
		}
		flagValue, err := s.problemRepo.GetActiveInstanceFlagValue(userID, meta.ID)
		if err != nil {
			return false, apperr.Wrap(apperr.ErrInternal, "查询实例 flag 失败", err)
		}
		if flagValue == "" {
			return false, apperr.New(apperr.ErrBadRequest, "请先启动题目实例后再提交 flag")
		}
		// 即席 hash 比对（保持与 static 相同的判题路径）。
		dynamicHash, hashErr := utils.PrepareStaticFlagHash(rule, flagValue)
		if hashErr != nil {
			return false, apperr.Wrap(apperr.ErrInternal, "动态 flag 哈希失败", hashErr)
		}
		matched, evalErr := utils.EvaluateFlag(rule, dynamicHash, "", submitted)
		if evalErr != nil {
			return false, apperr.Wrap(apperr.ErrInternal, "动态 flag 评估失败", evalErr)
		}
		return matched, nil
	}

	// static 路径：根据 rule 决定走 hash 还是 pattern。
	switch rule {
	case utils.FlagRuleExact, utils.FlagRuleCaseInsensitive:
		matched, evalErr := utils.EvaluateFlag(rule, meta.FlagHash, "", submitted)
		if evalErr != nil {
			return false, apperr.Wrap(apperr.ErrInternal, "flag 评估失败", evalErr)
		}
		return matched, nil
	case utils.FlagRuleRegex, utils.FlagRuleContains:
		matched, evalErr := utils.EvaluateFlag(rule, "", meta.FlagPattern, submitted)
		if evalErr != nil {
			return false, apperr.Wrap(apperr.ErrInternal, "flag 评估失败", evalErr)
		}
		return matched, nil
	default:
		return false, apperr.New(apperr.ErrInternal, "不支持的 flag_rule")
	}
}

func validateSubmissionInput(problemID string, flag string) error {
	if strings.TrimSpace(problemID) == "" {
		return apperr.New(apperr.ErrBadRequest, "无效的题目 ID")
	}
	if flag == "" {
		return apperr.New(apperr.ErrBadRequest, "Flag 不能为空")
	}
	return nil
}

func validateProblemSubmissionPolicy(problem *models.ProblemSubmissionMeta, contestID *string) error {
	if problem == nil {
		return apperr.New(apperr.ErrBadRequest, "题目元数据为空")
	}

	if problem.Status != "ready" {
		return apperr.New(apperr.ErrForbidden, "题目当前不可提交")
	}

	return nil
}

// GetMySubmissions 获取用户的提交记录
func (s *SubmissionServiceImpl) GetMySubmissions(userID string) ([]*models.Submission, error) {
	submissions, err := s.submissionRepo.GetSubmissionsByUserID(userID)
	if err != nil {
		return nil, apperr.Wrap(apperr.ErrInternal, "获取提交记录失败", err)
	}

	return submissions, nil
}

// CheckRateLimit 检查提交频率限制
func (s *SubmissionServiceImpl) CheckRateLimit(userID string) (bool, int64, error) {
	ctx := context.Background()
	key := fmt.Sprintf("submission:rate_limit:%s", userID)

	// 获取当前时间戳
	now := time.Now().Unix()
	windowStart := now - int64(s.windowSeconds)

	// 删除窗口期之前的记录
	err := s.redisClient.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart)).Err()
	if err != nil {
		return false, 0, apperr.Wrap(apperr.ErrInternal, "清理旧记录失败", err)
	}

	// 获取窗口期内的提交数量
	count, err := s.redisClient.ZCount(ctx, key, fmt.Sprintf("%d", windowStart), "+inf").Result()
	if err != nil {
		return false, 0, apperr.Wrap(apperr.ErrInternal, "获取提交数量失败", err)
	}

	// 检查是否超过限制
	if count >= int64(s.rateLimit) {
		// 获取最早提交的时间
		oldest, err := s.redisClient.ZRangeWithScores(ctx, key, 0, 0).Result()
		if err != nil || len(oldest) == 0 {
			return false, int64(s.windowSeconds), nil
		}
		// 计算剩余时间
		remaining := int64(s.windowSeconds) - (now - int64(oldest[0].Score))
		if remaining < 0 {
			remaining = 0
		}
		return false, remaining, nil
	}

	return true, 0, nil
}

// recordSubmission 记录提交
func (s *SubmissionServiceImpl) recordSubmission(userID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("submission:rate_limit:%s", userID)
	now := time.Now().Unix()

	// 添加当前时间戳到有序集合
	err := s.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  float64(now),
		Member: now,
	}).Err()
	if err != nil {
		return apperr.Wrap(apperr.ErrInternal, "记录提交失败", err)
	}

	// 设置键的过期时间
	expireTime := time.Duration(s.windowSeconds+60) * time.Second // 额外60秒缓冲
	err = s.redisClient.Expire(ctx, key, expireTime).Err()
	if err != nil {
		return apperr.Wrap(apperr.ErrInternal, "设置过期时间失败", err)
	}

	return nil
}
