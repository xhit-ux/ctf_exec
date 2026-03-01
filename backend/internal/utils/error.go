package utils

import (
	"encoding/json"
	"net/http"

	"github.com/ctf_exec/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// WriteError 写入错误响应（用于标准 http.Handler）
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Code:    statusCode,
		Message: message,
	})
}

// GinError 返回 Gin 错误响应
func GinError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"code":    statusCode,
		"message": message,
	})
}