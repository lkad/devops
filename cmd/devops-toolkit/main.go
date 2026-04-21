package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devops-toolkit/internal/config"
	"github.com/devops-toolkit/internal/device"
	"github.com/devops-toolkit/internal/logs"
	"github.com/devops-toolkit/internal/metrics"
	"github.com/devops-toolkit/internal/alerts"
	"github.com/devops-toolkit/internal/pipeline"
	"github.com/devops-toolkit/internal/k8s"
	"github.com/devops-toolkit/internal/discovery"
	"github.com/devops-toolkit/internal/physicalhost"
	"github.com/devops-toolkit/internal/websocket"
	"github.com/gorilla/mux"
)

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

	deviceMgr, err := device.NewManager(cfg.Database.DSN())
	if err != nil {
		log.Printf("Warning: Device manager unavailable (DB connection failed): %v", err)
		deviceMgr = nil
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

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// Device routes
	r.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
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
	r.HandleFunc("/api/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
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
	r.HandleFunc("/api/devices/search", func(w http.ResponseWriter, r *http.Request) {
		if deviceMgr == nil {
			http.Error(w, "device manager unavailable", http.StatusServiceUnavailable)
			return
		}
		deviceMgr.SearchDevicesHTTP(w, r)
	})

	// Pipeline routes
	r.HandleFunc("/api/pipelines", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			pipelineMgr.ListPipelinesHTTP(w, r)
		} else if r.Method == "POST" {
			pipelineMgr.CreatePipelineHTTP(w, r)
		}
	})
	r.HandleFunc("/api/pipelines/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			pipelineMgr.GetPipelineHTTP(w, r)
		} else if r.Method == "DELETE" {
			pipelineMgr.DeletePipelineHTTP(w, r)
		}
	})
	r.HandleFunc("/api/pipelines/{id}/execute", pipelineMgr.ExecutePipelineHTTP)

	// Log routes
	r.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logMgr.QueryLogsHTTP(w, r)
		} else if r.Method == "POST" {
			logMgr.CreateLogHTTP(w, r)
		}
	})
	r.HandleFunc("/api/logs/stats", logMgr.GetStatsHTTP)
	r.HandleFunc("/api/logs/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logMgr.ListAlertRulesHTTP(w, r)
		} else if r.Method == "POST" {
			logMgr.CreateAlertRuleHTTP(w, r)
		}
	})

	// Metrics
	r.HandleFunc("/metrics", metricsMgr.ServePrometheus)
	r.HandleFunc("/api/metrics", metricsMgr.ServeJSON)

	// Alert routes
	r.HandleFunc("/api/alerts/channels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			alertsMgr.ListChannelsHTTP(w, r)
		} else if r.Method == "POST" {
			alertsMgr.AddChannelHTTP(w, r)
		}
	})
	r.HandleFunc("/api/alerts/history", alertsMgr.GetHistoryHTTP)

	// WebSocket
	r.HandleFunc("/ws", wsHub.HandleWebSocket)

	// K8s routes
	r.HandleFunc("/api/k8s/clusters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			k8sMgr.ListClustersHTTP(w, r)
		} else if r.Method == "POST" {
			k8sMgr.CreateClusterHTTP(w, r)
		}
	})
	r.HandleFunc("/api/k8s/clusters/{name}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			k8sMgr.DeleteClusterHTTP(w, r)
		}
	})
	r.HandleFunc("/api/k8s/clusters/{name}/health", k8sMgr.HealthCheckHTTP)

	// Physical host routes
	r.HandleFunc("/api/physical-hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			physicalhostMgr.ListHostsHTTP(w, r)
		} else if r.Method == "POST" {
			physicalhostMgr.CreateHostHTTP(w, r)
		}
	})
	r.HandleFunc("/api/physical-hosts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			physicalhostMgr.GetHostHTTP(w, r)
		} else if r.Method == "DELETE" {
			physicalhostMgr.DeleteHostHTTP(w, r)
		}
	})

	// Discovery routes
	r.HandleFunc("/api/discovery/status", discoveryMgr.GetStatusHTTP)
	r.HandleFunc("/api/discovery/scan", discoveryMgr.ScanHTTP)

	// Static files (frontend)
	frontendDir := "./devops-toolkit/frontend"
	if _, err := os.Stat(frontendDir); err == nil {
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(frontendDir)))
		log.Printf("Serving static files from %s", frontendDir)
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
