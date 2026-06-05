package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_DefaultsToInfoLevel(t *testing.T) {
	// GIVEN a logger created with no options
	// WHEN the level is queried
	// THEN it is INFO
	var buf bytes.Buffer
	l := New(WithWriter(&buf), WithLevel(""))
	if got := l.Level(); got != slog.LevelInfo {
		t.Errorf("default level = %v, want Info", got)
	}
}

func TestNew_ParseLevel(t *testing.T) {
	// GIVEN various level strings
	// WHEN parsed
	// THEN they map to the right slog levels
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // safe default
		{"", slog.LevelInfo},
	}
	for _, tt := range tests {
		l := New(WithLevel(tt.in))
		if got := l.Level(); got != tt.want {
			t.Errorf("level %q: got %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLogger_InfoWritesJSON(t *testing.T) {
	// GIVEN a JSON logger
	// WHEN Info is called with a key/value pair
	// THEN the output is valid JSON containing msg, level, and the pair
	var buf bytes.Buffer
	l := New(WithWriter(&buf), WithFormat("json"))
	l.Info("hello", "user", "alice")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not JSON: %q (%v)", buf.String(), err)
	}
	if got["msg"] != "hello" {
		t.Errorf("msg = %v", got["msg"])
	}
	if got["user"] != "alice" {
		t.Errorf("user = %v", got["user"])
	}
	if got["level"] != "INFO" {
		t.Errorf("level = %v", got["level"])
	}
}

func TestLogger_TextFormat(t *testing.T) {
	// GIVEN a text logger
	// WHEN Info is called
	// THEN the output is human-readable text (not JSON)
	var buf bytes.Buffer
	l := New(WithWriter(&buf), WithFormat("text"))
	l.Info("hello", "user", "alice")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Errorf("missing msg: %q", out)
	}
	if !strings.Contains(out, "user=alice") {
		t.Errorf("missing key=value: %q", out)
	}
	// text format should not be parseable as JSON
	if json.Valid(buf.Bytes()) {
		t.Errorf("text output should not be JSON: %q", out)
	}
}

func TestLogger_RespectsLevel(t *testing.T) {
	// GIVEN a logger with level=Warn
	// WHEN Info is called
	// THEN nothing is written
	var buf bytes.Buffer
	l := New(WithWriter(&buf), WithLevel("warn"))
	l.Info("should not appear")
	if buf.Len() != 0 {
		t.Errorf("info emitted at warn level: %q", buf.String())
	}
	// AND Warn is written
	l.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("warn not emitted: %q", buf.String())
	}
}

func TestLogger_NilSafe(t *testing.T) {
	// GIVEN a logger created without a writer
	// WHEN methods are called
	// THEN it does not panic and writes to io.Discard
	l := New()
	l.Info("safe call")
	// no assertion needed beyond not panicking
}

func TestMaskValue(t *testing.T) {
	// GIVEN a sensitive key like "password"
	// WHEN MaskValue is applied
	// THEN the value is replaced with "***"
	// AND a non-sensitive key passes through
	if got := MaskValue("password", "secret123"); got != "***" {
		t.Errorf("password mask = %q", got)
	}
	if got := MaskValue("Password", "secret123"); got != "***" {
		t.Errorf("Password mask = %q", got)
	}
	if got := MaskValue("token", "abc"); got != "***" {
		t.Errorf("token mask = %q", got)
	}
	if got := MaskValue("user", "alice"); got != "alice" {
		t.Errorf("user should pass through, got %q", got)
	}
}
