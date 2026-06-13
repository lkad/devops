package discovery

import (
	"context"
	"net"
	"sync"
	"testing"

	"gorm.io/gorm"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/internal/audit"
)

// recordingDiscoveryEmitter captures every AuditEvent for
// assertion. It satisfies the audit.Emitter interface used by
// audit.Service.
type recordingDiscoveryEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingDiscoveryEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingDiscoveryEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newDiscoveryAuditFixture wires a fresh in-memory DB, a
// discovery service, and a recording audit service. The audit
// service is the one used by Service.audit; the emitter
// captures every emit for assertion.
func newDiscoveryAuditFixture(t *testing.T) (*Service, *Repository, *recordingDiscoveryEmitter) {
	t.Helper()
	db := openDiscoveryWithDeviceDB(t)
	dRepo := NewRepository(db)
	devRepo := devicepkg.NewRepository(db)
	emitter := &recordingDiscoveryEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(db), Emitter: emitter})
	// An empty host list and an empty probe-result map give a
	// zero-result run: the scanner yields no IPs to probe, the
	// prober is never consulted, and the run lands in the
	// "empty" terminal status. The audit emit still fires.
	s := NewService(dRepo, devRepo,
		NewFakeScanner(nil, nil),
		NewFakeProber(map[string]ProbeResult{}, nil),
		auditSvc)
	return s, dRepo, emitter
}

// silence unused import warnings if gorm.io/gorm is not used
// in this file after refactors; this keeps the linter quiet
// while the helper signature stabilises.
var _ = (*gorm.DB)(nil)

// TestService_StartRun_EmitsAudit pins the service-layer
// audit emission for discovery-run creation. The P0 #3
// audit-trail commit added this emit; the test pins the
// resource type and the action so a future refactor cannot
// silently drop the row.
func TestService_StartRun_EmitsAudit(t *testing.T) {
	svc, _, emitter := newDiscoveryAuditFixture(t)
	// 10.0.0.0/30 has 2 usable hosts; FakeScanner returns
	// an empty list, so the run completes with hosts_found=0
	// and status=empty.
	_, cidr, err := net.ParseCIDR("10.0.0.0/30")
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}
	run, err := svc.StartRun(context.Background(), StartRunInput{CIDR: cidr.String()})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	events := emitter.all()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	if events[0].Action != audit.ActionCreate {
		t.Errorf("action = %q, want %q", events[0].Action, audit.ActionCreate)
	}
	if events[0].ResourceType != audit.ResourceDiscoveryRun {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceDiscoveryRun)
	}
	if events[0].ResourceID != run.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, run.ID)
	}
}
