package contracts

import (
	"encoding/json"
	"testing"
)

func TestListResponse_JSONShape(t *testing.T) {
	// GIVEN a list response with two items and pagination
	resp := ListResponse{
		Data: []string{"a", "b"},
		Pagination: &Pagination{
			Total:   2,
			Limit:   20,
			Offset:  0,
			HasMore: false,
		},
	}

	// WHEN marshaled to JSON
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// THEN the shape matches the spec: data + pagination
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["data"]; !ok {
		t.Error("expected 'data' field")
	}
	pag, ok := got["pagination"].(map[string]any)
	if !ok {
		t.Fatal("expected 'pagination' object")
	}
	for _, k := range []string{"total", "limit", "offset", "has_more"} {
		if _, ok := pag[k]; !ok {
			t.Errorf("pagination missing %q", k)
		}
	}
}

func TestErrorResponse_JSONShape(t *testing.T) {
	// GIVEN an error response with code, message, and details
	resp := ErrorResponse{
		Error: ErrorBody{
			Code:    "NOT_FOUND",
			Message: "device 42 not found",
			Details: map[string]any{"id": "42"},
		},
	}

	// WHEN marshaled to JSON
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// THEN the shape matches: { "error": { "code", "message", "details" } }
	var got struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", got.Error.Code)
	}
	if got.Error.Message != "device 42 not found" {
		t.Errorf("message = %q", got.Error.Message)
	}
	if got.Error.Details["id"] != "42" {
		t.Errorf("details.id = %v", got.Error.Details["id"])
	}
}

func TestListResponse_OmitsPaginationWhenEmpty(t *testing.T) {
	// GIVEN a single-item response (no pagination needed)
	resp := ListResponse{Data: []int{1}}

	// WHEN marshaled
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// THEN pagination is omitted (not present as null)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := got["pagination"]; present {
		t.Error("expected pagination to be omitted when zero-valued")
	}
}
