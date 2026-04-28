package errors

import (
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	VALIDATION_ERROR    ErrorCode = "VALIDATION_ERROR"
	NOT_FOUND           ErrorCode = "NOT_FOUND"
	UNAUTHORIZED        ErrorCode = "UNAUTHORIZED"
	FORBIDDEN           ErrorCode = "FORBIDDEN"
	CONFLICT            ErrorCode = "CONFLICT"
	INTERNAL_ERROR      ErrorCode = "INTERNAL_ERROR"
	BAD_REQUEST         ErrorCode = "BAD_REQUEST"
	SERVICE_UNAVAILABLE ErrorCode = "SERVICE_UNAVAILABLE"
)

type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func New(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func NewWithDetails(code ErrorCode, message string, details any) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

func (e *AppError) WithDetails(details any) *AppError {
	return NewWithDetails(e.Code, e.Message, details)
}

func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case VALIDATION_ERROR:
		return http.StatusBadRequest
	case NOT_FOUND:
		return http.StatusNotFound
	case UNAUTHORIZED:
		return http.StatusUnauthorized
	case FORBIDDEN:
		return http.StatusForbidden
	case CONFLICT:
		return http.StatusConflict
	case INTERNAL_ERROR:
		return http.StatusInternalServerError
	case BAD_REQUEST:
		return http.StatusBadRequest
	case SERVICE_UNAVAILABLE:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func ValidationError(message string) *AppError {
	return New(VALIDATION_ERROR, message)
}

func NotFound(entity string) *AppError {
	return Newf(NOT_FOUND, "%s not found", entity)
}

func Newf(code ErrorCode, format string, args ...any) *AppError {
	return New(code, fmt.Sprintf(format, args...))
}

func Unauthorized(message string) *AppError {
	return New(UNAUTHORIZED, message)
}

func Forbidden(message string) *AppError {
	return New(FORBIDDEN, message)
}

func Conflict(entity string) *AppError {
	return Newf(CONFLICT, "%s already exists", entity)
}

func InternalError(message string) *AppError {
	return New(INTERNAL_ERROR, message)
}

func BadRequest(message string) *AppError {
	return New(BAD_REQUEST, message)
}
