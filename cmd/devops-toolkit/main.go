// Command devops-toolkit is the entry point for the DevOps Toolkit backend.
// Phase 1 wires up: config loader, slog logger, GORM database connection,
// and a Gin HTTP server with /health and a /api/v1/capabilities placeholder.
// Subsequent phases add module routes via the same router.
package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/alerts"
	"github.com/devops-toolkit/backend/internal/auth/ldap"
	"github.com/devops-toolkit/backend/internal/config"
	dbpkg "github.com/devops-toolkit/backend/internal/database"
	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/internal/discovery"
	"github.com/devops-toolkit/backend/internal/handler"
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "configs/templates/config-dev.yaml"
	}
	cfg, err := config.Load(config.WithConfigPath(cfgPath))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(
		logger.WithLevel(cfg.App.LogLevel),
		logger.WithFormat(envOr("LOG_FORMAT", "json")),
	)
	log.Info("starting devops-toolkit", "env", cfg.App.Env, "config", renderConfig(cfg))

	db, err := dbpkg.Open(toDBConfig(cfg.Database))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	log.Info("database connected", "driver", cfg.Database.Driver)

	router := buildRouter(log)
	if eng, ok := router.(*gin.Engine); ok {
		registerAuthRoutes(eng, cfg, log)
		registerProjectRoutes(eng, db, log)
		registerDeviceRoutes(eng, db, log)
		registerPhysicalHostRoutes(eng, db, log)
		registerDiscoveryRoutes(eng, db, log)
		registerK8sClusterRoutes(eng, db, log)
		registerHostProjectLinkRoutes(eng, db, log)
		registerPipelineRoutes(eng, db, log)
		registerLogsRoutes(eng, db, log)
		registerMetricsRoutes(eng, db, log)
		registerAlertsRoutes(eng, db, log)
	} else {
		log.Warn("router is not a *gin.Engine; auth routes not registered")
	}
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		log.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

// buildRouter returns the HTTP handler tree. Kept as a separate
// function so tests can call it without booting a listener.
func buildRouter(log *logger.Logger) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		handler.WriteJSON(c.Writer, http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	// Standard 404 fallback renders the api-contract error envelope.
	r.NoRoute(func(c *gin.Context) {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: fmt.Sprintf("route %s %s not found", c.Request.Method, c.Request.URL.Path),
		})
	})

	v1 := r.Group("/api/v1")
	{
		v1.GET("/capabilities", func(c *gin.Context) {
			// Phase-1 placeholder; later phases add module-specific capability probes.
			sections := []string{"app", "database", "logs", "ldap", "alerts", "k8s", "physicalhost", "websocket"}
			handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
				"sections":  sections,
				"phase":     1,
				"build_tag": "foundation",
			})
		})
	}
	return r
}

// toDBConfig maps the config-tree DatabaseConfig to the database
// package's ConnectionConfig. Kept here so config stays free of any
// GORM dependency.
func toDBConfig(c config.DatabaseConfig) dbpkg.ConnectionConfig {
	return dbpkg.ConnectionConfig{
		Driver:          c.Driver,
		DSN:             c.DSN,
		Host:            c.Host,
		Port:            c.Port,
		User:            c.User,
		Password:        c.Password,
		DBName:          c.DBName,
		SSLMode:         c.SSLMode,
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
	}
}

