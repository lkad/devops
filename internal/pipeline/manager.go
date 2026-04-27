package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/apierror"
	"github.com/devops-toolkit/internal/pagination"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	mu        sync.RWMutex
	pipelines map[string]*Pipeline
	runs      map[string][]*Run
}

type Pipeline struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Repository  string                 `json:"repository"`
	Branch      string                 `json:"branch"`
	Stages      []string               `json:"stages"`
	Strategy    *StrategyConfig        `json:"strategy,omitempty"`
	Config      map[string]interface{} `json:"config"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

type Run struct {
	ID           string    `json:"id"`
	PipelineID  string    `json:"pipeline_id"`
	PipelineName string    `json:"pipeline_name"`
	Status      RunStatus `json:"status"`
	Stages      []*Stage  `json:"stages"`
	Trigger     Trigger   `json:"trigger"`
	Logs        []*LogEntry `json:"logs"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

type RunStatus string

const (
	StatusPending   RunStatus = "pending"
	StatusRunning   RunStatus = "running"
	StatusSuccess   RunStatus = "success"
	StatusFailed    RunStatus = "failed"
	StatusCancelled RunStatus = "cancelled"
)

type Stage struct {
	Name       string    `json:"name"`
	Status    RunStatus `json:"status"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Logs      []string  `json:"logs,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type Trigger struct {
	Type    string `json:"type"`
	By      string `json:"by"`
	Comment string `json:"comment,omitempty"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Stage     string    `json:"stage"`
	Message   string    `json:"message"`
}

// YAML Configuration Types

// StageConfig represents the YAML configuration for a pipeline stage
type StageConfig struct {
	Name     string            `yaml:"name"`
	Commands []string          `yaml:"commands"`
	Env      map[string]string `yaml:"env"`
	Timeout  string            `yaml:"timeout"`
}

// PipelineConfig represents the YAML configuration for a complete pipeline
type PipelineConfig struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Repository  string        `yaml:"repository"`
	Branch      string        `yaml:"branch"`
	Stages      []StageConfig `yaml:"stages"`
	Strategy    StrategyConfig `yaml:"strategy"`
}

func NewManager() *Manager {
	return &Manager{
		pipelines: make(map[string]*Pipeline),
		runs:       make(map[string][]*Run),
	}
}

func (m *Manager) CreatePipeline(name, repo, branch string, stages []string) *Pipeline {
	p := &Pipeline{
		ID:         uuid.New().String(),
		Name:       name,
		Repository: repo,
		Branch:     branch,
		Stages:     stages,
		Config:     make(map[string]interface{}),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if p.Stages == nil {
		p.Stages = []string{"validate", "build", "test", "deploy"}
	}

	m.mu.Lock()
	m.pipelines[p.ID] = p
	m.mu.Unlock()

	return p
}

func (m *Manager) GetPipeline(id string) *Pipeline {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pipelines[id]
}

func (m *Manager) ListPipelines() []*Pipeline {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ps := make([]*Pipeline, 0, len(m.pipelines))
	for _, p := range m.pipelines {
		ps = append(ps, p)
	}
	return ps
}

func (m *Manager) DeletePipeline(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pipelines[id]; ok {
		delete(m.pipelines, id)
		return true
	}
	return false
}

func (m *Manager) ExecutePipeline(id string, trigger Trigger) (*Run, error) {
	m.mu.RLock()
	p, ok := m.pipelines[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", id)
	}

	run := &Run{
		ID:          uuid.New().String(),
		PipelineID:  p.ID,
		PipelineName: p.Name,
		Status:      StatusRunning,
		Stages:      make([]*Stage, len(p.Stages)),
		Trigger:     trigger,
		Logs:        make([]*LogEntry, 0),
		StartedAt:   time.Now(),
	}

	for i, stageName := range p.Stages {
		run.Stages[i] = &Stage{
			Name:   stageName,
			Status: StatusPending,
		}
	}

	m.mu.Lock()
	m.runs[p.ID] = append(m.runs[p.ID], run)
	m.mu.Unlock()

	// Execute stages asynchronously
	go m.executeStages(run, p.Stages)

	return run, nil
}

func (m *Manager) executeStages(run *Run, stageNames []string) {
	for _, stageName := range stageNames {
		for _, s := range run.Stages {
			if s.Name == stageName {
				m.runStage(run, s)
				break
			}
		}
		if run.Status == StatusFailed || run.Status == StatusCancelled {
			break
		}
	}
}

func (m *Manager) runStage(run *Run, stage *Stage) {
	now := time.Now()
	stage.Status = StatusRunning
	stage.StartedAt = &now

	// Simulate stage execution
	time.Sleep(500 * time.Millisecond)

	stage.Status = StatusSuccess
	finish := time.Now()
	stage.FinishedAt = &finish

	run.Logs = append(run.Logs, &LogEntry{
		Timestamp: now,
		Stage:     stage.Name,
		Message:   fmt.Sprintf("Stage %s completed successfully", stage.Name),
	})
}

func (m *Manager) GetRuns(pipelineID string) []*Run {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runs[pipelineID]
}

func (m *Manager) GetRun(pipelineID, runID string) *Run {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.runs[pipelineID] {
		if r.ID == runID {
			return r
		}
	}
	return nil
}

func (m *Manager) CancelRun(pipelineID, runID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.runs[pipelineID] {
		if r.ID == runID && r.Status == StatusRunning {
			r.Status = StatusCancelled
			finish := time.Now()
			r.FinishedAt = &finish
			return true
		}
	}
	return false
}

// HTTP handlers
func (m *Manager) ListPipelinesHTTP(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	pipelines := m.ListPipelines()
	// Apply pagination in-memory
	total := len(pipelines)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedPipelines := pipelines[start:end]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagination.NewPaginatedResponse(paginatedPipelines, total, limit, offset))
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (m *Manager) CreatePipelineHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string   `json:"name"`
		Repository string   `json:"repository"`
		Branch     string   `json:"branch"`
		Stages     []string `json:"stages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}

	p := m.CreatePipeline(input.Name, input.Repository, input.Branch, input.Stages)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (m *Manager) GetPipelineHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	p := m.GetPipeline(id)
	if p == nil {
		apierror.NotFound(w, "pipeline not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (m *Manager) DeletePipelineHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if m.DeletePipeline(id) {
		w.WriteHeader(http.StatusNoContent)
	} else {
		apierror.NotFound(w, "pipeline not found")
	}
}

func (m *Manager) ExecutePipelineHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	trigger := Trigger{Type: "manual", By: "api"}
	run, err := m.ExecutePipeline(id, trigger)
	if err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(run)
}

