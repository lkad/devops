// Command devops-toolkit is the entry point for the DevOps Toolkit backend.
// Phase 1 wires up: config loader, slog logger, GORM database connection,
// and a Gin HTTP server with /health and a /api/v1/capabilities placeholder.
// Subsequent phases add module routes via the same router.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devops-toolkit/backend/internal/config"
	dbpkg "github.com/devops-toolkit/backend/internal/database"
	"github.com/devops-toolkit/backend/internal/handler"
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
	_ = db // Phase 1 only verifies the connection; AutoMigrate runs once modules register.
	log.Info("database connected", "driver", cfg.Database.Driver)

	router := buildRouter(log)
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
