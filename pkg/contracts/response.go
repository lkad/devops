// Package contracts defines the cross-module shared types used throughout
// the backend: response envelopes, error codes, pagination, and base models.
// Every module imports this package; nothing in here imports a module.
package contracts

import "encoding/json"

// ListResponse is the standard envelope returned by every list endpoint.
// When Pagination is nil, it is omitted from the JSON output so
// single-resource and empty responses stay clean.
type ListResponse struct {
	Data       any         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// ErrorResponse is the standard envelope returned for every non-2xx response.
// It is intentionally distinct from ListResponse so handlers cannot
// accidentally mix success and error shapes.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the body of an ErrorResponse. Details is optional and
// carries field-level validation errors or context for diagnostics.
type ErrorBody struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// MarshalJSON for ErrorResponse is overridden only to guarantee the
// Details key is omitted when empty (not rendered as null) — this
// matches the rest of the package's omit-when-zero convention.
func (e ErrorResponse) MarshalJSON() ([]byte, error) {
	type alias ErrorResponse
	return json.Marshal(alias(e))
}
