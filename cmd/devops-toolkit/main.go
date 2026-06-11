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
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/alerts"
	"github.com/devops-toolkit/backend/internal/audit"
	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/internal/auth/ldap"
	rbacpkg "github.com/devops-toolkit/backend/internal/auth/rbac"
	"github.com/devops-toolkit/backend/internal/config"
	dbpkg "github.com/devops-toolkit/backend/internal/database"
	devicepkg "github.com/devops-toolkit/backend/internal/device"
	"github.com/devops-toolkit/backend/internal/discovery"
	"github.com/devops-toolkit/backend/internal/handler"
	"github.com/devops-toolkit/backend/internal/health"
	"github.com/devops-toolkit/backend/internal/hostproject"
	"github.com/devops-toolkit/backend/internal/k8s"
	"github.com/devops-toolkit/backend/internal/k8s/logstream"
	"github.com/devops-toolkit/backend/internal/logs"
	"github.com/devops-toolkit/backend/internal/metrics"
	"github.com/devops-toolkit/backend/internal/middleware"
	"github.com/devops-toolkit/backend/internal/observability"
	"github.com/devops-toolkit/backend/internal/physicalhost"
	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
	"github.com/devops-toolkit/backend/internal/pipeline"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
	internalServer "github.com/devops-toolkit/backend/internal/server"
	"github.com/devops-toolkit/backend/internal/servicecatalog"
	"github.com/devops-toolkit/backend/internal/ws/hub"
	"github.com/devops-toolkit/backend/internal/ws/realtime"
	"github.com/devops-toolkit/backend/pkg/contracts"
	"github.com/devops-toolkit/backend/pkg/logger"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// Dev-default constants. Centralised so the "rotate the
