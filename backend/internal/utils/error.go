package utils

import (
	"encoding/json"
	"net/http"
)

// WriteError 写入错误响应（用于标准 http.Handler）
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
