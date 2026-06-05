// Package handler contains the cross-cutting HTTP response helpers.
// Per the api-contract spec, every endpoint renders either a
// contracts.ListResponse / single resource on success or a
// contracts.ErrorResponse on failure; this package is the only place
// that knows how to write them onto a net/http.ResponseWriter.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// WriteJSON serialises payload as JSON, sets the Content-Type header,
// and writes the supplied status. The payload is any envelope type
// from pkg/contracts; callers are responsible for shaping it.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError renders a contracts.APIError as an ErrorResponse with
// the matching HTTP status. A nil error falls back to a 500
// INTERNAL_ERROR so we never leak an empty body to the client.
func WriteError(w http.ResponseWriter, err *contracts.APIError) {
	if err == nil {
		err = &contracts.APIError{Code: contracts.CodeInternal, Message: "internal server error"}
	}
	body := contracts.ErrorResponse{
		Error: contracts.ErrorBody{
			Code:    err.Code,
			Message: err.Message,
			Details: err.Details,
		},
	}
	WriteJSON(w, err.Code.HTTPStatus(), body)
}

// WriteList is a convenience wrapper for the common list endpoint
// pattern: data + pagination envelope at status 200.
func WriteList(w http.ResponseWriter, data any, p *contracts.Pagination) {
	WriteJSON(w, http.StatusOK, contracts.ListResponse{Data: data, Pagination: p})
}

// WriteCreated is the standard 201 Created response. Callers are
// expected to set the Location header before calling if the client
// needs a follow-up URL.
func WriteCreated(w http.ResponseWriter, resource any) {
	WriteJSON(w, http.StatusCreated, resource)
}

// WriteNoContent renders the standard 204 No Content.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
