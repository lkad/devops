package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/devops-toolkit/internal/config"
	"github.com/devops-toolkit/internal/device"
	"github.com/devops-toolkit/internal/logs"
	"github.com/devops-toolkit/internal/metrics"
	"github.com/devops-toolkit/internal/alerts"
	"github.com/devops-toolkit/internal/auth"
	"github.com/devops-toolkit/internal/auth/ldap"
	"github.com/devops-toolkit/internal/pipeline"
	"github.com/devops-toolkit/internal/k8s"
	"github.com/devops-toolkit/internal/discovery"
	"github.com/devops-toolkit/internal/physicalhost"
	"github.com/devops-toolkit/internal/project"
	"github.com/devops-toolkit/internal/websocket"
	"github.com/gorilla/mux"
)

type statusCodeWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCodeWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Printf("Config load error (using defaults): %v", err)
		cfg = &config.Config{
			Server: config.ServerConfig{Port: 3000, Host: "0.0.0.0"},
		}
	}

	// Initialize managers
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize LDAP client (uses env vars: LDAP_URL, LDAP_BASE_DN, etc.)
	ldapClient, err := ldap.NewClient(ldap.DefaultConfig())
	if err != nil {
		log.Printf("Warning: LDAP client unavailable: %v", err)
		ldapClient = nil
	}

	// Initialize auth handler
	authHandler := auth.NewHandler(ldapClient, &cfg.Auth)

	deviceMgr, err := device.NewManager(cfg.Database.DSN())
	if err != nil {
		log.Printf("Warning: Device manager unavailable (DB connection failed): %v", err)
		deviceMgr = nil
	}
	projectMgr, err := project.NewManagerWithDSN(cfg.Database.DSN())
	if err != nil {
		log.Printf("Warning: Project manager unavailable (DB connection failed): %v", err)
		projectMgr = nil
	}
	logMgr := logs.NewManager(logs.LogsConfig{
		Backend:       cfg.Logs.Backend,
		RetentionDays: cfg.Logs.RetentionDays,
	}, func(entry *logs.Entry) {
		wsHub.BroadcastLog(entry)
	})
	metricsMgr := metrics.NewCollector()
	alertsMgr := alerts.NewManager(metricsMgr)
	pipelineMgr := pipeline.NewManager()
	k8sMgr := k8s.NewClusterManager()
	discoveryMgr := discovery.NewManager()
	physicalhostMgr := physicalhost.NewManager()

	// Create router
	r := mux.NewRouter()

	// HTTP metrics middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			path := r.URL.Path
			method := r.Method

			// Wrap response writer to capture status code
			wrapped := &statusCodeWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Milliseconds()
			status := fmt.Sprintf("%d", wrapped.statusCode)
			metricsMgr.RecordHTTPRequest(path, method, status, float64(duration))
		})
	})

	// Determine base path for API routes
	basePath := cfg.Server.BasePath
	if basePath == "" {
		basePath = "/"
	}

	// Create API router using subrouter for base path
	// When base path is set, routes must be registered WITH the base path on the subrouter
	// For dual-path compatibility (direct and via proxy), we use main router directly
	var apiRouter *mux.Router
	if cfg.Server.BasePath != "" && cfg.Server.BasePath != "/" {
		apiRouter = r.PathPrefix(cfg.Server.BasePath).Subrouter()
		log.Printf("Base path configured: %s", cfg.Server.BasePath)
	} else {
		apiRouter = r
	}

	// Helper to register routes on both main router and API subrouter for dual access
	registerRoute := func(path string, handler http.HandlerFunc) {
		apiRouter.HandleFunc(path, handler)
		if cfg.Server.BasePath != "" && cfg.Server.BasePath != "/" {
			r.HandleFunc(path, handler)
		}
	}

	// Health check (always at root for direct access and proxy health checks)
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// Health check on API subrouter too (for proxy access via base path)
	if cfg.Server.BasePath != "" && cfg.Server.BasePath != "/" {
		apiRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		})
		// Static file handler - only serve known static asset paths
		apiRouter.PathPrefix("/assets/").Handler(http.StripPrefix(cfg.Server.BasePath, http.FileServer(http.Dir("./devops-toolkit/frontend/dist"))))
		// Serve favicon
		apiRouter.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "./devops-toolkit/frontend/dist/favicon.svg")
		})
		// Also serve root
		apiRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "./devops-toolkit/frontend/dist/index.html")
		})
	}

	// Auth routes
	registerRoute("/api/auth/login", authHandler.Login)
	registerRoute("/api/auth/logout", authHandler.Logout)
	registerRoute("/api/auth/me", authHandler.Me)

	// Device routes
	registerRoute("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		if deviceMgr == nil {
			http.Error(w, "device manager unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method == "GET" {
			deviceMgr.ListDevicesHTTP(w, r)
		} else if r.Method == "POST" {
			deviceMgr.CreateDeviceHTTP(w, r)
		}
	})
	registerRoute("/api/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if deviceMgr == nil {
			http.Error(w, "device manager unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method == "GET" {
			deviceMgr.GetDeviceHTTP(w, r)
		} else if r.Method == "PUT" {
			deviceMgr.UpdateDeviceHTTP(w, r)
		} else if r.Method == "DELETE" {
			deviceMgr.DeleteDeviceHTTP(w, r)
		}
	})
	registerRoute("/api/devices/search", func(w http.ResponseWriter, r *http.Request) {
		if deviceMgr == nil {
			http.Error(w, "device manager unavailable", http.StatusServiceUnavailable)
			return
		}
		deviceMgr.SearchDevicesHTTP(w, r)
	})

	// Pipeline routes
	registerRoute("/api/pipelines", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			pipelineMgr.ListPipelinesHTTP(w, r)
		} else if r.Method == "POST" {
			pipelineMgr.CreatePipelineHTTP(w, r)
		}
	})
	registerRoute("/api/pipelines/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			pipelineMgr.GetPipelineHTTP(w, r)
		} else if r.Method == "DELETE" {
			pipelineMgr.DeletePipelineHTTP(w, r)
		}
	})
	registerRoute("/api/pipelines/{id}/execute", pipelineMgr.ExecutePipelineHTTP)

	// Log routes
	registerRoute("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logMgr.QueryLogsHTTP(w, r)
		} else if r.Method == "POST" {
			logMgr.CreateLogHTTP(w, r)
		}
	})
	registerRoute("/api/logs/stats", logMgr.GetStatsHTTP)
	registerRoute("/api/logs/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logMgr.ListAlertRulesHTTP(w, r)
		} else if r.Method == "POST" {
			logMgr.CreateAlertRuleHTTP(w, r)
		}
	})

	// Metrics
	registerRoute("/metrics", metricsMgr.ServePrometheus)
	registerRoute("/api/metrics", metricsMgr.ServeJSON)

	// Alert routes
	registerRoute("/api/alerts/channels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			alertsMgr.ListChannelsHTTP(w, r)
		} else if r.Method == "POST" {
			alertsMgr.AddChannelHTTP(w, r)
		}
	})
	registerRoute("/api/alerts/history", alertsMgr.GetHistoryHTTP)

	// K8s routes
	registerRoute("/api/k8s/clusters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			k8sMgr.ListClustersHTTP(w, r)
		} else if r.Method == "POST" {
			k8sMgr.CreateClusterHTTP(w, r)
		}
	})
	registerRoute("/api/k8s/clusters/{name}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			k8sMgr.DeleteClusterHTTP(w, r)
		}
	})
	registerRoute("/api/k8s/clusters/{name}/health", k8sMgr.HealthCheckHTTP)
	registerRoute("/api/k8s/clusters/{cluster}/nodes", k8sMgr.GetNodesHTTP)
	registerRoute("/api/k8s/clusters/{cluster}/namespaces", k8sMgr.GetNamespacesHTTP)
	registerRoute("/api/k8s/clusters/{cluster}/pods", k8sMgr.GetPodsHTTP)
	registerRoute("/api/k8s/clusters/{cluster}/pods/{pod}/logs", k8sMgr.GetPodLogsHTTP)
	registerRoute("/api/k8s/maintenance", k8sMgr.MaintenanceOpHTTP)

	// Physical host routes
	registerRoute("/api/physical-hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			physicalhostMgr.ListHostsHTTP(w, r)
		} else if r.Method == "POST" {
			physicalhostMgr.CreateHostHTTP(w, r)
		}
	})
	registerRoute("/api/physical-hosts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			physicalhostMgr.GetHostHTTP(w, r)
		} else if r.Method == "DELETE" {
			physicalhostMgr.DeleteHostHTTP(w, r)
		}
	})

	// Discovery routes
	registerRoute("/api/discovery/status", discoveryMgr.GetStatusHTTP)
	registerRoute("/api/discovery/scan", discoveryMgr.ScanHTTP)

	// WebSocket (always at root for proxy compatibility)
	r.HandleFunc("/ws", wsHub.HandleWebSocket)
	// Also on subrouter for proxy access
	if cfg.Server.BasePath != "" && cfg.Server.BasePath != "/" {
		apiRouter.HandleFunc("/ws", wsHub.HandleWebSocket)
	}

	// Project management routes (if available)
	if projectMgr != nil {
		// Project Types
		registerRoute("/api/org/project-types", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.ListProjectTypesHTTP(w, r)
			} else if r.Method == "POST" {
				projectMgr.CreateProjectTypeHTTP(w, r)
			}
		})
		registerRoute("/api/org/project-types/{id}", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "PUT" {
				projectMgr.UpdateProjectTypeHTTP(w, r)
			} else if r.Method == "DELETE" {
				projectMgr.DeleteProjectTypeHTTP(w, r)
			}
		})

		// Business Lines
		registerRoute("/api/org/business-lines", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.ListBusinessLinesHTTP(w, r)
			} else if r.Method == "POST" {
				projectMgr.CreateBusinessLineHTTP(w, r)
			}
		})
		registerRoute("/api/org/business-lines/{id}", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.GetBusinessLineHTTP(w, r)
			} else if r.Method == "PUT" {
				projectMgr.UpdateBusinessLineHTTP(w, r)
			} else if r.Method == "DELETE" {
				projectMgr.DeleteBusinessLineHTTP(w, r)
			}
		})

		// Systems
		registerRoute("/api/org/business-lines/{bl_id}/systems", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.ListSystemsHTTP(w, r)
			} else if r.Method == "POST" {
				projectMgr.CreateSystemHTTP(w, r)
			}
		})
		registerRoute("/api/org/systems/{id}", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.GetSystemHTTP(w, r)
			} else if r.Method == "PUT" {
				projectMgr.UpdateSystemHTTP(w, r)
			} else if r.Method == "DELETE" {
				projectMgr.DeleteSystemHTTP(w, r)
			}
		})

		// Projects
		registerRoute("/api/org/systems/{sys_id}/projects", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.ListProjectsHTTP(w, r)
			} else if r.Method == "POST" {
				projectMgr.CreateProjectHTTP(w, r)
			}
		})
		registerRoute("/api/org/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.GetProjectHTTP(w, r)
			} else if r.Method == "PUT" {
				projectMgr.UpdateProjectHTTP(w, r)
			} else if r.Method == "DELETE" {
				projectMgr.DeleteProjectHTTP(w, r)
			}
		})

		// Resource linking
		registerRoute("/api/org/projects/{id}/resources", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.ListProjectResourcesHTTP(w, r)
			} else if r.Method == "POST" {
				projectMgr.LinkResourceHTTP(w, r)
			}
		})
		registerRoute("/api/org/projects/{id}/resources/{resource_id}", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				projectMgr.UnlinkResourceHTTP(w, r)
			}
		})

		// Permissions
		registerRoute("/api/org/projects/{id}/permissions", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				projectMgr.ListProjectPermissionsHTTP(w, r)
			} else if r.Method == "POST" {
				projectMgr.GrantPermissionHTTP(w, r)
			}
		})
		registerRoute("/api/org/permissions/{perm_id}", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				projectMgr.RevokePermissionHTTP(w, r)
			}
		})

		// FinOps export
		registerRoute("/api/org/reports/finops", projectMgr.ExportFinOpsHTTP)

		// Audit logs
		registerRoute("/api/org/audit-logs", projectMgr.ListAuditLogsHTTP)
	}

	// Static files (frontend) - always served at root (proxy strips base path before forwarding)
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	frontendDir := filepath.Join(exeDir, "frontend", "dist")
	if _, err := os.Stat(frontendDir); err == nil {
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(frontendDir)))
		log.Printf("Serving static files from %s", frontendDir)
	} else {
		// Fallback: try frontend without dist
		frontendDir = filepath.Join(exeDir, "frontend")
		if _, err := os.Stat(frontendDir); err == nil {
			r.PathPrefix("/").Handler(http.FileServer(http.Dir(frontendDir)))
			log.Printf("Serving static files from %s", frontendDir)
		} else {
			// Fallback: try relative path from current working directory
			if _, err := os.Stat("./devops-toolkit/frontend/dist"); err == nil {
				r.PathPrefix("/").Handler(http.FileServer(http.Dir("./devops-toolkit/frontend/dist")))
				log.Printf("Serving static files from ./devops-toolkit/frontend/dist")
			} else if _, err := os.Stat("./devops-toolkit/frontend"); err == nil {
				r.PathPrefix("/").Handler(http.FileServer(http.Dir("./devops-toolkit/frontend")))
				log.Printf("Serving static files from ./devops-toolkit/frontend")
			}
		}
	}

	// Add JWT auth middleware for API routes (if LDAP is available or dev bypass enabled)
	if ldapClient != nil || cfg.Auth.DevBypass {
		apiRouter.Use(auth.Middleware(&cfg.Auth))
	}

	// Start server
	addr := cfg.Server.Addr()
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
