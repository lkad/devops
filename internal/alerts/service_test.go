package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newServiceWithDB stands up a Service backed by an in-memory
// sqlite db and a FakeDispatcher so test assertions can be made
// on the recorded payloads.
func newServiceWithDB(t *testing.T) (*Service, *Repository, *FakeDispatcher, *FakeSuppressionChecker) {
	t.Helper()
	repo := NewRepository(openTestDB(t))
	disp := NewFakeDispatcher()
	supp := NewFakeSuppressionChecker()
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Dispatcher:  disp,
		Suppression: supp,
		Logger:      slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
	})
	return svc, repo, disp, supp
}

// TestService_CreateAlert covers the spec scenario "Trigger
// Alert API" and the underlying CRUD path: a well-formed alert
// is persisted and dispatched.
func TestService_CreateAlert(t *testing.T) {
	svc, _, disp, _ := newServiceWithDB(t)
	a, err := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name:       "high_cpu",
		Severity:   SeverityWarning,
		SourceType: SourceTypeCustom,
		SourceID:   "src-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == "" {
		t.Error("ID not assigned")
	}
	if a.State != StateFiring {
		t.Errorf("default state: got %q want firing", a.State)
	}
	if len(disp.Calls) != 1 {
		t.Errorf("dispatcher not called: %d", len(disp.Calls))
	}
}

