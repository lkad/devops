package logs

import (
	"errors"
	"testing"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestErrBackendUnavailable_Status is the spec scenario "Backend
// unavailable": 503 with the BACKEND_UNAVAILABLE code.
func TestErrBackendUnavailable_Status(t *testing.T) {
	e := ErrBackendUnavailable("elasticsearch", errors.New("dial tcp: i/o timeout"))
	if e.Code != contracts.CodeBackendUnavailable {
		t.Errorf("Code = %v, want %v", e.Code, contracts.CodeBackendUnavailable)
	}
	if e.Code.HTTPStatus() != 503 {
		t.Errorf("HTTPStatus = %d, want 503", e.Code.HTTPStatus())
	}
	if e.Details["backend_name"] != "elasticsearch" {
		t.Errorf("Details missing backend_name: %+v", e.Details)
	}
}

// TestErrQueryTimeout_Status matches the spec table: 504 with
// QUERY_TIMEOUT.
func TestErrQueryTimeout_Status(t *testing.T) {
	e := ErrQueryTimeout(5)
	if e.Code != contracts.CodeQueryTimeout {
		t.Errorf("Code = %v, want %v", e.Code, contracts.CodeQueryTimeout)
	}
	if e.Code.HTTPStatus() != 504 {
		t.Errorf("HTTPStatus = %d, want 504", e.Code.HTTPStatus())
	}
}

// TestErrQueryTooLong_Status: 400 with QUERY_TOO_LONG.
func TestErrQueryTooLong_Status(t *testing.T) {
	e := ErrQueryTooLong(7000, 5000)
	if e.Code != contracts.CodeQueryTooLong {
		t.Errorf("Code = %v, want %v", e.Code, contracts.CodeQueryTooLong)
	}
	if e.Code.HTTPStatus() != 400 {
		t.Errorf("HTTPStatus = %d, want 400", e.Code.HTTPStatus())
	}
}

// TestErrTimeRangeExceeded_Status: per docs/LOG-QUERY-API.md §7.2 the
// table shows 400, but the task brief mandates 422 for the 30-day
// hard-reject. Both are documented — the spec uses 400 in the table
// and 422 in the explicit Loki 30-day scenario; the task brief picks
// 422 for the K8s/Loki case. We follow the task brief: TIME_RANGE_EXCEEDED
// is mapped to 422 (same status as the existing CodeInvalidState, so
// the client can distinguish via the error code, not the status).
func TestErrTimeRangeExceeded_Status(t *testing.T) {
	e := ErrTimeRangeExceeded(60*24, 30*24)
	if e.Code != contracts.CodeTimeRangeExceeded {
		t.Errorf("Code = %v, want %v", e.Code, contracts.CodeTimeRangeExceeded)
	}
	if e.Code.HTTPStatus() != 422 {
		t.Errorf("HTTPStatus = %d, want 422", e.Code.HTTPStatus())
	}
	if e.Details["max_hours"] != 24*30 {
		t.Errorf("Details: %+v", e.Details)
	}
}
