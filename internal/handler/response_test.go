package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

func TestWriteJSON_SetsContentType(t *testing.T) {
	// GIVEN an http.ResponseWriter
	// WHEN WriteJSON is called
	// THEN the Content-Type is application/json
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusOK, contracts.ListResponse{Data: []int{1}})

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestWriteJSON_ListShape(t *testing.T) {
	// GIVEN a list payload
	// WHEN WriteJSON is called
	// THEN the body matches the spec's data+pagination envelope
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusOK, contracts.ListResponse{
		Data: []string{"a"},
		Pagination: &contracts.Pagination{Total: 1, Limit: 20, Offset: 0, HasMore: false},
	})

	var got struct {
		Data       []string             `json:"data"`
		Pagination *contracts.Pagination `json:"pagination"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 1 || got.Data[0] != "a" {
		t.Errorf("data = %v", got.Data)
	}
	if got.Pagination == nil || got.Pagination.Total != 1 {
		t.Errorf("pagination = %v", got.Pagination)
	}
}

func TestWriteError_SetsCorrectStatus(t *testing.T) {
	// GIVEN various error codes
	// WHEN WriteError is called
	// THEN the HTTP status matches the code
	tests := []struct {
		code contracts.ErrorCode
		want int
	}{
		{contracts.CodeValidation, 400},
		{contracts.CodeUnauthorized, 401},
		{contracts.CodeForbidden, 403},
		{contracts.CodeNotFound, 404},
		{contracts.CodeConflict, 409},
		{contracts.CodeInvalidState, 422},
		{contracts.CodeRateLimited, 429},
		{contracts.CodeInternal, 500},
	}
	for _, tt := range tests {
		rr := httptest.NewRecorder()
		WriteError(rr, &contracts.APIError{Code: tt.code, Message: "boom"})
		if rr.Code != tt.want {
			t.Errorf("code %v: status = %d, want %d", tt.code, rr.Code, tt.want)
		}
		if got := rr.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("code %v: Content-Type = %q", tt.code, got)
		}
	}
}

func TestWriteError_BodyShape(t *testing.T) {
	// GIVEN an APIError with details
	// WHEN WriteError is called
	// THEN the body is { "error": { "code", "message", "details" } }
	rr := httptest.NewRecorder()
	WriteError(rr, &contracts.APIError{
		Code:    contracts.CodeNotFound,
		Message: "device 42 missing",
		Details: map[string]any{"id": "42"},
	})

	var got contracts.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error.Code != contracts.CodeNotFound {
		t.Errorf("code = %q", got.Error.Code)
	}
	if got.Error.Message != "device 42 missing" {
		t.Errorf("message = %q", got.Error.Message)
	}
	if got.Error.Details["id"] != "42" {
		t.Errorf("details.id = %v", got.Error.Details["id"])
	}
}

func TestWriteError_NilErrorFallsBackToInternal(t *testing.T) {
	// GIVEN a nil APIError
	// WHEN WriteError is called
	// THEN 500 with INTERNAL_ERROR is rendered (defensive)
	rr := httptest.NewRecorder()
	WriteError(rr, nil)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
