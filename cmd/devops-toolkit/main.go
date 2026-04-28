package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/devops-toolkit/internal/alerts"
	"github.com/devops-toolkit/internal/auth"
	"github.com/devops-toolkit/internal/auth/ldap"
	"github.com/devops-toolkit/internal/config"
	"github.com/devops-toolkit/internal/device"
	"github.com/devops-toolkit/internal/discovery"
	"github.com/devops-toolkit/internal/ginadapter"
	"github.com/devops-toolkit/internal/k8s"
	"github.com/devops-toolkit/internal/logs"
	"github.com/devops-toolkit/internal/metrics"
	"github.com/devops-toolkit/internal/physicalhost"
	"github.com/devops-toolkit/internal/pipeline"
	"github.com/devops-toolkit/internal/project"
	"github.com/devops-toolkit/internal/websocket"
	"github.com/gin-gonic/gin"
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

	// Create user provider that extracts user from request context
	userProvider := &authUserProvider{}

	projectMgr, err := project.NewManagerWithDSN(cfg.Database.DSN(), userProvider)
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

	// Create Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// HTTP metrics middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start).Milliseconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		metricsMgr.RecordHTTPRequest(path, method, status, float64(duration))
	})

	// Determine base path for API routes
	basePath := cfg.Server.BasePath

	// Create API router using group for base path
	var api gin.IRouter
	if basePath != "" && basePath != "/" {
		api = r.Group(basePath)
		log.Printf("Base path configured: %s", basePath)
	} else {
		api = r
	}

	// Health check (always at root for direct access and proxy health checks)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Auth routes (using Gin methods)
	api.POST("/api/auth/login", authHandler.LoginGin)
	api.POST("/api/auth/logout", authHandler.LogoutGin)
	api.GET("/api/auth/me", authHandler.MeGin)

	// Device routes (using adapter for existing HTTP handlers)
	if deviceMgr != nil {
		api.GET("/api/devices", ginfadapter.GinToHTTPHandler(deviceMgr.ListDevicesHTTP))
		api.POST("/api/devices", ginfadapter.GinToHTTPHandler(deviceMgr.CreateDeviceHTTP))
		api.GET("/api/devices/:id", ginfadapter.GinToHTTPHandler(deviceMgr.GetDeviceHTTP, "id"))
		api.PUT("/api/devices/:id", ginfadapter.GinToHTTPHandler(deviceMgr.UpdateDeviceHTTP, "id"))
		api.DELETE("/api/devices/:id", ginfadapter.GinToHTTPHandler(deviceMgr.DeleteDeviceHTTP, "id"))
		api.GET("/api/devices/search", ginfadapter.GinToHTTPHandler(deviceMgr.SearchDevicesHTTP))
	}

	// Pipeline routes
	api.GET("/api/pipelines", ginfadapter.GinToHTTPHandler(pipelineMgr.ListPipelinesHTTP))
	api.POST("/api/pipelines", ginfadapter.GinToHTTPHandler(pipelineMgr.CreatePipelineHTTP))
	api.GET("/api/pipelines/:id", ginfadapter.GinToHTTPHandler(pipelineMgr.GetPipelineHTTP, "id"))
	api.DELETE("/api/pipelines/:id", ginfadapter.GinToHTTPHandler(pipelineMgr.DeletePipelineHTTP, "id"))
	api.POST("/api/pipelines/:id/execute", ginfadapter.GinToHTTPHandler(pipelineMgr.ExecutePipelineHTTP, "id"))

	// Log routes
	api.GET("/api/logs", ginfadapter.GinToHTTPHandler(logMgr.QueryLogsHTTP))
	api.POST("/api/logs", ginfadapter.GinToHTTPHandler(logMgr.CreateLogHTTP))
	api.GET("/api/logs/stats", ginfadapter.GinToHTTPHandler(logMgr.GetStatsHTTP))
	api.GET("/api/logs/alerts", ginfadapter.GinToHTTPHandler(logMgr.ListAlertRulesHTTP))
	api.POST("/api/logs/alerts", ginfadapter.GinToHTTPHandler(logMgr.CreateAlertRuleHTTP))
	api.GET("/api/logs/retention", ginfadapter.GinToHTTPHandler(logMgr.GetRetentionPolicyHTTP))
	api.PUT("/api/logs/retention", ginfadapter.GinToHTTPHandler(logMgr.UpdateRetentionPolicyHTTP))
	api.POST("/api/logs/retention/apply", ginfadapter.GinToHTTPHandler(logMgr.ApplyRetentionPolicyHTTP))
	api.GET("/api/logs/filters", ginfadapter.GinToHTTPHandler(logMgr.ListSavedFiltersHTTP))
	api.POST("/api/logs/filters", ginfadapter.GinToHTTPHandler(logMgr.CreateSavedFilterHTTP))
	api.POST("/api/logs/generate", ginfadapter.GinToHTTPHandler(logMgr.GenerateSampleLogsHTTP))

	// Metrics
	r.GET("/metrics", func(c *gin.Context) {
		metricsMgr.ServePrometheus(c.Writer, c.Request)
	})
	api.GET("/api/metrics", ginfadapter.GinToHTTPHandler(metricsMgr.ServeJSON))

	// Alert routes
	api.GET("/api/alerts/channels", ginfadapter.GinToHTTPHandler(alertsMgr.ListChannelsHTTP))
	api.POST("/api/alerts/channels", ginfadapter.GinToHTTPHandler(alertsMgr.AddChannelHTTP))
	api.GET("/api/alerts/history", ginfadapter.GinToHTTPHandler(alertsMgr.GetHistoryHTTP))

	// K8s routes
	api.GET("/api/k8s/clusters", ginfadapter.GinToHTTPHandler(k8sMgr.ListClustersHTTP))
	api.POST("/api/k8s/clusters", ginfadapter.GinToHTTPHandler(k8sMgr.CreateClusterHTTP))
	api.DELETE("/api/k8s/clusters/:name", ginfadapter.GinToHTTPHandler(k8sMgr.DeleteClusterHTTP, "name"))
	api.GET("/api/k8s/clusters/:name/health", ginfadapter.GinToHTTPHandler(k8sMgr.HealthCheckHTTP, "name"))
	api.GET("/api/k8s/clusters/:cluster/nodes", ginfadapter.GinToHTTPHandler(k8sMgr.GetNodesHTTP, "cluster"))
	api.GET("/api/k8s/clusters/:cluster/namespaces", ginfadapter.GinToHTTPHandler(k8sMgr.GetNamespacesHTTP, "cluster"))
	api.GET("/api/k8s/clusters/:cluster/pods", ginfadapter.GinToHTTPHandler(k8sMgr.GetPodsHTTP, "cluster"))
	api.GET("/api/k8s/clusters/:cluster/pods/:pod/logs", ginfadapter.GinToHTTPHandler(k8sMgr.GetPodLogsHTTP, "cluster", "pod"))
	api.GET("/api/k8s/clusters/:cluster/namespaces/:ns/pods/:pod/logs", ginfadapter.GinToHTTPHandler(k8sMgr.GetPodLogsWithNamespaceHTTP, "cluster", "ns", "pod"))
	api.POST("/api/k8s/clusters/:cluster/namespaces/:ns/pods/:pod/exec", ginfadapter.GinToHTTPHandler(k8sMgr.PodExecHTTP, "cluster", "ns", "pod"))
	api.GET("/api/k8s/clusters/:cluster/metrics", ginfadapter.GinToHTTPHandler(k8sMgr.GetClusterMetricsHTTP, "cluster"))
	api.POST("/api/k8s/maintenance", ginfadapter.GinToHTTPHandler(k8sMgr.MaintenanceOpHTTP))

	// Physical host routes
	api.GET("/api/physical-hosts", ginfadapter.GinToHTTPHandler(physicalhostMgr.ListHostsHTTP))
	api.POST("/api/physical-hosts", ginfadapter.GinToHTTPHandler(physicalhostMgr.CreateHostHTTP))
	api.GET("/api/physical-hosts/:id", ginfadapter.GinToHTTPHandler(physicalhostMgr.GetHostHTTP, "id"))
	api.DELETE("/api/physical-hosts/:id", ginfadapter.GinToHTTPHandler(physicalhostMgr.DeleteHostHTTP, "id"))
	api.GET("/api/physical-hosts/:id/services", ginfadapter.GinToHTTPHandler(physicalhostMgr.ListServicesHTTP, "id"))
	api.POST("/api/physical-hosts/:id/config", ginfadapter.GinToHTTPHandler(physicalhostMgr.PushConfigHTTP, "id"))

	// Discovery routes
	api.GET("/api/discovery/status", ginfadapter.GinToHTTPHandler(discoveryMgr.GetStatusHTTP))
	api.POST("/api/discovery/scan", ginfadapter.GinToHTTPHandler(discoveryMgr.ScanHTTP))

	// WebSocket (always at root for proxy compatibility)
	r.GET("/ws", func(c *gin.Context) {
		wsHub.HandleWebSocket(c.Writer, c.Request)
	})

	// Project management routes (if available)
	if projectMgr != nil {
		// Project Types
		api.GET("/api/org/project-types", ginfadapter.GinToHTTPHandler(projectMgr.ListProjectTypesHTTP))
		api.POST("/api/org/project-types", ginfadapter.GinToHTTPHandler(projectMgr.CreateProjectTypeHTTP))
		api.PUT("/api/org/project-types/:id", ginfadapter.GinToHTTPHandler(projectMgr.UpdateProjectTypeHTTP, "id"))
		api.DELETE("/api/org/project-types/:id", ginfadapter.GinToHTTPHandler(projectMgr.DeleteProjectTypeHTTP, "id"))

		// Business Lines
		api.GET("/api/org/business-lines", ginfadapter.GinToHTTPHandler(projectMgr.ListBusinessLinesHTTP))
		api.POST("/api/org/business-lines", ginfadapter.GinToHTTPHandler(projectMgr.CreateBusinessLineHTTP))
		api.GET("/api/org/business-lines/:id", ginfadapter.GinToHTTPHandler(projectMgr.GetBusinessLineHTTP, "id"))
		api.PUT("/api/org/business-lines/:id", ginfadapter.GinToHTTPHandler(projectMgr.UpdateBusinessLineHTTP, "id"))
		api.DELETE("/api/org/business-lines/:id", ginfadapter.GinToHTTPHandler(projectMgr.DeleteBusinessLineHTTP, "id"))

		// Systems
		api.GET("/api/org/business-lines/:bl_id/systems", ginfadapter.GinToHTTPHandler(projectMgr.ListSystemsHTTP, "bl_id"))
		api.POST("/api/org/business-lines/:bl_id/systems", ginfadapter.GinToHTTPHandler(projectMgr.CreateSystemHTTP, "bl_id"))
		api.GET("/api/org/systems/:id", ginfadapter.GinToHTTPHandler(projectMgr.GetSystemHTTP, "id"))
		api.PUT("/api/org/systems/:id", ginfadapter.GinToHTTPHandler(projectMgr.UpdateSystemHTTP, "id"))
		api.DELETE("/api/org/systems/:id", ginfadapter.GinToHTTPHandler(projectMgr.DeleteSystemHTTP, "id"))

		// Projects
		api.GET("/api/org/systems/:sys_id/projects", ginfadapter.GinToHTTPHandler(projectMgr.ListProjectsHTTP, "sys_id"))
		api.POST("/api/org/systems/:sys_id/projects", ginfadapter.GinToHTTPHandler(projectMgr.CreateProjectHTTP, "sys_id"))
		api.GET("/api/org/projects/:id", ginfadapter.GinToHTTPHandler(projectMgr.GetProjectHTTP, "id"))
		api.PUT("/api/org/projects/:id", ginfadapter.GinToHTTPHandler(projectMgr.UpdateProjectHTTP, "id"))
		api.DELETE("/api/org/projects/:id", ginfadapter.GinToHTTPHandler(projectMgr.DeleteProjectHTTP, "id"))

		// Resource linking
		api.GET("/api/org/projects/:id/resources", ginfadapter.GinToHTTPHandler(projectMgr.ListProjectResourcesHTTP, "id"))
		api.POST("/api/org/projects/:id/resources", ginfadapter.GinToHTTPHandler(projectMgr.LinkResourceHTTP, "id"))
		api.DELETE("/api/org/projects/:id/resources/:resource_id", ginfadapter.GinToHTTPHandler(projectMgr.UnlinkResourceHTTP, "id", "resource_id"))

		// Permissions
		api.GET("/api/org/projects/:id/permissions", ginfadapter.GinToHTTPHandler(projectMgr.ListProjectPermissionsHTTP, "id"))
		api.POST("/api/org/projects/:id/permissions", ginfadapter.GinToHTTPHandler(projectMgr.GrantPermissionHTTP, "id"))
		api.DELETE("/api/org/permissions/:perm_id", ginfadapter.GinToHTTPHandler(projectMgr.RevokePermissionHTTP, "perm_id"))

		// FinOps export
		api.GET("/api/org/reports/finops", ginfadapter.GinToHTTPHandler(projectMgr.ExportFinOpsHTTP))

		// Audit logs
		api.GET("/api/org/audit-logs", ginfadapter.GinToHTTPHandler(projectMgr.ListAuditLogsHTTP))
	}

	// Static files (frontend) - always served at root (proxy strips base path before forwarding)
	if basePath != "" && basePath != "/" {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)
		frontendDir := filepath.Join(exeDir, "frontend", "dist")
		if _, err := os.Stat(frontendDir); err == nil {
			r.Static("/assets", filepath.Join(frontendDir, "assets"))
			r.GET("/favicon.svg", func(c *gin.Context) {
				c.File(filepath.Join(frontendDir, "favicon.svg"))
			})
			r.NoRoute(func(c *gin.Context) {
				c.File(filepath.Join(frontendDir, "index.html"))
			})
			log.Printf("Serving static files from %s", frontendDir)
		}
	} else {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)
		frontendDir := filepath.Join(exeDir, "frontend", "dist")
		if _, err := os.Stat(frontendDir); err == nil {
			r.Static("/assets", filepath.Join(frontendDir, "assets"))
			r.GET("/favicon.svg", func(c *gin.Context) {
				c.File(filepath.Join(frontendDir, "favicon.svg"))
			})
			r.NoRoute(func(c *gin.Context) {
				c.File(filepath.Join(frontendDir, "index.html"))
			})
			log.Printf("Serving static files from %s", frontendDir)
		} else {
			// Fallback: try relative path from current working directory
			if _, err := os.Stat("./devops-toolkit/frontend/dist"); err == nil {
				r.Static("/assets", "./devops-toolkit/frontend/dist/assets")
				r.GET("/favicon.svg", func(c *gin.Context) {
					c.File("./devops-toolkit/frontend/dist/favicon.svg")
				})
				r.NoRoute(func(c *gin.Context) {
					c.File("./devops-toolkit/frontend/dist/index.html")
				})
				log.Printf("Serving static files from ./devops-toolkit/frontend/dist")
			} else if _, err := os.Stat("./devops-toolkit/frontend"); err == nil {
				r.NoRoute(func(c *gin.Context) {
					c.File("./devops-toolkit/frontend/index.html")
				})
				log.Printf("Serving static files from ./devops-toolkit/frontend")
			}
		}
	}

	// Add JWT auth middleware for API routes (if LDAP is available or dev bypass enabled)
	if ldapClient != nil || cfg.Auth.DevBypass {
		api.Use(auth.MiddlewareGin(&cfg.Auth))
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

// authUserProvider implements project.UserProvider by extracting user from request context
type authUserProvider struct{}

func (a *authUserProvider) GetUserFromRequest(r *http.Request) *project.User {
	if user := auth.GetUserFromContext(r.Context()); user != nil {
		return &project.User{Username: user.Username}
	}
	return nil
}
