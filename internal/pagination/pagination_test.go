package pagination

import (
	"net/http/httptest"
	"testing"
)

func TestParseParams(t *testing.T) {
	tests := []struct {
		url        string
		wantLimit  int
		wantOffset int
	}{
		{"/", 50, 0},
		{"/?limit=10", 10, 0},
		{"/?offset=20", 50, 20},
		{"/?limit=25&offset=50", 25, 50},
		{"/?limit=-1", 50, 0},
		{"/?offset=-5", 50, 0},
		{"/?limit=200", 50, 0},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.url, nil)
		limit, offset := ParseParams(req)
		if limit != tt.wantLimit {
			t.Errorf("ParseParams(%s): limit = %d, want %d", tt.url, limit, tt.wantLimit)
		}
		if offset != tt.wantOffset {
			t.Errorf("ParseParams(%s): offset = %d, want %d", tt.url, offset, tt.wantOffset)
		}
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		total    int
		limit    int
		offset   int
		wantMore bool
	}{
		{
			name:     "has more items",
			data:     []string{"a", "b", "c"},
			total:    100,
			limit:    10,
			offset:   0,
			wantMore: true,
		},
		{
			name:     "no more items - at end",
			data:     []string{"a", "b", "c"},
			total:    3,
			limit:    10,
			offset:   0,
			wantMore: false,
		},
		{
			name:     "empty data",
			data:     []string{},
			total:    100,
			limit:    10,
			offset:   0,
			wantMore: false,
		},
		{
			name:     "nil data",
			data:     nil,
			total:    100,
			limit:    10,
			offset:   0,
			wantMore: false,
		},
		{
			name:     "full page with more",
			data:     []string{"a", "b", "c", "d", "e"},
			total:    10,
			limit:    5,
			offset:   0,
			wantMore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewPaginatedResponse(tt.data, tt.total, tt.limit, tt.offset)
			if resp.Pagination.Total != tt.total {
				t.Errorf("expected total %d, got %d", tt.total, resp.Pagination.Total)
			}
			if resp.Pagination.Limit != tt.limit {
				t.Errorf("expected limit %d, got %d", tt.limit, resp.Pagination.Limit)
			}
			if resp.Pagination.Offset != tt.offset {
				t.Errorf("expected offset %d, got %d", tt.offset, resp.Pagination.Offset)
			}
			if resp.Pagination.HasMore != tt.wantMore {
				t.Errorf("expected hasMore %v, got %v", tt.wantMore, resp.Pagination.HasMore)
			}
		})
	}
}

func TestGetSliceLen(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		wantLen  int
	}{
		{"nil", nil, 0},
		{"empty slice", []string{}, 0},
		{"string slice", []string{"a", "b", "c"}, 3},
		{"int slice", []int{1, 2, 3, 4}, 4},
		{"struct slice", []struct{ Name string }{{Name: "a"}, {Name: "b"}}, 2},
		{"non-slice", "not a slice", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSliceLen(tt.data)
			if got != tt.wantLen {
				t.Errorf("getSliceLen(%v) = %d, want %d", tt.data, got, tt.wantLen)
			}
		})
	}
}
