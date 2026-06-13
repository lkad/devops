package hostproject

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// newServiceWithDB wires a fully-wired Service (repo +
// project service + device getter) over a fresh in-memory
// DB and returns the DB so seed helpers can write to the
// same instance. Every helper below routes through this
// single DB so the service and the seeds see the same
// data.
//
// The membership checker is wired to projectSvc.MembershipChecker()
// so the v0.3.0.0 P0 #2 cross-tenant guard in projectSvc is
// satisfied for the seed projects (the test creates them
// without an actor, but the checker allows any caller in
// the test scenario).
func newServiceWithDB(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := openTestDB(t)
	repo := NewRepository(db)
	projRepo := projectpkg.NewRepository(db)
	projSvc := projectpkg.NewService(projRepo)
	projSvc.SetMembershipChecker(func(ctx context.Context, userID string) (map[string]struct{}, error) {
		// Allow any caller in tests so seed projects pass
		// the cross-tenant guard. Production wires
		// projSvc.MembershipChecker() which is the real
		// repo-backed lookup.
		return map[string]struct{}{}, nil
	})
	devRepo := devicepkg.NewRepository(db)
	svc := NewService(repo, projSvc)
	svc.SetDeviceGetter(devRepo.Get)
	return svc, db
}

// seedDevice creates a Device in the supplied DB.
func seedDeviceDB(t *testing.T, db *gorm.DB, name string) *devicepkg.Device {
	t.Helper()
	d := &devicepkg.Device{
		Name:  name,
		Type:  devicepkg.DeviceTypePhysicalHost,
		State: devicepkg.DeviceStateOnline,
	}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}
	return d
}

// seedProjectDB creates a ProjectType + Project in the
// supplied DB and returns the project.
func seedProjectDB(t *testing.T, db *gorm.DB, code string, parent *projectpkg.Project) *projectpkg.Project {
	t.Helper()
	pt := &projectpkg.ProjectType{Name: code + "-type", Weight: 1}
	if err := db.Create(pt).Error; err != nil {
		t.Fatalf("seed project type: %v", err)
	}
	var parentID *string
	if parent != nil {
		id := parent.ID
		parentID = &id
	}
	p := &projectpkg.Project{
		Name:     code,
		Code:     code,
		TypeID:   pt.ID,
		ParentID: parentID,
		Weight:   1,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return p
}

// apiErr extracts a *contracts.APIError from err.
func apiErr(t *testing.T, err error) *contracts.APIError {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ae, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("expected *contracts.APIError, got %T: %v", err, err)
	}
	return ae
}

// TestService_Link_CreatesRow covers the spec scenario
// "List projects linked to a physical host" and "Link
// physical host to project from project page": the
// service must persist the link and return the row.
func TestService_Link_CreatesRow(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)

	link, err := svc.Link(dev.ID, prj.ID, "actor-1")
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if link.DeviceID != dev.ID || link.ProjectID != prj.ID {
		t.Errorf("link = %+v, want device=%s project=%s", link, dev.ID, prj.ID)
	}
	if link.LinkedBy != "actor-1" {
		t.Errorf("LinkedBy = %q, want actor-1", link.LinkedBy)
	}
	if link.LinkedAt.IsZero() {
		t.Error("LinkedAt should be populated")
	}
}

// TestService_Link_RejectsUnknownDevice covers the
// "device does not exist" boundary.
func TestService_Link_RejectsUnknownDevice(t *testing.T) {
	svc, db := newServiceWithDB(t)
	prj := seedProjectDB(t, db, "p1", nil)

	_, err := svc.Link("ghost-device", prj.ID, "actor")
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeNotFound {
		t.Errorf("code = %q, want NOT_FOUND", ae.Code)
	}
	if !strings.Contains(strings.ToLower(ae.Message), "device") {
		t.Errorf("message = %q, want mention of device", ae.Message)
	}
}

// TestService_Link_RejectsUnknownProject mirrors the
// unknown-project boundary.
func TestService_Link_RejectsUnknownProject(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")

	_, err := svc.Link(dev.ID, "ghost-project", "actor")
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeNotFound {
		t.Errorf("code = %q, want NOT_FOUND", ae.Code)
	}
}

