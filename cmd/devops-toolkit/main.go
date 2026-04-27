package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devops-toolkit/internal/auth"
	"github.com/devops-toolkit/internal/device"
	"github.com/devops-toolkit/internal/logs"
	"github.com/devops-toolkit/internal/metrics"
	"github.com/devops-toolkit/internal/websocket"
	"github.com/devops-toolkit/pkg/config"
	"github.com/devops-toolkit/pkg/database"
	"github.com/devops-toolkit/pkg/middleware"
)

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize database connection
	database.Set(db)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	websocket.SetHub(wsHub)
	go wsHub.Run()

	// Initialize services
	jwtService := auth.NewJWTService(cfg.Auth.JWT_SECRET, cfg.Auth.TokenExpiry)

	// Initialize LDAP connection pool
	var ldapClient *auth.LDAPClient
	if cfg.Auth.LDAP.Enabled {
		ldapPool, err := auth.NewLDAPPool(&cfg.Auth.LDAP, auth.PoolConfig{
			MaxConnections: 5,
			MaxAge:        5 * time.Minute,
		})
		if err != nil {
			log.Printf("Warning: failed to create LDAP pool: %v, falling back to per-request connections", err)
			ldapClient = auth.NewLDAPClient(&cfg.Auth.LDAP)
		} else {
			ldapClient = auth.NewLDAPClientWithPool(&cfg.Auth.LDAP, ldapPool)
			defer ldapPool.Close()
		}
	} else {
		ldapClient = auth.NewLDAPClient(&cfg.Auth.LDAP)
	}

	authService := auth.NewAuthService(jwtService, ldapClient)

	deviceRepo := device.NewRepository(db)
	deviceService := device.NewService(deviceRepo)
	logBackend := logs.NewLocalBackend()
	logService := logs.NewService(logBackend)
	logService.SetHub(wsHub)

	// Initialize handlers
	authHandler := auth.NewAuthHandler(authService, jwtService)
	deviceHandler := device.NewHandler(deviceService)
	logHandler := logs.NewHandler(logService)
	metricsHandler := metrics.Handler()

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Setup routes
	mux := http.NewServeMux()
	setupRoutes(mux, cfg, authHandler, authService, deviceHandler, logHandler, metricsHandler, wsHub, ldapClient)

	// Apply middleware chain
	handler := middleware.Chain(mux,
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.CORS(),
		middleware.Logging(),
	)

	server.Handler = handler

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s:%d", cfg.App.Host, cfg.App.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func initDatabase(cfg *config.Config) (*database.DB, error) {
	dbCfg := &database.DBConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		Name:            cfg.Database.Name,
		MaxConnections:  cfg.Database.MaxConnections,
		SSLMode:         cfg.Database.SSLMode,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}
	return database.New(dbCfg)
}

func setupRoutes(
	mux *http.ServeMux,
	cfg *config.Config,
	authHandler *auth.AuthHandler,
	authService *auth.AuthService,
	deviceHandler *device.Handler,
	logHandler *logs.Handler,
	metricsHandler http.HandlerFunc,
	wsHub *websocket.Hub,
	ldapClient *auth.LDAPClient,
) {
	// Health check (public)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/health/ldap", ldapHealthHandler(ldapClient))

	// Metrics (public)
	if cfg.Metrics.Enabled {
		mux.Handle(cfg.Metrics.Path, metricsHandler)
	}

	// WebSocket (public)
	mux.HandleFunc("/ws", websocket.Handler(wsHub))

	// Auth routes (public)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/me", authHandler.Me)

	// Device routes
	deviceHandler.RegisterRoutes(mux)

	// Log routes
	logHandler.RegisterRoutes(mux)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func ldapHealthHandler(ldapClient *auth.LDAPClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		healthy, reason := ldapClient.HealthCheck()
		if healthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","service":"ldap","reason":"` + reason + `"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"error","service":"ldap","reason":"` + reason + `"}`))
		}
	}
}