// TestService_Fire_DispatchesByDefault is the spec's
// "Within rate limit" scenario: a fired alert is dispatched
// immediately to the configured channel. The audit-log
// dispatcher is also called once, so the total is 2.
func TestService_Fire_DispatchesByDefault(t *testing.T) {
	svc, _, disp, _ := newServiceWithDB(t)
	// Seed a channel.
	ch, err := svc.CreateChannel(context.Background(), CreateChannelInput{
		Type:    ChannelTypeSlack,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	in := FireInput{
		Name:       "x",
		Severity:   SeverityInfo,
		SourceType: SourceTypeCustom,
		SourceID:   "s",
		ChannelIDs: []string{ch.ID},
	}
	if _, err := svc.Fire(context.Background(), in); err != nil {
		t.Fatalf("fire: %v", err)
	}
	// Expect 2: the audit-log dispatch plus the configured
	// slack channel.
	if len(disp.Calls) != 2 {
		t.Errorf("expected 2 dispatches, got %d", len(disp.Calls))
	}
	// The configured channel should be among them.
	foundSlack := false
	for _, c := range disp.Calls {
		if c.Channel.ID == ch.ID {
			foundSlack = true
		}
	}
	if !foundSlack {
		t.Errorf("expected slack channel in dispatches: %+v", disp.Calls)
	}
}

// TestService_Fire_SuppressedByMaintenance covers the spec's
// "Suppress external notification" scenario: an alert whose
// source is a physical host in maintenance must NOT be
// dispatched to external channels, but the alert row is still
// recorded with Suppressed=true and a reason. The audit-log
// dispatch and any configured log channel both still fire.
func TestService_Fire_SuppressedByMaintenance(t *testing.T) {
	svc, _, disp, supp := newServiceWithDB(t)
	supp.SetMaintenance("host-1", true)
	// Seed one external + one log channel.
	ext, _ := svc.CreateChannel(context.Background(), CreateChannelInput{Type: ChannelTypeSlack, Enabled: true})
	log, _ := svc.CreateChannel(context.Background(), CreateChannelInput{Type: ChannelTypeLog, Enabled: true})
	in := FireInput{
		Name:       "x",
		Severity:   SeverityCritical,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "host-1",
		ChannelIDs: []string{ext.ID, log.ID},
	}
	a, err := svc.Fire(context.Background(), in)
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if !a.Suppressed {
		t.Errorf("expected Suppressed=true on alert")
	}
	// Expected dispatches: audit log + configured log channel.
	// The slack channel is suppressed.
	if len(disp.Calls) != 2 {
		t.Fatalf("expected 2 dispatches (audit + log), got %d", len(disp.Calls))
	}
	for _, c := range disp.Calls {
		if c.Channel.Type == ChannelTypeSlack {
			t.Errorf("slack channel should be suppressed: %+v", c)
		}
	}
}

// TestService_Fire_NotSuppressedForNonPhysicalHost covers the
// "Non-physical-host alerts not affected" scenario: a pipeline
// or log-source alert is dispatched normally even if some other
// host is in maintenance. Expect 2 dispatches: audit log + the
// configured slack channel.
func TestService_Fire_NotSuppressedForNonPhysicalHost(t *testing.T) {
	svc, _, disp, supp := newServiceWithDB(t)
	supp.SetMaintenance("host-1", true) // unrelated host
	ch, _ := svc.CreateChannel(context.Background(), CreateChannelInput{Type: ChannelTypeSlack, Enabled: true})
	_, err := svc.Fire(context.Background(), FireInput{
		Name:       "pipeline_fail",
		Severity:   SeverityCritical,
		SourceType: SourceTypePipeline,
		SourceID:   "pipe-1",
		ChannelIDs: []string{ch.ID},
	})
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if len(disp.Calls) != 2 {
		t.Errorf("expected 2 dispatches, got %d", len(disp.Calls))
	}
	foundSlack := false
	for _, c := range disp.Calls {
		if c.Channel.Type == ChannelTypeSlack {
			foundSlack = true
		}
	}
	if !foundSlack {
		t.Errorf("expected slack dispatch: %+v", disp.Calls)
	}
}

// TestService_Fire_AlertResumesAfterMaintenanceExit covers the
// "Alert resumes after maintenance exit" scenario: once the
// host leaves maintenance, subsequent alerts for that host are
// dispatched normally. While in maintenance, only the audit
// log is dispatched; after exit, the audit log + the configured
// slack channel are dispatched.
func TestService_Fire_AlertResumesAfterMaintenanceExit(t *testing.T) {
	svc, _, disp, supp := newServiceWithDB(t)
	supp.SetMaintenance("host-1", true)
	ch, _ := svc.CreateChannel(context.Background(), CreateChannelInput{Type: ChannelTypeSlack, Enabled: true})
	// While in maintenance: only the audit log is dispatched.
	_, _ = svc.Fire(context.Background(), FireInput{
		Name: "x", Severity: SeverityWarning,
		SourceType: SourceTypePhysicalHost, SourceID: "host-1",
		ChannelIDs: []string{ch.ID},
	})
	if len(disp.Calls) != 1 {
		t.Fatalf("expected 1 dispatch (audit log) while in maintenance, got %d", len(disp.Calls))
	}
	// Exit maintenance.
	supp.SetMaintenance("host-1", false)
	_, _ = svc.Fire(context.Background(), FireInput{
		Name: "x", Severity: SeverityWarning,
		SourceType: SourceTypePhysicalHost, SourceID: "host-1",
		ChannelIDs: []string{ch.ID},
	})
	if len(disp.Calls) != 3 {
		t.Errorf("expected 3 dispatches after exit (1 prior + 2 new), got %d", len(disp.Calls))
	}
	// Verify the last two dispatches include the slack channel.
	last := disp.Calls[len(disp.Calls)-1]
	if last.Channel.Type != ChannelTypeSlack {
		t.Errorf("last dispatch: got %q want slack", last.Channel.Type)
	}
}

// TestService_Acknowledge covers the POST /alerts/:id/acknowledge
// scenario: the alert's State flips to acknowledged, the
// AcknowledgedBy / AcknowledgedAt fields are set, and the
// caller is recorded (a header-driven user id is used in tests
// because the auth middleware is not in the path).
func TestService_Acknowledge(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "x", Severity: SeverityInfo,
		SourceType: SourceTypeCustom, SourceID: "s",
	})
	acked, err := svc.Acknowledge(context.Background(), a.ID, "alice")
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	if acked.State != StateAcknowledged {
		t.Errorf("state: got %q", acked.State)
	}
	if acked.AcknowledgedBy == nil || *acked.AcknowledgedBy != "alice" {
		t.Errorf("ack by: %+v", acked.AcknowledgedBy)
	}
	if acked.AcknowledgedAt == nil {
		t.Errorf("ack at: nil")
	}
}

