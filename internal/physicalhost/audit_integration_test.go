package physicalhost

import (
	"context"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/audit"
)

// TestMaintenance_PersistsAuditRow verifies that the production
// audit emitter writes a row to the audit table when
// maintenance is entered/exited. The old logAuditEmitter only
// logged to slog; the spec requires durable rows.
func TestMaintenance_PersistsAuditRow(t *testing.T) {
	repo := NewRepository(openDB(t))
	auditRepo := audit.NewRepository(openDBWithAudit(t))
	dbEmitter := audit.NewDBEmitter(auditRepo)
	auditSvc := audit.NewService(audit.ServiceConfig{Emitter: dbEmitter})
	maint := NewMaintenanceService(MaintenanceConfig{
		Repo:    repo,
		Auditor: NewAuditEmitterAdapter(auditSvc),
	})

	p := &PhysicalHost{DeviceID: "d-audit-1", IPAddress: "10.0.0.1", SSHUser: "root", SSHPort: 22, State: StateOnline}
	if err := repo.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, _, err := maint.EnterMaintenance(context.Background(), p.ID, "kernel upgrade", "alice"); err != nil {
		t.Fatalf("enter: %v", err)
	}
	if _, _, err := maint.ExitMaintenance(context.Background(), p.ID, "alice"); err != nil {
		t.Fatalf("exit: %v", err)
	}

	// Give the BufferedEmitter worker a tick to drain.
	time.Sleep(50 * time.Millisecond)

	rows, _, err := auditRepo.List(audit.AuditFilter{ResourceID: p.ID, Limit: 10})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("audit recorded %d events, want 2 (enter+exit); rows=%+v", len(rows), rows)
	}
	// DESC occurred_at: exit (later) comes first.
	if rows[0].Action != "maintenance_exit" {
		t.Errorf("first event action = %q, want maintenance_exit (most recent)", rows[0].Action)
	}
	if rows[1].Action != "maintenance_enter" {
		t.Errorf("second event action = %q, want maintenance_enter (earlier)", rows[1].Action)
	}
}

// TestMaintenance_HistoryEndpoint_AggregatesAudit: end-to-end
// check that the handler reads the audit table and renders the
// maintenance history for a given host.
func TestMaintenance_HistoryEndpoint_AggregatesAudit(t *testing.T) {
	r := handlerWithAudit(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d-hist-1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != 201 {
		t.Fatalf("seed: %v", body)
	}
	id, _ := body["id"].(string)

	// Enter + exit, twice.
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance", map[string]any{"reason": "r1"})
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance/exit", nil)
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance", map[string]any{"reason": "r2"})
	_, _ = doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+id+"/maintenance/exit", nil)

	time.Sleep(50 * time.Millisecond)

	rr, body = doPHRequest(t, r, "GET", "/api/v1/physical-hosts/"+id+"/maintenance-history", nil)
	if rr.Code != 200 {
		t.Fatalf("history: %d %v", rr.Code, body)
	}
	data, _ := body["data"].([]any)
	if len(data) != 4 {
		t.Errorf("len(history) = %d, want 4 (2 enters + 2 exits)", len(data))
	}
}

// openDBWithAudit migrates the audit tables on top of the
// physicalhost schema. Both packages live in the same sqlite
// DB so a single AutoMigrate pass covers both.
func openDBWithAudit(t *testing.T) *gorm.DB {
	t.Helper()
	db := openDB(t)
	if err := db.AutoMigrate(audit.AllModels()...); err != nil {
		t.Fatalf("audit migrate: %v", err)
	}
	return db
}

// handlerWithAudit wires the handler with a real audit
// service so the maintenance flow produces audit rows.
func handlerWithAudit(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openDBWithAudit(t)
	repo := NewRepository(db)
	auditRepo := audit.NewRepository(db)
	dbEmitter := audit.NewDBEmitter(auditRepo)
	auditSvc := audit.NewService(audit.ServiceConfig{Emitter: dbEmitter})
	prober := NewFake()
	mon := NewMonitorService(MonitorConfig{Repo: repo, Prober: prober, ConsecutiveFailures: 3, CheckInterval: time.Second})
	maint := NewMaintenanceService(MaintenanceConfig{
		Repo:    repo,
		Auditor: NewAuditEmitterAdapter(auditSvc),
	})
	mon.SetMaintenance(maint)
	h := NewHandler(HandlerConfig{Repo: repo, Monitor: mon, Maintenance: maint, Audit: auditSvc, AuditRepo: auditRepo})
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.Register(v1)
	return r
}
