package alerts

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/devops-toolkit/backend/internal/audit"
)

// recordingAlertsEmitter captures every AuditEvent for
// assertion. It satisfies the audit.Emitter interface used by
// audit.Service.
type recordingAlertsEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingAlertsEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingAlertsEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newAlertsAuditFixture wires a fresh in-memory DB, an alerts
// service, and a recording audit service. The audit service is
// the one used by Service.audit; the emitter captures every
// emit for assertion.
func newAlertsAuditFixture(t *testing.T) (*Service, *recordingAlertsEmitter) {
	t.Helper()
	repo := NewRepository(openTestDB(t))
	disp := NewFakeDispatcher()
	supp := NewFakeSuppressionChecker()
	emitter := &recordingAlertsEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(openTestDB(t)), Emitter: emitter})
	svc := NewService(ServiceConfig{
		Repo:        repo,
		Dispatcher:  disp,
		Suppression: supp,
		Audit:       auditSvc,
		Logger:      slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)),
	})
	return svc, emitter
}

// TestService_CreateAlert_EmitsAudit pins the service-layer
// audit emission for alert creation. The plan's Task 19
// ("alerts rule Create/Update/Delete") mapped to the alerts
// service; rules themselves live in the logs module. The
// alerts service exposes Alert + Channel CRUD which are the
// actual mutating surfaces; this test pins both.
func TestService_CreateAlert_EmitsAudit(t *testing.T) {
	svc, emitter := newAlertsAuditFixture(t)
	a, err := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name:       "high-cpu",
		Severity:   SeverityCritical,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "host-1",
		Message:    "cpu > 90%",
	})
	if err != nil {
		t.Fatalf("create alert: %v", err)
	}
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	if events[0].ResourceType != audit.ResourceAlert {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceAlert)
	}
	if events[0].ResourceID != a.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, a.ID)
	}
}

// TestService_DeleteAlert_EmitsAudit pins the delete path.
func TestService_DeleteAlert_EmitsAudit(t *testing.T) {
	svc, emitter := newAlertsAuditFixture(t)
	a, _ := svc.CreateAlert(context.Background(), CreateAlertInput{
		Name:       "high-cpu",
		Severity:   SeverityCritical,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "host-1",
	})
	if err := svc.Delete(context.Background(), a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionDelete {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionDelete)
	}
	if last.ResourceID != a.ID {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, a.ID)
	}
}

// TestService_CreateChannel_EmitsAudit pins the channel create
// path. Channels are part of the alert subsystem's write
// surface and round out the audit-trail coverage for the
// module.
func TestService_CreateChannel_EmitsAudit(t *testing.T) {
	svc, emitter := newAlertsAuditFixture(t)
	c, err := svc.CreateChannel(context.Background(), CreateChannelInput{
		Type:    ChannelTypeSlack,
		Config:  JSONMap{"webhook_url": "https://hooks.slack.com/services/T0/B0/XXX"},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	if events[0].ResourceType != audit.ResourceAlert {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceAlert)
	}
	if events[0].ResourceID != c.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, c.ID)
	}
}
