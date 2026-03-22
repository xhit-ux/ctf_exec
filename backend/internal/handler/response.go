package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func genericErrorMessage(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "请求参数有误"
	case http.StatusUnauthorized:
		return "请先登录"
	case http.StatusForbidden:
		return "当前操作未授权"
	case http.StatusNotFound:
		return "请求的资源不存在"
	case http.StatusConflict:
		return "请求冲突，请稍后重试"
	case http.StatusTooManyRequests:
		return "请求过于频繁，请稍后再试"
	default:
		return "服务器开小差了，请稍后再试"
	}
}

func logError(c *gin.Context, statusCode int, err error) {
	userID := "anonymous"
	if claims, claimsErr := middleware.GetClaims(c); claimsErr == nil && claims != nil {
		userID = claims.UserID
	}

	log.Printf("[api-error] status=%d method=%s path=%s ip=%s user_id=%s err=%v", statusCode, c.Request.Method, c.FullPath(), realClientIP(c), userID, err)
}

func publicErrorMessage(err error, statusCode int) string {
	if statusCode >= http.StatusInternalServerError {
		return genericErrorMessage(statusCode)
	}

	var appErr *apperr.AppError
	if errors.As(err, &appErr) && appErr != nil && appErr.Msg != "" {
		return appErr.Msg
	}

	return genericErrorMessage(statusCode)
}

// writeError 统一输出对外错误响应，详细错误仅写日志。
func writeError(c *gin.Context, err error, defaultStatus int) {
	statusCode := statusCodeByError(err, defaultStatus)
	logError(c, statusCode, err)
	c.JSON(statusCode, gin.H{"error": publicErrorMessage(err, statusCode)})
}
