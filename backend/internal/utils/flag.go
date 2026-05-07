package utils

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Flag 评估器（v2.1 + flag_mode_explain.md）
// ============================================================
//
// v2.1 把 Flag 拆成三个正交维度：
//   - flag_type   : static / dynamic
//   - flag_rule   : exact / case_insensitive / regex / contains
//   - flag_inject : env / sidecar
//
// 评估器只关心 flag_rule，不关心 type/inject —— 这是 service 层职责。
// 调用方在确定本次提交的"参考 flag"后（静态题取 problem_flags.flag_hash 或
// problem_flags.flag_pattern；动态题取 challenge_instances.flag_value 的哈希），
// 把它喂给本评估器。

const (
	FlagRuleExact           = "exact"
	FlagRuleCaseInsensitive = "case_insensitive"
	FlagRuleRegex           = "regex"
	FlagRuleContains        = "contains"
)

// ErrUnsupportedFlagRule 未知的 flag_rule。
var ErrUnsupportedFlagRule = errors.New("不支持的 flag_rule")

// ErrFlagPatternEmpty regex/contains 规则缺少 pattern。
var ErrFlagPatternEmpty = errors.New("flag_pattern 不能为空")

// ErrFlagHashEmpty exact/case_insensitive 规则缺少 hash。
var ErrFlagHashEmpty = errors.New("flag_hash 不能为空")

// ErrFlagPatternTooLong pattern 超过最大长度（防 DoS）。
var ErrFlagPatternTooLong = errors.New("flag_pattern 超过最大长度")

// ErrFlagSubmissionEmpty 提交内容为空。
var ErrFlagSubmissionEmpty = errors.New("提交内容不能为空")

// MaxFlagPatternLength 单个 pattern 的最大字符数（v2.1 安全约束）。
const MaxFlagPatternLength = 512

// MaxFlagSubmissionLength 单次提交内容的最大长度（防大字符串触发病态正则）。
const MaxFlagSubmissionLength = 4096

// regexEvalTimeout 单次正则评估的硬超时（防 ReDoS）。
const regexEvalTimeout = 200 * time.Millisecond

// regexCache 编译过的正则缓存，按 pattern 字符串去重。
var (
	regexCacheMu sync.RWMutex
	regexCache   = make(map[string]*regexp.Regexp, 64)
)

// NormalizeFlagRule 将原始字符串规范化为合法的 flag_rule。
// 空值 → exact；不识别的值返回空字符串供调用方拒绝。
func NormalizeFlagRule(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return FlagRuleExact
	}
	switch v {
	case FlagRuleExact, FlagRuleCaseInsensitive, FlagRuleRegex, FlagRuleContains:
		return v
	default:
		return ""
	}
}

// ValidateFlagPattern 校验 pattern 长度并预编译（仅 regex 时才编译）。
// 用于 handler 在保存 flag 配置前做校验。
func ValidateFlagPattern(rule, pattern string) error {
	rule = NormalizeFlagRule(rule)
	pattern = strings.TrimSpace(pattern)
	if rule == FlagRuleRegex || rule == FlagRuleContains {
		if pattern == "" {
			return ErrFlagPatternEmpty
		}
		if len(pattern) > MaxFlagPatternLength {
			return ErrFlagPatternTooLong
		}
		if rule == FlagRuleRegex {
			if _, err := compileFlagRegex(pattern); err != nil {
				return fmt.Errorf("正则编译失败: %w", err)
			}
		}
	}
	return nil
}

// EvaluateFlag 按规则比对提交的 flag 是否正确。
//
// 参数：
//   - rule       : exact / case_insensitive / regex / contains
//   - storedHash : 仅 exact / case_insensitive 使用；带 v2 前缀的 HMAC 哈希
//   - pattern    : 仅 regex / contains 使用；正则或子串
//   - submitted  : 用户提交的明文
//
// 返回：(matched, err)。错误为非业务错误（规则不支持、pattern 缺失、正则编译失败等）。
func EvaluateFlag(rule, storedHash, pattern, submitted string) (bool, error) {
	if submitted == "" {
		return false, ErrFlagSubmissionEmpty
	}
	if len(submitted) > MaxFlagSubmissionLength {
		submitted = submitted[:MaxFlagSubmissionLength]
	}
	rule = NormalizeFlagRule(rule)
	switch rule {
	case FlagRuleExact:
		if strings.TrimSpace(storedHash) == "" {
			return false, ErrFlagHashEmpty
		}
		return VerifyFlagHash(submitted, storedHash), nil

	case FlagRuleCaseInsensitive:
		if strings.TrimSpace(storedHash) == "" {
			return false, ErrFlagHashEmpty
		}
		return VerifyFlagHash(strings.ToLower(submitted), storedHash), nil

	case FlagRuleContains:
		needle := strings.TrimSpace(pattern)
		if needle == "" {
			return false, ErrFlagPatternEmpty
		}
		return strings.Contains(strings.ToLower(submitted), strings.ToLower(needle)), nil

	case FlagRuleRegex:
		patt := strings.TrimSpace(pattern)
		if patt == "" {
			return false, ErrFlagPatternEmpty
		}
		re, err := compileFlagRegex(patt)
		if err != nil {
			return false, fmt.Errorf("正则编译失败: %w", err)
		}
		return matchRegexWithTimeout(re, submitted, regexEvalTimeout)

	default:
		return false, ErrUnsupportedFlagRule
	}
}

// PrepareStaticFlagHash 在保存 static + (exact|case_insensitive) 的 flag 时，
// 按规则把明文标准化后再做 HMAC。
func PrepareStaticFlagHash(rule, plain string) (string, error) {
	rule = NormalizeFlagRule(rule)
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", errors.New("static flag 明文不能为空")
	}
	switch rule {
	case FlagRuleExact:
		return HashFlag(plain), nil
	case FlagRuleCaseInsensitive:
		return HashFlag(strings.ToLower(plain)), nil
	default:
		return "", fmt.Errorf("rule=%s 不应使用 hash 模式", rule)
	}
}

// ============================================================
// 内部实现
// ============================================================

func compileFlagRegex(pattern string) (*regexp.Regexp, error) {
	regexCacheMu.RLock()
	if re, ok := regexCache[pattern]; ok {
		regexCacheMu.RUnlock()
		return re, nil
	}
	regexCacheMu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	regexCacheMu.Lock()
	defer regexCacheMu.Unlock()
	if existing, ok := regexCache[pattern]; ok {
		return existing, nil
	}
	if len(regexCache) >= 1024 {
		regexCache = make(map[string]*regexp.Regexp, 64)
	}
	regexCache[pattern] = re
	return re, nil
}

// matchRegexWithTimeout 在独立 goroutine 中执行 MatchString，超过 timeout 视为失败（防 ReDoS）。
// regexp.Regexp 是 Go 标准库实现（RE2），无指数级回溯，但仍加 timeout 兜底。
func matchRegexWithTimeout(re *regexp.Regexp, input string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct {
		match bool
		err   error
	}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- struct {
					match bool
					err   error
				}{false, fmt.Errorf("正则匹配 panic: %v", r)}
			}
		}()
		done <- struct {
			match bool
			err   error
		}{re.MatchString(input), nil}
	}()

	select {
	case res := <-done:
		return res.match, res.err
	case <-ctx.Done():
		return false, fmt.Errorf("正则匹配超时（%s）", timeout)
	}
}
