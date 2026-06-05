// Package alerts implements the alert-notification subsystem.
// See openspec/specs/alert-notification/spec.md for the
// authoritative requirements.
//
// Layered rules (per the implementation playbook):
//
//	handler -> service -> repository / dispatcher
//	dsl     -> standalone package-level type
//
// The DSL parser is its own file because it has no GORM / Gin
// dependencies and is exercised by both the repository (when
// validating AlertRule.ConditionDSL) and the runtime evaluation
// path (Phase 6 will wire the evaluator; here we only parse).
package alerts

import (
	"strings"
	"testing"
)

// TestDSL_Parse_Valid covers the happy-path grammar accepted by
// ParseDSL: "<metric> <op> <threshold>" where op is one of the
// five comparison operators and threshold is a float literal.
// The metric and threshold are exposed on the returned Condition
// for the evaluator (Phase 6).
func TestDSL_Parse_Valid(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		metric    string
		op        ConditionOp
		thresh    float64
	}{
		{"gt", "cpu > 80", "cpu", OpGT, 80},
		{"lt", "mem < 0.5", "mem", OpLT, 0.5},
		{"gte", "disk >= 95", "disk", OpGTE, 95},
		{"lte", "load <= 2.0", "load", OpLTE, 2.0},
		{"eq", "active == 1", "active", OpEQ, 1},
		{"negative threshold", "temp < -5", "temp", OpLT, -5},
		{"metric with underscore", "http_5xx > 10", "http_5xx", OpGT, 10},
		{"metric with dot", "node.cpu >= 90", "node.cpu", OpGTE, 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ParseDSL(tc.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if c.Metric != tc.metric {
				t.Errorf("metric: got %q want %q", c.Metric, tc.metric)
			}
			if c.Op != tc.op {
				t.Errorf("op: got %q want %q", c.Op, tc.op)
			}
			if c.Threshold != tc.thresh {
				t.Errorf("threshold: got %v want %v", c.Threshold, tc.thresh)
			}
		})
	}
}

// TestDSL_Parse_Invalid covers the negative paths. Every malformed
// input must return a non-nil error wrapping an APIError with
// VALIDATION_ERROR so the handler can render a 400.
func TestDSL_Parse_Invalid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"missing threshold", "cpu > "},
		{"missing op", "cpu 80"},
		{"bad op", "cpu ~~ 80"},
		{"non-numeric threshold", "cpu > eighty"},
		{"just metric", "cpu"},
		{"just op", ">"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDSL(tc.input)
			if err == nil {
				t.Fatalf("expected error for %q", tc.input)
			}
			if !IsValidation(err) {
				t.Errorf("expected VALIDATION_ERROR APIError, got %T %v", err, err)
			}
		})
	}
}

// TestDSL_String verifies the round-trip representation so a
// stored AlertRule.ConditionDSL renders as the canonical form.
func TestDSL_String(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"cpu > 80", "cpu > 80"},
		{"mem < 0.5", "mem < 0.5"},
		{"disk >= 95", "disk >= 95"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			c, err := ParseDSL(tc.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if c.String() != tc.expected {
				t.Errorf("String: got %q want %q", c.String(), tc.expected)
			}
		})
	}
}

// TestDSL_Eval smoke-tests the evaluation path. Phase 6 will wire
// the metric source; here we use an inline map so the operator
// matrix is exercised without a database.
func TestDSL_Eval(t *testing.T) {
	cases := []struct {
		input   string
		value   float64
		want    bool
	}{
		{"cpu > 80", 90, true},
		{"cpu > 80", 80, false},
		{"cpu >= 80", 80, true},
		{"cpu < 50", 40, true},
		{"cpu <= 50", 50, true},
		{"cpu == 50", 50, true},
		{"cpu == 50", 51, false},
	}
	for _, tc := range cases {
		t.Run(strings.ReplaceAll(tc.input, " ", "_"), func(t *testing.T) {
			c, err := ParseDSL(tc.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := c.Eval(tc.value); got != tc.want {
				t.Errorf("Eval(%v): got %v want %v", tc.value, got, tc.want)
			}
		})
	}
}
