package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

const flagHashV2Prefix = "hmac-sha256$"

var (
	flagHashSecretOnce sync.Once
	flagHashSecretKey  []byte
)

const defaultFlagHashSecret = "ctf_exec_default_flag_secret_change_me"

// InitFlagHashSecret 在启动时注入 flag 哈希密钥。
// secret 为空时回退为 fallbackSecret；两者都为空时使用内置默认值。
func InitFlagHashSecret(secret, fallbackSecret string) {
	flagHashSecretOnce.Do(func() {
		candidate := strings.TrimSpace(secret)
		if candidate == "" {
			candidate = strings.TrimSpace(fallbackSecret)
		}
		if candidate == "" {
			candidate = defaultFlagHashSecret
		}
		flagHashSecretKey = []byte(candidate)
	})
}

func getFlagHashSecret() []byte {
	InitFlagHashSecret("", "")

	return flagHashSecretKey
}

func hashFlagHMACHex(flag string) string {
	mac := hmac.New(sha256.New, getFlagHashSecret())
	_, _ = mac.Write([]byte(flag))
	return hex.EncodeToString(mac.Sum(nil))
}

// HashFlag 生成带版本前缀的 HMAC-SHA256 哈希。
func HashFlag(flag string) string {
	return fmt.Sprintf("%s%s", flagHashV2Prefix, hashFlagHMACHex(flag))
}

// VerifyFlagHash 验证 Flag 哈希。
// 支持两种格式：
// 1) hmac-sha256$<hex>（新格式）
// 2) <sha256-hex>（旧格式，兼容历史数据）
func VerifyFlagHash(flag, storedHash string) bool {
	storedHash = strings.TrimSpace(storedHash)
	if storedHash == "" {
		return false
	}

	if strings.HasPrefix(storedHash, flagHashV2Prefix) {
		expected := strings.TrimPrefix(storedHash, flagHashV2Prefix)
		computed := hashFlagHMACHex(flag)
		return hmac.Equal([]byte(expected), []byte(computed))
	}

	legacy := sha256.Sum256([]byte(flag))
	legacyHex := hex.EncodeToString(legacy[:])
	return subtle.ConstantTimeCompare([]byte(legacyHex), []byte(storedHash)) == 1
}

// MustHashFlag 哈希flag，如果失败则panic（理论上不会失败）
func MustHashFlag(flag string) string {
	return HashFlag(flag)
}

// HashFlagWithPrefix 可选：带前缀的哈希
func HashFlagWithPrefix(flag, prefix string) string {
	data := prefix + flag
	return fmt.Sprintf("%s%s", flagHashV2Prefix, hashFlagHMACHex(data))
}

// VerifyFlagHashWithPrefix 验证带前缀的flag哈希
func VerifyFlagHashWithPrefix(flag, storedHash, prefix string) bool {
	computedHash := HashFlagWithPrefix(flag, prefix)
	return computedHash == storedHash
}
