// Package pipeline tests
// NOTE: These are DEV TESTS - may use httptest mocks for fast local development.
// For QA tests with real environment, see manager_integration_test.go

package pipeline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManager_CreatePipeline(t *testing.T) {
	m := NewManager()

	pipeline := m.CreatePipeline("test-pipeline", "https://github.com/repo", "main", []string{"build", "test", "deploy"})

	if pipeline.Name != "test-pipeline" {
		t.Errorf("expected name 'test-pipeline', got '%s'", pipeline.Name)
	}
	if pipeline.Repository != "https://github.com/repo" {
		t.Errorf("expected repository 'https://github.com/repo', got '%s'", pipeline.Repository)
	}
	if len(pipeline.Stages) != 3 {
		t.Errorf("expected 3 stages, got %d", len(pipeline.Stages))
	}
	if pipeline.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestManager_GetPipeline(t *testing.T) {
	m := NewManager()

	created := m.CreatePipeline("test-pipeline", "", "", nil)
	retrieved := m.GetPipeline(created.ID)

	if retrieved == nil {
		t.Fatal("expected to retrieve pipeline, got nil")
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, retrieved.ID)
	}

	// Non-existent pipeline
	missing := m.GetPipeline("non-existent")
	if missing != nil {
		t.Error("expected nil for non-existent pipeline")
	}
}

