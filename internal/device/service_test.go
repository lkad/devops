package device

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devops-toolkit/backend/internal/auth/caller"
	"github.com/devops-toolkit/backend/pkg/contracts"
)


// newDeveloperCaller returns a *caller.Caller for the
// v0.3.0.0 P0 #2 cross-tenant tests.
func newDeveloperCaller(id string) *caller.Caller {
	return caller.New(&contracts.User{ID: id, Username: id, Role: contracts.RoleDeveloper})
}

// newSuperAdminCaller returns a *caller.Caller for the
// v0.3.0.0 P0 #2 cross-tenant tests.
func newSuperAdminCaller() *caller.Caller {
	return caller.New(&contracts.User{ID: "root", Username: "root", Role: contracts.RoleSuperAdmin})
}
// deviceSvcFixture builds a Service backed by a fresh in-memory
// repository. The Service has no external dependencies — every
// behaviour is exercised through Repository — so a single
// constructor is enough.
func deviceSvcFixture(t *testing.T) *Service {
	t.Helper()
	return NewService(NewRepository(openDeviceDB(t)))
}

// TestService_Create_DefaultsState verifies that a Create with
// an empty State fills in DeviceStateOnline so callers can omit
// the field. The model hook also sets this; the service asserts
// it as a contract.
func TestService_Create_DefaultsState(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, err := svc.Create(CreateDeviceInput{
		Name: "h1",
		Type: DeviceTypePhysicalHost,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if d.State != DeviceStateOnline {
		t.Errorf("State = %q, want online", d.State)
	}
}

// TestService_Create_ValidatesType rejects an unknown type with a
// 400-mappable APIError. The handler depends on contracts.CodeValidation.
func TestService_Create_ValidatesType(t *testing.T) {
	svc := deviceSvcFixture(t)
	_, err := svc.Create(CreateDeviceInput{
		Name: "h1",
		Type: DeviceType("satellite"),
	})
	if err == nil {
		t.Fatal("expected validation error for unknown type")
	}
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("expected *contracts.APIError, got %T", err)
	}
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
}

// TestService_Create_ValidatesName rejects blank names.
func TestService_Create_ValidatesName(t *testing.T) {
	svc := deviceSvcFixture(t)
	_, err := svc.Create(CreateDeviceInput{Type: DeviceTypePhysicalHost})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
	apiErr := err.(*contracts.APIError)
	if apiErr.Code != contracts.CodeValidation {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "name") {
		t.Errorf("message = %q, want mention of 'name'", apiErr.Message)
	}
}

// TestService_Get_NotFound covers the service's handling of a
// missing row. The handler maps CodeNotFound to a 404.
func TestService_Get_NotFound(t *testing.T) {
	svc := deviceSvcFixture(t)
	_, err := svc.Get("missing")
	if err == nil {
		t.Fatal("expected not-found")
	}
	apiErr := err.(*contracts.APIError)
	if apiErr.Code != contracts.CodeNotFound {
		t.Errorf("code = %q, want NOT_FOUND", apiErr.Code)
	}
}

// TestService_Get_OK pins the happy path so the 200 render is
// stable across refactors.
func TestService_Get_OK(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, err := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Get(d.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != d.ID {
		t.Errorf("ID = %q, want %q", got.ID, d.ID)
	}
}

// TestService_Update_ValidatesName rejects blank names on
// update — the same rule as Create.
func TestService_Update_ValidatesName(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost})
	blank := ""
	_, err := svc.Update(d.ID, UpdateDeviceInput{Name: &blank})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Update_AllowsPartialChanges: only the supplied
// fields are touched. A nil Name must not blank the row.
func TestService_Update_AllowsPartialChanges(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost})
	newState := DeviceStateMaintenance
	_, err := svc.Update(d.ID, UpdateDeviceInput{State: &newState})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := svc.Get(d.ID)
	if got.State != DeviceStateMaintenance {
		t.Errorf("State = %q, want maintenance", got.State)
	}
	if got.Name != "h" {
		t.Errorf("Name = %q, want h (unchanged)", got.Name)
	}
}

// TestService_Delete_SoftDelete_HidesFromGet confirms a deleted
// device is invisible to Get (the row stays in the DB but is
// soft-deleted).
func TestService_Delete_SoftDelete_HidesFromGet(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost})
	if err := svc.Delete(d.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(d.ID); err == nil {
		t.Error("expected not-found after delete")
	}
}

