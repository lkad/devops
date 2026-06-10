package pipeline

import (
	"testing"
)

// TestService_Create_RejectsUnknownServiceID pins the spec
// rule: a pipeline with a service_id that does not exist
// returns 400. The catalog is the source of truth for
// which services exist.
func TestService_Create_RejectsUnknownServiceID(t *testing.T) {
	repo := NewRepository(openPipelineDB(t))
	// No servicecatalog.Service registered; any service_id
	// is unknown.
	svc := NewService(repo, &Fake{})

	_, err := svc.Create(CreatePipelineInput{
		Name:        "x",
		ProjectID:   "p1",
		TargetType:  TargetTypeProject,
		ServiceID:   "svc-does-not-exist",
		Trigger:     "manual",
		Steps:       StepList{{Name: "s", Type: StepTypeShell}},
	})
	if err == nil {
		t.Fatalf("expected error for unknown service_id")
	}
}

// TestService_Create_AcceptsKnownServiceID pins the
// happy path: a known service_id is persisted. The test
// injects a fake ServiceValidator that always returns
// nil for the "known" ID, mirroring how the catalog
// will be wired in main.go.
func TestService_Create_AcceptsKnownServiceID(t *testing.T) {
	repo := NewRepository(openPipelineDB(t))
	svc := NewService(repo, &Fake{}).
		WithServiceValidator(func(id string) error {
			if id == "svc-known" {
				return nil
			}
			return errUnknownService
		})

	out, err := svc.Create(CreatePipelineInput{
		Name:        "x",
		ProjectID:   "p1",
		TargetType:  TargetTypeProject,
		ServiceID:   "svc-known",
		Trigger:     "manual",
		Steps:       StepList{{Name: "s", Type: StepTypeShell}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.ServiceID != "svc-known" {
		t.Errorf("ServiceID = %q, want svc-known", out.ServiceID)
	}
}
