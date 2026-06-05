package logs

import (
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrBackendUnavailable is the spec-mandated 503 for "ES or Loki
// unreachable". The cause is wrapped so log lines retain the
// underlying error; the wire envelope exposes only the structured
// code and a stable human message.
func ErrBackendUnavailable(name string, cause error) *contracts.APIError {
	return &contracts.APIError{
		Code:    contracts.CodeBackendUnavailable,
		Message: "log storage backend is unavailable",
		Details: map[string]any{"backend_name": name},
		Cause:   cause,
	}
}

// ErrQueryTimeout maps a backend-side timeout to the spec's 504
// QUERY_TIMEOUT. The `seconds` argument surfaces the timeout value
// in the error details so a client can adapt its retry strategy.
func ErrQueryTimeout(seconds int) *contracts.APIError {
	return &contracts.APIError{
		Code:    contracts.CodeQueryTimeout,
		Message: "log query timed out",
		Details: map[string]any{"timeout_seconds": seconds},
	}
}

// ErrQueryTooLong is a 400 — the search text exceeded the backend's
// MaxQueryLength. We do not silently truncate.
func ErrQueryTooLong(got, max int) *contracts.APIError {
	return &contracts.APIError{
		Code:    contracts.CodeQueryTooLong,
		Message: "query text exceeds backend maximum",
		Details: map[string]any{"got": got, "max": max},
	}
}

// ErrTimeRangeExceeded is the K8s/Loki 30-day hard-reject (422).
// Per the task brief, the wire status is 422 even though the
// LOG-QUERY-API.md error table lists 400 — the task brief is
// authoritative for the Loki-limit scenario.
func ErrTimeRangeExceeded(requestedHours, maxHours int) *contracts.APIError {
	return &contracts.APIError{
		Code:    contracts.CodeTimeRangeExceeded,
		Message: "time range exceeds backend maximum",
		Details: map[string]any{
			"requested_hours": requestedHours,
			"max_hours":       maxHours,
		},
	}
}