// TestService_Link_DuplicateReturnsConflict covers the
// spec's "composite (DeviceID, ProjectID) unique"
// invariant.
func TestService_Link_DuplicateReturnsConflict(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)

	if _, err := svc.Link(dev.ID, prj.ID, "actor"); err != nil {
		t.Fatalf("first link: %v", err)
	}
	_, err := svc.Link(dev.ID, prj.ID, "actor")
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeConflict {
		t.Errorf("code = %q, want CONFLICT", ae.Code)
	}
}

// TestService_Link_RejectsEmptyActor covers the trivial
// validation: LinkedBy must be supplied so audit
// attribution is complete.
func TestService_Link_RejectsEmptyActor(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)

	_, err := svc.Link(dev.ID, prj.ID, "")
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeValidation {
		t.Errorf("code = %q, want VALIDATION_ERROR", ae.Code)
	}
}

// TestService_Unlink_RemovesActiveLink covers the
// "Unlink host from project" spec scenario.
func TestService_Unlink_RemovesActiveLink(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	if _, err := svc.Link(dev.ID, prj.ID, "actor"); err != nil {
		t.Fatalf("link: %v", err)
	}

	if err := svc.Unlink(dev.ID, prj.ID); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	// Unlink a second time: must be a 404.
	err := svc.Unlink(dev.ID, prj.ID)
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeNotFound {
		t.Errorf("code = %q, want NOT_FOUND on missing unlink", ae.Code)
	}
}

// TestService_BulkLink_AddsAll verifies the bulk link
// creates every (device, project) pair.
func TestService_BulkLink_AddsAll(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	p1 := seedProjectDB(t, db, "p1", nil)
	p2 := seedProjectDB(t, db, "p2", nil)
	p3 := seedProjectDB(t, db, "p3", nil)

	added, err := svc.BulkLink(dev.ID, []string{p1.ID, p2.ID, p3.ID}, "actor")
	if err != nil {
		t.Fatalf("bulk link: %v", err)
	}
	if len(added) != 3 {
		t.Errorf("added = %d, want 3", len(added))
	}
}

// TestService_BulkLink_Dedupes verifies the bulk endpoint
// does not error when the same project is supplied twice
// in a single request.
func TestService_BulkLink_Dedupes(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	p1 := seedProjectDB(t, db, "p1", nil)

	added, err := svc.BulkLink(dev.ID, []string{p1.ID, p1.ID}, "actor")
	if err != nil {
		t.Fatalf("bulk link with duplicate: %v", err)
	}
	if len(added) != 1 {
		t.Errorf("added = %d, want 1 (duplicate deduped)", len(added))
	}
}

// TestService_BulkUnlink_RemovesAll verifies that bulk
// unlink removes every supplied (device, project) pair.
// Missing pairs are silently skipped.
func TestService_BulkUnlink_RemovesAll(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	p1 := seedProjectDB(t, db, "p1", nil)
	p2 := seedProjectDB(t, db, "p2", nil)
	if _, err := svc.BulkLink(dev.ID, []string{p1.ID, p2.ID}, "actor"); err != nil {
		t.Fatalf("seed links: %v", err)
	}

	if err := svc.BulkUnlink(dev.ID, []string{p1.ID, p2.ID, "ghost"}); err != nil {
		t.Fatalf("bulk unlink: %v", err)
	}
	links, err := svc.ListByDevice(dev.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0", len(links))
	}
}