// TestService_Resolve covers the POST /alerts/:id/resolve
// scenario: the alert's State flips to resolved and ResolvedAt
// is set.
func TestService_Resolve(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "x", Severity: SeverityInfo,
		SourceType: SourceTypeCustom, SourceID: "s",
	})
	resolved, err := svc.Resolve(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.State != StateResolved {
		t.Errorf("state: got %q", resolved.State)
	}
	if resolved.ResolvedAt == nil {
		t.Errorf("resolved at: nil")
	}
}

// TestService_List covers the GET /alerts scenario: every
// persisted alert is returned in insertion order (or by created
// timestamp).
func TestService_List(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	for i := 0; i < 3; i++ {
		_, _ = svc.CreateAlert(context.Background(), CreateAlertInput{
			Name: "x", Severity: SeverityInfo,
			SourceType: SourceTypeCustom, SourceID: "s",
		})
	}
	rows, total, err := svc.List(AlertFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Errorf("list: total=%d rows=%d", total, len(rows))
	}
}

// TestService_Stats covers the spec's "Get alert stats" scenario:
// the service can compute a small aggregate view used by
// GET /alerts/stats.
func TestService_Stats(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	for _, sev := range []Severity{SeverityInfo, SeverityInfo, SeverityCritical} {
		_, _ = svc.CreateAlert(context.Background(), CreateAlertInput{
			Name: "x", Severity: sev,
			SourceType: SourceTypeCustom, SourceID: "s",
		})
	}
	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("total: got %d want 3", stats.Total)
	}
	if stats.BySeverity[SeverityInfo] != 2 {
		t.Errorf("by-severity info: got %d want 2", stats.BySeverity[SeverityInfo])
	}
	if stats.BySeverity[SeverityCritical] != 1 {
		t.Errorf("by-severity critical: got %d want 1", stats.BySeverity[SeverityCritical])
	}
}

// TestService_ValidateAlert covers the field-level validation
// rules: severity / state / source_type must be from their
// allowed sets.
func TestService_ValidateAlert(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	_, err := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name:       "x",
		Severity:   Severity("bogus"),
		SourceType: SourceTypeCustom,
		SourceID:   "s",
	})
	if err == nil {
		t.Fatal("expected validation error for bad severity")
	}
	if !IsValidation(err) {
		t.Errorf("expected VALIDATION_ERROR, got %v", err)
	}
}

