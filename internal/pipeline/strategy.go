// Strategy types and planners for the deployment strategies
// the spec calls for: blue-green, canary, rolling.
//
// A "phase" is the unit the existing pipeline executor
// already understands. The strategy planner emits a
// []Phase for one trigger, and the service layer turns
// each Phase into a PipelineStep so the existing
// step-by-step executor can run it. The result is that
// "blue_green" looks like a 4-step pipeline to the
// executor, and the strategy planner is the only place
// that knows the difference.

package pipeline

import (
	"errors"
	"fmt"
	"math"
)

// StrategyType is the strategy enum persisted on the
// Pipeline row. Empty string means "no strategy" — the
// existing linear step execution kicks in.
type StrategyType string

const (
	StrategyBlueGreen StrategyType = "blue_green"
	StrategyCanary    StrategyType = "canary"
	StrategyRolling   StrategyType = "rolling"
)

// Valid reports whether s is one of the three recognised
// strategies. An empty string is invalid (callers must
// treat that as "no strategy" before calling Valid).
func (s StrategyType) Valid() bool {
	switch s {
	case StrategyBlueGreen, StrategyCanary, StrategyRolling:
		return true
	}
	return false
}

// Phase is one step in a strategy's execution plan. A
// planner emits one or more phases; the service layer
// turns them into PipelineStep records so the existing
// step-execution machinery can run them.
type Phase struct {
	Name        string `json:"name"`
	Env         string `json:"env,omitempty"`         // "blue" | "green" | "canary" | "stable"
	TrafficPct  int    `json:"traffic_pct,omitempty"` // 0-100
	Command     string `json:"command,omitempty"`      // shell command the executor runs
	Healthcheck string `json:"healthcheck,omitempty"`  // URL or cmd for the canary health gate
}

// BlueGreenConfig is the configuration block for the
// blue-green strategy. ActiveEnv is the env currently
// serving traffic; InactiveEnv is where the new build
// goes before the cutover.
type BlueGreenConfig struct {
	ActiveEnv   string
	InactiveEnv string
}

// CanaryConfig is the configuration block for the canary
// strategy. Stages is the percentages of traffic the
// candidate receives at each gate; each stage is gated by
// the optional Healthcheck. The spec mandates ascending
// percentages — a planner that fails validation is itself
// a test failure (NewCanaryStrategyValidate).
type CanaryConfig struct {
	Baseline    string
	Candidate   string
	Stages      []int
	Healthcheck string
}

// RollingConfig is the configuration block for the rolling
// strategy. TotalInstances is the size of the fleet;
// MaxSurgePct controls how many old+new instances coexist
// during the rollout (the spec's example is 20%).
type RollingConfig struct {
	TotalInstances int
	MaxSurgePct    int
}

// BlueGreenStrategy implements the blue-green deployment
// planner. The 4-phase plan matches the spec's
// "deploy-inactive → smoke-inactive → switch-traffic →
// decommission" sequence.
type BlueGreenStrategy struct {
	cfg BlueGreenConfig
}

// NewBlueGreenStrategy builds a planner. The constructor
// does not validate; an empty ActiveEnv/InactiveEnv is
// tolerated and surfaces in the rendered Phase.Env.
func NewBlueGreenStrategy(cfg BlueGreenConfig) *BlueGreenStrategy {
	return &BlueGreenStrategy{cfg: cfg}
}

// Plan emits the 4-phase blue-green sequence.
func (s *BlueGreenStrategy) Plan() []Phase {
	inactive := s.cfg.InactiveEnv
	active := s.cfg.ActiveEnv
	return []Phase{
		{Name: "deploy-inactive", Env: inactive, Command: "deploy.sh " + inactive},
		{Name: "smoke-inactive", Env: inactive, Command: "smoke.sh " + inactive, Healthcheck: "/healthz"},
		{Name: "switch-traffic", Env: inactive, Command: "traffic-switch.sh " + active + " " + inactive},
		{Name: "decommission", Env: active, Command: "teardown.sh " + active},
	}
}

// CanaryStrategy implements the canary deployment
// planner. The spec mandates strict ascending traffic
// percentages; NewCanaryStrategyValidate is the constructor
// that enforces this; NewCanaryStrategy does not (and is
// the path the service uses for in-DB config that has
// already been validated).
type CanaryStrategy struct {
	cfg CanaryConfig
}

