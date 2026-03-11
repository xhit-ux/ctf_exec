package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashFlag SHA256哈希flag
func HashFlag(flag string) string {
	hash := sha256.Sum256([]byte(flag))
	return hex.EncodeToString(hash[:])
}

// VerifyFlagHash 验证flag的SHA256哈希
func VerifyFlagHash(flag, storedHash string) bool {
	computedHash := HashFlag(flag)
	return computedHash == storedHash
}

// MustHashFlag 哈希flag，如果失败则panic（理论上不会失败）
func MustHashFlag(flag string) string {
	return HashFlag(flag)
}

// HashFlagWithPrefix 可选：带前缀的哈希
func HashFlagWithPrefix(flag, prefix string) string {
	data := prefix + flag
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// VerifyFlagHashWithPrefix 验证带前缀的flag哈希
func VerifyFlagHashWithPrefix(flag, storedHash, prefix string) bool {
	computedHash := HashFlagWithPrefix(flag, prefix)
	return computedHash == storedHash
}