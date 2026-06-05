package contracts

import "testing"

func TestErrorCode_HTTPStatus(t *testing.T) {
	// GIVEN each standard error code
	// WHEN HTTPStatus() is called
	// THEN it returns the matching HTTP status
	tests := []struct {
		code ErrorCode
		want int
	}{
		{CodeValidation, 400},
		{CodeUnauthorized, 401},
		{CodeForbidden, 403},
		{CodeNotFound, 404},
		{CodeConflict, 409},
		{CodeInvalidState, 422},
		{CodeRateLimited, 429},
		{CodeInternal, 500},
	}
	for _, tt := range tests {
		if got := tt.code.HTTPStatus(); got != tt.want {
			t.Errorf("%v.HTTPStatus() = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestErrorCode_UnknownReturns500(t *testing.T) {
	// GIVEN an unrecognized code
	// WHEN HTTPStatus() is called
	// THEN 500 is returned (safe default)
	if got := ErrorCode("BANANA").HTTPStatus(); got != 500 {
		t.Errorf("unknown code status = %d, want 500", got)
	}
}

func TestAPIError_Error(t *testing.T) {
	// GIVEN an APIError
	// WHEN .Error() is called
	// THEN it returns a readable string with the code and message
	e := &APIError{
		Code:    CodeNotFound,
		Message: "device 42 not found",
	}
	if got := e.Error(); got != "NOT_FOUND: device 42 not found" {
		t.Errorf("Error() = %q", got)
	}
}