func TestManager_ListPipelines(t *testing.T) {
	m := NewManager()

	m.CreatePipeline("pipeline-1", "", "", nil)
	m.CreatePipeline("pipeline-2", "", "", nil)

	pipelines := m.ListPipelines()
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestManager_DeletePipeline(t *testing.T) {
	m := NewManager()

	p1 := m.CreatePipeline("pipeline-1", "", "", nil)
	m.CreatePipeline("pipeline-2", "", "", nil)

	deleted := m.DeletePipeline(p1.ID)
	if !deleted {
		t.Error("expected DeletePipeline to return true")
	}

	// Verify only one remains
	pipelines := m.ListPipelines()
	if len(pipelines) != 1 {
		t.Errorf("expected 1 pipeline after delete, got %d", len(pipelines))
	}
}

func TestManager_ExecutePipeline(t *testing.T) {
	m := NewManager()

	p := m.CreatePipeline("test-pipeline", "", "", []string{"build", "test"})
	trigger := Trigger{Type: "manual", By: "test"}

	run, err := m.ExecutePipeline(p.ID, trigger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.PipelineID != p.ID {
		t.Errorf("expected PipelineID '%s', got '%s'", p.ID, run.PipelineID)
	}
	if run.Status != StatusRunning {
		t.Errorf("expected status 'running', got '%s'", run.Status)
	}
	if len(run.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(run.Stages))
	}
}

func TestManager_ExecutePipeline_NotFound(t *testing.T) {
	m := NewManager()
	trigger := Trigger{Type: "manual", By: "test"}

	_, err := m.ExecutePipeline("non-existent", trigger)
	if err == nil {
		t.Error("expected error for non-existent pipeline")
	}
}

func TestManager_GetRuns(t *testing.T) {
	m := NewManager()

	p := m.CreatePipeline("test", "", "", nil)
	trigger := Trigger{Type: "manual", By: "test"}
	m.ExecutePipeline(p.ID, trigger)
	m.ExecutePipeline(p.ID, trigger)

	runs := m.GetRuns(p.ID)
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

func TestManager_CreatePipelineHTTP(t *testing.T) {
	m := NewManager()

	body := `{"name":"http-pipeline","stages":["build","test"]}`
	req := httptest.NewRequest("POST", "/api/pipelines", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.CreatePipelineHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var pipeline Pipeline
	if err := json.NewDecoder(w.Body).Decode(&pipeline); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if pipeline.Name != "http-pipeline" {
		t.Errorf("expected name 'http-pipeline', got '%s'", pipeline.Name)
	}
}

func TestManager_ListPipelinesHTTP(t *testing.T) {
	m := NewManager()
	m.CreatePipeline("pipeline-1", "", "", nil)

	req := httptest.NewRequest("GET", "/api/pipelines", nil)
	w := httptest.NewRecorder()

	m.ListPipelinesHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data       []*Pipeline `json:"data"`
		Pagination struct {
			Total   int  `json:"total"`
			Limit   int  `json:"limit"`
			Offset  int  `json:"offset"`
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 pipeline, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Pagination.Total)
	}
}

// YAML Parsing Tests

func TestParsePipelineYAML(t *testing.T) {
	yamlData := []byte(`
name: my-pipeline
description: Test pipeline
repository: https://github.com/org/repo
branch: main
stages:
  - name: build
    commands:
      - go build ./...
    env:
      BUILD_ENV: production
    timeout: 5m
  - name: test
    commands:
      - go test ./...
    timeout: 10m
strategy:
  type: rolling
  max_surge: 20
  max_unavailable: 10
`)
	config, err := ParsePipelineYAML(yamlData)
	if err != nil {
		t.Fatalf("unexpected error parsing YAML: %v", err)
	}
	if config.Name != "my-pipeline" {
		t.Errorf("expected name 'my-pipeline', got '%s'", config.Name)
	}
	if config.Description != "Test pipeline" {
		t.Errorf("expected description 'Test pipeline', got '%s'", config.Description)
	}
	if len(config.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(config.Stages))
	}
	if config.Stages[0].Name != "build" {
		t.Errorf("expected first stage name 'build', got '%s'", config.Stages[0].Name)
	}
	if len(config.Stages[0].Commands) != 1 {
		t.Errorf("expected 1 command in build stage, got %d", len(config.Stages[0].Commands))
	}
	if config.Stages[0].Commands[0] != "go build ./..." {
		t.Errorf("expected command 'go build ./...', got '%s'", config.Stages[0].Commands[0])
	}
	if config.Stages[0].Env["BUILD_ENV"] != "production" {
		t.Errorf("expected env BUILD_ENV=production, got '%s'", config.Stages[0].Env["BUILD_ENV"])
	}
	if config.Stages[0].Timeout != "5m" {
		t.Errorf("expected timeout '5m', got '%s'", config.Stages[0].Timeout)
	}
	if config.Strategy.Type != "rolling" {
		t.Errorf("expected strategy type 'rolling', got '%s'", config.Strategy.Type)
	}
	if config.Strategy.MaxSurge != 20 {
		t.Errorf("expected max_surge 20, got %d", config.Strategy.MaxSurge)
	}
	if config.Strategy.MaxUnavailable != 10 {
		t.Errorf("expected max_unavailable 10, got %d", config.Strategy.MaxUnavailable)
	}
}

func TestParsePipelineYAML_Invalid(t *testing.T) {
	invalidYAML := []byte(`name: test
stages: invalid`)
	_, err := ParsePipelineYAML(invalidYAML)
	if err == nil {
		t.Error("expected error parsing invalid YAML")
	}
}

func TestValidatePipelineConfig(t *testing.T) {
	// Valid config
	validConfig := &PipelineConfig{
		Name: "test-pipeline",
		Stages: []StageConfig{
			{Name: "build"},
		},
		Strategy: StrategyConfig{Type: "rolling"},
	}
	if err := ValidatePipelineConfig(validConfig); err != nil {
		t.Errorf("unexpected error for valid config: %v", err)
	}

	// Missing name
	noNameConfig := &PipelineConfig{
		Stages: []StageConfig{{Name: "build"}},
	}
	if err := ValidatePipelineConfig(noNameConfig); err == nil {
		t.Error("expected error for missing name")
	}

	// No stages
	noStagesConfig := &PipelineConfig{
		Name: "test-pipeline",
	}
	if err := ValidatePipelineConfig(noStagesConfig); err == nil {
		t.Error("expected error for no stages")
	}

	// Stage with no name
	unnamedStageConfig := &PipelineConfig{
		Name: "test-pipeline",
		Stages: []StageConfig{
			{Name: ""},
		},
	}
	if err := ValidatePipelineConfig(unnamedStageConfig); err == nil {
		t.Error("expected error for unnamed stage")
	}

	// Invalid strategy type
	invalidStrategyConfig := &PipelineConfig{
		Name: "test-pipeline",
		Stages: []StageConfig{
			{Name: "build"},
		},
		Strategy: StrategyConfig{Type: "invalid"},
	}
	if err := ValidatePipelineConfig(invalidStrategyConfig); err == nil {
		t.Error("expected error for invalid strategy type")
	}
}

func TestCreatePipelineFromYAML(t *testing.T) {
	m := NewManager()

	yamlData := []byte(`
name: yaml-pipeline
description: Created from YAML
repository: https://github.com/org/repo
branch: main
stages:
  - name: build
    commands:
      - go build ./...
    env:
      BUILD_ENV: production
    timeout: 5m
  - name: test
    commands:
      - go test ./...
strategy:
  type: rolling
  max_surge: 20
  max_unavailable: 10
`)
	pipeline, err := m.CreatePipelineFromYAML(yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.Name != "yaml-pipeline" {
		t.Errorf("expected name 'yaml-pipeline', got '%s'", pipeline.Name)
	}
	if pipeline.Description != "Created from YAML" {
		t.Errorf("expected description 'Created from YAML', got '%s'", pipeline.Description)
	}
	if len(pipeline.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(pipeline.Stages))
	}
	if pipeline.Stages[0] != "build" {
		t.Errorf("expected first stage 'build', got '%s'", pipeline.Stages[0])
	}
	if pipeline.Stages[1] != "test" {
		t.Errorf("expected second stage 'test', got '%s'", pipeline.Stages[1])
	}
	if pipeline.Strategy == nil {
		t.Fatal("expected strategy to be set")
	}
	if pipeline.Strategy.Type != "rolling" {
		t.Errorf("expected strategy type 'rolling', got '%s'", pipeline.Strategy.Type)
	}
	if pipeline.Config == nil {
		t.Fatal("expected config to be set")
	}
	stages, ok := pipeline.Config["stages"].([]StageConfig)
	if !ok {
		t.Fatal("expected stages in config")
	}
	if len(stages) != 2 {
		t.Errorf("expected 2 stage configs, got %d", len(stages))
	}
}

func TestCreatePipelineFromYAML_Invalid(t *testing.T) {
	m := NewManager()

	// Invalid YAML
	_, err := m.CreatePipelineFromYAML([]byte(`invalid: yaml`))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}

	// Valid YAML but missing required fields
	_, err = m.CreatePipelineFromYAML([]byte(`name: test`))
	if err == nil {
		t.Error("expected error for missing stages")
	}
}

