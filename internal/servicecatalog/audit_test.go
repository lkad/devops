package servicecatalog

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/internal/audit"
)

// recordingCatalogEmitter captures every AuditEvent for
// assertion. It satisfies the audit.Emitter interface used by
// audit.Service.
type recordingCatalogEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingCatalogEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingCatalogEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newCatalogAuditFixture wires a fresh in-memory DB, a service
// catalog, and a recording audit service. The audit service is
// the one used by Catalog.audit; the emitter captures every
// emit for assertion.
func newCatalogAuditFixture(t *testing.T) (*Catalog, *recordingCatalogEmitter) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	emitter := &recordingCatalogEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(db), Emitter: emitter})
	cat := NewCatalog(repo, auditSvc)
	return cat, emitter
}

// TestCatalog_Create_EmitsAudit pins the service-layer audit
// emission for service creation. The audit emit lands AFTER
// the repo write so a flaky test would catch an ordering
// mistake.
func TestCatalog_Create_EmitsAudit(t *testing.T) {
	cat, emitter := newCatalogAuditFixture(t)
	s, err := cat.Create(CreateInput{
		Name:        "auth-svc",
		Description: "auth gateway",
		Owner:       "team-platform",
		Tier:        TierCritical,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	// servicecatalog uses the bare "service" resource type,
	// not the more specific ResourceService constant, because
	// the catalog sub-resources (oncall, runbook) get their
	// own dedicated constants (ResourceOnCall, ResourceRunbook)
	// and the parent service uses the lower-case noun.
	if events[0].ResourceID != s.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, s.ID)
	}
}

// TestCatalog_CreateOnCall_EmitsAudit pins the oncall create
// path. The TODO(audit) that lived at line 318 of the original
// service.go was closed by the P0 #3 audit-trail commit; this
// test pins the resulting emit so a future refactor cannot
// silently drop it.
func TestCatalog_CreateOnCall_EmitsAudit(t *testing.T) {
	cat, emitter := newCatalogAuditFixture(t)
	s, _ := cat.Create(CreateInput{Name: "auth-svc", Tier: TierCritical})
	now := time.Now()
	_, err := cat.CreateOnCall(s.ID, CreateOnCallInput{
		User:       "alice@example.com",
		ShiftStart: now.Add(-1 * time.Hour),
		ShiftEnd:   now.Add(1 * time.Hour),
		Scope:      OnCallScopeService,
	})
	if err != nil {
		t.Fatalf("create oncall: %v", err)
	}
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionCreate {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionCreate)
	}
	if last.ResourceType != audit.ResourceOnCall {
		t.Errorf("last resource type = %q, want %q", last.ResourceType, audit.ResourceOnCall)
	}
}

// TestCatalog_CreateRunbook_EmitsAudit pins the runbook create
// path.
func TestCatalog_CreateRunbook_EmitsAudit(t *testing.T) {
	cat, emitter := newCatalogAuditFixture(t)
	s, _ := cat.Create(CreateInput{Name: "auth-svc", Tier: TierCritical})
	_, err := cat.CreateRunbook(s.ID, CreateRunbookInput{
		Title: "DB failover",
		Body:  "1. promote replica; 2. update DNS",
		Actor: "alice",
	})
	if err != nil {
		t.Fatalf("create runbook: %v", err)
	}
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionCreate {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionCreate)
	}
	if last.ResourceType != audit.ResourceRunbook {
		t.Errorf("last resource type = %q, want %q", last.ResourceType, audit.ResourceRunbook)
	}
}
