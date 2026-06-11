package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devops-toolkit/backend/internal/config"
	"github.com/devops-toolkit/backend/pkg/logger"
)

func TestBuildRouter_HealthEndpoint(t *testing.T) {
	// GIVEN a router built with a working logger
	// WHEN /health is requested
	// THEN 200 OK with {"status":"ok"} is returned
	log := logger.New(logger.WithWriter(&bytes.Buffer{}), logger.WithLevel("error"))
	r, _, _ := buildRouter(log, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q", got)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %v", body["status"])
	}
}

func TestBuildRouter_UnknownRouteReturns404Envelope(t *testing.T) {
	// GIVEN a router
	// WHEN an unknown path is requested
	// THEN 404 with a contracts-style error envelope is returned
	log := logger.New(logger.WithWriter(&bytes.Buffer{}), logger.WithLevel("error"))
	r, _, _ := buildRouter(log, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' object, got %v", body)
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("code = %v, want NOT_FOUND", errObj["code"])
	}
}

func TestBuildRouter_APIv1Placeholder(t *testing.T) {
	// GIVEN a router
	// WHEN /api/v1/capabilities is requested
	// THEN 200 with the listed sections is returned
	// (the endpoint exists in Phase 1 as a placeholder; modules add their own routes in later phases)
	log := logger.New(logger.WithWriter(&bytes.Buffer{}), logger.WithLevel("error"))
	r, _, _ := buildRouter(log, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["sections"]; !ok {
		t.Error("expected 'sections' field")
	}
}

func TestRenderConfigString_MasksPasswords(t *testing.T) {
	// GIVEN a config with a database password
	// WHEN rendered
	// THEN the password does not appear in the output
	cfg := &config.Config{
		App: config.AppConfig{Name: "x", Env: "dev", Port: 8080},
		Database: config.DatabaseConfig{Driver: "sqlite", Password: "supersecret"},
	}
	out := renderConfig(cfg)
	if strings.Contains(out, "supersecret") {
		t.Errorf("password leaked in render: %s", out)
	}
}