func TestCreatePipelineFromYAMLHTTP(t *testing.T) {
	m := NewManager()

	body := `
name: http-yaml-pipeline
repository: https://github.com/org/repo
branch: main
stages:
  - name: build
    commands:
      - go build ./...
`
	req := httptest.NewRequest("POST", "/api/pipelines/yaml", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/yaml")
	w := httptest.NewRecorder()

	m.CreatePipelineFromYAMLHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var pipeline Pipeline
	if err := json.NewDecoder(w.Body).Decode(&pipeline); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if pipeline.Name != "http-yaml-pipeline" {
		t.Errorf("expected name 'http-yaml-pipeline', got '%s'", pipeline.Name)
	}
}

// Deployment Strategy Tests

func TestExecuteDeployment_BlueGreen(t *testing.T) {
	m := NewManager()

	p := m.CreatePipeline("bg-pipeline", "", "", []string{"build", "deploy"})
	strategy := &StrategyConfig{
		Type:              "blue_green",
		BlueGreenReplicas: 3,
	}

	deployment, err := m.ExecuteDeployment(p.ID, strategy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected deployment, got nil")
	}
	if deployment.Strategy != StrategyBlueGreen {
		t.Errorf("expected strategy blue_green, got %s", deployment.Strategy)
	}
	if deployment.Status != DeploymentStatusRunning && deployment.Status != DeploymentStatusPending {
		t.Errorf("expected status pending or running, got %s", deployment.Status)
	}
	if deployment.TotalStages != 2 {
		t.Errorf("expected 2 stages for blue-green, got %d", deployment.TotalStages)
	}
}

func TestExecuteDeployment_Canary(t *testing.T) {
	m := NewManager()

	p := m.CreatePipeline("canary-pipeline", "", "", []string{"build", "deploy"})
	strategy := &StrategyConfig{
		Type:         "canary",
		CanaryStages: []int{1, 5, 25, 100},
	}

	deployment, err := m.ExecuteDeployment(p.ID, strategy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected deployment, got nil")
	}
	if deployment.Strategy != StrategyCanary {
		t.Errorf("expected strategy canary, got %s", deployment.Strategy)
	}
	if deployment.TotalStages != 4 {
		t.Errorf("expected 4 canary stages, got %d", deployment.TotalStages)
	}
}

func TestExecuteDeployment_Rolling(t *testing.T) {
	m := NewManager()

	p := m.CreatePipeline("rolling-pipeline", "", "", []string{"build", "deploy"})
	strategy := &StrategyConfig{
		Type:           "rolling",
		MaxSurge:       20,
		MaxUnavailable: 10,
	}

	deployment, err := m.ExecuteDeployment(p.ID, strategy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected deployment, got nil")
	}
	if deployment.Strategy != StrategyRolling {
		t.Errorf("expected strategy rolling, got %s", deployment.Strategy)
	}
}

func TestExecuteDeployment_PipelineNotFound(t *testing.T) {
	m := NewManager()

	strategy := &StrategyConfig{Type: "rolling"}
	_, err := m.ExecuteDeployment("non-existent", strategy)
	if err == nil {
		t.Error("expected error for non-existent pipeline")
	}
}

func TestExecuteDeployment_UnsupportedStrategy(t *testing.T) {
	m := NewManager()

	p := m.CreatePipeline("test", "", "", nil)
	strategy := &StrategyConfig{Type: "unsupported"}

	_, err := m.ExecuteDeployment(p.ID, strategy)
	if err == nil {
		t.Error("expected error for unsupported strategy")
	}
}

func TestBlueGreenDeployer_GetNextStage(t *testing.T) {
	deployer := NewBlueGreenDeployer(&BlueGreenConfig{
		ActiveEnvironment:   "blue",
		InactiveEnvironment: "green",
		AutoSwitch:          true,
	})

	deployment := &Deployment{Stage: 1, TotalStages: 2}
	next, total, err := deployer.GetNextStage(deployment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != 2 {
		t.Errorf("expected next stage 2, got %d", next)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
}

func TestCanaryDeployer_GetNextStage(t *testing.T) {
	deployer := NewCanaryDeployer(&CanaryConfig{
		Stages: []CanaryStage{
			{Percentage: 1, Name: "1%"},
			{Percentage: 5, Name: "5%"},
			{Percentage: 25, Name: "25%"},
			{Percentage: 100, Name: "100%"},
		},
	})

	deployment := &Deployment{Stage: 2, TotalStages: 4}
	next, total, err := deployer.GetNextStage(deployment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != 3 {
		t.Errorf("expected next stage 3, got %d", next)
	}
	if total != 4 {
		t.Errorf("expected total 4, got %d", total)
	}
}

func TestRollingDeployer_GetNextStage(t *testing.T) {
	deployer := NewRollingDeployer(&RollingConfig{
		MaxSurge:       "20%",
		MaxUnavailable: "10%",
		BatchSize:       3,
	})

	deployment := &Deployment{Stage: 1, TotalStages: 3}
	next, total, err := deployer.GetNextStage(deployment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != 2 {
		t.Errorf("expected next stage 2, got %d", next)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
}

func TestConvertToRuntimeConfig_BlueGreen(t *testing.T) {
	config := &StrategyConfig{
		Type:              "blue_green",
		BlueGreenReplicas: 3,
	}

	convertToRuntimeConfig(config)

	if config.BlueGreen == nil {
		t.Fatal("expected BlueGreen config to be set")
	}
	if config.BlueGreen.ActiveEnvironment != "blue" {
		t.Errorf("expected active environment 'blue', got '%s'", config.BlueGreen.ActiveEnvironment)
	}
	if config.BlueGreen.InactiveEnvironment != "green" {
		t.Errorf("expected inactive environment 'green', got '%s'", config.BlueGreen.InactiveEnvironment)
	}
	if !config.BlueGreen.AutoSwitch {
		t.Error("expected AutoSwitch to be true")
	}
}

func TestConvertToRuntimeConfig_Canary(t *testing.T) {
	config := &StrategyConfig{
		Type:         "canary",
		CanaryStages: []int{1, 5, 25, 100},
	}

	convertToRuntimeConfig(config)

	if config.Canary == nil {
		t.Fatal("expected Canary config to be set")
	}
	if len(config.Canary.Stages) != 4 {
		t.Errorf("expected 4 canary stages, got %d", len(config.Canary.Stages))
	}
	if config.Canary.Stages[0].Percentage != 1 {
		t.Errorf("expected first stage 1%%, got %d%%", config.Canary.Stages[0].Percentage)
	}
	if !config.Canary.AutoPromote {
		t.Error("expected AutoPromote to be true")
	}
}

func TestConvertToRuntimeConfig_Rolling(t *testing.T) {
	config := &StrategyConfig{
		Type:           "rolling",
		MaxSurge:       20,
		MaxUnavailable: 10,
	}

	convertToRuntimeConfig(config)

	if config.Rolling == nil {
		t.Fatal("expected Rolling config to be set")
	}
	if config.Rolling.MaxSurge != "20%" {
		t.Errorf("expected max_surge '20%%', got '%s'", config.Rolling.MaxSurge)
	}
	if config.Rolling.MaxUnavailable != "10%" {
		t.Errorf("expected max_unavailable '10%%', got '%s'", config.Rolling.MaxUnavailable)
	}
	if config.Rolling.BatchSize != 2 {
		t.Errorf("expected default batch size 2, got %d", config.Rolling.BatchSize)
	}
}

func TestDeploymentTypes(t *testing.T) {
	// Test DeploymentStrategy constants
	if StrategyBlueGreen != "blue_green" {
		t.Errorf("expected StrategyBlueGreen to be 'blue_green', got '%s'", StrategyBlueGreen)
	}
	if StrategyCanary != "canary" {
		t.Errorf("expected StrategyCanary to be 'canary', got '%s'", StrategyCanary)
	}
	if StrategyRolling != "rolling" {
		t.Errorf("expected StrategyRolling to be 'rolling', got '%s'", StrategyRolling)
	}

	// Test DeploymentStatus constants
	if DeploymentStatusPending != "pending" {
		t.Errorf("expected DeploymentStatusPending to be 'pending', got '%s'", DeploymentStatusPending)
	}
	if DeploymentStatusSuccess != "success" {
		t.Errorf("expected DeploymentStatusSuccess to be 'success', got '%s'", DeploymentStatusSuccess)
	}
	if DeploymentStatusFailed != "failed" {
		t.Errorf("expected DeploymentStatusFailed to be 'failed', got '%s'", DeploymentStatusFailed)
	}
}
