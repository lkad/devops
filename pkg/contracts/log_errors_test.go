package contracts

import "testing"

// TestErrorCode_LogAggregationStatus covers the new codes introduced
// for the log-aggregation spec: TIME_RANGE_EXCEEDED, QUERY_TIMEOUT,
// QUERY_TOO_LONG, BACKEND_UNAVAILABLE. These must map to the HTTP
// statuses called out in docs/LOG-QUERY-API.md.
func TestErrorCode_LogAggregationStatus(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want int
	}{
		{CodeBackendUnavailable, 503},
		{CodeQueryTimeout, 504},
		{CodeQueryTooLong, 400},
		{CodeTimeRangeExceeded, 422},
	}
	for _, tt := range tests {
		if got := tt.code.HTTPStatus(); got != tt.want {
			t.Errorf("%v.HTTPStatus() = %d, want %d", tt.code, got, tt.want)
		}
	}
}

// TestErrorCode_LogAggregationValues asserts the wire-format string
// values do not change accidentally. Clients branch on these strings.
func TestErrorCode_LogAggregationValues(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{CodeBackendUnavailable, "BACKEND_UNAVAILABLE"},
		{CodeQueryTimeout, "QUERY_TIMEOUT"},
		{CodeQueryTooLong, "QUERY_TOO_LONG"},
		{CodeTimeRangeExceeded, "TIME_RANGE_EXCEEDED"},
	}
	for _, tt := range tests {
		if string(tt.code) != tt.want {
			t.Errorf("ErrorCode = %q, want %q", string(tt.code), tt.want)
		}
	}
}
