package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/config"
	dbpkg "github.com/devops-toolkit/backend/internal/database"
	devicepkg "github.com/devops-toolkit/backend/internal/device"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
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

	_ = logger.New(logger.WithLevel("error"), logger.WithFormat("json"))

	gin.SetMode(gin.TestMode)
	r := gin.New()

	projectRepo := projectpkg.NewRepository(db)
	projectSvc := projectpkg.NewService(projectRepo)
	projectH := projectpkg.NewHandler(projectSvc, projectRepo)
	v1 := r.Group("/api/v1")
	projectH.Register(v1)

	deviceRepo := devicepkg.NewRepository(db)
	deviceSvc := devicepkg.NewService(deviceRepo)
	deviceH := devicepkg.NewHandler(deviceSvc)
	deviceH.Register(v1)

	groupRepo := devicepkg.NewGroupRepository(db)
	groupSvc := devicepkg.NewGroupService(groupRepo)
	devicepkg.NewGroupHandler(groupSvc).Register(v1)

	tmplRepo := devicepkg.NewTemplateRepository(db)
	tmplSvc := devicepkg.NewTemplateService(tmplRepo)
	devicepkg.NewTemplateHandler(tmplSvc).Register(v1)

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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
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
