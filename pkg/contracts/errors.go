package contracts

import "fmt"

// ErrorCode is the machine-readable identifier for an error category.
// The string values are part of the public API contract — clients
// branch on them, so renaming a code is a breaking change.
type ErrorCode string

const (
	CodeValidation   ErrorCode = "VALIDATION_ERROR"
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeForbidden    ErrorCode = "FORBIDDEN"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeConflict     ErrorCode = "CONFLICT"
	CodeInvalidState ErrorCode = "INVALID_STATE"
	CodeRateLimited  ErrorCode = "RATE_LIMITED"
	CodeInternal     ErrorCode = "INTERNAL_ERROR"
)

// HTTPStatus maps an ErrorCode to its HTTP status code per the
// api-contract spec. Unknown codes fall back to 500 — a safe default
// that never leaks a 2xx to the client.
func (c ErrorCode) HTTPStatus() int {
	switch c {
	case CodeValidation:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeInvalidState:
		return 422
	case CodeRateLimited:
		return 429
	case CodeInternal:
		return 500
	default:
		return 500
	}
}

// APIError is the in-process error type that handlers convert to
// ErrorResponse. It carries everything needed to render a response:
// the wire-format code, the human message, and optional details.
type APIError struct {
	Code    ErrorCode
	Message string
	Details map[string]any
	Cause   error
}

// Error implements the error interface. The format is "CODE: message"
// so log lines are immediately readable without rendering the envelope.
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap allows errors.Is / errors.As to traverse to the underlying cause.
func (e *APIError) Unwrap() error {
	return e.Cause
}
