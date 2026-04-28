package response

import (
	"encoding/json"
	"net/http"

	"github.com/devops-toolkit/pkg/errors"
)

type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type ListResponse struct {
	Items      any   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	HasMore    bool  `json:"hasMore"`
}

func Success(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func Error(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*errors.AppError); ok {
		JSON(w, appErr.HTTPStatus(), Response{
			Success: false,
			Error: &ErrorDetail{
				Code:    string(appErr.Code),
				Message: appErr.Message,
				Details: appErr.Details,
			},
		})
		return
	}

	JSON(w, http.StatusInternalServerError, Response{
		Success: false,
		Error: &ErrorDetail{
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
		},
	})
}

func List(w http.ResponseWriter, items any, total int64, page, pageSize int) {
	hasMore := int64(page*pageSize) < total
	JSON(w, http.StatusOK, Response{
		Success: true,
		Data: ListResponse{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  hasMore,
		},
	})
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
