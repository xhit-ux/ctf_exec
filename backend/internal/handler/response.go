package handler

import (
	"net/http"

	"github.com/ctf_exec/backend/internal/apierror"
	"github.com/ctf_exec/backend/internal/apperr"
	"github.com/gin-gonic/gin"
)

// writeError 统一输出对外错误响应，详细错误仅写日志。
func writeError(c *gin.Context, err error, defaultStatus int) {
	apierror.Write(c, err, defaultStatus)
}

func writeBadRequest(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrBadRequest, msg), http.StatusBadRequest)
}

func writeUnauthorized(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrUnauthorized, msg), http.StatusUnauthorized)
}

func writeForbidden(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrForbidden, msg), http.StatusForbidden)
}

func writeNotFound(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrNotFound, msg), http.StatusNotFound)
}

func writeConflict(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrConflict, msg), http.StatusConflict)
}

func writeTooManyRequests(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrTooManyRequests, msg), http.StatusTooManyRequests)
}

func writeInternal(c *gin.Context, msg string) {
	writeError(c, apperr.New(apperr.ErrInternal, msg), http.StatusInternalServerError)
}
