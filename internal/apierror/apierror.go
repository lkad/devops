package apierror

import (
	"encoding/json"
	"net/http"
)

// Error codes matching the API contract spec
const (
	CodeValidationError = "VALIDATION_ERROR"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeInvalidState    = "INVALID_STATE"
	CodeRateLimited     = "RATE_LIMITED"
	CodeInternalError   = "INTERNAL_ERROR"
)

// ErrorResponse represents the standard JSON error response format
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// RespondWithError sends a JSON error response and returns the HTTP status code
func RespondWithError(w http.ResponseWriter, code string, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: nil,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// RespondWithErrorDetails sends a JSON error response with details
func RespondWithErrorDetails(w http.ResponseWriter, code string, message string, details interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// ValidationError returns a 400 VALIDATION_ERROR response
func ValidationError(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeValidationError, message, http.StatusBadRequest)
}

// ValidationErrorDetails returns a 400 VALIDATION_ERROR response with details
func ValidationErrorDetails(w http.ResponseWriter, message string, details interface{}) {
	RespondWithErrorDetails(w, CodeValidationError, message, details, http.StatusBadRequest)
}

// Unauthorized returns a 401 UNAUTHORIZED response
func Unauthorized(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeUnauthorized, message, http.StatusUnauthorized)
}

// Forbidden returns a 403 FORBIDDEN response
func Forbidden(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeForbidden, message, http.StatusForbidden)
}

// NotFound returns a 404 NOT_FOUND response
func NotFound(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeNotFound, message, http.StatusNotFound)
}

// Conflict returns a 409 CONFLICT response
func Conflict(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeConflict, message, http.StatusConflict)
}

// InvalidState returns a 422 INVALID_STATE response
func InvalidState(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeInvalidState, message, http.StatusUnprocessableEntity)
}

// RateLimited returns a 429 RATE_LIMITED response
func RateLimited(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeRateLimited, message, http.StatusTooManyRequests)
}

// InternalError returns a 500 INTERNAL_ERROR response
func InternalError(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeInternalError, message, http.StatusInternalServerError)
}

// InternalErrorFromErr returns a 500 INTERNAL_ERROR response with error message
func InternalErrorFromErr(w http.ResponseWriter, err error) {
	RespondWithError(w, CodeInternalError, err.Error(), http.StatusInternalServerError)
}

// MethodNotAllowed returns a 400 response for invalid HTTP method
func MethodNotAllowed(w http.ResponseWriter) {
	RespondWithError(w, CodeValidationError, "method not allowed", http.StatusMethodNotAllowed)
}

// ServiceUnavailable returns a 503 response
func ServiceUnavailable(w http.ResponseWriter, message string) {
	RespondWithError(w, CodeInternalError, message, http.StatusServiceUnavailable)
}