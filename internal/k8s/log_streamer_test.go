package k8s

import (
	"testing"

	"github.com/devops-toolkit/internal/websocket"
)

func TestInferLogLevel(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		// Error level patterns
		{"This is an ERROR message", "error"},
		{"This is an Error message", "error"},
		{"This is an error message", "error"},
		{"Operation FAILED", "error"},
		{"Server FATAL error", "error"},
		{"Application PANIC", "error"},

		// Warn level patterns
		{"This is a WARN message", "warn"},
		{"This is a Warning message", "warn"},
		{"This is a WARNING", "warn"},
		{"Low memory warning", "warn"},

		// Debug level patterns
		{"This is a DEBUG message", "debug"},
		{"This is a TRACE message", "debug"},
		{"Debugging information", "debug"},

		// Default level
		{"This is a normal info message", "info"},
		{"GET /api/users 200 OK", "info"},
		{"Request completed successfully", "info"},
		{"Starting application", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.message[:min(30, len(tt.message))], func(t *testing.T) {
			got := InferLogLevel(tt.message)
			if got != tt.want {
				t.Errorf("InferLogLevel(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantMsg   string
		wantValid bool
	}{
		{
			name:      "valid timestamp with message",
			line:      "2026-04-27T10:00:00.000000000Z GET /api/health 200",
			wantMsg:   "GET /api/health 200",
			wantValid: true,
		},
		{
			name:      "timestamp only",
			line:      "2026-04-27T10:00:00.000000000Z",
			wantMsg:   "",
			wantValid: true,
		},
		{
			name:      "invalid timestamp",
			line:      "Not a timestamp line",
			wantMsg:   "Not a timestamp line",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, msg, ok := parseTimestamp(tt.line)
			if ok != tt.wantValid {
				t.Errorf("parseTimestamp(%q) ok = %v, want %v", tt.line, ok, tt.wantValid)
			}
			if msg != tt.wantMsg {
				t.Errorf("parseTimestamp(%q) msg = %q, want %q", tt.line, msg, tt.wantMsg)
			}
			if tt.wantValid && ts.IsZero() {
				t.Errorf("parseTimestamp(%q) returned zero time", tt.line)
			}
		})
	}
}

func TestSubscriptionKey(t *testing.T) {
	key := subscriptionKey("cluster-1", "default", "nginx-abc123", "nginx")
	expected := "cluster-1/default/nginx-abc123/nginx"
	if key != expected {
		t.Errorf("subscriptionKey() = %q, want %q", key, expected)
	}
}

func TestContainerLogEntry(t *testing.T) {
	entry := websocket.ContainerLogEntry{
		ClusterID:   "cluster-1",
		ClusterName: "prod-cluster",
		Namespace:   "default",
		Pod:         "nginx-abc123",
		Container:   "nginx",
		Message:     "2026-04-27 10:00:00 GET /health 200",
		Level:       "info",
		Timestamp:   "2026-04-27T10:00:00Z",
	}

	if entry.ClusterID != "cluster-1" {
		t.Errorf("expected ClusterID cluster-1, got %s", entry.ClusterID)
	}
	if entry.Level != "info" {
		t.Errorf("expected level info, got %s", entry.Level)
	}
}

func TestContainerLogSubscribeRequest(t *testing.T) {
	req := websocket.ContainerLogSubscribeRequest{
		ClusterID:   "cluster-1",
		ClusterName: "prod-cluster",
		Namespace:   "default",
		Pod:         "nginx-abc123",
		Container:   "nginx",
	}

	if req.ClusterID != "cluster-1" {
		t.Errorf("expected ClusterID cluster-1, got %s", req.ClusterID)
	}
	if req.Pod != "nginx-abc123" {
		t.Errorf("expected Pod nginx-abc123, got %s", req.Pod)
	}
	if req.Container != "nginx" {
		t.Errorf("expected Container nginx, got %s", req.Container)
	}
}

func TestContainerLogSubscribeRequest_NoContainer(t *testing.T) {
	req := websocket.ContainerLogSubscribeRequest{
		ClusterID:   "cluster-1",
		ClusterName: "prod-cluster",
		Namespace:   "default",
		Pod:         "nginx-abc123",
		Container:   "",
	}

	if req.Container != "" {
		t.Errorf("expected empty Container, got %s", req.Container)
	}
}

func TestGetEnv(t *testing.T) {
	val := getEnv("NON_EXISTENT_ENV_VAR_12345", "default")
	if val != "default" {
		t.Errorf("getEnv with non-existent key = %q, want %q", val, "default")
	}
}

func TestInferLogLevel_EmptyMessage(t *testing.T) {
	level := InferLogLevel("")
	if level != "info" {
		t.Errorf("InferLogLevel(%q) = %q, want %q", "", level, "info")
	}
}

func TestInferLogLevel_CaseInsensitive(t *testing.T) {
	tests := []string{"error", "ERROR", "Error", "ErRoR"}
	for _, msg := range tests {
		level := InferLogLevel(msg)
		if level != "error" {
			t.Errorf("InferLogLevel(%q) = %q, want %q", msg, level, "error")
		}
	}
}

func TestInferLogLevel_WarnCaseInsensitive(t *testing.T) {
	tests := []string{"warn", "WARN", "Warn", "WaRn"}
	for _, msg := range tests {
		level := InferLogLevel(msg)
		if level != "warn" {
			t.Errorf("InferLogLevel(%q) = %q, want %q", msg, level, "warn")
		}
	}
}

func TestInferLogLevel_Priority(t *testing.T) {
	msg := "WARNING: Operation failed with ERROR"
	level := InferLogLevel(msg)
	if level != "error" {
		t.Errorf("InferLogLevel(%q) = %q, want %q (error should take priority)", msg, level, "error")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}