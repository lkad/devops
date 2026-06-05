package alerts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ConditionOp is the comparison operator in a parsed DSL rule.
// The five values are the only ones accepted by ParseDSL.
type ConditionOp string

const (
	OpGT  ConditionOp = ">"
	OpLT  ConditionOp = "<"
	OpGTE ConditionOp = ">="
	OpLTE ConditionOp = "<="
	OpEQ  ConditionOp = "=="
)

// Condition is the parsed form of an alert rule's ConditionDSL.
// The parser splits the DSL into the three parts the evaluator
// needs: metric (LHS), op (operator), threshold (RHS).
type Condition struct {
	Metric    string
	Op        ConditionOp
	Threshold float64
}

// String renders the condition in canonical form. Inverse of
// ParseDSL: a round-trip preserves the input (modulo whitespace).
func (c Condition) String() string {
	return fmt.Sprintf("%s %s %s", c.Metric, c.Op, formatThreshold(c.Threshold))
}

// formatThreshold renders the threshold as Go's strconv would:
//   - integer-valued floats are rendered without a decimal
//   - everything else keeps the default float format
// This keeps the canonical form readable ("cpu > 80" instead of
// "cpu > 80.000000").
func formatThreshold(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// ParseDSL parses a condition DSL string of the form
// "<metric> <op> <threshold>". op must be one of >, <, >=, <=, ==.
// metric may contain letters, digits, underscores, and dots.
// threshold is a numeric literal (integer or float, may be
// negative).
//
// Returns a *contracts.APIError with VALIDATION_ERROR on any
// malformed input so the handler can render a 400 unchanged.
func ParseDSL(s string) (Condition, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Condition{}, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "condition DSL is empty",
		}
	}
	// Find the operator with two-char priority so ">=" is matched
	// before ">".
	for _, op := range []ConditionOp{OpGTE, OpLTE, OpEQ, OpGT, OpLT} {
		// We need to split on the operator as a token, not a
		// substring. Use a regex-free approach: look for a space-
		// padded or trailing operator.
		idx := indexOp(s, string(op))
		if idx < 0 {
			continue
		}
		metric := strings.TrimSpace(s[:idx])
		rest := strings.TrimSpace(s[idx+len(op):])
		if metric == "" {
			return Condition{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "condition DSL is missing the metric (LHS) name",
			}
		}
		if !validMetricName(metric) {
			return Condition{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("metric name %q contains illegal characters", metric),
			}
		}
		if rest == "" {
			return Condition{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "condition DSL is missing the threshold (RHS) value",
			}
		}
		thresh, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return Condition{}, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("threshold %q is not a number", rest),
			}
		}
		return Condition{Metric: metric, Op: op, Threshold: thresh}, nil
	}
	return Condition{}, &contracts.APIError{
		Code:    contracts.CodeValidation,
		Message: fmt.Sprintf("condition DSL %q has no recognised operator (use >, <, >=, <=, ==)", s),
	}
}

// indexOp returns the index of the first occurrence of op in s
// that is bounded by whitespace (or string start / end). It
// returns -1 if no such occurrence exists. This is what makes
// "metric_x>10" not split as "metric_x" and "10".
func indexOp(s, op string) int {
	for i := 0; i+len(op) <= len(s); i++ {
		if s[i:i+len(op)] != op {
			continue
		}
		// Left boundary: start of string or whitespace.
		leftOK := i == 0 || isSpace(s[i-1])
		// Right boundary: end of string or whitespace.
		rightOK := i+len(op) == len(s) || isSpace(s[i+len(op)])
		if leftOK && rightOK {
			return i
		}
	}
	return -1
}

// isSpace mirrors unicode.IsSpace for the ASCII subset the
// grammar accepts.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// validMetricName reports whether s is non-empty and contains
// only letters, digits, underscores, and dots. Dots are allowed
// so "node.cpu" is a valid metric name (K8s convention).
func validMetricName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

// Eval applies the parsed condition to a single sample value.
// The evaluator returns true if the sample satisfies the
// condition. It is a method on Condition so a rule lookup is the
// only lookup needed at evaluation time.
func (c Condition) Eval(value float64) bool {
	switch c.Op {
	case OpGT:
		return value > c.Threshold
	case OpLT:
		return value < c.Threshold
	case OpGTE:
		return value >= c.Threshold
	case OpLTE:
		return value <= c.Threshold
	case OpEQ:
		return value == c.Threshold
	}
	return false
}