// NewCanaryStrategy builds a planner without validating
// stage monotonicity. Use NewCanaryStrategyValidate when
// the input comes from a fresh user request.
func NewCanaryStrategy(cfg CanaryConfig) *CanaryStrategy {
	return &CanaryStrategy{cfg: cfg}
}

// NewCanaryStrategyValidate wraps NewCanaryStrategy with a
// monotonicity check on cfg.Stages. Returns an error so
// the service can surface a 400 to the caller.
func NewCanaryStrategyValidate(cfg CanaryConfig) (*CanaryStrategy, error) {
	for i := 1; i < len(cfg.Stages); i++ {
		if cfg.Stages[i] <= cfg.Stages[i-1] {
			return nil, fmt.Errorf("canary stages must be strictly ascending; got %v", cfg.Stages)
		}
	}
	if len(cfg.Stages) == 0 {
		return nil, errors.New("canary stages must not be empty")
	}
	return NewCanaryStrategy(cfg), nil
}

// Plan emits the canary sequence: one deploy-candidate
// phase, then one phase per stage gating traffic.
func (s *CanaryStrategy) Plan() []Phase {
	phases := []Phase{
		{Name: "deploy-candidate", Env: s.cfg.Candidate, Command: "deploy.sh " + s.cfg.Candidate},
	}
	for _, pct := range s.cfg.Stages {
		phases = append(phases, Phase{
			Name:        fmt.Sprintf("canary-%d%%", pct),
			Env:         s.cfg.Candidate,
			TrafficPct:  pct,
			Command:     fmt.Sprintf("traffic-shift.sh %d %s", pct, s.cfg.Candidate),
			Healthcheck: s.cfg.Healthcheck,
		})
	}
	return phases
}

// RollingStrategy implements the rolling-update planner.
// The batch size is ceil(total * surgePct / 100), so the
// spec's "max_surge 20%" on a 10-instance fleet yields 5
// batches of 2. The last batch may be smaller if the
// total is not evenly divisible.
type RollingStrategy struct {
	cfg RollingConfig
}

// NewRollingStrategy builds a rolling-update planner. The
// constructor applies sane defaults; MaxSurgePct of 0
// falls back to 20% (the spec's example).
func NewRollingStrategy(cfg RollingConfig) *RollingStrategy {
	if cfg.MaxSurgePct <= 0 {
		cfg.MaxSurgePct = 20
	}
	return &RollingStrategy{cfg: cfg}
}

// Plan splits TotalInstances into batches of ceil(total *
// surgePct / 100). The remaining tail is the last batch
// (smaller if the math doesn't divide evenly). Batch
// ranges ascend: 1-2/10, 3-4/10, … so the rendered
// command line is human-friendly.
func (s *RollingStrategy) Plan() []Phase {
	batchSize := int(math.Ceil(float64(s.cfg.TotalInstances) * float64(s.cfg.MaxSurgePct) / 100.0))
	if batchSize <= 0 {
		batchSize = 1
	}
	var phases []Phase
	done := 0
	batchNum := 0
	for done < s.cfg.TotalInstances {
		batchNum++
		end := done + batchSize
		if end > s.cfg.TotalInstances {
			end = s.cfg.TotalInstances
		}
		phases = append(phases, Phase{
			Name:    fmt.Sprintf("rolling-batch-%d", batchNum),
			Env:     "production",
			Command: fmt.Sprintf("rolling-update.sh %d-%d/%d", done+1, end, s.cfg.TotalInstances),
		})
		done = end
	}
	return phases
}

// PlanForPipeline is the dispatcher the service layer
// uses. Returns nil, nil when the pipeline has no
// strategy set (the caller should fall through to the
// existing linear step execution).
func PlanForPipeline(p *Pipeline) ([]Phase, error) {
	if p == nil {
		return nil, errors.New("pipeline is nil")
	}
	switch p.Strategy {
	case "":
		return nil, nil
	case StrategyBlueGreen:
		return NewBlueGreenStrategy(p.BlueGreenConfig).Plan(), nil
	case StrategyCanary:
		return NewCanaryStrategy(p.CanaryConfig).Plan(), nil
	case StrategyRolling:
		return NewRollingStrategy(p.RollingConfig).Plan(), nil
	default:
		return nil, fmt.Errorf("unknown strategy %q", p.Strategy)
	}
}
