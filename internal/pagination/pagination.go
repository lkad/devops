package pagination

import (
	"net/http"
	"reflect"
	"strconv"
)

// Pagination holds pagination metadata per API spec
type Pagination struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// PaginatedResponse wraps a list response with pagination per API spec
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// ParseParams extracts limit and offset from query params
// Default limit is 50, max limit is 100
func ParseParams(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// getSliceLen returns the length of any slice via reflection
func getSliceLen(data interface{}) int {
	if data == nil {
		return 0
	}
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice {
		return 0
	}
	return v.Len()
}

// NewPaginatedResponse creates a paginated response per API spec
func NewPaginatedResponse(data interface{}, total, limit, offset int) *PaginatedResponse {
	dataLen := getSliceLen(data)
	hasMore := offset+dataLen < total
	if dataLen == 0 {
		hasMore = false
	}
	return &PaginatedResponse{
		Data: data,
		Pagination: Pagination{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: hasMore,
		},
	}
}