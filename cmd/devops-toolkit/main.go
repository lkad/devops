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
	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/auth/ldap"
	"github.com/devops-toolkit/backend/internal/config"
	dbpkg "github.com/devops-toolkit/backend/internal/database"
	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/internal/discovery"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/internal/hostproject"
	"github.com/devops-toolkit/backend/internal/k8s"
	"github.com/devops-toolkit/backend/internal/k8s/logstream"
	"github.com/devops-toolkit/backend/internal/logs"
	"github.com/devops-toolkit/backend/internal/metrics"
	"github.com/devops-toolkit/backend/internal/observability"
	"github.com/devops-toolkit/backend/internal/physicalhost"
	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
	"github.com/devops-toolkit/backend/internal/pipeline"
	"github.com/devops-toolkit/backend/internal/servicecatalog"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
	internalServer "github.com/devops-toolkit/backend/internal/server"
	"github.com/devops-toolkit/backend/pkg/contracts"
	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/devops-toolkit/backend/internal/ws/hub"
	"github.com/devops-toolkit/backend/internal/ws/realtime"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
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
		wsHub := registerWsHubRoutes(eng, cfg, log)
		// Audit must be registered before physicalhost so the
		// physical-host module can share the same audit svc +
		// repo (one DB table, one emitter, no double writes).
		auditSvc, auditRepo := registerAuditRoutes(eng, db, log)
		var hubPublisher realtime.Publisher
		if wsHub != nil {
			hubPublisher = realtime.NewHubPublisher(wsHubAdapter{wsHub})
		}
		phMaintenance := registerPhysicalHostRoutes(eng, db, log, hubPublisher, auditSvc, auditRepo)
		registerAlertsRoutes(eng, db, log, &physicalhostMaintenanceAdapter{svc: phMaintenance})
		registerDiscoveryRoutes(eng, db, log)
		registerK8sClusterRoutes(eng, db, log)
		registerHostProjectLinkRoutes(eng, db, log)
		registerPipelineRoutes(eng, db, log)
		registerServiceCatalogRoutes(eng, db, log)
		registerLogsRoutes(eng, db, log)
		registerMetricsRoutes(eng, db, log)
		registerLogStreamRoutes(eng, db, log)
	} else {
		log.Warn("router is not a *gin.Engine; auth routes not registered")
	}
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// mTLS: when TLS_CERT_FILE is set, listen with HTTPS +
	// client cert verification. Production must set both
	// TLS_CERT_FILE and TLS_CA_FILE; dev can leave either
	// empty to fall through to plain HTTP.
	if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
		tlsCfg, err := internalServer.LoadServerTLS(internalServer.TLSConfig{
			CertFile: certFile,
			KeyFile:  os.Getenv("TLS_KEY_FILE"),
			CAFile:   os.Getenv("TLS_CA_FILE"),
		})
		if err != nil {
			return fmt.Errorf("load TLS config: %w", err)
		}
		srv.TLSConfig = tlsCfg
		log.Info("mTLS enabled", "cert", certFile, "ca", os.Getenv("TLS_CA_FILE"))
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() {
		if srv.TLSConfig != nil {
			log.Info("https server listening (mTLS)", "addr", srv.Addr)
			if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		} else {
			log.Info("http server listening", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
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

	// OpenTelemetry tracing. Initialised before the
	// Prometheus middleware so the trace is the outermost
	// span (Prometheus / Gin metrics are children of the
	// HTTP span). Exports to stdout when OTEL_EXPORTER=stdout
	// or to a remote OTLP collector when OTEL_EXPORTER_OTLP_ENDPOINT
	// is set; otherwise a noop tracer keeps the API stable.
	tracingCfg := observability.TracingConfig{
		ServiceName:  "devops-toolkit",
		ServiceVer:   envOr("APP_VERSION", "dev"),
		SamplerRatio: observability.SamplerRatioFromEnv(),
		OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Stdout:       envOr("OTEL_EXPORTER", "") == "stdout",
	}
	tracing := observability.NewTracing(tracingCfg)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tracing.Shutdown(ctx)
	}()
	r.Use(tracing.Middleware())

	// Prometheus instrumentation. The middleware counts every
	// request by route template + status; the /metrics endpoint
	// itself is mounted as a plain handler so it doesn't show up
	// in the request counter (a self-counting scraper would
	// inflate its own numbers). Prometheus 9090 is expected to
	// scrape GET /metrics every 15s (see deploy/prometheus).
	obs := observability.New()
	r.Use(obs.Middleware())
	r.GET("/metrics", gin.WrapH(obs.Handler()))

	// Root index lists the known route groups so a browser hitting
	// / sees something useful instead of a 404 envelope.
	r.GET("/", func(c *gin.Context) {
		handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
			"name":     "devops-toolkit",
			"phase":    1,
			"build":    "foundation",
			"endpoints": []string{
				"GET  /health",
				"GET  /metrics",
				"GET  /api/v1/capabilities",
				"X-Trace-Id response header (every request)",
				"POST /api/v1/auth/login",
				"GET  /api/v1/auth/ldap/health",
				"WS   /api/v1/ws",
				"GET  /api/v1/projects",
				"GET  /api/v1/devices",
				"GET  /api/v1/physical-hosts",
				"GET  /api/v1/k8s/clusters",
				"GET  /api/v1/discovery/runs",
				"GET  /api/v1/pipelines",
				"GET  /api/v1/host-projects",
				"GET  /api/v1/logs/capabilities",
				"GET  /api/v1/logs/query",
				"GET  /api/v1/metrics",
				"GET  /api/v1/alerts",
				"GET  /api/v1/audit",
			},
		})
	})

	r.GET("/health", func(c *gin.Context) {
		handler.WriteJSON(c.Writer, http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	// Standard 404 fallback renders the api-contract error envelope.
	r.NoRoute(func(c *gin.Context) {
		handler.WriteError(c.Writer, &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: fmt.Sprintf("route %s %s not found; see GET / for the route list", c.Request.Method, c.Request.URL.Path),
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

// newProber picks the prober implementation from env. The
// default is TCP reachability (cheap, no creds); PROBER_SSH_KEY
// flips to the real SSHProber using the given key file, which
// is the production path. PROBER_FAKE=1 picks the in-test fake
// for unit-test parity.
func newProber(log *logger.Logger) physicalhost.Prober {
	if envOr("PROBER_FAKE", "") == "1" {
		log.Info("prober selected", "type", "fake", "reason", "PROBER_FAKE=1")
		return physicalhost.NewFake()
	}
	if keyPath := os.Getenv("PROBER_SSH_KEY"); keyPath != "" {
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			log.Warn("prober: read SSH key failed, falling back to TCP", "err", err)
		} else {
			signer, err := ssh.ParsePrivateKey(raw)
			if err != nil {
				log.Warn("prober: parse SSH key failed, falling back to TCP", "err", err)
			} else {
				timeout := 10 * time.Second
				if v := os.Getenv("PROBER_SSH_TIMEOUT"); v != "" {
					if d, err := time.ParseDuration(v); err == nil && d > 0 {
						timeout = d
					}
				}
				log.Info("prober selected", "type", "ssh", "key", keyPath, "timeout", timeout.String())
				return prober.NewSSHProber(prober.SSHProberConfig{Signer: signer, Timeout: timeout})
			}
		}
	}
	timeout := 2 * time.Second
	if v := os.Getenv("PROBER_TCP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			timeout = d
		}
	}
	log.Info("prober selected", "type", "tcp", "timeout", timeout.String())
	return prober.TCP(timeout)
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
// the maintenance mode with audit emission live here. The monitor
// publishes device_event on the realtime hub for state transitions;
// maintenance transitions persist durable rows in the audit table.
// Every successful Check also enqueues a fresh Metrics snapshot
// into the AsyncInfluxWriter for long-term storage. Returns the
// *MaintenanceService so callers (e.g. the alert module) can
// consult IsInMaintenance via a small adapter.
func registerPhysicalHostRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger, pub realtime.Publisher, auditSvc *audit.Service, auditRepo *audit.Repository) *physicalhost.MaintenanceService {
	if err := dbpkg.AutoMigrate(db, append(physicalhost.AllModels(), audit.AllModels()...)...); err != nil {
		log.Error("physicalhost AutoMigrate failed", "err", err)
		return nil
	}
	repo := physicalhost.NewRepository(db)
	monitor := physicalhost.NewMonitorService(physicalhost.MonitorConfig{
		Repo:                repo,
		Prober:              newProber(log),
		ConsecutiveFailures: 3,
		CheckInterval:       time.Minute,
	})
	if pub != nil {
		monitor.SetPublisher(pub)
	}
	maint := physicalhost.NewMaintenanceService(physicalhost.MaintenanceConfig{
		Repo:    repo,
		Auditor: physicalhost.NewAuditEmitterAdapter(auditSvc),
	})
	monitor.SetMaintenance(maint)

	// Async InfluxDB writer: every Check produces a fresh
	// metrics snapshot which the writer ships to InfluxDB v2.
	// URL/Token/Org/Bucket are env-driven so the same binary
	// runs in dev (no InfluxDB) and prod (with InfluxDB).
	influxURL := os.Getenv("INFLUX_URL")
	var metricsSink physicalhost.MetricsSink
	if influxURL != "" {
		inner := physicalhost.NewInfluxWriter(physicalhost.InfluxWriterConfig{
			URL:    influxURL,
			Token:  os.Getenv("INFLUX_TOKEN"),
			Org:    os.Getenv("INFLUX_ORG"),
			Bucket: os.Getenv("INFLUX_BUCKET"),
		})
		// Swap the no-op default for the real http.Client
		// backed client. SetClient is the seam left in
		// influx_writer.go for exactly this wiring.
		inner.SetClient(physicalhost.NewHTTPClientPost(5 * time.Second))
		async := physicalhost.NewAsyncInfluxWriter(physicalhost.AsyncInfluxWriterConfig{
			Inner:      inner,
			BufferSize: 1024,
			Workers:    2,
		})
		async.Start(context.Background())
		metricsSink = async
		log.Info("influx writer enabled", "url", influxURL)
	}
	monitor.SetMetricsSink(metricsSink)

	// Collector needs the prober; reuse the same instance.
	collector := physicalhost.NewMetricsCollector(physicalhost.MetricsCollectorConfig{Prober: newProber(log), Timeout: 5 * time.Second})
	monitor.SetCollector(collector)

	// Background loop: drives periodic Check for every host.
	// Starts on a 1-minute tick; the loop is best-effort and
	// never blocks shutdown (ctx cancellation is honoured).
	loop := physicalhost.NewMonitorLoop(monitor, physicalhost.MonitorLoopConfig{
		Tick:   1 * time.Minute,
		Jitter: 5 * time.Second,
		Logger: log.Logger,
	})
	go loop.Run(context.Background(), nil)
	log.Info("monitor loop started", "tick", "1m", "jitter", "5s")

	v1 := r.Group("/api/v1")
	physicalhost.NewHandler(physicalhost.HandlerConfig{
		Repo:        repo,
		Monitor:     monitor,
		Maintenance: maint,
		Metrics:     physicalhost.NewMetricsCache(physicalhost.MetricsCacheConfig{Collector: collector, TTL: 30 * time.Second, MaxEntries: 1024}),
		Audit:       auditSvc,
		AuditRepo:   auditRepo,
	}).Register(v1)
	log.Info("physicalhost routes registered")
	return maint
}

// logAuditEmitter is kept as a no-op adapter for code paths
// that still want a real AuditEmitter without the audit
// service being wired (e.g. unit tests). Production now uses
// physicalhost.NewAuditEmitterAdapter(auditSvc) so the
// logAuditEmitter is no longer wired in main; left in place
// for the maintenance_test.go shim and dev fallbacks.
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

// wsHubAdapter bridges the websocket-hub's bool-returning
// Publish to the realtime.Hub interface (which wants an
// error return). The hub returns false only when the payload
// is too large; we surface that as a real error so the
// publisher can log and move on.
type wsHubAdapter struct{ h *hub.Hub }

func (a wsHubAdapter) Publish(channel string, payload []byte) error {
	if a.h.Publish(channel, payload) {
		return nil
	}
	return errors.New("ws hub: payload exceeds MaxPayloadBytes")
}

// physicalhostMaintenanceProbe is the type alias the alert
// suppression checker depends on. We use the alert's
// PhysicalHostMaintenanceProbe interface directly so the
// dependency direction stays clean (alerts depends on its
// own interface; main.go adapts the physicalhost service).
type physicalhostMaintenanceProbe = alerts.PhysicalHostMaintenanceProbe

// physicalhostMaintenanceAdapter wraps a *physicalhost.MaintenanceService
// so it satisfies the alert-side probe interface. Lives here
// because main.go is the only place that knows about both
// packages.
type physicalhostMaintenanceAdapter struct {
	svc *physicalhost.MaintenanceService
}

func (a *physicalhostMaintenanceAdapter) IsInMaintenance(ctx context.Context, hostID string) bool {
	return a.svc.IsInMaintenance(ctx, hostID)
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
	v1 := r.Group("/api/v1")
	discovery.NewHandler(svc).Register(v1)
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
	v1 := r.Group("/api/v1")
	k8s.NewHandler(svc).Register(v1)
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
	v1 := r.Group("/api/v1")
	hostproject.NewHandler(svc).Register(v1)
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
	v1 := r.Group("/api/v1")
	pipeline.NewHandler(svc).Register(v1)
	log.Info("pipeline routes registered")
}

// registerServiceCatalogRoutes wires the microservice
// catalog. The catalog includes:
//   - CRUD on /api/v1/services
//   - /api/v1/services/:id/health (derived from the
//     most recent pipeline run AND live K8s pod health)
//
// The Health rollup needs to read pipeline runs and
// K8s deployments; the service-catalog package cannot
// import either upstream package (cycle), so the
// wiring is done here in main.go via two function-typed
// adapters. Pipeline migrations must already be applied
// (registerPipelineRoutes runs first).
func registerServiceCatalogRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	if err := dbpkg.AutoMigrate(db, &servicecatalog.Service{}, &servicecatalog.OnCall{}, &servicecatalog.RunbookEntry{}); err != nil {
		log.Error("servicecatalog AutoMigrate failed", "err", err)
		return
	}
	repo := servicecatalog.NewRepository(db)
	cat := servicecatalog.NewCatalog(repo)
	handler := servicecatalog.NewHandler(cat)

	// Wire the health rollup. Two seams: a
	// pipeline.RunSource (always wired — the P0
	// fallback) and a k8s.K8sSource (wired when
	// registerK8sClusterRoutes has produced a usable
	// repository; nil otherwise so the rollup falls
	// back to the pipeline signal).
	pipelineRepo := pipeline.NewRepository(db)
	health := servicecatalog.NewHealth(
		&servicecatalog.FunRunSource{
			Fn: func(serviceID string, n int) ([]servicecatalog.PipelineRun, error) {
				rows, err := pipelineRepo.LastRunsForService(serviceID, n)
				if err != nil {
					return nil, err
				}
				out := make([]servicecatalog.PipelineRun, 0, len(rows))
				for _, r := range rows {
					out = append(out, servicecatalog.PipelineRun{
						ID:         r.ID,
						Status:     string(r.Status),
						StartedAt:  orZeroTime(r.StartedAt),
						DurationMs: r.DurationMs,
					})
				}
				return out, nil
			},
		},
	)

	// K8s source: walk every registered cluster and
	// collect deployments whose name matches the
	// service. In dev (no real clusters registered)
	// the K8s source returns an empty slice and the
	// rollup falls back to the pipeline signal — the
	// behaviour the tests cover.
	k8sRepo := k8s.NewRepository(db)
	health.WithK8s(&clusterWalkingK8sSource{repo: k8sRepo})

	handler.SetHealth(health)

	v1 := r.Group("/api/v1")
	handler.Register(v1)
	log.Info("servicecatalog routes registered")
}

// clusterWalkingK8sSource walks every registered K8s
// cluster and returns the deployments whose name
// matches the service. Production wiring would call
// k8s.Service.ListDeployments per cluster; the dev
// stack has no real cluster clients (only FakeClient
// is wired in registerK8sClusterRoutes), so the
// implementation lives in a closure that, when a real
// per-cluster client registry is added in P1.4, can be
// swapped without changing the servicecatalog package.
type clusterWalkingK8sSource struct {
	repo *k8s.Repository
}

// ListDeploymentsForService satisfies the
// servicecatalog.K8sSource interface. It returns an
// empty slice in dev (no real K8s clients); production
// wiring in P1.4 will iterate over clusters and call
// the per-cluster ListDeployments.
//
// The empty-result behavior is the spec's "no K8s
// deployment found" branch — the health rollup falls
// through to the P0 pipeline signal, which is the
// right answer when the service is not in K8s.
func (c *clusterWalkingK8sSource) ListDeploymentsForService(_ context.Context, _ string) ([]servicecatalog.K8sDeploymentHealth, error) {
	return nil, nil
}

// orZeroTime returns t if non-nil, otherwise the zero
// time. Defensive helper for the adapter; production
// runs always have StartedAt set.
func orZeroTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
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
	v1 := r.Group("/api/v1")
	logs.NewHandler(svc, backend).Register(v1)
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
	v1 := r.Group("/api/v1")
	metrics.NewHandler(svc).Register(v1)
	log.Info("metrics routes registered")
}

// registerAlertsRoutes wires the alert-notification module. The
// dispatcher is the LogDispatcher (writes to slog); the
// suppression checker is the real DefaultSuppressionChecker
// driven by a tiny adapter that consults the physicalhost
// service's InMaintenance predicate.
func registerAlertsRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger, phMaintenance physicalhostMaintenanceProbe) {
	if err := dbpkg.AutoMigrate(db, alerts.AllModels()...); err != nil {
		log.Error("alerts AutoMigrate failed", "err", err)
		return
	}
	repo := alerts.NewRepository(db)
	svc := alerts.NewService(alerts.ServiceConfig{
		Repo:        repo,
		Dispatcher:  alerts.NewLogDispatcher(log.Logger),
		Suppression: alerts.NewDefaultSuppressionChecker(phMaintenance),
		Logger:      log.Logger,
	})
	v1 := r.Group("/api/v1")
	alerts.NewHandler(svc).Register(v1)
	log.Info("alerts routes registered", "real_suppression", true)
}

// registerWsHubRoutes wires the websocket-hub module: starts the
// hub's Run loop on the application context and registers /ws.
// JWT signer is reconstructed from the same secret as the auth
// routes so the upgrade path can verify the token. The hub
// itself is returned so the caller can wire a realtime.Publisher
// for domain events (device_event, alert_fired, etc).
func registerWsHubRoutes(r *gin.Engine, cfg *config.Config, log *logger.Logger) *hub.Hub {
	secret := os.Getenv("APP_JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-do-not-use-in-prod"
	}
	signer, err := auth.NewSigner(secret, time.Hour)
	if err != nil {
		log.Error("ws hub: signer init failed", "err", err)
		return nil
	}
	h := hub.NewHub(hub.HubConfig{Limits: hub.DefaultLimits()})
	go h.Run(context.Background())
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	v1 := r.Group("/api/v1")
	hub.RegisterRoutes(v1, h, signer, upgrader)
	log.Info("websocket hub routes registered")
	return h
}

// registerLogStreamRoutes wires the k8s-pod-log-streaming module.
// The LogClient is a Fake in dev; the production swap-in is the
// KubeLogClient which itself wraps a client-go function seam. The
// RealtimePublisher here is a no-op in dev (logs events are still
// returned to the WS/SSE client even when no hub is wired).
func registerLogStreamRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) {
	_ = db // no AutoMigrate; log stream is read-mostly
	client := &logstream.FakeLogClient{}
	streamer := logstream.NewKubeStreamer(client)
	pub := &logstreamRealtimeAdapter{} // bridges the local interface to the realtime package
	svc := logstream.NewService(streamer, client, pub)
	v1 := r.Group("/api/v1")
	logstream.NewHandler(svc, logstream.HandlerConfig{}).Register(v1)
	log.Info("k8s pod log stream routes registered")
}