// TestService_Channel_CRUD covers the channel service CRUD
// methods: create / list / update / delete.
func TestService_Channel_CRUD(t *testing.T) {
	svc, _, _, _ := newServiceWithDB(t)
	c, err := svc.CreateChannel(context.Background(), CreateChannelInput{Type: ChannelTypeEmail, Enabled: true, Config: JSONMap{"recipients": []any{"a@x"}}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == "" {
		t.Error("ID not assigned")
	}
	rows, err := svc.ListChannels()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("list: got %d", len(rows))
	}
	enabled := false
	if err := svc.UpdateChannel(context.Background(), c.ID, UpdateChannelInput{Enabled: &enabled}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := svc.DeleteChannel(context.Background(), c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rows, _ = svc.ListChannels()
	if len(rows) != 0 {
		t.Errorf("delete failed: still %d rows", len(rows))
	}
}

// newHandlerRouter returns a Gin engine wired with the alerts
// handler in isolation. The user id is taken from the
// X-User-Id header to keep the tests free of auth middleware.
func newHandlerRouter(t *testing.T) (*gin.Engine, *Service, *FakeDispatcher) {
	t.Helper()
	repo := NewRepository(openTestDB(t))
	disp := NewFakeDispatcher()
	supp := NewFakeSuppressionChecker()
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Dispatcher:  disp,
		Suppression: supp,
		Logger:      slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
	})
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.Register(v1, rbac.NoopPermFactory())
	return r, svc, disp
}

// TestHandler_CreateAlert covers the POST /api/v1/alerts
// scenario: a well-formed body produces 201 and the persisted
// row. The body specifies a channel_id so the dispatcher
// receives exactly 2 calls: the audit-log dispatch and the
// configured log channel.
func TestHandler_CreateAlert(t *testing.T) {
	r, svc, disp := newHandlerRouter(t)
	ch, _ := svc.CreateChannel(context.Background(), CreateChannelInput{Type: ChannelTypeLog, Enabled: true})
	body := `{"name":"high_cpu","severity":"warning","source_type":"custom","source_id":"src-1","channel_ids":["` + ch.ID + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	if len(disp.Calls) != 2 {
		t.Errorf("expected 2 dispatches (audit + channel), got %d", len(disp.Calls))
	}
}

// TestHandler_ListAlerts covers the GET /api/v1/alerts scenario:
// the response is the standard ListResponse envelope.
func TestHandler_ListAlerts(t *testing.T) {
	r, _, _ := newHandlerRouter(t)
	// Seed one.
	body := `{"name":"x","severity":"info","source_type":"custom","source_id":"s"}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), postReq)
	// List.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d", w.Code)
	}
	var resp struct {
		Data       []Alert                `json:"data"`
		Pagination contracts.Pagination `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if len(resp.Data) != 1 {
		t.Errorf("rows: got %d want 1", len(resp.Data))
	}
}

// TestHandler_Acknowledge covers the POST /alerts/:id/acknowledge
// scenario: the X-User-Id header supplies the user id (in
// production the JWT middleware would set it from the token).
func TestHandler_Acknowledge(t *testing.T) {
	r, svc, _ := newHandlerRouter(t)
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "x", Severity: SeverityInfo,
		SourceType: SourceTypeCustom, SourceID: "s",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+a.ID+"/acknowledge", nil)
	req.Header.Set("X-User-Id", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	got, _ := svc.GetAlert(a.ID)
	if got.State != StateAcknowledged {
		t.Errorf("state: got %q", got.State)
	}
	if got.AcknowledgedBy == nil || *got.AcknowledgedBy != "alice" {
		t.Errorf("ack by: %+v", got.AcknowledgedBy)
	}
}

// TestHandler_Resolve covers the POST /alerts/:id/resolve
// scenario: a 200 is returned and the alert is resolved.
func TestHandler_Resolve(t *testing.T) {
	r, svc, _ := newHandlerRouter(t)
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "x", Severity: SeverityInfo,
		SourceType: SourceTypeCustom, SourceID: "s",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+a.ID+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	got, _ := svc.GetAlert(a.ID)
	if got.State != StateResolved {
		t.Errorf("state: got %q", got.State)
	}
}

// TestHandler_DeleteAlert covers the DELETE /alerts/:id scenario.
// A 204 is returned on success.
func TestHandler_DeleteAlert(t *testing.T) {
	r, svc, _ := newHandlerRouter(t)
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "x", Severity: SeverityInfo,
		SourceType: SourceTypeCustom, SourceID: "s",
	})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/"+a.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d", w.Code)
	}
}