// TestService_ListByDevice_ReturnsProjectDetails covers
// the "each project shows: name, system, business line,
// link date" requirement.
func TestService_ListByDevice_ReturnsProjectDetails(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	if _, err := svc.Link(dev.ID, prj.ID, "actor"); err != nil {
		t.Fatalf("link: %v", err)
	}

	rows, err := svc.ListProjectDetailsByDevice(dev.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Project.ID != prj.ID {
		t.Errorf("Project.ID = %s, want %s", r.Project.ID, prj.ID)
	}
	if r.Link.LinkedBy != "actor" {
		t.Errorf("Link.LinkedBy = %q, want actor", r.Link.LinkedBy)
	}
	if r.Link.LinkedAt.IsZero() {
		t.Error("Link.LinkedAt should be populated")
	}
}

// TestService_ListByDevice_ExcludesOrphaned covers the
// cascade: orphaning on the device side hides the link
// from the device's effective-link list.
func TestService_ListByDevice_ExcludesOrphaned(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	if _, err := svc.Link(dev.ID, prj.ID, "actor"); err != nil {
		t.Fatalf("link: %v", err)
	}

	if err := svc.OrphanByDevice(dev.ID); err != nil {
		t.Fatalf("orphan: %v", err)
	}

	links, err := svc.ListByDevice(dev.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("len(links) = %d, want 0 (orphaned)", len(links))
	}
}

// TestService_ListDevicesByProject_WalksHierarchy covers
// the "Project hierarchy cascade" rule. A link to a
// BusinessLine must show up in the device list for every
// sub-project.
func TestService_ListDevicesByProject_WalksHierarchy(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	bl := seedProjectDB(t, db, "bl", nil)
	sys := seedProjectDB(t, db, "sys", bl)
	prj := seedProjectDB(t, db, "prj", sys)

	if _, err := svc.Link(dev.ID, bl.ID, "actor"); err != nil {
		t.Fatalf("link: %v", err)
	}

	for _, p := range []*projectpkg.Project{bl, sys, prj} {
		rows, err := svc.ListDeviceDetailsByProject(p.ID)
		if err != nil {
			t.Fatalf("list for %s: %v", p.ID, err)
		}
		if len(rows) != 1 {
			t.Errorf("project %s: len(rows) = %d, want 1", p.Code, len(rows))
		}
		if len(rows) > 0 && rows[0].Device.ID != dev.ID {
			t.Errorf("project %s: device = %s, want %s", p.Code, rows[0].Device.ID, dev.ID)
		}
	}
}

// TestService_ListDevicesByProject_OnlyDescendants
// verifies a sibling branch does NOT see the link.
func TestService_ListDevicesByProject_OnlyDescendants(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	blA := seedProjectDB(t, db, "bl-a", nil)
	sysA := seedProjectDB(t, db, "sys-a", blA)
	blB := seedProjectDB(t, db, "bl-b", nil)
	sysB := seedProjectDB(t, db, "sys-b", blB)

	if _, err := svc.Link(dev.ID, blA.ID, "actor"); err != nil {
		t.Fatalf("link: %v", err)
	}

	rows, err := svc.ListDeviceDetailsByProject(sysB.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("sysB len(rows) = %d, want 0 (sibling)", len(rows))
	}
	rows, err = svc.ListDeviceDetailsByProject(sysA.ID)
	if err != nil {
		t.Fatalf("list sysA: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("sysA len(rows) = %d, want 1", len(rows))
	}
}

// TestService_BulkLink_RejectsEmptyActor covers the
// validation rule for the bulk path.
func TestService_BulkLink_RejectsEmptyActor(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	p1 := seedProjectDB(t, db, "p1", nil)

	_, err := svc.BulkLink(dev.ID, []string{p1.ID}, "")
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeValidation {
		t.Errorf("code = %q, want VALIDATION_ERROR", ae.Code)
	}
}

// TestService_BulkLink_RejectsUnknownDevice covers the
// "device does not exist" boundary for the bulk path.
func TestService_BulkLink_RejectsUnknownDevice(t *testing.T) {
	svc, db := newServiceWithDB(t)
	p1 := seedProjectDB(t, db, "p1", nil)

	_, err := svc.BulkLink("ghost", []string{p1.ID}, "actor")
	ae := apiErr(t, err)
	if ae.Code != contracts.CodeNotFound {
		t.Errorf("code = %q, want NOT_FOUND", ae.Code)
	}
}

// TestService_OrphanByDevice_RemovesFromProjectView
// covers the cross-side rule.
func TestService_OrphanByDevice_RemovesFromProjectView(t *testing.T) {
	svc, db := newServiceWithDB(t)
	dev := seedDeviceDB(t, db, "h1")
	prj := seedProjectDB(t, db, "p1", nil)
	if _, err := svc.Link(dev.ID, prj.ID, "actor"); err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := svc.OrphanByDevice(dev.ID); err != nil {
		t.Fatalf("orphan: %v", err)
	}
	rows, err := svc.ListDeviceDetailsByProject(prj.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0 (orphaned)", len(rows))
	}
}

// compile-time guard
var _ = (*gorm.DB)(nil)
