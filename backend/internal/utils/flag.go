package utils

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	FlagRuleExact           = "exact"
	FlagRuleCaseInsensitive = "case_insensitive"
	FlagRuleRegex           = "regex"
	FlagRuleContains        = "contains"
)

var ErrUnsupportedFlagRule = errors.New("不支持的 flag_rule")
var ErrFlagPatternEmpty = errors.New("flag_pattern 不能为空")
var ErrFlagHashEmpty = errors.New("flag_hash 不能为空")
var ErrFlagPatternTooLong = errors.New("flag_pattern 超过最大长度")
var ErrFlagSubmissionEmpty = errors.New("提交内容不能为空")

const MaxFlagPatternLength = 512
const MaxFlagSubmissionLength = 4096
const regexEvalTimeout = 200 * time.Millisecond

var (
	regexCacheMu sync.RWMutex
	regexCache   = make(map[string]*regexp.Regexp, 64)
)

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

func GenerateDynamicFlag(template string) string {
	tmpl := normalizeDynamicFlagTemplate(template)
	return fmt.Sprintf(tmpl, generateUUID32())
}

func generateUUID32() string {
	// 标准 UUID 去掉连字符后是 32 位十六进制字符串。
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func normalizeDynamicFlagTemplate(template string) string {
	tmpl := strings.TrimSpace(template)
	if tmpl == "" {
		return "flag{%s}"
	}
	if strings.Contains(tmpl, "%s") {
		return tmpl
	}
	if strings.HasSuffix(tmpl, "}") && strings.Contains(tmpl, "{") {
		return strings.TrimSuffix(tmpl, "}") + "%s}"
	}
	return tmpl + "%s"
}

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

func matchRegexWithTimeout(re *regexp.Regexp, input string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		match bool
		err   error
	}
	done := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{false, fmt.Errorf("正则匹配 panic: %v", r)}
			}
		}()
		done <- result{re.MatchString(input), nil}
	}()

	select {
	case res := <-done:
		return res.match, res.err
	case <-ctx.Done():
		return false, fmt.Errorf("正则匹配超时（%s）", timeout)
	}
}
