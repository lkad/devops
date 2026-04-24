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

	var pipelines []*Pipeline
	if err := json.NewDecoder(w.Body).Decode(&pipelines); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(pipelines) != 1 {
		t.Errorf("expected 1 pipeline, got %d", len(pipelines))
	}
}