// renderConfig is exposed for the test in this package. The masking
// itself lives on (*config.Config).String.
func renderConfig(c *config.Config) string {
	return c.String()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// registerAuthRoutes wires the LDAP login + health endpoints onto the
// supplied Gin engine. It is a separate function (rather than being
// inlined into buildRouter) so the middleware agent can call it from
// the same router without colliding with route registrations.
//
// In dev_bypass mode the LDAP client is the in-memory Fake loaded
// from config.LDAP.DevUsers; otherwise the Real client is used. The
// JWT secret is read from LDAP.JWTSecret or, if empty, falls back to
// a deployment-injected APP_JWT_SECRET env var, with a deterministic
// dev-only default if neither is set (we log a warning so it never
// silently ships to production).
func registerAuthRoutes(r *gin.Engine, cfg *config.Config, log *logger.Logger) {
	client, err := buildLDAPClient(cfg)
	if err != nil {
		log.Error("ldap client init failed; auth routes will not be registered", "err", err)
		return
	}
	svc := ldap.NewService(ldap.ServiceConfig{
		Client:          client,
		MaxFailedLogins: 5,
		RateLimitWindow: time.Minute,
	})

	secret := os.Getenv("APP_JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-do-not-use-in-prod"
		log.Warn("no APP_JWT_SECRET configured; using dev-only fallback")
	}

	ttl := int64(3600)
	if v := os.Getenv("APP_JWT_TTL_SECONDS"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			ttl = n
		}
	}
	h := ldap.NewHandler(ldap.HandlerConfig{
		Service:   svc,
		JWTSecret: secret,
		TokenTTL:  ttl,
		Logger:    log.Logger,
	})

	auth := r.Group("/api/v1/auth")
	auth.POST("/login", h.Login)
	auth.GET("/ldap/health", h.Health)

	log.Info("auth routes registered",
		"dev_bypass", cfg.LDAP.DevBypass,
		"dev_users", len(cfg.LDAP.DevUsers),
		"token_ttl_seconds", ttl,
	)
}

// buildLDAPClient selects the Fake or Real client based on the
// configured DevBypass flag. The function lives here so the main
// package is the only place that knows about the config flag.
func buildLDAPClient(cfg *config.Config) (ldap.Client, error) {
	if cfg.LDAP.DevBypass {
		f := ldap.NewFake()
		for _, u := range cfg.LDAP.DevUsers {
			groups := devRoleToGroups(u.Role)
			f.AddUser(u.Username, u.Password, "", groups)
		}
		return f, nil
	}
	return ldap.NewReal(ldap.RealConfig{
		URL:          cfg.LDAP.URL,
		BindDN:       cfg.LDAP.BindDN,
		BindPassword: cfg.LDAP.BindPassword,
		BaseDN:       cfg.LDAP.BaseDN,
		UserFilter:   cfg.LDAP.UserFilter,
		Timeout:      5 * time.Second,
	}), nil
}

// devRoleToGroups synthesises a small LDAP group list for a dev
// user so the role mapping in the service has something to match
// against. Production deployments configure the real group DNs
// in LDAPConfig.
func devRoleToGroups(role string) []string {
	switch role {
	case "SuperAdmin":
		return []string{"cn=SRE_Lead,ou=Groups,dc=example,dc=com"}
	case "Operator":
		return []string{"cn=IT_Ops,ou=Groups,dc=example,dc=com"}
	case "Developer":
		return []string{"cn=DevTeam_Payments,ou=Groups,dc=example,dc=com"}
	case "Auditor":
		return []string{"cn=Security_Auditors,ou=Groups,dc=example,dc=com"}
	default:
		return nil
	}
}

// registerProjectRoutes wires the project-hierarchy module onto the
// Gin engine. Phase 3 owns this registration. AutoMigrate registers
// ProjectType, Project, and ProjectMember; the service enforces the
// 3-level depth cap at create/update time.
func registerProjectRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db,
		&projectpkg.ProjectType{},
		&projectpkg.Project{},
		&projectpkg.ProjectMember{},
	); err != nil {
		log.Error("project AutoMigrate failed", "err", err)
		return
	}
	repo := projectpkg.NewRepository(db)
	svc := projectpkg.NewService(repo)
	h := projectpkg.NewHandler(svc, repo)
	v1 := r.Group("/api/v1")
	h.Register(v1)
	log.Info("project routes registered")
}

