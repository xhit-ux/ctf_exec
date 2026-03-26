package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ctf_exec/backend/internal/config"
)

const flagHashV2Prefix = "hmac-sha256$"

var (
	flagHashSecretOnce sync.Once
	flagHashSecretKey  []byte
)

func getFlagHashSecret() []byte {
	flagHashSecretOnce.Do(func() {
		secret := strings.TrimSpace(os.Getenv("FLAG_HASH_SECRET"))
		if secret == "" {
			if cfg, err := config.LoadConfig(); err == nil {
				secret = strings.TrimSpace(cfg.JWT.Secret)
			}
		}
		if secret == "" {
			secret = "ctf_exec_default_flag_secret_change_me"
		}
		flagHashSecretKey = []byte(secret)
	})

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
