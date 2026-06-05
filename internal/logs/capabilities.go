package logs

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// capabilitiesResponse is the wire shape of the /capabilities
// endpoint. The "version" field is a constant for now — the
// schema spec pins it at 1.0; future revisions go in version
// bumps not field renames.
type capabilitiesResponse struct {
	Backend      string                  `json:"backend"`
	Version      string                  `json:"version"`
	Capabilities capabilitiesPayload     `json:"capabilities"`
	Limits       map[string]any          `json:"limits,omitempty"`
}

type capabilitiesPayload struct {
	SupportsAggregation bool   `json:"supports_aggregation"`
	MaxTimeRange        string `json:"max_time_range"`
	MaxQueryLength      int    `json:"max_query_length"`
	BackendName         string `json:"backend_name"`
}

// CapabilitiesHandler renders the /capabilities endpoint. It is
// the only endpoint in this module that never returns 5xx — per
// the spec scenario "Capabilities query never fails".
type CapabilitiesHandler struct {
	svc *Service
}

// NewCapabilitiesHandler builds the handler.
func NewCapabilitiesHandler(svc *Service) *CapabilitiesHandler {
	return &CapabilitiesHandler{svc: svc}
}

// Get handles GET /api/v1/logs/capabilities.
func (h *CapabilitiesHandler) Get(c *gin.Context) {
	caps := h.svc.Capabilities()
	resp := capabilitiesResponse{
		Backend: caps.BackendName,
		Version: "1.0.0",
		Capabilities: capabilitiesPayload{
			SupportsAggregation: caps.SupportsAggregation,
			MaxTimeRange:        caps.MaxTimeRange.String(),
			MaxQueryLength:      caps.MaxQueryLength,
			BackendName:         caps.BackendName,
		},
	}
	// 200 even on degraded / unavailable — the spec requires
	// the endpoint to never fail.
	if caps.BackendName == "unavailable" {
		resp.Capabilities.SupportsAggregation = false
		resp.Capabilities.MaxQueryLength = 0
	}
	c.JSON(http.StatusOK, resp)
}
