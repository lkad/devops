package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	ValidationError(w, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeValidationError {
		t.Errorf("expected code %s, got %s", CodeValidationError, resp.Error.Code)
	}
	if resp.Error.Message != "invalid input" {
		t.Errorf("expected message 'invalid input', got '%s'", resp.Error.Message)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	Unauthorized(w, "not logged in")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeUnauthorized {
		t.Errorf("expected code %s, got %s", CodeUnauthorized, resp.Error.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	Forbidden(w, "access denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeForbidden {
		t.Errorf("expected code %s, got %s", CodeForbidden, resp.Error.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeNotFound {
		t.Errorf("expected code %s, got %s", CodeNotFound, resp.Error.Code)
	}
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()
	Conflict(w, "resource already exists")

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeConflict {
		t.Errorf("expected code %s, got %s", CodeConflict, resp.Error.Code)
	}
}

func TestInvalidState(t *testing.T) {
	w := httptest.NewRecorder()
	InvalidState(w, "invalid state transition")

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status %d, got %d", http.StatusUnprocessableEntity, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeInvalidState {
		t.Errorf("expected code %s, got %s", CodeInvalidState, resp.Error.Code)
	}
}

func TestRateLimited(t *testing.T) {
	w := httptest.NewRecorder()
	RateLimited(w, "too many requests")

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeRateLimited {
		t.Errorf("expected code %s, got %s", CodeRateLimited, resp.Error.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	InternalError(w, "something went wrong")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeInternalError {
		t.Errorf("expected code %s, got %s", CodeInternalError, resp.Error.Code)
	}
}

func TestInternalErrorFromErr(t *testing.T) {
	w := httptest.NewRecorder()
	err := &testError{"database connection failed"}
	InternalErrorFromErr(w, err)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeInternalError {
		t.Errorf("expected code %s, got %s", CodeInternalError, resp.Error.Code)
	}
	if resp.Error.Message != "database connection failed" {
		t.Errorf("expected message 'database connection failed', got '%s'", resp.Error.Message)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	w := httptest.NewRecorder()
	MethodNotAllowed(w)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeValidationError {
		t.Errorf("expected code %s, got %s", CodeValidationError, resp.Error.Code)
	}
}

func TestServiceUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	ServiceUnavailable(w, "service down")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeInternalError {
		t.Errorf("expected code %s, got %s", CodeInternalError, resp.Error.Code)
	}
}

func TestValidationErrorDetails(t *testing.T) {
	w := httptest.NewRecorder()
	details := map[string]string{"field": "name", "issue": "required"}
	ValidationErrorDetails(w, "validation failed", details)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeValidationError {
		t.Errorf("expected code %s, got %s", CodeValidationError, resp.Error.Code)
	}
	if resp.Error.Details == nil {
		t.Error("expected details to be present")
	}
}

func TestContentType(t *testing.T) {
	w := httptest.NewRecorder()
	NotFound(w, "not found")

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}