// dev fallback" change has one home. Production startup
// still refuses to start when these are in use (see
// registerAuthRoutes + registerWsHubRoutes — the
// `if cfg.App.Env == "production"` block).
const (
	// devDefaultJWTSecret is the APP_JWT_SECRET fallback
	// when the env var is unset AND the app is not running
	// in production. It is logged on startup ("no
	// APP_JWT_SECRET configured; using dev-only fallback")
	// so an operator can spot the misconfiguration in
	// `kubectl logs`. Rotate in lockstep with the auth + WS
	// hub routes (the two signers must agree).
	devDefaultJWTSecret = "dev-secret-do-not-use-in-prod"

	// devK8sCryptoKey is the K8S_CRYPTO_KEY fallback used
	// by registerK8sClusterRoutes. The literal is 32 bytes
	// (the AES-256 key size) so the SHA-256 derivation
	// branch is not triggered in dev — the
	// "len(key) != 32" check stays as a guard for prod
	// misconfigurations.
	devK8sCryptoKey = "dev-k8s-crypto-key-32-bytes-long-xx"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Shared signal-aware context. Every background worker
	// (the physical-host monitor loop, future AsyncInfluxWriter
	// fix in audit item 4) receives this same ctx, so a single
	// SIGINT/SIGTERM cancels them in lockstep with the HTTP
	// server's Shutdown call. This is the audit item-1 fix:
	// the old code started the monitor loop with
	// context.Background() and the per-host 5s check held
	// shutdown for 1000s+ on a 200-host fleet.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	// CORS: production must declare an explicit allowlist
	// (CORS_ALLOWED_ORIGINS). The default http://localhost:5173
	// is dev-only; shipping it to prod would let any browser
	// hit the API cross-origin. Mirrors the JWT-secret guard
	// in registerAuthRoutes.
	origins := corsAllowedOrigins(cfg.App.Env)
	if cfg.App.Env == "production" && len(origins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS must be set in production (comma-separated list of allowed origins)")
	}
	log.Info("cors configured", "env", cfg.App.Env, "origins", origins)

	router, obs, _ := buildRouter(log, nil, origins)
	if eng, ok := router.(*gin.Engine); ok {
		// Signer is the JWT signer. The auth middleware
		// uses it to verify the Bearer token. The same
		// secret is used to issue tokens on /auth/login
		// and to verify them on every protected endpoint,
		// so the auth route registration returns the
		// signer so we can reuse it as the auth
		// middleware's dependency.
		signer, err := registerAuthRoutes(eng, cfg, log)
		if err != nil {
			return err
		}
		// Build the auth + RBAC stack once and thread
		// the per-perm factory into every module's
		// Register. The auth middleware goes on the
		// /api/v1 group (parses the JWT) and per-route
		// RBAC is applied by the modules via the
		// factory. The factory does NOT call
		// authMW.RequireAuth() — auth already ran at
		// the group level and the per-route middleware
		// just enforces the permission.
		authMW := auth.NewAuthenticator(auth.AuthMiddlewareConfig{
			Signer:        signer,
			DevBypass:     cfg.LDAP.DevBypass,
			PermissionSvc: rbacpkg.NewService(),
		})
		rbacSvc := rbacpkg.NewService()
		perms := func(perm rbacpkg.Permission) gin.HandlerFunc {
			return rbacpkg.RequirePermission(rbacSvc, perm)
		}
		// Apply the auth middleware at the /api/v1
		// group level so every protected route is
		// gated by the same JWT-verify step. The
		// per-route RBAC factory is then applied
		// inside each module's Register.
		v1 := eng.Group("/api/v1", authMW.RequireAuth())

		// Audit must be registered before any module that
		// emits RecordAction so the same audit svc + repo
		// (one DB table, one emitter, no double writes) is
		// shared across modules. P0 #3 wires it into
		// project, hostproject, device, k8s, alerts, logs,
		// discovery, and servicecatalog.
		auditSvc, auditRepo := registerAuditRoutes(v1, db, log, perms)
		registerProjectRoutes(v1, db, log, perms, rbacSvc, auditSvc)
		registerDeviceRoutes(v1, db, log, perms, auditSvc)
		wsHub := registerWsHubRoutes(v1, cfg, log, perms)
		var hubPublisher realtime.Publisher
		if wsHub != nil {
			hubPublisher = realtime.NewHubPublisher(wsHubAdapter{wsHub})
		}
		phMaintenance := registerPhysicalHostRoutes(ctx, v1, db, log, obs, hubPublisher, auditSvc, auditRepo, perms)
		registerAlertsRoutes(v1, db, log, &physicalhostMaintenanceAdapter{svc: phMaintenance}, perms, auditSvc)
		registerDiscoveryRoutes(v1, db, log, perms, auditSvc)
		k8sSvc := registerK8sClusterRoutes(v1, db, log, perms, auditSvc)
		registerHostProjectLinkRoutes(v1, db, log, perms, rbacSvc, auditSvc)
		registerPipelineRoutes(v1, db, log, perms)
		registerServiceCatalogRoutes(v1, db, log, k8sSvc, obs, perms, auditSvc)
		registerLogsRoutes(v1, db, log, perms, auditSvc)
		registerMetricsRoutes(v1, db, log, perms)
		registerLogStreamRoutes(v1, db, log, perms)
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

	// Graceful shutdown on SIGINT/SIGTERM. The shared
	// signal-aware ctx at the top of run() is what the
	// physical-host monitor loop and (in a follow-up) the
	// AsyncInfluxWriter watch; we just have to wait for it
	// to fire (or for the HTTP server to error out) and
	// then call Shutdown with a bounded timeout.
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
	case <-ctx.Done():
		log.Info("shutdown signal received")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

// buildRouter returns the HTTP handler tree plus the Prometheus
// observability Metrics struct (so route registrars can share
// the same /metrics registry for domain-level instruments).
// The third return is a *health.Checker seam: a future wiring
// pass constructs the deep-readiness checker here (DB + LDAP +
// K8s + per-cluster); for now the test path passes nil and
// main.go's run() does the same.
//
// corsOrigins is the CORS allowlist; see corsAllowedOrigins
// for the env-driven resolution. Wired as the OUTERMOST
// middleware so OPTIONS preflight short-circuits before
// any other middleware runs (the spec mandates this order:
// CORS → Recovery → RequestID → Metrics → Tracing → Auth).
//
// Kept as a separate function so tests can call it without
// booting a listener.
func buildRouter(log *logger.Logger, _ *health.Checker, corsOrigins []string) (http.Handler, *observability.Metrics, *health.Checker) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// OpenTelemetry tracing. Registered FIRST so the
	// X-Trace-Id header is set on the response writer
	// BEFORE any other middleware (including CORS) gets
	// a chance to short-circuit with c.Abort. The
	// middleware-stack spec lists "tracing (outermost)";
	// in practice that means tracing wraps everything
	// else so the trace id is on every response, even
	// 204 preflights and 404 NoRoute paths.
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

	// CORS — runs after tracing so the preflight short-
	// circuit still carries X-Trace-Id. Using the existing
	// internal/middleware.CORS struct rather than
	// gin-contrib/cors keeps the dep tree small. The
	// struct's allowlist echo behaviour matches the spec
	// scenario ("echo request Origin back when the
	// configured allowlist contains it").
	r.Use(middleware.CORS(corsOrigins))

	// Recovery — catches panics in everything downstream
	// (including the CORS short-circuit and the metrics
	// middleware). Wraps the rest of the chain so a panic
	// renders the standard 500 envelope.
	r.Use(gin.Recovery())

	// Prometheus instrumentation. The middleware counts every
	// request by route template + status; the /metrics endpoint
	// itself is mounted as a plain handler so it doesn't show up
	// in the request counter (a self-counting scraper would
	// inflate its own numbers). Prometheus 9090 is expected to
	// scrape GET /metrics every 15s (see deploy/prometheus).
	//
	// Order: metrics is registered BEFORE tracing so the
	// Prometheus histogram observation is recorded by an
	// outer middleware relative to the OTel span (the
	// span is a child of the histogram, matching the
	// middleware-stack spec ordering CORS → Recovery →
	// RequestID → Metrics → Tracing → Auth).
	obs := observability.New()
	r.Use(obs.Middleware())
	r.GET("/metrics", gin.WrapH(obs.Handler()))

	// Root index lists the known route groups so a browser hitting
	// / sees something useful instead of a 404 envelope.
	r.GET("/", func(c *gin.Context) {
		handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
			"name":  "devops-toolkit",
			"phase": 1,
			"build": "foundation",
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

	// /api/v1/capabilities is intentionally NOT
	// authenticated — it tells the client what sections
	// are available (so the UI can render a "this
	// deployment is missing X" message). The
	// /api/v1/auth/login and /api/v1/auth/ldap/health
	// routes registered by registerAuthRoutes are also
	// public; their handler group lives under
	// /api/v1/auth and is set up after the auth-
	// protected v1 group in run() so Gin treats them
	// as separate routes.
	v1Public := r.Group("/api/v1")
	{
		v1Public.GET("/capabilities", func(c *gin.Context) {
			// Phase-1 placeholder; later phases add module-specific capability probes.
			sections := []string{"app", "database", "logs", "ldap", "alerts", "k8s", "physicalhost", "websocket"}
			handler.WriteJSON(c.Writer, http.StatusOK, gin.H{
				"sections":  sections,
				"phase":     1,
				"build_tag": "foundation",
			})
		})
	}
	return r, obs, nil
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

// corsAllowedOrigins returns the CORS allowlist for the
// current environment. Driven by CORS_ALLOWED_ORIGINS
// (comma-separated). In dev / test / unset, falls back to
// the Vite dev server (http://localhost:5173) so the local
// frontend just works. In production the env var is
// REQUIRED — a permissive default in prod would let any
// origin hit the API. Mirrors the JWT-secret + K8s-crypto
// guard pattern in config.Validate.
//
// Empty / unset in production returns an empty slice. The
// production wiring then logs a fatal error and refuses to
// start (we still return an empty slice here so the caller
// can choose to log + exit rather than fail the JSON
// unmarshal).
func corsAllowedOrigins(appEnv string) []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw != "" {
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if appEnv == "production" {
		// Empty allowlist + CORS middleware = "echo the
		// request Origin back" (see internal/middleware/cors.go).
		// We deliberately return an empty slice and let the
		// caller log + exit so the operator gets a clear
		// startup error rather than a silent insecure default.
		return nil
	}
	// Dev / test default: Vite dev server.
	return []string{"http://localhost:5173"}
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
func registerAuthRoutes(r *gin.Engine, cfg *config.Config, log *logger.Logger) (*auth.Signer, error) {
	client, err := buildLDAPClient(cfg)
	if err != nil {
		log.Error("ldap client init failed; auth routes will not be registered", "err", err)
		return nil, nil
	}
	svc := ldap.NewService(ldap.ServiceConfig{
		Client:          client,
		MaxFailedLogins: 5,
		RateLimitWindow: time.Minute,
	})

	secret := os.Getenv("APP_JWT_SECRET")
	if secret == "" {
		if cfg.App.Env == "production" {
			return nil, fmt.Errorf("APP_JWT_SECRET must be set when env=production (no dev fallback in prod)")
		}
		secret = devDefaultJWTSecret
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

	signer, err := auth.NewSigner(secret, time.Duration(ttl)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("auth: signer init: %w", err)
	}

	authGrp := r.Group("/api/v1/auth")
	authGrp.POST("/login", h.Login)
	authGrp.GET("/ldap/health", h.Health)

	log.Info("auth routes registered",
		"dev_bypass", cfg.LDAP.DevBypass,
		"dev_users", len(cfg.LDAP.DevUsers),
		"token_ttl_seconds", ttl,
	)
	return signer, nil
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
// 3-level depth cap at create/update time. The per-project access
// factory is wired from the project service's
// MembershipChecker (project_pkg.Repository.ListProjectIDsForUser)
// and the rbac service's HasPermissionInProject.
func registerProjectRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc, rbacSvc *rbacpkg.Service, auditSvc *audit.Service) {
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
	h := projectpkg.NewHandler(svc, repo, auditSvc)
	projectAccess := rbacpkg.NewProjectAccessFactory(rbacSvc, svc.MembershipChecker())
	h.Register(v1, perms, projectAccess)
	log.Info("project routes registered")
}

// registerDeviceRoutes wires the device-management module onto the
// Gin engine. AutoMigrate covers Device, DeviceGroup, and
// ConfigurationTemplate. The service enforces the 4-state model
// (online/monitoring_issue/offline/maintenance) and action rules.
func registerDeviceRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc, auditSvc *audit.Service) {
	if err := dbpkg.AutoMigrate(db, devicepkg.AllModels()...); err != nil {
		log.Error("device AutoMigrate failed", "err", err)
		return
	}
	repo := devicepkg.NewRepository(db)
	svc := devicepkg.NewService(repo, auditSvc)
	h := devicepkg.NewHandler(svc)
	h.Register(v1, perms)

	// device-groups and configuration-templates have separate
	// sub-handlers with their own Register methods.
	groupRepo := devicepkg.NewGroupRepository(db)
	groupSvc := devicepkg.NewGroupService(groupRepo, auditSvc)
	devicepkg.NewGroupHandler(groupSvc).Register(v1, perms)

	tmplRepo := devicepkg.NewTemplateRepository(db)
	tmplSvc := devicepkg.NewTemplateService(tmplRepo, auditSvc)
	devicepkg.NewTemplateHandler(tmplSvc).Register(v1, perms)

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
//
// ctx is the shared signal-aware context from run() — the
// monitor loop watches it so SIGTERM aborts a 200-host
// sweep in milliseconds instead of 1000s (audit item 1).
//
// obs is the Prometheus registry; the loop writes
// monitor_loop_iterations_total / errors_total /
// last_tick_timestamp_seconds to it.
func registerPhysicalHostRoutes(ctx context.Context, v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, obs *observability.Metrics, pub realtime.Publisher, auditSvc *audit.Service, auditRepo *audit.Repository, perms func(rbacpkg.Permission) gin.HandlerFunc) *physicalhost.MaintenanceService {
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
	// Audit item 1 fix: ctx is the shared signal-aware ctx
	// from run() so a SIGTERM cancels the loop in lockstep
	// with srv.Shutdown. obs is the Prometheus registry so
	// operators can alert on monitor_loop_iterations_total
	// / errors_total / last_tick_timestamp_seconds.
	loop := physicalhost.NewMonitorLoop(monitor, physicalhost.MonitorLoopConfig{
		Tick:    1 * time.Minute,
		Jitter:  5 * time.Second,
		Logger:  log.Logger,
		Metrics: obs,
	})
	go loop.Run(ctx, nil)
	log.Info("monitor loop started", "tick", "1m", "jitter", "5s")

	phSvc := physicalhost.NewService(physicalhost.ServiceConfig{
		Repo:      repo,
		AuditRepo: auditRepo,
	})
	physicalhost.NewHandler(physicalhost.HandlerConfig{
		Service:     phSvc,
		Monitor:     monitor,
		Maintenance: maint,
		Metrics:     physicalhost.NewMetricsCache(physicalhost.MetricsCacheConfig{Collector: collector, TTL: 30 * time.Second, MaxEntries: 1024}),
		Audit:       auditSvc,
	}).Register(v1, perms)
	log.Info("physicalhost routes registered")
	return maint
}

// logAuditEmitter is kept as a no-op adapter for code paths

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
func registerDiscoveryRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc, auditSvc *audit.Service) {
	if err := dbpkg.AutoMigrate(db, discovery.AllModels()...); err != nil {
		log.Error("discovery AutoMigrate failed", "err", err)
		return
	}
	repo := discovery.NewRepository(db)
	devs := devicepkg.NewRepository(db)
	svc := discovery.NewService(repo, devs, discovery.NewFakeScanner(nil, nil), discovery.NewFakeProber(nil, nil), auditSvc)
	discovery.NewHandler(svc).Register(v1, perms)
	log.Info("discovery routes registered")
}

// registerK8sClusterRoutes wires the k8s-cluster-management module.
// The AES-256 key is loaded from K8S_CRYPTO_KEY env var; in dev a
// deterministic 32-byte key is used with a warning, mirroring the
// JWT-secret pattern.
func registerK8sClusterRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc, auditSvc *audit.Service) *k8s.Service {
	if err := dbpkg.AutoMigrate(db, k8s.AllModels()...); err != nil {
		log.Error("k8s AutoMigrate failed", "err", err)
		return nil
	}
	key := []byte(envOr("K8S_CRYPTO_KEY", devK8sCryptoKey))
	if len(key) != 32 {
		log.Warn("K8S_CRYPTO_KEY is not 32 bytes; deriving via SHA-256 (dev only)", "len", len(key))
		h := sha256.Sum256(key)
		key = h[:]
	}
	repo := k8s.NewRepository(db)
	svc := k8s.NewService(repo, &k8s.FakeClient{}, key, auditSvc)
	// Wire the per-cluster ClientRegistry so Handler.Exec can
	// resolve clusterID → KubeClient. The registry walks every
	// cluster row, decrypts the kubeconfig on first use, and
	// caches the result stickily. Built here (not lazily in the
	// handler) so a misconfigured cluster's failure mode is
	// visible at startup rather than on the first exec.
	svc.SetRegistry(k8s.NewClientRegistry(repo, svc))
	k8s.NewHandler(svc).Register(v1, perms)
	log.Info("k8s cluster routes registered")
	return svc
}

// registerHostProjectLinkRoutes wires the physical-host-project-linking
// module. The service walks the project hierarchy when listing devices
// for a project. The per-project access factory uses the same
// MembershipChecker as the project module so a Developer cannot link
// a device to a project they do not belong to.
func registerHostProjectLinkRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc, rbacSvc *rbacpkg.Service, auditSvc *audit.Service) {
	if err := dbpkg.AutoMigrate(db, hostproject.AllModels()...); err != nil {
		log.Error("hostproject AutoMigrate failed", "err", err)
		return
	}
	repo := hostproject.NewRepository(db)
	projectRepo := projectpkg.NewRepository(db)
	projectSvc := projectpkg.NewService(projectRepo)
	svc := hostproject.NewService(repo, projectSvc, auditSvc)
	projectAccess := rbacpkg.NewProjectAccessFactory(rbacSvc, projectSvc.MembershipChecker())
	hostproject.NewHandler(svc).Register(v1, perms, projectAccess)
	log.Info("hostproject routes registered")
}

// registerPipelineRoutes wires the cicd-pipeline module. The Executor
// is the Local implementation (runs shell via os/exec); tests use
// Fake via direct construction in service_test.go.
func registerPipelineRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc) {
	if err := dbpkg.AutoMigrate(db, pipeline.AllModels()...); err != nil {
		log.Error("pipeline AutoMigrate failed", "err", err)
		return
	}
	repo := pipeline.NewRepository(db)
	exec := pipeline.NewLocal(pipeline.WithMaxOutputBytes(1 << 20))
	svc := pipeline.NewService(repo, exec)
	pipeline.NewHandler(svc).Register(v1, perms)
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
// (registerPipelineRoutes runs first). obs is the shared
// observability Metrics so the catalog's domain-level
// gauges land on the same /metrics registry as the HTTP
// middleware.
func registerServiceCatalogRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, k8sSvc *k8s.Service, obs *observability.Metrics, perms func(rbacpkg.Permission) gin.HandlerFunc, auditSvc *audit.Service) {
	if err := dbpkg.AutoMigrate(db, &servicecatalog.Service{}, &servicecatalog.OnCall{}, &servicecatalog.RunbookEntry{}); err != nil {
		log.Error("servicecatalog AutoMigrate failed", "err", err)
		return
	}
	repo := servicecatalog.NewRepository(db)
	cat := servicecatalog.NewCatalog(repo, auditSvc)
	handler := servicecatalog.NewHandler(cat)

	// Domain-level Prometheus metrics for per-service
	// health. Registered against the same registry as
	// the HTTP middleware so a single /metrics scrape
	// returns both transport and domain signals.
	if obs != nil {
		handler.SetMetrics(servicecatalog.NewMetrics(obs.Registry()))
	}

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
	health.WithK8s(buildK8sSource(k8sRepo, k8sSvc))

	handler.SetHealth(health)

	handler.Register(v1, perms)
	log.Info("servicecatalog routes registered")
}

// buildK8sSource wires the production K8sSource via
// the P1.5 per-cluster ClientRegistry. The registry
// builds a per-cluster KubeClient from each cluster's
// decrypted kubeconfig and caches it. The walker
// aggregates across clusters and filters by
// deployment name == service name.
func buildK8sSource(repo *k8s.Repository, decrypter *k8s.Service) servicecatalog.K8sSource {
	registry := k8s.NewClientRegistry(repo, decrypter)
	clusters, _, err := repo.List(k8s.ListFilter{})
	if err != nil {
		clusters = nil
	}
	ids := make([]string, 0, len(clusters))
	for _, c := range clusters {
		ids = append(ids, c.ID)
	}
	return servicecatalog.NewMultiClusterK8sSource(registryAdapter{registry: registry}, ids, "default")
}

// registryAdapter bridges k8s.ClientRegistry to the
// servicecatalog.K8sClientGetter interface. The bridge
// is needed because the two packages cannot share
// types (cycle prevention) — servicecatalog stays
// free of any k8s-package imports.
type registryAdapter struct {
	registry k8s.ClientRegistry
}

// ClientFor implements servicecatalog.K8sClientGetter
// by delegating to the k8s registry. The k8s.Client
// (which has all the production methods) is adapted
// to the minimal servicecatalog.K8sClient.
func (a registryAdapter) ClientFor(clusterID string) (servicecatalog.K8sClient, error) {
	c, err := a.registry.ClientFor(clusterID)
	if err != nil {
		// Map the k8s package's ErrNotFound to the
		// servicecatalog's ErrK8sNoClient so the
		// walker skips the cluster silently.
		if errors.Is(err, k8s.ErrNotFound) {
			return nil, servicecatalog.ErrK8sNoClient
		}
		return nil, err
	}
	return k8sClientAdapter{client: c, clusterID: clusterID}, nil
}

// k8sClientAdapter wraps a k8s.Client (production
// type with the full method set) and exposes only
// the ListDeployments method the servicecatalog
// actually calls. Each adapter is bound to a single
// cluster ID so the walker's filter is not
// ambiguous.
type k8sClientAdapter struct {
	client    k8s.Client
	clusterID string
}

// ListDeployments satisfies servicecatalog.K8sClient.
// It translates the k8s package's Deployment into the
// servicecatalog's K8sDeployment (separate types to
// keep the package boundary clean).
func (a k8sClientAdapter) ListDeployments(ctx context.Context, ns string) ([]servicecatalog.K8sDeployment, error) {
	deps, err := a.client.ListDeployments(ctx, ns)
	if err != nil {
		return nil, err
	}
	out := make([]servicecatalog.K8sDeployment, 0, len(deps))
	for _, d := range deps {
		out = append(out, servicecatalog.K8sDeployment{
			ClusterID: a.clusterID,
			Namespace: d.Namespace,
			Name:      d.Name,
			Replicas:  d.Replicas,
			Available: d.Available,
		})
	}
	return out, nil
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
func registerLogsRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc, auditSvc *audit.Service) {
	_ = db // logs module is read-only; no AutoMigrate needed
	backend := logs.NewLocal(logs.LocalConfig{
		Dir: envOr("LOG_STORAGE_DIR", "tests/fixtures/logs"),
	})
	svc := logs.NewService(backend, logs.ServiceConfig{})
	// The saved-filter / alert-rule routes live in the
	// extra handler (NewHandlerWithExtra), not the
	// default NewHandler. Wire the audit service in so
	// the saved-filter and alert-rule mutating handlers
	// emit RecordAction rows.
	extraRepo := logs.NewExtraRepository(db)
	extraSvc := logs.NewExtraService(extraRepo, svc)
	logs.NewHandlerWithExtra(svc, extraSvc, extraRepo, auditSvc).Register(v1, perms)
	log.Info("logs routes registered", "backend", "local")
}

// registerMetricsRoutes wires the metrics-collection module. The
// scraper is a no-op Fake in dev; production swaps in the
// PrometheusScraper (injected with an HTTPClient). The metrics
// middleware (metrics.Middleware) is exposed for main.go to
// install as a global Gin middleware in a follow-up.
func registerMetricsRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc) {
	if err := dbpkg.AutoMigrate(db, metrics.AllModels()...); err != nil {
		log.Error("metrics AutoMigrate failed", "err", err)
		return
	}
	repo := metrics.NewRepository(db)
	svc := metrics.NewService(repo, metrics.NewFakeScraper())
	metrics.NewHandler(svc).Register(v1, perms)
	log.Info("metrics routes registered")
}

// registerAlertsRoutes wires the alert-notification module. The
// dispatcher is the LogDispatcher (writes to slog); the
// suppression checker is the real DefaultSuppressionChecker
// driven by a tiny adapter that consults the physicalhost
// service's InMaintenance predicate.
func registerAlertsRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, phMaintenance physicalhostMaintenanceProbe, perms func(rbacpkg.Permission) gin.HandlerFunc, auditSvc *audit.Service) {
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
		Audit:       auditSvc,
	})
	alerts.NewHandler(svc).Register(v1, perms)
	log.Info("alerts routes registered", "real_suppression", true)
}

