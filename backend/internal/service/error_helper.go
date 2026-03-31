package service

import (
	"errors"

	"github.com/ctf_exec/backend/internal/apperr"
)

func isKnownAppError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, apperr.ErrBadRequest) ||
		errors.Is(err, apperr.ErrUnauthorized) ||
		errors.Is(err, apperr.ErrForbidden) ||
		errors.Is(err, apperr.ErrNotFound) ||
		errors.Is(err, apperr.ErrConflict) ||
		errors.Is(err, apperr.ErrTooManyRequests) ||
		errors.Is(err, apperr.ErrInternal)
}

func wrapServiceError(msg string, err error) error {
	if err == nil {
		return nil
	}
	if isKnownAppError(err) {
		return err
	}
	return apperr.Wrap(apperr.ErrInternal, msg, err)
}