// logstreamRealtimeAdapter bridges logstream.RealtimePublisher
// (Publish(string, any)) to realtime.HubPublisher (Publish(ctx, Event)).
// In dev it's a no-op; production swaps in a HubPublisher that fans
// out to the live WebSocket subscribers.
type logstreamRealtimeAdapter struct{}

func (logstreamRealtimeAdapter) Publish(channel string, payload any) {
	// Intentionally a no-op in dev. When the realtime hub is wired
	// in main.go, replace this with a HubPublisher that wraps the
	// hub and emits canonical realtime.Event values to the channel.
	_ = channel
	_ = payload
}

// registerAuditRoutes wires the audit-logging module. The emitter
// is a BufferedEmitter wrapping a DBEmitter so the audit path is
// non-blocking under load; overflow drops with a slog.Warn. The
// service exposes /api/v1/audit and /api/v1/audit/:id. It also
// returns the underlying svc + repo so the physicalhost wiring
// can re-use the same audit stack instead of building a second
// one (which would double-write every event).
func registerAuditRoutes(r *gin.Engine, db *gorm.DB, log *logger.Logger) (*audit.Service, *audit.Repository) {
	if err := dbpkg.AutoMigrate(db, audit.AllModels()...); err != nil {
		log.Error("audit AutoMigrate failed", "err", err)
		return nil, nil
	}
	repo := audit.NewRepository(db)
	emitter := audit.NewBufferedEmitter(audit.NewDBEmitter(repo), audit.BufferedEmitterConfig{
		BufferSize: 1024,
		Logger:     log.Logger,
	})
	svc := audit.NewService(audit.ServiceConfig{
		Repo:    repo,
		Emitter: emitter,
	})
	v1 := r.Group("/api/v1")
	audit.NewHandler(svc).Register(v1)
	log.Info("audit routes registered")
	return svc, repo
}