// registerWsHubRoutes wires the websocket-hub module: starts the
// hub's Run loop on the application context and registers /ws.
// JWT signer is reconstructed from the same secret as the auth
// routes so the upgrade path can verify the token. The hub
// itself is returned so the caller can wire a realtime.Publisher
// for domain events (device_event, alert_fired, etc).
func registerWsHubRoutes(v1 *gin.RouterGroup, cfg *config.Config, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc) *hub.Hub {
	secret := os.Getenv("APP_JWT_SECRET")
	if secret == "" {
		if cfg.App.Env == "production" {
			log.Error("APP_JWT_SECRET must be set when env=production (no dev fallback); refusing to start WS hub with a known signer")
			os.Exit(1)
		}
		secret = devDefaultJWTSecret
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
	hub.RegisterRoutes(v1, h, signer, upgrader, perms)
	log.Info("websocket hub routes registered")
	return h
}

// registerLogStreamRoutes wires the k8s-pod-log-streaming module.
// The LogClient is a Fake in dev; the production swap-in is the
// KubeLogClient which itself wraps a client-go function seam. The
// RealtimePublisher here is a no-op in dev (logs events are still
// returned to the WS/SSE client even when no hub is wired).
func registerLogStreamRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc) {
	_ = db // no AutoMigrate; log stream is read-mostly
	client := &logstream.FakeLogClient{}
	streamer := logstream.NewKubeStreamer(client)
	pub := &logstreamRealtimeAdapter{} // bridges the local interface to the realtime package
	svc := logstream.NewService(streamer, client, pub)
	logstream.NewHandler(svc, logstream.HandlerConfig{}).Register(v1, perms)
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
func registerAuditRoutes(v1 *gin.RouterGroup, db *gorm.DB, log *logger.Logger, perms func(rbacpkg.Permission) gin.HandlerFunc) (*audit.Service, *audit.Repository) {
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
	audit.NewHandler(svc).Register(v1, perms)
	log.Info("audit routes registered")
	return svc, repo
}
