package errors

import (
	"errors"
	"net/http"
)

// Standard error types
var (
	ErrNotFound          = errors.New("resource not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrConflict          = errors.New("resource conflict")
	ErrInternal          = errors.New("internal server error")
	ErrTemporaryFailure  = errors.New("temporary failure")
	ErrPermanentFailure  = errors.New("permanent failure")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrTimeout           = errors.New("timeout")
	ErrRateLimited       = errors.New("rate limited")
)

// AppError represents a structured application error with context
type AppError struct {
	Err error
	StatusCode int
	Message string
	Retryable bool
	Context map[string]interface{}
}

// Error returns the error message
func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithContext adds additional context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewAppError creates a new AppError with the given parameters
func NewAppError(err error, message string, statusCode int, retryable bool) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  retryable,
		Context:    make(map[string]interface{}),
	}
}

// IsRetryable checks if the error is retryable
func IsRetryable(err error) bool {
	var appErr *AppError

	if errors.As(err, &appErr) {
		return appErr.Retryable
	}

	// By default, classify some standard errors as retryable
	return errors.Is(err, ErrTemporaryFailure) ||
		errors.Is(err, ErrServiceUnavailable) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrRateLimited)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string) *AppError {
	return NewAppError(ErrNotFound, message, http.StatusNotFound, false)
}

// NewInvalidInputError creates an invalid input error
func NewInvalidInputError(message string) *AppError {
	return NewAppError(ErrInvalidInput, message, http.StatusBadRequest, false)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	return NewAppError(ErrUnauthorized, message, http.StatusUnauthorized, false)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	return NewAppError(ErrForbidden, message, http.StatusForbidden, false)
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return NewAppError(ErrConflict, message, http.StatusConflict, false)
}

// NewInternalError creates an internal server error
func NewInternalError(message string) *AppError {
	return NewAppError(ErrInternal, message, http.StatusInternalServerError, true)
}

// NewTemporaryError creates a temporary error
func NewTemporaryError(message string) *AppError {
	return NewAppError(ErrTemporaryFailure, message, http.StatusServiceUnavailable, true)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string) *AppError {
	return NewAppError(ErrTimeout, message, http.StatusGatewayTimeout, true)
}

// NewRateLimitedError creates a rate limited error
func NewRateLimitedError(message string) *AppError {
	return NewAppError(ErrRateLimited, message, http.StatusTooManyRequests, true)
}