// YAML Parsing Functions

// ParsePipelineYAML parses a YAML byte slice into a PipelineConfig
func ParsePipelineYAML(data []byte) (*PipelineConfig, error) {
	var config PipelineConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return &config, nil
}

// ValidatePipelineConfig validates the pipeline configuration structure
func ValidatePipelineConfig(config *PipelineConfig) error {
	if config.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(config.Stages) == 0 {
		return fmt.Errorf("at least one stage is required")
	}
	for i, stage := range config.Stages {
		if stage.Name == "" {
			return fmt.Errorf("stage at index %d has no name", i)
		}
	}
	// Validate strategy type if provided
	if config.Strategy.Type != "" {
		validTypes := map[string]bool{"rolling": true, "canary": true, "blue_green": true}
		if !validTypes[config.Strategy.Type] {
			return fmt.Errorf("invalid strategy type: %s (valid: rolling, canary, blue_green)", config.Strategy.Type)
		}
	}
	return nil
}

// CreatePipelineFromYAML creates a Pipeline from YAML configuration
func (m *Manager) CreatePipelineFromYAML(data []byte) (*Pipeline, error) {
	config, err := ParsePipelineYAML(data)
	if err != nil {
		return nil, err
	}
	if err := ValidatePipelineConfig(config); err != nil {
		return nil, err
	}
	// Extract stage names for the basic Pipeline struct
	stageNames := make([]string, len(config.Stages))
	for i, stage := range config.Stages {
		stageNames[i] = stage.Name
	}
	p := m.CreatePipeline(config.Name, config.Repository, config.Branch, stageNames)
	p.Description = config.Description
	// Store the strategy in the Pipeline struct
	p.Strategy = &config.Strategy
	// Also store the full config including commands, env, timeout
	p.Config = map[string]interface{}{
		"stages": config.Stages,
	}
	return p, nil
}