// TestHandler_Channel_CRUD covers the channel endpoints
// (POST/GET/PUT/DELETE /api/v1/alerts/channels).
func TestHandler_Channel_CRUD(t *testing.T) {
	r, _, _ := newHandlerRouter(t)
	body := `{"type":"slack","enabled":true,"config":{"webhook_url":"https://x","channel":"#a"}}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", strings.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, postReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status=%d body=%s", w.Code, w.Body.String())
	}
	// Mask check: api_token (not set here) would be redacted;
	// webhook_url is not sensitive and should pass through.
	var got Channel
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Config["webhook_url"] != "https://x" {
		t.Errorf("config passthrough: %+v", got.Config)
	}
	// List.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/channels", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, listReq)
	if w.Code != http.StatusOK {
		t.Errorf("list: status=%d", w.Code)
	}
	// Update.
	updBody := `{"enabled":false}`
	updReq := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/channels/"+got.ID, strings.NewReader(updBody))
	updReq.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, updReq)
	if w.Code != http.StatusOK {
		t.Errorf("update: status=%d body=%s", w.Code, w.Body.String())
	}
	// Delete.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/"+got.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, delReq)
	if w.Code != http.StatusNoContent {
		t.Errorf("delete: status=%d", w.Code)
	}
}

// TestHandler_MasksSensitiveOnGet verifies the channel GET
// endpoint redacts api_token / password / secret fields.
func TestHandler_MasksSensitiveOnGet(t *testing.T) {
	r, svc, _ := newHandlerRouter(t)
	c, _ := svc.CreateChannel(context.Background(), CreateChannelInput{
		Type:    ChannelTypeSlack,
		Enabled: true,
		Config:  JSONMap{"api_token": "very-secret", "channel": "#a"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/channels/"+c.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"api_token":"***"`) {
		t.Errorf("api_token not masked: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"channel":"#a"`) {
		t.Errorf("channel not preserved: %s", w.Body.String())
	}
}

// TestHandler_History covers the GET /api/v1/alerts/history
// scenario from the spec: a paginated list of triggered alerts
// is returned.
func TestHandler_History(t *testing.T) {
	r, _, _ := newHandlerRouter(t)
	for i := 0; i < 2; i++ {
		body := `{"name":"x","severity":"info","source_type":"custom","source_id":"s"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status: %d", w.Code)
	}
}

// TestHandler_HistoryFilterByName covers the
// "Filter by alert name" scenario: ?name=x narrows the list.
func TestHandler_HistoryFilterByName(t *testing.T) {
	r, _, _ := newHandlerRouter(t)
	for _, n := range []string{"a", "b"} {
		body := `{"name":"` + n + `","severity":"info","source_type":"custom","source_id":"s"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/history?name=a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var resp struct {
		Data []Alert `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Data[0].Name != "a" {
		t.Errorf("filter: got %+v", resp.Data)
	}
}

// TestHandler_SuppressedList covers the
// "List suppressed alerts" scenario:
// GET /api/v1/alerts/history?suppressed=true returns only the
// alerts that were suppressed due to maintenance.
func TestHandler_SuppressedList(t *testing.T) {
	r, svc, _ := newHandlerRouter(t)
	// Seed an alert that was suppressed.
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "suppressed", Severity: SeverityWarning,
		SourceType: SourceTypePhysicalHost, SourceID: "host-1",
	})
	_, _ = svc.MarkSuppressed(a.ID, "host_in_maintenance")
	// And one not suppressed.
	_, _ = svc.CreateAlert(context.Background(), CreateAlertInput{
		Name: "not-suppressed", Severity: SeverityInfo,
		SourceType: SourceTypeCustom, SourceID: "s",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/history?suppressed=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var resp struct {
		Data []Alert `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || !resp.Data[0].Suppressed {
		t.Errorf("suppressed list: %+v", resp.Data)
	}
}

// TestHandler_Stats covers the GET /api/v1/alerts/stats
// scenario: total / by-severity / suppressed_count are
// returned.
func TestHandler_Stats(t *testing.T) {
	r, _, _ := newHandlerRouter(t)
	for i := 0; i < 2; i++ {
		body := `{"name":"x","severity":"info","source_type":"custom","source_id":"s"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"total":2`) {
		t.Errorf("total: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"suppressed_count":0`) {
		t.Errorf("suppressed_count: %s", w.Body.String())
	}
}

// TestHandler_ValidationError covers the negative path: a
// malformed JSON body returns 400 with the api-contract
// envelope.
func TestHandler_ValidationError(t *testing.T) {
	r, _, _ := newHandlerRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", w.Code)
	}
	var er contracts.ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &er)
	if er.Error.Code != contracts.CodeValidation {
		t.Errorf("code: got %q", er.Error.Code)
	}
	_ = handler.WriteList // keep import alive for tooling
}
