package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/alerts"
	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/config"
	dbpkg "github.com/devops-toolkit/backend/internal/database"
	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/internal/discovery"
	"github.com/devops-toolkit/backend/internal/hostproject"
	"github.com/devops-toolkit/backend/internal/k8s"
	"github.com/devops-toolkit/backend/internal/logs"
	"github.com/devops-toolkit/backend/internal/metrics"
	"github.com/devops-toolkit/backend/internal/physicalhost"
	"github.com/devops-toolkit/backend/internal/pipeline"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
	"github.com/devops-toolkit/backend/pkg/contracts"
	"github.com/devops-toolkit/backend/pkg/logger"
)

// TestRouteSmoke_ProjectAndDeviceRegistered is a Phase 3 integration
// check. It mirrors what main() does at runtime: open a sqlite DB,
// wire the project + device handlers, and probe their list endpoints.
// It exists to catch "I added the call site but the handler refuses
// to register" regressions before we have to launch a real server.
func TestRouteSmoke_ProjectAndDeviceRegistered(t *testing.T) {
	// Use a temp file for sqlite (file-based avoids the in-memory
	// shared-cache quirk seen in earlier rounds).
	tmpDir := t.TempDir()
	dsn := filepath.Join(tmpDir, "smoke.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver:          "sqlite",
			DSN:             dsn,
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: 60,
		},
		App: config.AppConfig{
			Env:      "test",
			LogLevel: "error",
		},
	}

	db, err := dbpkg.Open(toDBConfig(cfg.Database))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := dbpkg.AutoMigrate(db,
		&projectpkg.ProjectType{},
		&projectpkg.Project{},
		&projectpkg.ProjectMember{},
	); err != nil {
		t.Fatalf("project migrate: %v", err)
	}
	if err := dbpkg.AutoMigrate(db, devicepkg.AllModels()...); err != nil {
		t.Fatalf("device migrate: %v", err)
	}
	if err := dbpkg.AutoMigrate(db, physicalhost.AllModels()...); err != nil {
		t.Fatalf("physicalhost migrate: %v", err)
	}
	if err := dbpkg.AutoMigrate(db, discovery.AllModels()...); err != nil {
		t.Fatalf("discovery migrate: %v", err)
	}
	if err := dbpkg.AutoMigrate(db, k8s.AllModels()...); err != nil {
		t.Fatalf("k8s migrate: %v", err)
	}
	if err := dbpkg.AutoMigrate(db, hostproject.AllModels()...); err != nil {
		t.Fatalf("hostproject migrate: %v", err)
	}
	if err := dbpkg.AutoMigrate(db, pipeline.AllModels()...); err != nil {
		t.Fatalf("pipeline migrate: %v", err)
	}

	_ = logger.New(logger.WithLevel("error"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// noopPerms disables the per-route RBAC factory so
	// this smoke test exercises the route registration +
	// the handler code paths without requiring a fully
	// configured auth stack. Auth itself is bypassed by
	// setting the X-User header on every test request
	// (dev_bypass mode in the auth middleware).
	noopPerms := rbac.NoopPermFactory()

	projectRepo := projectpkg.NewRepository(db)
	projectSvc := projectpkg.NewService(projectRepo)
	projectH := projectpkg.NewHandler(projectSvc, projectRepo)
	v1 := r.Group("/api/v1")
	projectH.Register(v1, noopPerms)

	deviceRepo := devicepkg.NewRepository(db)
	deviceSvc := devicepkg.NewService(deviceRepo)
	deviceH := devicepkg.NewHandler(deviceSvc)
	deviceH.Register(v1, noopPerms)

	groupRepo := devicepkg.NewGroupRepository(db)
	groupSvc := devicepkg.NewGroupService(groupRepo)
	devicepkg.NewGroupHandler(groupSvc).Register(v1, noopPerms)

	tmplRepo := devicepkg.NewTemplateRepository(db)
	tmplSvc := devicepkg.NewTemplateService(tmplRepo)
	devicepkg.NewTemplateHandler(tmplSvc).Register(v1, noopPerms)

	// Phase 4: physical-host, discovery, k8s, host-project-link, pipeline.
	phRepo := physicalhost.NewRepository(db)
	phMonitor := physicalhost.NewMonitorService(physicalhost.MonitorConfig{
		Repo:                phRepo,
		Prober:              physicalhost.NewFake(),
		ConsecutiveFailures: 3,
		CheckInterval:       time.Minute,
	})
	phMaint := physicalhost.NewMaintenanceService(physicalhost.MaintenanceConfig{
		Repo:    phRepo,
		Auditor: noopAuditEmitter{},
	})
	physicalhost.NewHandler(physicalhost.HandlerConfig{
		Repo: phRepo, Monitor: phMonitor, Maintenance: phMaint,
	}).Register(v1, noopPerms)

	discRepo := discovery.NewRepository(db)
	discSvc := discovery.NewService(discRepo, devicepkg.NewRepository(db),
		discovery.NewFakeScanner(nil, nil), discovery.NewFakeProber(nil, nil))
	discovery.NewHandler(discSvc).Register(v1, noopPerms)

	kRepo := k8s.NewRepository(db)
	kSvc := k8s.NewService(kRepo, &k8s.FakeClient{}, make([]byte, 32))
	k8s.NewHandler(kSvc).Register(v1, noopPerms)

	hpRepo := hostproject.NewRepository(db)
	hpSvc := hostproject.NewService(hpRepo, projectSvc)
	hostproject.NewHandler(hpSvc).Register(v1, noopPerms)

	plRepo := pipeline.NewRepository(db)
	plExec := &pipeline.Fake{}
	plSvc := pipeline.NewService(plRepo, plExec)
	pipeline.NewHandler(plSvc).Register(v1, noopPerms)

	// Phase 5: logs (no DB), metrics, alerts.
	logBackend := logs.NewLocal(logs.LocalConfig{})
	logs.NewHandler(logs.NewService(logBackend, logs.ServiceConfig{}), logBackend).Register(v1, noopPerms)

	if err := dbpkg.AutoMigrate(db, metrics.AllModels()...); err != nil {
		t.Fatalf("metrics migrate: %v", err)
	}
	mRepo := metrics.NewRepository(db)
	mSvc := metrics.NewService(mRepo, metrics.NewFakeScraper())
	metrics.NewHandler(mSvc).Register(v1, noopPerms)

	if err := dbpkg.AutoMigrate(db, alerts.AllModels()...); err != nil {
		t.Fatalf("alerts migrate: %v", err)
	}
	aRepo := alerts.NewRepository(db)
	aSvc := alerts.NewService(alerts.ServiceConfig{
		Repo:        aRepo,
		Dispatcher:  alerts.NewFakeDispatcher(),
		Suppression: alerts.NewFakeSuppressionChecker(),
	})
	alerts.NewHandler(aSvc).Register(v1, noopPerms)

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"project-types list", http.MethodGet, "/api/v1/project-types", http.StatusOK},
		{"project create no body -> 400", http.MethodPost, "/api/v1/projects", http.StatusBadRequest},
		{"device list empty", http.MethodGet, "/api/v1/devices", http.StatusOK},
		{"device create no body -> 400", http.MethodPost, "/api/v1/devices", http.StatusBadRequest},
		{"device-group list", http.MethodGet, "/api/v1/device-groups", http.StatusOK},
		{"config-template list", http.MethodGet, "/api/v1/configuration-templates", http.StatusOK},
		// Phase 4 additions
		{"physical-hosts list empty", http.MethodGet, "/api/v1/physical-hosts", http.StatusOK},
		{"physical-hosts create no body -> 400", http.MethodPost, "/api/v1/physical-hosts", http.StatusBadRequest},
		{"discovery runs list empty", http.MethodGet, "/api/v1/discovery/runs", http.StatusOK},
		{"k8s clusters list empty", http.MethodGet, "/api/v1/k8s/clusters", http.StatusOK},
		{"k8s clusters create no body -> 400", http.MethodPost, "/api/v1/k8s/clusters", http.StatusBadRequest},
		{"pipelines list empty", http.MethodGet, "/api/v1/pipelines", http.StatusOK},
		{"pipelines create no body -> 400", http.MethodPost, "/api/v1/pipelines", http.StatusBadRequest},
		// Phase 5 additions
		{"logs capabilities", http.MethodGet, "/api/v1/logs/capabilities", http.StatusOK},
		{"logs query (empty)", http.MethodGet, "/api/v1/logs/query", http.StatusOK},
		{"logs streams (empty)", http.MethodGet, "/api/v1/logs/streams", http.StatusOK},
		{"metrics list empty", http.MethodGet, "/api/v1/metrics", http.StatusOK},
		{"metrics series empty", http.MethodGet, "/api/v1/metrics/series", http.StatusOK},
		{"alerts list empty", http.MethodGet, "/api/v1/alerts", http.StatusOK},
		{"alert channels list empty", http.MethodGet, "/api/v1/alerts/channels", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			// Dev-bypass header lets the route reach the
			// handler without a real JWT. Production
			// uses Bearer auth + rbac.RequirePermission;
			// this test exercises the handler code paths
			// only.
			req.Header.Set("X-User", "smoke-tester")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s %s: code=%d want=%d body=%s", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	// Avoid unused-import false-positives on minimal refactors.
	_, _ = os.Getenv, gorm.ErrRecordNotFound
}

// TestRouteSmoke_Auth_Protected is the Phase 6 P0 #1 follow-up:
// a fresh Gin engine with the same module registrars but
// the real auth + RBAC middleware (no dev-bypass, no
// noop perm factory) must reject every protected request
// without a valid token. The login + capabilities routes
// remain public; everything else is gated.
//
// The /api/v1/auth/login and /api/v1/capabilities routes
// are public (login is the entry point, capabilities is
// informational). Every other /api/v1/* route is gated.
func TestRouteSmoke_Auth_Protected(t *testing.T) {
	tmpDir := t.TempDir()
	dsn := filepath.Join(tmpDir, "auth.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Driver: "sqlite", DSN: dsn,
			MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxLifetime: 60,
		},
		App: config.AppConfig{Env: "test", LogLevel: "error"},
	}
	db, err := dbpkg.Open(toDBConfig(cfg.Database))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	_ = logger.New(logger.WithLevel("error"), logger.WithFormat("json"))
	gin.SetMode(gin.TestMode)

	// Build the same authenticator the production
	// wiring uses. dev_bypass stays false so every
	// request without a Bearer token is rejected.
	signer, err := auth.NewSigner("smoke-test-secret", time.Hour)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	authMW := auth.NewAuthenticator(auth.AuthMiddlewareConfig{
		Signer:        signer,
		DevBypass:     false,
		PermissionSvc: rbac.NewService(),
	})
	rbacSvc := rbac.NewService()
	perms := func(perm rbac.Permission) gin.HandlerFunc {
		return rbac.RequirePermission(rbacSvc, perm)
	}

	r := gin.New()
	// NoRoute / NoMethod envelopes match the
	// production renderer.
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "no route"}})
	})
	v1 := r.Group("/api/v1", authMW.RequireAuth())

	// Mount enough of the module surface to exercise
	// the auth gate; the per-module services only need
	// to exist long enough for Handler.Register.
	_ = dbpkg.AutoMigrate(db,
		&projectpkg.ProjectType{},
		&projectpkg.Project{},
		&projectpkg.ProjectMember{},
	)
	projectRepo := projectpkg.NewRepository(db)
	projectSvc := projectpkg.NewService(projectRepo)
	projectpkg.NewHandler(projectSvc, projectRepo).Register(v1, perms)

	_ = dbpkg.AutoMigrate(db, devicepkg.AllModels()...)
	deviceRepo := devicepkg.NewRepository(db)
	deviceSvc := devicepkg.NewService(deviceRepo)
	devicepkg.NewHandler(deviceSvc).Register(v1, perms)

	// Probe a representative set of routes that
	// span the protected surface. Each must 401 without
	// a Bearer token. The capabilities / login routes
	// are public and must NOT 401. The test only
	// registers project + device handlers (sufficient
	// to exercise the auth gate on two distinct
	// permission-protected routes), so the other
	// surface returns 404 (the NoRoute envelope).
	cases := []struct {
		name     string
		method   string
		path     string
		wantAuth int
	}{
		// Protected — must 401 without a token.
		{"projects list no token -> 401", http.MethodGet, "/api/v1/projects", http.StatusUnauthorized},
		{"devices list no token -> 401", http.MethodGet, "/api/v1/devices", http.StatusUnauthorized},
		// Unmounted routes — 404, not 401 (no handler).
		{"physical-hosts not mounted -> 404", http.MethodGet, "/api/v1/physical-hosts", http.StatusNotFound},
		{"k8s clusters not mounted -> 404", http.MethodGet, "/api/v1/k8s/clusters", http.StatusNotFound},
		{"pipelines not mounted -> 404", http.MethodGet, "/api/v1/pipelines", http.StatusNotFound},
		{"alerts not mounted -> 404", http.MethodGet, "/api/v1/alerts", http.StatusNotFound},
		{"metrics not mounted -> 404", http.MethodGet, "/api/v1/metrics", http.StatusNotFound},
		{"logs query not mounted -> 404", http.MethodGet, "/api/v1/logs/query", http.StatusNotFound},
		{"audit not mounted -> 404", http.MethodGet, "/api/v1/audit", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.wantAuth {
				t.Errorf("%s %s: code=%d want=%d body=%s", tc.method, tc.path, rec.Code, tc.wantAuth, rec.Body.String())
			}
		})
	}

	// A valid token lets the request through. The
	// Developer role has PermissionViewDevices so the
	// /devices route returns 200.
	developerToken, _, err := signer.Issue(&contracts.User{ID: "u-dev", Username: "dev", Role: contracts.RoleDeveloper})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer "+developerToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("developer on /devices: code=%d want=200 body=%s", rec.Code, rec.Body.String())
	}

	// A bogus token is rejected with 401 before any
	// permission check runs.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bogus token: code=%d want=401", rec.Code)
	}
}

// noopAuditEmitter satisfies physicalhost.AuditEmitter for the test
// without dragging in a real logger.
type noopAuditEmitter struct{}

func (noopAuditEmitter) EmitMaintenanceEnter(_ context.Context, _ physicalhost.AuditEvent) {}
func (noopAuditEmitter) EmitMaintenanceExit(_ context.Context, _ physicalhost.AuditEvent)  {}