// CreatePipelineFromYAMLHTTP handles HTTP requests with YAML body
func (m *Manager) CreatePipelineFromYAMLHTTP(w http.ResponseWriter, r *http.Request) {
	data := make([]byte, r.ContentLength)
	if _, err := r.Body.Read(data); err != nil && err.Error() != "EOF" {
		apierror.ValidationError(w, "failed to read request body")
		return
	}
	p, err := m.CreatePipelineFromYAML(data)
	if err != nil {
		apierror.ValidationError(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// =============================================================================
// Deployment Strategy Execution
// =============================================================================

// ExecuteDeployment runs a deployment with the specified strategy
func (m *Manager) ExecuteDeployment(pipelineID string, strategy *StrategyConfig) (*Deployment, error) {
	m.mu.RLock()
	p, ok := m.pipelines[pipelineID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", pipelineID)
	}

	// Convert YAML StrategyConfig to runtime config
	convertToRuntimeConfig(strategy)

	deployment := &Deployment{
		ID:          uuid.New().String(),
		PipelineID:  p.ID,
		Strategy:    DeploymentStrategy(strategy.Type),
		Status:      DeploymentStatusPending,
		StartedAt:   time.Now(),
		Logs:        make([]string, 0),
	}

	switch DeploymentStrategy(strategy.Type) {
	case StrategyBlueGreen:
		deployment.TotalStages = 2 // deploy, switch
	case StrategyCanary:
		if strategy.Canary != nil && len(strategy.Canary.Stages) > 0 {
			deployment.TotalStages = len(strategy.Canary.Stages)
		} else {
			deployment.TotalStages = 4 // default stages
		}
	case StrategyRolling:
		deployment.TotalStages = 1
	default:
		return nil, fmt.Errorf("unsupported deployment strategy: %s", strategy.Type)
	}

	m.mu.Lock()
	m.runs[p.ID] = append(m.runs[p.ID], &Run{
		ID:        deployment.ID,
		PipelineID: p.ID,
		Status:    StatusRunning,
	})
	m.mu.Unlock()

	go m.runDeployment(deployment, strategy)

	return deployment, nil
}

// convertToRuntimeConfig converts YAML StrategyConfig fields to runtime config objects
func convertToRuntimeConfig(config *StrategyConfig) {
	if config == nil {
		return
	}

	switch config.Type {
	case "blue_green":
		config.BlueGreen = &BlueGreenConfig{
			ActiveEnvironment:   "blue",
			InactiveEnvironment: "green",
			AutoSwitch:          true,
		}
	case "canary":
		stages := make([]CanaryStage, 0)
		if len(config.CanaryStages) > 0 {
			for _, pct := range config.CanaryStages {
				stages = append(stages, CanaryStage{
					Percentage: pct,
					Name:       fmt.Sprintf("%d%%", pct),
					Duration:   500 * time.Millisecond,
				})
			}
		} else {
			// Default canary stages
			stages = []CanaryStage{
				{Percentage: 1, Name: "1%", Duration: 500 * time.Millisecond},
				{Percentage: 5, Name: "5%", Duration: 500 * time.Millisecond},
				{Percentage: 25, Name: "25%", Duration: 500 * time.Millisecond},
				{Percentage: 100, Name: "100%", Duration: 500 * time.Millisecond},
			}
		}
		config.Canary = &CanaryConfig{
			Stages:      stages,
			AutoPromote: true,
		}
	case "rolling":
		config.Rolling = &RollingConfig{
			MaxSurge:       fmt.Sprintf("%d%%", config.MaxSurge),
			MaxUnavailable: fmt.Sprintf("%d%%", config.MaxUnavailable),
			BatchSize:      2,
			PauseDuration:  500 * time.Millisecond,
		}
	}
}

func (m *Manager) runDeployment(deployment *Deployment, strategy *StrategyConfig) {
	deployment.Status = DeploymentStatusRunning

	switch DeploymentStrategy(strategy.Type) {
	case StrategyBlueGreen:
		m.runBlueGreenDeployment(deployment, strategy.BlueGreen)
	case StrategyCanary:
		m.runCanaryDeployment(deployment, strategy.Canary)
	case StrategyRolling:
		m.runRollingDeployment(deployment, strategy.Rolling)
	}
}

// Blue-Green Deployment: Deploy to inactive environment, then switch traffic
func (m *Manager) runBlueGreenDeployment(deployment *Deployment, config *BlueGreenConfig) {
	log := func(msg string) {
		deployment.Logs = append(deployment.Logs, msg)
	}

	if config == nil {
		config = &BlueGreenConfig{
			ActiveEnvironment:   "blue",
			InactiveEnvironment: "green",
			AutoSwitch:          true,
		}
	}

	log(fmt.Sprintf("Starting Blue-Green deployment to %s environment", config.InactiveEnvironment))
	deployment.Stage = 1
	deployment.Message = fmt.Sprintf("Deploying to inactive environment: %s", config.InactiveEnvironment)

	// Simulate deployment to inactive environment
	time.Sleep(1 * time.Second)
	log(fmt.Sprintf("Successfully deployed to %s environment", config.InactiveEnvironment))

	if config.AutoSwitch {
		log(fmt.Sprintf("Switching traffic from %s to %s", config.ActiveEnvironment, config.InactiveEnvironment))
		deployment.Stage = 2
		deployment.Message = fmt.Sprintf("Switching traffic from %s to %s", config.ActiveEnvironment, config.InactiveEnvironment)
		time.Sleep(500 * time.Millisecond)
		log("Traffic switched successfully")
	}

	finish := time.Now()
	deployment.FinishedAt = &finish
	deployment.Status = DeploymentStatusSuccess
	deployment.Message = "Blue-Green deployment completed successfully"
	log("Blue-Green deployment completed")
}

// Canary Deployment: Gradually shift traffic through configured stages
func (m *Manager) runCanaryDeployment(deployment *Deployment, config *CanaryConfig) {
	log := func(msg string) {
		deployment.Logs = append(deployment.Logs, msg)
	}

	if config == nil || len(config.Stages) == 0 {
		config = &CanaryConfig{
			Stages: []CanaryStage{
				{Percentage: 1, Name: "1%", Duration: 500 * time.Millisecond},
				{Percentage: 5, Name: "5%", Duration: 500 * time.Millisecond},
				{Percentage: 25, Name: "25%", Duration: 500 * time.Millisecond},
				{Percentage: 100, Name: "100%", Duration: 500 * time.Millisecond},
			},
			AutoPromote: true,
		}
	}

	log(fmt.Sprintf("Starting Canary deployment with %d stages", len(config.Stages)))
	deployment.TotalStages = len(config.Stages)

	for i, stage := range config.Stages {
		deployment.Stage = i + 1
		deployment.CurrentWeight = stage.Percentage
		deployment.TargetWeight = stage.Percentage
		deployment.Message = fmt.Sprintf("Canary stage %s: routing %d%% traffic", stage.Name, stage.Percentage)

		log(fmt.Sprintf("Canary stage %d/%d: Routing %d%% traffic to canary", i+1, len(config.Stages), stage.Percentage))

		// Simulate canary stage execution
		time.Sleep(stage.Duration)

		if stage.Percentage == 100 {
			log("Canary deployment reached 100% - deployment complete")
		} else if config.AutoPromote && i < len(config.Stages)-1 {
			nextPercentage := config.Stages[i+1].Percentage
			log(fmt.Sprintf("Auto-promoting to next stage: %d%% traffic", nextPercentage))
		}
	}

	finish := time.Now()
	deployment.FinishedAt = &finish
	deployment.Status = DeploymentStatusSuccess
	deployment.CurrentWeight = 100
	deployment.TargetWeight = 100
	deployment.Message = "Canary deployment completed successfully"
}

// Rolling Deployment: Upgrade instances in batches respecting max_surge
func (m *Manager) runRollingDeployment(deployment *Deployment, config *RollingConfig) {
	log := func(msg string) {
		deployment.Logs = append(deployment.Logs, msg)
	}

	if config == nil {
		config = &RollingConfig{
			MaxSurge:       "20%",
			MaxUnavailable: "10%",
			BatchSize:       2,
			PauseDuration:  500 * time.Millisecond,
		}
	}

	// Parse batch size
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 2
	}

	log(fmt.Sprintf("Starting Rolling deployment with batch size %d, max_surge=%s, max_unavailable=%s",
		batchSize, config.MaxSurge, config.MaxUnavailable))

	deployment.TotalStages = batchSize

	for i := 0; i < batchSize; i++ {
		deployment.Stage = i + 1
		deployment.Message = fmt.Sprintf("Rolling update batch %d/%d", i+1, batchSize)

		log(fmt.Sprintf("Rolling update batch %d/%d: Updating instances...", i+1, batchSize))

		// Simulate batch update
		time.Sleep(config.PauseDuration)
		log(fmt.Sprintf("Rolling update batch %d/%d: Instances updated successfully", i+1, batchSize))

		if i < batchSize-1 {
			log(fmt.Sprintf("Pausing for %v before next batch...", config.PauseDuration))
		}
	}

	finish := time.Now()
	deployment.FinishedAt = &finish
	deployment.Status = DeploymentStatusSuccess
	deployment.Message = "Rolling deployment completed successfully"
	log("Rolling deployment completed")
}

// GetDeployment retrieves a deployment by pipeline ID and deployment ID
func (m *Manager) GetDeployment(pipelineID, deploymentID string) *Deployment {
	// This would typically be stored and retrieved from a deployments map
	// For now, return nil as deployments are ephemeral during execution
	return nil
}

// =============================================================================
// Deployer Interface Implementation
// =============================================================================

// Deploy implements the Deployer interface for Blue-Green
func (d *BlueGreenDeployer) Deploy(deployment *Deployment) error {
	return nil
}

// Rollback implements the Deployer interface for Blue-Green
func (d *BlueGreenDeployer) Rollback(deployment *Deployment) error {
	return nil
}

// GetNextStage implements the Deployer interface for Blue-Green
func (d *BlueGreenDeployer) GetNextStage(deployment *Deployment) (int, int, error) {
	if deployment.Stage >= deployment.TotalStages {
		return deployment.Stage, deployment.TotalStages, nil
	}
	return deployment.Stage + 1, deployment.TotalStages, nil
}

// Deploy implements the Deployer interface for Canary
func (d *CanaryDeployer) Deploy(deployment *Deployment) error {
	return nil
}

// Rollback implements the Deployer interface for Canary
func (d *CanaryDeployer) Rollback(deployment *Deployment) error {
	return nil
}

// GetNextStage implements the Deployer interface for Canary
func (d *CanaryDeployer) GetNextStage(deployment *Deployment) (int, int, error) {
	if d.config == nil || len(d.config.Stages) == 0 {
		return 1, 1, nil
	}
	if deployment.Stage >= len(d.config.Stages) {
		return deployment.Stage, len(d.config.Stages), nil
	}
	return deployment.Stage + 1, len(d.config.Stages), nil
}

// Deploy implements the Deployer interface for Rolling
func (d *RollingDeployer) Deploy(deployment *Deployment) error {
	return nil
}

// Rollback implements the Deployer interface for Rolling
func (d *RollingDeployer) Rollback(deployment *Deployment) error {
	return nil
}

// GetNextStage implements the Deployer interface for Rolling
func (d *RollingDeployer) GetNextStage(deployment *Deployment) (int, int, error) {
	if d.config == nil || d.config.BatchSize <= 0 {
		return 1, 1, nil
	}
	if deployment.Stage >= d.config.BatchSize {
		return deployment.Stage, d.config.BatchSize, nil
	}
	return deployment.Stage + 1, d.config.BatchSize, nil
}