// registerDeviceRoutes wires the device-management module onto the
// Gin engine. AutoMigrate covers Device, DeviceGroup, and
// ConfigurationTemplate. The service enforces the 4-state model
// (online/monitoring_issue/offline/maintenance) and action rules.
func registerDeviceRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, devicepkg.AllModels()...); err != nil {
		log.Error("device AutoMigrate failed", "err", err)
		return
	}
	repo := devicepkg.NewRepository(db)
	svc := devicepkg.NewService(repo)
	h := devicepkg.NewHandler(svc)
	v1 := r.Group("/api/v1")
	h.Register(v1)

	// device-groups and configuration-templates have separate
	// sub-handlers with their own Register methods.
	groupRepo := devicepkg.NewGroupRepository(db)
	groupSvc := devicepkg.NewGroupService(groupRepo)
	devicepkg.NewGroupHandler(groupSvc).Register(v1)

	tmplRepo := devicepkg.NewTemplateRepository(db)
	tmplSvc := devicepkg.NewTemplateService(tmplRepo)
	devicepkg.NewTemplateHandler(tmplSvc).Register(v1)

	log.Info("device routes registered")
}

// registerPhysicalHostRoutes wires the physical-host monitoring module.
// The 4-state machine (online/monitoring_issue/offline/maintenance) and
// the maintenance mode with audit emission live here. Audit events are
// logged via a no-op emitter (Phase 7 will replace it with the
// audit-logging service).
func registerPhysicalHostRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, physicalhost.AllModels()...); err != nil {
		log.Error("physicalhost AutoMigrate failed", "err", err)
		return
	}
	repo := physicalhost.NewRepository(db)
	monitor := physicalhost.NewMonitorService(physicalhost.MonitorConfig{
		Repo:                repo,
		Prober:              physicalhost.NewFake(),
		ConsecutiveFailures: 3,
		CheckInterval:       time.Minute,
	})
	maint := physicalhost.NewMaintenanceService(physicalhost.MaintenanceConfig{
		Repo:    repo,
		Auditor: logAuditEmitter{log: log},
	})
	physicalhost.NewHandler(physicalhost.HandlerConfig{
		Repo:        repo,
		Monitor:     monitor,
		Maintenance: maint,
	}).Register(&r.RouterGroup)
	log.Info("physicalhost routes registered")
}

// logAuditEmitter is a no-op AuditEmitter that just logs the event.
// Phase 7 (audit-logging) will replace this with a real emitter.
type logAuditEmitter struct{ log *logger.Logger }

func (a logAuditEmitter) EmitMaintenanceEnter(_ context.Context, ev physicalhost.AuditEvent) {
	a.log.Info("physicalhost audit",
		"action", "enter_maintenance",
		"host_id", ev.HostID,
		"user_id", ev.UserID,
		"reason", ev.Reason,
		"at", ev.At,
	)
}

func (a logAuditEmitter) EmitMaintenanceExit(_ context.Context, ev physicalhost.AuditEvent) {
	a.log.Info("physicalhost audit",
		"action", "exit_maintenance",
		"host_id", ev.HostID,
		"user_id", ev.UserID,
		"at", ev.At,
	)
}

// registerDiscoveryRoutes wires the network-discovery module. Scanner
// and Prober are fakes in dev mode; production deployments swap them
// for real nmap/ICMP and SNMP impls.
func registerDiscoveryRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, discovery.AllModels()...); err != nil {
		log.Error("discovery AutoMigrate failed", "err", err)
		return
	}
	repo := discovery.NewRepository(db)
	devs := devicepkg.NewRepository(db)
	svc := discovery.NewService(repo, devs, discovery.NewFakeScanner(nil, nil), discovery.NewFakeProber(nil, nil))
	discovery.NewHandler(svc).Register(&r.RouterGroup)
	log.Info("discovery routes registered")
}

// registerK8sClusterRoutes wires the k8s-cluster-management module.
// The AES-256 key is loaded from K8S_CRYPTO_KEY env var; in dev a
// deterministic 32-byte key is used with a warning, mirroring the
// JWT-secret pattern.
func registerK8sClusterRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, k8s.AllModels()...); err != nil {
		log.Error("k8s AutoMigrate failed", "err", err)
		return
	}
	key := []byte(envOr("K8S_CRYPTO_KEY", "dev-k8s-crypto-key-32-bytes-long-xx"))
	if len(key) != 32 {
		log.Warn("K8S_CRYPTO_KEY is not 32 bytes; deriving via SHA-256 (dev only)", "len", len(key))
		h := sha256.Sum256(key)
		key = h[:]
	}
	repo := k8s.NewRepository(db)
	svc := k8s.NewService(repo, &k8s.FakeClient{}, key)
	k8s.NewHandler(svc).Register(&r.RouterGroup)
	log.Info("k8s cluster routes registered")
}

