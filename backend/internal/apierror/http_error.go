package apierror

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/ctf_exec/backend/internal/utils"
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

func statusCodeByError(err error, defaultStatus int) int {
	switch {
	case errors.Is(err, apperr.ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, apperr.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, apperr.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, apperr.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apperr.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, apperr.ErrTooManyRequests):
		return http.StatusTooManyRequests
	case errors.Is(err, apperr.ErrInternal):
		return http.StatusInternalServerError
	default:
		return defaultStatus
	}
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

func userIDFromContext(c *gin.Context) string {
	if c == nil {
		return "anonymous"
	}
	v, ok := c.Get("claims")
	if !ok {
		return "anonymous"
	}
	claims, ok := v.(*utils.Claims)
	if !ok || claims == nil || claims.UserID == "" {
		return "anonymous"
	}
	return claims.UserID
}

func logError(c *gin.Context, statusCode int, err error) {
	path := ""
	method := ""
	ip := ""
	if c != nil && c.Request != nil {
		path = c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method = c.Request.Method
		ip = c.ClientIP()
	}

	log.Printf("[api-error] status=%d method=%s path=%s ip=%s user_id=%s err=%v", statusCode, method, path, ip, userIDFromContext(c), err)
}

func Write(c *gin.Context, err error, defaultStatus int) {
	statusCode := statusCodeByError(err, defaultStatus)
	if statusCode == http.StatusTooManyRequests {
		retryAfter := 60
		var appErr *apperr.AppError
		if errors.As(err, &appErr) && appErr != nil && appErr.RetryAfterSeconds > 0 {
			retryAfter = appErr.RetryAfterSeconds
		}
		c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
	}
	logError(c, statusCode, err)
	c.JSON(statusCode, gin.H{"error": publicErrorMessage(err, statusCode)})
}
