package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Handler is the HTTP layer for the metrics subsystem. It is
// intentionally thin: parse, call service, render. All
// validation, orchestration, and persistence live in the
// service / repository.
type Handler struct {
	svc *Service
}

// NewHandler builds a Handler. The Service is the only
// dependency; future middleware (RBAC, audit logging) can
// be injected here without changing the service contract.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register attaches the metrics routes to the supplied
// router group. The group is expected to live under
// /api/v1; the handler is agnostic about the surrounding
// stack.
//
// Routes:
//
//	GET    /metrics                  list (filters: name, target_type, target_id, from, to, limit)
//	GET    /metrics/series           list unique series
//	GET    /metrics/series/:name     single series (requires target_type + target_id)
//	POST   /metrics                  ingest a single Metric
//
// perms is the per-route permission factory; pass a no-op
// factory in unit tests that don't exercise auth.
func (h *Handler) Register(r *gin.RouterGroup, perms func(rbac.Permission) gin.HandlerFunc) {
	viewP := perms(rbac.PermissionViewMetrics)
	writeP := perms(rbac.PermissionWriteMetrics)
	r.GET("/metrics", viewP, h.List)
	r.POST("/metrics", writeP, h.Create)
	r.GET("/metrics/series", viewP, h.ListSeries)
	r.GET("/metrics/series/:name", viewP, h.GetSeries)
}

// metricRequest is the wire shape for POST /metrics. We keep
// it as a flat struct and map it onto IngestInput inside the
// handler so the service stays free of Gin / JSON tags.
type metricRequest struct {
	Name       string         `json:"name"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Value      float64        `json:"value"`
	Timestamp  string         `json:"timestamp"`
	Labels     map[string]any `json:"labels"`
}

func (r metricRequest) toIngest() (IngestInput, *contracts.APIError) {
	in := IngestInput{
		Name:       r.Name,
		TargetType: r.TargetType,
		TargetID:   r.TargetID,
		Value:      r.Value,
	}
	if r.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339Nano, r.Timestamp)
		if err != nil {
			// Tolerate RFC3339 (seconds) too — some
			// clients omit the nanosecond fraction.
			ts, err = time.Parse(time.RFC3339, r.Timestamp)
			if err != nil {
				return in, &contracts.APIError{
					Code:    contracts.CodeValidation,
					Message: "timestamp must be RFC3339",
				}
			}
		}
		in.Timestamp = ts
	}
	if r.Labels != nil {
		in.Labels = JSONMap(r.Labels)
	}
	return in, nil
}

// List handles GET /metrics. It honours the name /
// target_type / target_id / from / to / limit query
// parameters and renders the standard ListResponse
// envelope.
func (h *Handler) List(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		handler.WriteError(c.Writer, err)
		return
	}
	rows, total, svcErr := h.svc.List(filter)
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	page := contracts.Pagination{
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: filter.Limit > 0 && filter.Offset+filter.Limit < int(total),
	}
	handler.WriteList(c.Writer, rows, &page)
}

// Create handles POST /metrics. A 201 is returned with the
// freshly-stored metric.
func (h *Handler) Create(c *gin.Context) {
	var req metricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "request body must be JSON",
		})
		return
	}
	in, apiErr := req.toIngest()
	if apiErr != nil {
		handler.WriteError(c.Writer, apiErr)
		return
	}
	m, svcErr := h.svc.Ingest(in)
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteCreated(c.Writer, m)
}

// ListSeries handles GET /metrics/series.
func (h *Handler) ListSeries(c *gin.Context) {
	filter, err := parseListFilter(c)
	if err != nil {
		handler.WriteError(c.Writer, err)
		return
	}
	series, svcErr := h.svc.ListSeries(filter)
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteList(c.Writer, series, &contracts.Pagination{
		Total:   int64(len(series)),
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: false,
	})
}

// GetSeries handles GET /metrics/series/:name. The
// remaining key (target_type, target_id) is taken from the
// query string.
func (h *Handler) GetSeries(c *gin.Context) {
	name := c.Param("name")
	targetType := c.Query("target_type")
	targetID := c.Query("target_id")
	if targetType == "" || targetID == "" {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "target_type and target_id are required",
		})
		return
	}
	filter, err := parseListFilter(c)
	if err != nil {
		handler.WriteError(c.Writer, err)
		return
	}
	series, svcErr := h.svc.GetSeries(SeriesKey{
		Name:       name,
		TargetType: targetType,
		TargetID:   targetID,
	}, filter)
	if svcErr != nil {
		handler.WriteAPIError(c.Writer, svcErr)
		return
	}
	handler.WriteJSON(c.Writer, http.StatusOK, series)
}

// parseListFilter turns the query string into a ListFilter.
// Any error here is a 400 — bad limit/offset, unparseable
// timestamp, etc.
func parseListFilter(c *gin.Context) (ListFilter, *contracts.APIError) {
	f := ListFilter{
		Name:       c.Query("name"),
		TargetType: c.Query("target_type"),
		TargetID:   c.Query("target_id"),
	}
	if v := c.Query("from"); v != "" {
		ts, err := parseTime(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "from must be an RFC3339 timestamp",
			}
		}
		f.From = ts
	}
	if v := c.Query("to"); v != "" {
		ts, err := parseTime(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "to must be an RFC3339 timestamp",
			}
		}
		f.To = ts
	}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "limit must be an integer",
			}
		}
		f.Limit = n
	} else {
		f.Limit = 20
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "offset must be an integer",
			}
		}
		f.Offset = n
	}
	return f, nil
}

// parseTime accepts both RFC3339 and RFC3339Nano. The wire
// format is RFC3339Nano; some clients (curl, simple
// JavaScript Date.toISOString) emit only the seconds form.
func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// writeAPIError is a small adapter so we can pass an `error`
// returned from the service directly to the handler's
// WriteError, which expects a *contracts.APIError.