// TestService_Delete_NotFound covers double-delete / stale id.
func TestService_Delete_NotFound(t *testing.T) {
	svc := deviceSvcFixture(t)
	err := svc.Delete("missing")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_List_AppliesDefaultPagination verifies that a
// bare List returns up to the default page size and the total
// count.
func TestService_List_AppliesDefaultPagination(t *testing.T) {
	svc := deviceSvcFixture(t)
	for i := 0; i < 3; i++ {
		_, _ = svc.Create(CreateDeviceInput{Name: "n", Type: DeviceTypeContainer})
	}
	rows, total, err := svc.List(ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
}

// TestService_Action_EnterMaintenance transitions the device
// into the maintenance state.
func TestService_Action_EnterMaintenance(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost})
	out, err := svc.ApplyAction(d.ID, ActionEnterMaintenance)
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	if out.State != DeviceStateMaintenance {
		t.Errorf("State = %q, want maintenance", out.State)
	}
}

// TestService_Action_ExitMaintenance returns a device in
// maintenance to the online state.
func TestService_Action_ExitMaintenance(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost, State: DeviceStateMaintenance})
	out, err := svc.ApplyAction(d.ID, ActionExitMaintenance)
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	if out.State != DeviceStateOnline {
		t.Errorf("State = %q, want online", out.State)
	}
}

// TestService_Action_UnknownActionIsRejected with a 400.
func TestService_Action_UnknownActionIsRejected(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost})
	_, err := svc.ApplyAction(d.ID, "reboot")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Action_EnterMaintenanceRejectsIfAlreadyInMaintenance
// is the "invalid transition" scenario from the spec. We model
// the state machine conservatively: entering maintenance on a
// device that is already in maintenance is a 422 INVALID_STATE
// rather than a silent no-op, so the caller knows the action
// was a noop.
func TestService_Action_EnterMaintenanceRejectsIfAlreadyInMaintenance(t *testing.T) {
	svc := deviceSvcFixture(t)
	d, _ := svc.Create(CreateDeviceInput{Name: "h", Type: DeviceTypePhysicalHost, State: DeviceStateMaintenance})
	_, err := svc.ApplyAction(d.ID, ActionEnterMaintenance)
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("err = %v, want *contracts.APIError", err)
	}
	if apiErr.Code != contracts.CodeInvalidState {
		t.Errorf("code = %q, want INVALID_STATE", apiErr.Code)
	}
}

// TestService_Search runs a substring search through the
// service layer so the handler can call a single method.
func TestService_Search(t *testing.T) {
	svc := deviceSvcFixture(t)
	_, _ = svc.Create(CreateDeviceInput{Name: "webserver-01", Type: DeviceTypeContainer})
	_, _ = svc.Create(CreateDeviceInput{Name: "db-01", Type: DeviceTypeContainer})

	rows, total, err := svc.Search("web")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Name != "webserver-01" {
		t.Errorf("rows = %+v total=%d", rows, total)
	}
}

// TestService_GetDevice_CrossTenant_Denied covers the v0.3.0.0
// P0 #2 cross-tenant enforcement on the device module. A
// non-SuperAdmin caller is denied access; the per-device
// projectID filter is the follow-up work.
func TestService_GetDevice_CrossTenant_Denied(t *testing.T) {
	svc := deviceSvcFixture(t)
	cl := newDeveloperCaller("alice")
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.GetWithCaller(ctx, "any-id")
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

// TestService_GetDevice_SuperAdmin_Bypasses covers the spec
// rule that SuperAdmin is implicitly allowed to read any
// device. The ungoverned Get is then called; the read
// returns a not-found (the fixture has no rows) but the
// call MUST NOT short-circuit on the cross-tenant guard.
func TestService_GetDevice_SuperAdmin_Bypasses(t *testing.T) {
	svc := deviceSvcFixture(t)
	cl := newSuperAdminCaller()
	ctx := caller.WithContext(context.Background(), cl)
	_, err := svc.GetWithCaller(ctx, "any-id")
	// The not-found envelope is from the ungoverned path;
	// the cross-tenant guard passed (no ErrForbidden).
	if errors.Is(err, ErrForbidden) {
		t.Errorf("SuperAdmin should bypass cross-tenant, got ErrForbidden")
	}
}

// TestService_GetDevice_NilCaller_401 covers the fail-closed
// rule: a context without a caller MUST surface as
// ErrUnauthenticated.
func TestService_GetDevice_NilCaller_401(t *testing.T) {
	svc := deviceSvcFixture(t)
	_, err := svc.GetWithCaller(context.Background(), "any-id")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}
