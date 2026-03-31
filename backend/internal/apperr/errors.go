package apperr

import "errors"

var (
	ErrBadRequest      = errors.New("bad_request")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not_found")
	ErrConflict        = errors.New("conflict")
	ErrTooManyRequests = errors.New("too_many_requests")
	ErrInternal        = errors.New("internal")
)

// AppError keeps machine-readable kind and user-facing message separated.
type AppError struct {
	Kind              error
	Msg               string
	Cause             error
	RetryAfterSeconds int
}

func (e *AppError) Error() string {
	return e.Msg
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) Is(target error) bool {
	return e.Kind == target
}

func New(kind error, msg string) error {
	return &AppError{Kind: kind, Msg: msg}
}

func Wrap(kind error, msg string, cause error) error {
	return &AppError{Kind: kind, Msg: msg, Cause: cause}
}

func NewWithRetryAfter(kind error, msg string, retryAfterSeconds int) error {
	if retryAfterSeconds < 0 {
		retryAfterSeconds = 0
	}
	return &AppError{Kind: kind, Msg: msg, RetryAfterSeconds: retryAfterSeconds}
}

func WrapWithRetryAfter(kind error, msg string, cause error, retryAfterSeconds int) error {
	if retryAfterSeconds < 0 {
		retryAfterSeconds = 0
	}
	return &AppError{Kind: kind, Msg: msg, Cause: cause, RetryAfterSeconds: retryAfterSeconds}
}
