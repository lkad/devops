package logs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestService_Query_FillsDefaults: empty time range defaults to
// the last 24h. Limit defaults to 50 when unset.
func TestService_Query_FillsDefaults(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{})
	res, err := svc.Query(context.Background(), Query{Text: "x"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Meta.Backend != "local" {
		t.Errorf("Meta.Backend = %q, want local", res.Meta.Backend)
	}
}

// TestService_Query_RejectsExcessiveRange_K8sScenario: a 60-day
// range against the Loki backend triggers ErrTimeRangeExceeded.
// This is the "K8s logs are limited to 30 days max" hard rule.
func TestService_Query_RejectsExcessiveRange_K8sScenario(t *testing.T) {
	loki := NewLoki(LokiConfig{BaseURL: "http://loki.test:3100", HTTPClient: newFakeHTTPClient()})
	svc := NewService(loki, ServiceConfig{})
	_, err := svc.Query(context.Background(), Query{
		From:  time.Now().Add(-60 * 24 * time.Hour),
		To:    time.Now(),
		Limit: 10,
	})
	if err == nil {
		t.Fatal("expected error for 60d range against Loki")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("err is %T, want *contracts.APIError", err)
	}
	if apiErr.Code != contracts.CodeTimeRangeExceeded {
		t.Errorf("Code = %v, want %v", apiErr.Code, contracts.CodeTimeRangeExceeded)
	}
	if apiErr.Code.HTTPStatus() != 422 {
		t.Errorf("HTTPStatus = %d, want 422", apiErr.Code.HTTPStatus())
	}
}

// TestService_Query_RejectsQueryTooLong: when the text is longer
// than the backend's MaxQueryLength, ErrQueryTooLong is returned.
func TestService_Query_RejectsQueryTooLong(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{
		BackendName:    "local",
		MaxQueryLength: 5,
	}}, ServiceConfig{})
	_, err := svc.Query(context.Background(), Query{Text: "this is way too long"})
	if err == nil {
		t.Fatal("expected error for too-long query")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeQueryTooLong {
		t.Errorf("Code = %v, want %v", err, contracts.CodeQueryTooLong)
	}
}

// TestService_Query_RejectsUnsupportedStructuredQuery: structured
// query (Lucene/LogQL) on Local backend is the spec scenario
// "Lucene query on Local backend" — a 400 UNSUPPORTED_FEATURE.
func TestService_Query_RejectsUnsupportedStructuredQuery(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{
		RejectStructuredQueryOnLocal: true,
	})
	_, err := svc.Query(context.Background(), Query{
		Text:  "level:error AND host:web-*",
		Limit: 10,
	})
	if err == nil {
		t.Fatal("expected error for structured query on Local")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("err is %T, want *contracts.APIError", err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("Code = %v, want VALIDATION_ERROR", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "Local") {
		t.Errorf("Message = %q, want it to mention Local", apiErr.Message)
	}
}

// TestService_Query_AttachesMeta: every result carries a Meta
// block with the backend name. Local backend's meta includes
// the 7d limit.
func TestService_Query_AttachesMeta(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{})
	res, err := svc.Query(context.Background(), Query{Text: "x"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.Meta.Backend != "local" {
		t.Errorf("Meta.Backend = %q, want local", res.Meta.Backend)
	}
	if res.Meta.Degraded {
		t.Errorf("Meta.Degraded = true, want false (universal subset)")
	}
}

// TestService_Query_DegradedWhenLimited: when the Local backend
// is asked for more than its 1000 max page size, the service
// caps the limit and sets Degraded=true.
func TestService_Query_DegradedWhenLimited(t *testing.T) {
	// Backend that always returns 10 entries regardless of the
	// requested limit (so the service-level cap is what bounds
	// the result, and the total > limit triggers degradation).
	b := &capLimitedBackend{max: 10}
	svc := NewService(b, ServiceConfig{MaxPageSize: 5})
	res, err := svc.Query(context.Background(), Query{Limit: 100})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !res.Meta.Degraded {
		t.Errorf("Meta.Degraded = false, want true when limit was capped")
	}
	if res.Meta.Reason == "" {
		t.Errorf("Meta.Reason empty, want a degradation reason")
	}
}

// TestService_Capabilities: the Capabilities of the configured
// backend are returned unchanged.
func TestService_Capabilities(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local", MaxTimeRange: 7 * 24 * time.Hour}}, ServiceConfig{})
	c := svc.Capabilities()
	if c.BackendName != "local" {
		t.Errorf("BackendName = %q, want local", c.BackendName)
	}
}

// TestService_Streams: streams come from the configured backend.
func TestService_Streams(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{})
	streams, err := svc.Streams(context.Background())
	if err != nil {
		t.Fatalf("Streams: %v", err)
	}
	if len(streams) != 1 {
		t.Errorf("len(streams) = %d, want 1", len(streams))
	}
}

// capLimitedBackend truncates Query results to `max` rows so we
// can exercise the degradation path.
type capLimitedBackend struct {
	max int
	caps Capabilities
}

func (b *capLimitedBackend) Capabilities() Capabilities {
	if b.caps.BackendName == "" {
		b.caps.BackendName = "test"
	}
	return b.caps
}
func (b *capLimitedBackend) Query(_ context.Context, q Query) (Result, error) {
	rows := make([]LogEntry, b.max)
	for i := range rows {
		rows[i] = LogEntry{ID: "row", Message: "x"}
	}
	return Result{Entries: rows, Total: int64(b.max), Meta: Meta{Backend: b.caps.BackendName}}, nil
}
func (b *capLimitedBackend) Streams(_ context.Context) ([]Stream, error) {
	return []Stream{{Name: "test"}}, nil
}

// TestService_Streams_CrossTenant_Denied covers the v0.3.0.0
// P0 #2 cross-tenant enforcement: a non-SuperAdmin caller
// is denied access.
func TestService_Streams_CrossTenant_Denied(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{})
	cl := caller.New(&contracts.User{ID: "alice", Username: "alice", Role: contracts.RoleDeveloper})
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.StreamsWithCaller(ctx)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

// TestService_Streams_SuperAdmin_Bypasses covers the spec
// rule that SuperAdmin is implicitly allowed.
func TestService_Streams_SuperAdmin_Bypasses(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{})
	cl := caller.New(&contracts.User{ID: "root", Username: "root", Role: contracts.RoleSuperAdmin})
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.StreamsWithCaller(ctx)
	if errors.Is(err, ErrForbidden) {
		t.Errorf("SuperAdmin should bypass cross-tenant, got ErrForbidden")
	}
}

// TestService_Streams_NilCaller_401 covers the fail-closed
// rule: a context without a caller MUST surface as
// ErrUnauthenticated.
func TestService_Streams_NilCaller_401(t *testing.T) {
	svc := NewService(&fakeBackend{caps: Capabilities{BackendName: "local"}}, ServiceConfig{})
	_, err := svc.StreamsWithCaller(context.Background())
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}
