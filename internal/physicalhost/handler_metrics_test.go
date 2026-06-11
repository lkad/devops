package physicalhost

import (
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestHandler_Metrics_HappyPath is the contract for the new
// GET /physical-hosts/:id/metrics route. It exercises the
// full chain: handler → metrics service → prober → parsers.
func TestHandler_Metrics_HappyPath(t *testing.T) {
	r := handlerWithMetrics(t, func(p *Fake) {
		p.ScriptSSHExec("10.0.0.1", "nproc", []byte("8\n"), nil)
		p.ScriptSSHExec("10.0.0.1", "top -bn1 | head -5", []byte("%Cpu(s): 12.5 us,  3.2 sy,  0.0 ni, 84.3 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st\n"), nil)
		p.ScriptSSHExec("10.0.0.1", "free -m", []byte("              total        used        free      shared  buff/cache   available\nMem:          16384        8192        4096         256        4096        7584\nSwap:          2048         512        1536\n"), nil)
		p.ScriptSSHExec("10.0.0.1", "df -BG", []byte("Filesystem     1G-blocks  Used Available Use% Mounted on\n/dev/sda1             100G    50G        50G  50% /\n"), nil)
		p.ScriptSSHExec("10.0.0.1", "uptime -p", []byte("up 10 days, 3 hours"), nil)
		p.ScriptSSHExec("10.0.0.1", "cat /proc/uptime", []byte("943320.00 1234567.89"), nil)
	})

	// Seed one host.
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d-metrics", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("seed: status=%d body=%v", rr.Code, body)
	}
	id, _ := body["id"].(string)

	rr, body = doPHRequest(t, r, "GET", "/api/v1/physical-hosts/"+id+"/metrics", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET metrics: status=%d body=%v", rr.Code, body)
	}
	if got, _ := body["data_status"].(string); got != "fresh" {
		t.Errorf("data_status = %q, want fresh", got)
	}
	cpu, _ := body["cpu"].(map[string]any)
	if cores, _ := cpu["cores"].(float64); int(cores) != 8 {
		t.Errorf("cpu.cores = %v, want 8", cpu["cores"])
	}
	mem, _ := body["memory"].(map[string]any)
	if total, _ := mem["total_mib"].(float64); int(total) != 16384 {
		t.Errorf("memory.total_mib = %v, want 16384", mem["total_mib"])
	}
}

// TestHandler_Metrics_NotFound covers the missing-host branch.
func TestHandler_Metrics_NotFound(t *testing.T) {
	r := handlerWithMetrics(t, nil)
	rr, _ := doPHRequest(t, r, "GET", "/api/v1/physical-hosts/missing-id/metrics", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

// TestHandler_Metrics_StaleStatus confirms that when the prober
// fails on a single command, the response carries data_status
// = stale so the UI can render the warning.
func TestHandler_Metrics_StaleStatus(t *testing.T) {
	r := handlerWithMetrics(t, func(p *Fake) {
		p.ScriptSSHExec("10.0.0.1", "nproc", []byte("4\n"), nil)
		p.ScriptSSHExec("10.0.0.1", "top -bn1 | head -5", []byte("%Cpu(s): 1.0 us,  0.0 sy\n"), nil)
		p.ScriptSSHExec("10.0.0.1", "free -m", []byte("Mem:           1000         500         500\n"), nil)
		// "df -BG" scripted to return an error so the collector
		// records a warning and flips the snapshot to stale.
		p.ScriptSSHExec("10.0.0.1", "df -BG", nil, errors.New("disk io error"))
		p.ScriptSSHExec("10.0.0.1", "uptime -p", []byte("up 1 hour"), nil)
		p.ScriptSSHExec("10.0.0.1", "cat /proc/uptime", []byte("3600.00 0.00"), nil)
	})

	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d-stale", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("seed: %v", body)
	}
	id, _ := body["id"].(string)
	rr, body = doPHRequest(t, r, "GET", "/api/v1/physical-hosts/"+id+"/metrics", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if got, _ := body["data_status"].(string); got != "stale" {
		t.Errorf("data_status = %q, want stale", got)
	}
	if ws, _ := body["warnings"].([]any); len(ws) == 0 {
		t.Error("expected warnings for failed df probe")
	}
}

// handlerWithMetrics wires a Gin engine that has a real
// MetricsCollector + handler route. The optional seed callback
// programs the Fake prober before the route is exercised.
func handlerWithMetrics(t *testing.T, seed func(p *Fake)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := NewRepository(openDB(t))
	prober := NewFake()
	if seed != nil {
		seed(prober)
	}
	mc := NewMetricsCollector(MetricsCollectorConfig{Prober: prober, Timeout: time.Second})
	cache := NewMetricsCache(MetricsCacheConfig{Collector: mc, TTL: 30 * time.Second, MaxEntries: 16})
	mon := NewMonitorService(MonitorConfig{
		Repo: repo, Prober: prober,
		ConsecutiveFailures: 3, CheckInterval: time.Minute,
	})
	maint := NewMaintenanceService(MaintenanceConfig{Repo: repo, Auditor: &fakeAuditor{}})
	mon.SetMaintenance(maint)
	svc := NewService(ServiceConfig{Repo: repo})
	h := NewHandler(HandlerConfig{
		Service: svc, Monitor: mon, Maintenance: maint, Metrics: cache,
	})
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.Register(v1, rbac.NoopPermFactory())
	return r
}
