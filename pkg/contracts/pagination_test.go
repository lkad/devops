package contracts

import "testing"

func TestPagination_StartIndex(t *testing.T) {
	// GIVEN page 3, pageSize 20
	// WHEN StartIndex() is computed
	p := NewPagination(3, 20)

	// THEN start index = (3 - 1) * 20 = 40
	if got := p.StartIndex(); got != 40 {
		t.Errorf("StartIndex() = %d, want 40", got)
	}
}

func TestPagination_MoreAvailable(t *testing.T) {
	// GIVEN total=100, limit=20, offset=0
	// WHEN MoreAvailable is computed
	// THEN there are more pages (0+20=20 < 100)
	p := Pagination{Total: 100, Limit: 20, Offset: 0}
	if !p.MoreAvailable() {
		t.Error("MoreAvailable should be true (0+20=20 < 100)")
	}

	// GIVEN offset=80
	// WHEN we are on the last page
	// THEN MoreAvailable is false (80+20=100, not strictly less)
	p = Pagination{Total: 100, Limit: 20, Offset: 80}
	if p.MoreAvailable() {
		t.Error("MoreAvailable should be false (80+20=100, not strictly less)")
	}
}

func TestNewPagination_Defaults(t *testing.T) {
	// GIVEN a page request with invalid values
	// WHEN NewPagination is called
	// THEN defaults are applied (page >= 1, pageSize in [1, 200])
	tests := []struct {
		page, pageSize int
		wantPage       int
		wantSize       int
	}{
		{0, 0, 1, 20},    // both invalid → defaults
		{-1, -5, 1, 20},  // negatives → defaults
		{2, 500, 2, 200}, // pageSize too large → capped at 200
	}
	for _, tt := range tests {
		p := NewPagination(tt.page, tt.pageSize)
		if p.Limit != tt.wantSize {
			t.Errorf("page=%d size=%d: Limit=%d, want %d", tt.page, tt.pageSize, p.Limit, tt.wantSize)
		}
		if p.StartIndex() != (tt.wantPage-1)*tt.wantSize {
			t.Errorf("page=%d size=%d: StartIndex=%d, want %d", tt.page, tt.pageSize, p.StartIndex(), (tt.wantPage-1)*tt.wantSize)
		}
	}
}
