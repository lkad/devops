package k8s

import (
	"context"
	"sync"
	"testing"

	"github.com/devops-toolkit/backend/internal/audit"
)

// recordingK8sEmitter captures every AuditEvent for assertion.
// It satisfies the audit.Emitter interface used by
// audit.Service.
type recordingK8sEmitter struct {
	mu     sync.Mutex
	events []audit.AuditEvent
}

func (r *recordingK8sEmitter) Emit(_ context.Context, e audit.AuditEvent) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recordingK8sEmitter) all() []audit.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// newK8sAuditFixture wires a fresh in-memory DB, a k8s
// service, and a recording audit service. The audit service
// is the one used by Service.audit; the emitter captures
// every emit for assertion.
func newK8sAuditFixture(t *testing.T) (*Service, *recordingK8sEmitter) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	emitter := &recordingK8sEmitter{}
	auditSvc := audit.NewService(audit.ServiceConfig{Repo: audit.NewRepository(db), Emitter: emitter})
	svc := NewService(repo, &FakeClient{}, testCryptoKey, auditSvc)
	return svc, emitter
}

// validClusterInput returns a CreateClusterInput that passes
// the service's validation. Reused by the audit tests.
func validClusterInput() CreateClusterInput {
	return CreateClusterInput{
		Name:       "prod-1",
		APIServer:  "https://k8s.example.com:6443",
		Kubeconfig: `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://k8s.example.com:6443
  name: prod
contexts:
- context:
    cluster: prod
    user: admin
  name: prod
users:
- name: admin
  user:
    token: redacted
current-context: prod
`,
		Type: ClusterTypeStandard,
	}
}

// TestService_CreateCluster_EmitsAudit pins the service-layer
// audit emission for cluster creation. The audit emit lands
// AFTER the repo write so a flaky test would catch an
// ordering mistake.
func TestService_CreateCluster_EmitsAudit(t *testing.T) {
	svc, emitter := newK8sAuditFixture(t)
	c, err := svc.Create(validClusterInput())
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
	if events[0].ResourceType != audit.ResourceK8sCluster {
		t.Errorf("resource type = %q, want %q", events[0].ResourceType, audit.ResourceK8sCluster)
	}
	if events[0].ResourceID != c.ID {
		t.Errorf("resource id = %q, want %q", events[0].ResourceID, c.ID)
	}
}

// TestService_UpdateCluster_EmitsAudit pins the update path.
func TestService_UpdateCluster_EmitsAudit(t *testing.T) {
	svc, emitter := newK8sAuditFixture(t)
	c, _ := svc.Create(validClusterInput())
	newName := "prod-2"
	if _, err := svc.Update(c.ID, UpdateClusterInput{Name: &newName}); err != nil {
		t.Fatalf("update: %v", err)
	}
	events := emitter.all()
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	last := events[len(events)-1]
	if last.Action != audit.ActionUpdate {
		t.Errorf("last action = %q, want %q", last.Action, audit.ActionUpdate)
	}
	if last.ResourceID != c.ID {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, c.ID)
	}
}

// TestService_DeleteCluster_EmitsAudit pins the delete path.
func TestService_DeleteCluster_EmitsAudit(t *testing.T) {
	svc, emitter := newK8sAuditFixture(t)
	c, _ := svc.Create(validClusterInput())
	if err := svc.Delete(c.ID); err != nil {
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
	if last.ResourceID != c.ID {
		t.Errorf("last resource id = %q, want %q", last.ResourceID, c.ID)
	}
}
