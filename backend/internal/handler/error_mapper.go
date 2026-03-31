package handler

import (
	"errors"
	"net/http"

	"github.com/ctf_exec/backend/internal/apperr"
)

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
