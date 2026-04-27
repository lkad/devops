package pipeline

import "time"

// DeploymentStrategy represents the type of deployment strategy
type DeploymentStrategy string

const (
	StrategyBlueGreen DeploymentStrategy = "blue_green"
	StrategyCanary    DeploymentStrategy = "canary"
	StrategyRolling   DeploymentStrategy = "rolling"
	StrategyNone      DeploymentStrategy = "none"
)

// StrategyConfig holds the deployment strategy configuration
type StrategyConfig struct {
	Type             string              `yaml:"type"`
	MaxSurge         int                 `yaml:"max_surge"`
	MaxUnavailable   int                 `yaml:"max_unavailable"`
	CanaryStages     []int               `yaml:"canary_stages,omitempty"`
	BlueGreenReplicas int                `yaml:"blue_green_replicas,omitempty"`
	// Runtime config - set after YAML parsing
	BlueGreen        *BlueGreenConfig   `yaml:"-"`
	Canary           *CanaryConfig      `yaml:"-"`
	Rolling          *RollingConfig     `yaml:"-"`
}

// BlueGreenConfig holds blue-green deployment specific configuration
type BlueGreenConfig struct {
	ActiveEnvironment   string        `yaml:"active_environment"`
	InactiveEnvironment string        `yaml:"inactive_environment"`
	PreviewDuration     time.Duration `yaml:"preview_duration"`
	AutoSwitch          bool          `yaml:"auto_switch"`
}

// CanaryConfig holds canary deployment specific configuration
type CanaryConfig struct {
	Stages        []CanaryStage  `yaml:"stages"`
	AutoPromote   bool           `yaml:"auto_promote"`
	PauseDuration time.Duration  `yaml:"pause_duration"`
}

// CanaryStage represents a single canary deployment stage
type CanaryStage struct {
	Percentage int           `yaml:"percentage"`
	Name      string        `yaml:"name"`
	Duration  time.Duration `yaml:"duration"`
}

// RollingConfig holds rolling deployment specific configuration
type RollingConfig struct {
	MaxSurge       string        `yaml:"max_surge"`
	MaxUnavailable string        `yaml:"max_unavailable"`
	BatchSize      int          `yaml:"batch_size"`
	PauseDuration  time.Duration `yaml:"pause_duration"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusRunning   DeploymentStatus = "running"
	DeploymentStatusPaused    DeploymentStatus = "paused"
	DeploymentStatusSuccess   DeploymentStatus = "success"
	DeploymentStatusFailed    DeploymentStatus = "failed"
	DeploymentStatusRollback  DeploymentStatus = "rollback"
)

// Deployment holds information about a specific deployment execution
type Deployment struct {
	ID            string            `json:"id"`
	PipelineID    string            `json:"pipeline_id"`
	RunID         string            `json:"run_id"`
	Strategy      DeploymentStrategy `json:"strategy"`
	Status        DeploymentStatus  `json:"status"`
	Stage         int               `json:"stage"`
	TotalStages   int               `json:"total_stages"`
	CurrentWeight int               `json:"current_weight"`
	TargetWeight  int               `json:"target_weight"`
	Message       string            `json:"message"`
	Error         string            `json:"error,omitempty"`
	StartedAt     time.Time         `json:"started_at"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
	Logs          []string          `json:"logs,omitempty"`
}

// BlueGreenDeployer handles blue-green deployment logic
type BlueGreenDeployer struct {
	config *BlueGreenConfig
}

// CanaryDeployer handles canary deployment logic
type CanaryDeployer struct {
	config *CanaryConfig
}

// RollingDeployer handles rolling deployment logic
type RollingDeployer struct {
	config *RollingConfig
}

// Deployer interface implemented by all deployment strategies
type Deployer interface {
	Deploy(deployment *Deployment) error
	Rollback(deployment *Deployment) error
	GetNextStage(deployment *Deployment) (int, int, error)
}

// NewBlueGreenDeployer creates a new blue-green deployer
func NewBlueGreenDeployer(config *BlueGreenConfig) *BlueGreenDeployer {
	return &BlueGreenDeployer{config: config}
}

// NewCanaryDeployer creates a new canary deployer
func NewCanaryDeployer(config *CanaryConfig) *CanaryDeployer {
	return &CanaryDeployer{config: config}
}

// NewRollingDeployer creates a new rolling deployer
func NewRollingDeployer(config *RollingConfig) *RollingDeployer {
	return &RollingDeployer{config: config}
}