// registerHostProjectLinkRoutes wires the physical-host-project-linking
// module. The service walks the project hierarchy when listing devices
// for a project.
func registerHostProjectLinkRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, hostproject.AllModels()...); err != nil {
		log.Error("hostproject AutoMigrate failed", "err", err)
		return
	}
	repo := hostproject.NewRepository(db)
	projectRepo := projectpkg.NewRepository(db)
	projectSvc := projectpkg.NewService(projectRepo)
	svc := hostproject.NewService(repo, projectSvc)
	hostproject.NewHandler(svc).Register(&r.RouterGroup)
	log.Info("hostproject routes registered")
}

// registerPipelineRoutes wires the cicd-pipeline module. The Executor
// is the Local implementation (runs shell via os/exec); tests use
// Fake via direct construction in service_test.go.
func registerPipelineRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, pipeline.AllModels()...); err != nil {
		log.Error("pipeline AutoMigrate failed", "err", err)
		return
	}
	repo := pipeline.NewRepository(db)
	exec := pipeline.NewLocal(pipeline.WithMaxOutputBytes(1 << 20))
	svc := pipeline.NewService(repo, exec)
	pipeline.NewHandler(svc).Register(&r.RouterGroup)
	log.Info("pipeline routes registered")
}

// registerLogsRoutes wires the log-aggregation module. The backend
// is the Local filesystem reader in dev mode; production deployments
// swap to ES or Loki by changing cfg.Logs.Backend and instantiating
// the matching backend. The /capabilities endpoint always returns
// 200 (with a "unavailable" row if the backend is misconfigured).
func registerLogsRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	_ = db // logs module is read-only; no AutoMigrate needed
	backend := logs.NewLocal(logs.LocalConfig{
		Dir: envOr("LOG_STORAGE_DIR", "tests/fixtures/logs"),
	})
	svc := logs.NewService(backend, logs.ServiceConfig{})
	logs.NewHandler(svc, backend).Register(&r.RouterGroup)
	log.Info("logs routes registered", "backend", "local")
}

// registerMetricsRoutes wires the metrics-collection module. The
// scraper is a no-op Fake in dev; production swaps in the
// PrometheusScraper (injected with an HTTPClient). The metrics
// middleware (metrics.Middleware) is exposed for main.go to
// install as a global Gin middleware in a follow-up.
func registerMetricsRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, metrics.AllModels()...); err != nil {
		log.Error("metrics AutoMigrate failed", "err", err)
		return
	}
	repo := metrics.NewRepository(db)
	svc := metrics.NewService(repo, metrics.NewFakeScraper())
	metrics.NewHandler(svc).Register(&r.RouterGroup)
	log.Info("metrics routes registered")
}

// registerAlertsRoutes wires the alert-notification module. The
// dispatcher is the LogDispatcher (writes to slog); the suppression
// checker is a no-op (returns false) in dev. Production wires the
// real physicalhost service into a DefaultSuppressionChecker.
func registerAlertsRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, alerts.AllModels()...); err != nil {
		log.Error("alerts AutoMigrate failed", "err", err)
		return
	}
	repo := alerts.NewRepository(db)
	svc := alerts.NewService(alerts.ServiceConfig{
		Repo:        repo,
		Dispatcher:  alerts.NewLogDispatcher(log.Logger),
		Suppression: alerts.NewFakeSuppressionChecker(),
		Logger:      log.Logger,
	})
	alerts.NewHandler(svc).Register(&r.RouterGroup)
	log.Info("alerts routes registered")
}
