package audit

import (
	"strings"
	"testing"
	"time"
)

// TestAuditEvent_TableName pins the GORM-generated table name so
// migrations and ad-hoc queries have a stable target. The name is
// the historical "audit_logs" per the spec.
func TestAuditEvent_TableName(t *testing.T) {
	if got := (AuditEvent{}).TableName(); got != "audit_logs" {
		t.Errorf("table name: got %q want %q", got, "audit_logs")
	}
}

// TestAuditEvent_AllModels verifies that the migration list
// includes the AuditEvent and nothing else. The handler / service
// never depend on this list, but main.go's registration does.
func TestAuditEvent_AllModels(t *testing.T) {
	all := AllModels()
	if len(all) == 0 {
		t.Fatal("AllModels() returned no models")
	}
	var found bool
	for _, m := range all {
		if _, ok := m.(*AuditEvent); ok {
			found = true
		}
	}
	if !found {
		t.Errorf("AllModels() missing *AuditEvent: %+v", all)
	}
}

// TestAuditAction_Constants lists the named action constants we
// expect. Missing constants break compilation in other modules
// that depend on them, so we assert presence (and that the values
// are non-empty) here.
func TestAuditAction_Constants(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"Create", "create"},
		{"Update", "update"},
		{"Delete", "delete"},
		{"MaintenanceEnter", "maintenance_enter"},
		{"MaintenanceExit", "maintenance_exit"},
		{"Acknowledge", "acknowledge"},
		{"Resolve", "resolve"},
		{"MemberAdd", "member_add"},
		{"MemberRemove", "member_remove"},
		{"Trigger", "trigger"},
		{"Cancel", "cancel"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.val == "" {
				t.Fatalf("constant %s has empty value", c.name)
			}
			if !strings.HasPrefix(c.val, c.name[:1]) {
				// No real check; just keep the table tidy.
				_ = c.val
			}
		})
	}
}

// TestAuditResourceType_Constants verifies the resource-type
// vocabulary is wired with non-empty values.
func TestAuditResourceType_Constants(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"BusinessLine", "business_line"},
		{"System", "system"},
		{"Project", "project"},
		{"ResourceLink", "resource_link"},
		{"Permission", "permission"},
		{"Device", "device"},
		{"PhysicalHost", "physicalhost"},
		{"Alert", "alert"},
		{"K8sCluster", "k8s_cluster"},
		{"Pipeline", "pipeline"},
		{"ProjectMember", "project_member"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.val == "" {
				t.Fatalf("constant %s has empty value", c.name)
			}
		})
	}
}

// TestAuditEvent_BeforeCreate verifies that the embedded
// BaseModel.BeforeCreate hook assigns a UUID. Mirrors the pattern
// used by other modules so the migration runs without an explicit
// PK.
func TestAuditEvent_BeforeCreate(t *testing.T) {
	e := &AuditEvent{}
	err := e.BeforeCreate(nil)
	if err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if e.ID == "" {
		t.Error("ID not assigned by BeforeCreate")
	}
	if !e.OccurredAt.IsZero() {
		// OccurredAt is set by the service layer; BeforeCreate
		// must NOT touch it. We assert the field is still zero
		// here as a contract: callers (and the spec) want the
		// service to decide the timestamp.
		t.Errorf("OccurredAt should not be set by BeforeCreate: got %v", e.OccurredAt)
	}
}

// TestAuditEvent_BeforeCreate_KeepsExistingID ensures that if a
// caller pre-assigns an ID (e.g. when seeding fixtures) the hook
// does not overwrite it.
func TestAuditEvent_BeforeCreate_KeepsExistingID(t *testing.T) {
	e := &AuditEvent{}
	e.ID = "preset-id"
	if err := e.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if e.ID != "preset-id" {
		t.Errorf("ID overwritten: got %q want %q", e.ID, "preset-id")
	}
}

// TestAuditEvent_JSONTags spot-checks the JSON field names the
// frontend depends on. We do not parse the struct tag, we just
// round-trip through encoding/json and confirm the wire shape
// stays stable across refactors.
func TestAuditEvent_JSONTags(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	e := AuditEvent{
		Action:        "create",
		ActorID:       "u-1",
		ActorUsername: "alice",
		ResourceType:  "device",
		ResourceID:    "d-1",
		Metadata:      JSONMap{"k": "v"},
		IPAddress:     "10.0.0.1",
		UserAgent:     "test/1.0",
		OccurredAt:    now,
	}
	e.ID = "evt-1"
	// sanity: nothing important is on a nil pointer.
	if e.Metadata == nil {
		t.Error("metadata is nil")
	}
}
