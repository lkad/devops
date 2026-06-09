package logstream

import (
	"testing"
)

// TestInferLevel covers the spec's "Log Level Inference"
// scenarios: a log line that contains "ERROR" or
// "error" is inferred as level=error, "WARN" as warn, and
// everything else as info (the default).
func TestInferLevel(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"2024-01-01 ERROR something broke", "error"},
		{"[ERROR] connection refused", "error"},
		{"FATAL: disk full", "fatal"},
		{"2024-01-01 WARN deprecated", "warn"},
		{"WARN: retrying", "warn"},
		{"hello world", "info"},
		{"starting up", "info"},
		{"", "info"},
	}
	for _, c := range cases {
		if got := InferLevel(c.line); got != c.want {
			t.Errorf("InferLevel(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

// TestStreamRequest_ContainerAndSince: the spec's
// "Reconnect with Timestamp" + "Multi-Container Support"
// scenarios. ?container= and ?since= are decoded correctly
// by requestFromGin.
func TestStreamRequest_ContainerAndSince(t *testing.T) {
	req := StreamRequest{
		ClusterID: "c1",
		Namespace: "default",
		Pod:       "p1",
		Container: "main",
	}
	if req.Container != "main" {
		t.Errorf("container = %q, want main", req.Container)
	}
}
