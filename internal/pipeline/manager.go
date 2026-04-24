package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	mu        sync.RWMutex
	pipelines map[string]*Pipeline
	runs      map[string][]*Run
}

type Pipeline struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Repository  string            `json:"repository"`
	Branch      string            `json:"branch"`
	Stages      []string          `json:"stages"`
	Config      map[string]interface{} `json:"config"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type Run struct {
	ID           string       `json:"id"`
	PipelineID  string       `json:"pipeline_id"`
	PipelineName string       `json:"pipeline_name"`
	Status      RunStatus    `json:"status"`
	Stages      []*Stage     `json:"stages"`
	Trigger     Trigger      `json:"trigger"`
	Logs        []*LogEntry  `json:"logs"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
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
	Name      string    `json:"name"`
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
	pipelines := m.ListPipelines()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pipelines)
}

func (m *Manager) CreatePipelineHTTP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string   `json:"name"`
		Repository string   `json:"repository"`
		Branch     string   `json:"branch"`
		Stages     []string `json:"stages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, "pipeline not found", http.StatusNotFound)
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
		http.Error(w, "pipeline not found", http.StatusNotFound)
	}
}

func (m *Manager) ExecutePipelineHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	trigger := Trigger{Type: "manual", By: "api"}
	run, err := m.ExecutePipeline(id, trigger)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(run)
}
