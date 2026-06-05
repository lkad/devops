package contracts

const (
	defaultPageSize = 20
	maxPageSize     = 200
)

// Pagination is the standard pagination block embedded in ListResponse.
// Limit and Offset are the query-side view; Total and HasMore are the
// response-side view (filled in by the repository after counting).
type Pagination struct {
	Total   int64 `json:"total"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	HasMore bool  `json:"has_more"`
}

// StartIndex returns the starting index of the first row on the page.
// Kept as a method (instead of just reading Offset) so handlers don't
// reinvent the offset math; field and method names cannot share a name.
func (p Pagination) StartIndex() int {
	return p.Offset
}

// MoreAvailable reports whether another page exists after this one.
// True when Total exceeds the highest index returned so far.
func (p Pagination) MoreAvailable() bool {
	return p.Offset+p.Limit < int(p.Total)
}

// NewPagination builds a Pagination from a (page, pageSize) request,
// clamping page to >= 1 and pageSize to [1, maxPageSize].
// The total count and HasMore are filled in by the caller after listing.
func NewPagination(page, pageSize int) Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return Pagination{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}
}
