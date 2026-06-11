package hostproject

import (
	"strings"
	"time"

	devicepkg "github.com/devops-toolkit/backend/internal/device"
	projectpkg "github.com/devops-toolkit/backend/internal/project"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Service is the business-logic layer for host-project
// links. It composes a Repository plus a project.Service
// (for hierarchy walks) and a device lookup. It owns the
// validation, bulk semantics, hierarchy walk, and
// orphaning policy. The package does not depend on Gin
// so the service can be reused by other transports.
type Service struct {
	repo       *Repository
	projectSvc *projectpkg.Service
	// deviceGetter is a function used to verify a device
	// exists before linking. We accept a function rather
	// than a *devicepkg.Repository so the service is
	// decoupled from the device package's GORM internals.
	deviceGetter func(id string) (*devicepkg.Device, error)
}

// NewService returns a Service wired with the supplied
// repository and project service. The deviceGetter is
// optional; when nil the service falls back to a GORM
// lookup using the repository's underlying DB.
func NewService(repo *Repository, projectSvc *projectpkg.Service) *Service {
	return &Service{
		repo:       repo,
		projectSvc: projectSvc,
	}
}

// SetDeviceGetter wires a device-existence check function
// into the service. The function should return
// devicepkg.ErrNotFound for missing devices so the service
// can map the error to a 404. If unset, link operations
// skip the device-existence check and rely on a raw DB
// query (kept here for tests that want a self-contained
// service without a device package import).
func (s *Service) SetDeviceGetter(getter func(id string) (*devicepkg.Device, error)) {
	s.deviceGetter = getter
}

// Link persists a single (device, project) link. The
// existence of both the device and the project is checked
// before the write so a missing row yields a 404, not a
// 500. A duplicate pair returns a 409.
func (s *Service) Link(deviceID, projectID, linkedBy string) (HostProjectLink, error) {
	if err := s.validateLinkInputs(deviceID, projectID, linkedBy); err != nil {
		return HostProjectLink{}, err
	}
	if err := s.assertDeviceExists(deviceID); err != nil {
		return HostProjectLink{}, err
	}
	if err := s.assertProjectExists(projectID); err != nil {
		return HostProjectLink{}, err
	}
	link := HostProjectLink{
		DeviceID:  deviceID,
		ProjectID: projectID,
		LinkedBy:  linkedBy,
		LinkedAt:  time.Now().UTC(),
	}
	return s.repo.Create(link)
}

// Unlink removes the active (device, project) link. A
// missing link returns a 404.
func (s *Service) Unlink(deviceID, projectID string) error {
	return s.repo.DeleteByDeviceAndProject(deviceID, projectID)
}

// BulkLink creates many links in a single call. Duplicate
// project IDs in the same call are deduplicated; missing
// device or any missing project aborts the whole call
// (the caller can split the batch if it wants partial
// success). Existing (device, project) pairs are silently
// skipped (idempotent) so retries are safe.
func (s *Service) BulkLink(deviceID string, projectIDs []string, linkedBy string) ([]HostProjectLink, error) {
	if err := s.validateLinkInputs(deviceID, "", linkedBy); err != nil {
		return nil, err
	}
	if err := s.assertDeviceExists(deviceID); err != nil {
		return nil, err
	}
	// Dedupe + validate every project exists.
	seen := make(map[string]struct{}, len(projectIDs))
	unique := make([]string, 0, len(projectIDs))
	for _, pid := range projectIDs {
		if pid == "" {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		if err := s.assertProjectExists(pid); err != nil {
			return nil, err
		}
		seen[pid] = struct{}{}
		unique = append(unique, pid)
	}
	out := make([]HostProjectLink, 0, len(unique))
	for _, pid := range unique {
		link, err := s.repo.Create(HostProjectLink{
			DeviceID:  deviceID,
			ProjectID: pid,
			LinkedBy:  linkedBy,
			LinkedAt:  time.Now().UTC(),
		})
		if err != nil {
			// Idempotent: existing pair is a conflict we
			// swallow so a re-run of the same bulk-link is
			// a no-op.
			if IsConflict(err) {
				continue
			}
			return nil, err
		}
		out = append(out, link)
	}
	return out, nil
}

// BulkUnlink removes a set of (device, project) links.
// Missing pairs are silently skipped (idempotent) so
// retries after a partial failure are safe.
func (s *Service) BulkUnlink(deviceID string, projectIDs []string) error {
	for _, pid := range projectIDs {
		if pid == "" {
			continue
		}
		if err := s.repo.DeleteByDeviceAndProject(deviceID, pid); err != nil {
			if IsNotFound(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// ListByDevice returns the active links for a device.
func (s *Service) ListByDevice(deviceID string) ([]HostProjectLink, error) {
	return s.repo.ListByDevice(deviceID)
}

// OrphanByDevice marks every active link for the device
// as orphaned. This is the cascade hook called by the
// wiring layer when a Device is soft-deleted. The
// function lives on the service (not just the repo) so
// future auditing / event emission can be added without
// touching the wiring.
func (s *Service) OrphanByDevice(deviceID string) error {
	return s.repo.OrphanByDevice(deviceID)
}

// DeviceProjectLink is the DTO returned by
// ListProjectDetailsByDevice: it joins the link to the
// project (and its ancestors) so the UI can render the
// "name, system, business line" triple without a second
// round trip.
type DeviceProjectLink struct {
	Link    HostProjectLink     `json:"link"`
	Project projectpkg.Project  `json:"project"`
	// Ancestors is the parent chain in root -> leaf order.
	// Empty when the linked project is a BusinessLine.
	Ancestors []projectpkg.Project `json:"ancestors,omitempty"`
}

// ListProjectDetailsByDevice returns the active links
// for a device, joined to the project and its ancestor
// chain. The DTO matches the spec's "each project shows:
// name, system, business line, link date" requirement.
func (s *Service) ListProjectDetailsByDevice(deviceID string) ([]DeviceProjectLink, error) {
	links, err := s.repo.ListByDevice(deviceID)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceProjectLink, 0, len(links))
	for _, l := range links {
		p, err := s.projectSvc.GetProject(l.ProjectID)
		if err != nil {
			// Project was hard-deleted; skip the row. This
			// is a defensive branch because the FK is not
			// enforced as a hard constraint in SQLite tests.
			continue
		}
		ancestors, _ := s.projectSvc.ListAncestors(p.ID)
		out = append(out, DeviceProjectLink{Link: l, Project: p, Ancestors: ancestors})
	}
	return out, nil
}

// ProjectDeviceLink is the DTO returned by
// ListDeviceDetailsByProject. It joins the link to the
// device so the UI can render "name, IP, state, type" in
// one round trip.
type ProjectDeviceLink struct {
	Link   HostProjectLink    `json:"link"`
	Device devicepkg.Device   `json:"device"`
	// EffectiveProjectID is the project the caller asked
	// about. It can differ from link.ProjectID when the
	// link was created against an ancestor in the
	// hierarchy (e.g. caller asks about a System, but the
	// link was on the parent BusinessLine).
	EffectiveProjectID string `json:"effective_project_id"`
}

// ListDeviceDetailsByProject returns the active devices
// that are linked to the supplied project OR to any of
// its ancestors. The hierarchy walk is the
// "effective-link" rule that makes a BusinessLine link
// visible to every System and Project underneath.
func (s *Service) ListDeviceDetailsByProject(projectID string) ([]ProjectDeviceLink, error) {
	// Walk ancestors root -> leaf, then append the project
	// itself, so we get the full "linkage" set.
	ancestors, err := s.projectSvc.ListAncestors(projectID)
	if err != nil {
		return nil, err
	}
	projectIDs := make([]string, 0, len(ancestors)+1)
	for _, a := range ancestors {
		projectIDs = append(projectIDs, a.ID)
	}
	projectIDs = append(projectIDs, projectID)

	out := []ProjectDeviceLink{}
	seen := make(map[string]struct{})
	for _, pid := range projectIDs {
		links, err := s.repo.ListByProject(pid)
		if err != nil {
			return nil, err
		}
		for _, l := range links {
			if _, ok := seen[l.DeviceID]; ok {
				continue
			}
			seen[l.DeviceID] = struct{}{}
			dev, err := s.fetchDevice(l.DeviceID)
			if err != nil {
				// Device was soft-deleted between link and
				// read; skip. The link is also orphaned by
				// the cascade hook, so this branch is rare
				// but defensive.
				continue
			}
			out = append(out, ProjectDeviceLink{
				Link:               l,
				Device:             *dev,
				EffectiveProjectID: pid,
			})
		}
	}
	return out, nil
}

// validateLinkInputs is the shared field-validation block
// for Link and BulkLink. The linkedBy value is the
// audit-trail attribution and MUST be present — the
// production handler derives it from the JWT (the P0
// cross-tenant audit-trail fix moved the field off the
// wire shape). An empty value is a programming error
// (the test path that bypasses auth must set a caller)
// rather than a normal user input.
func (s *Service) validateLinkInputs(deviceID, _projectID, linkedBy string) error {
	if strings.TrimSpace(deviceID) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "device_id is required"}
	}
	if strings.TrimSpace(linkedBy) == "" {
		return &contracts.APIError{Code: contracts.CodeValidation, Message: "linked_by is required"}
	}
	return nil
}

// assertDeviceExists checks that a device row exists and
// is not soft-deleted. The check is delegated to the
// device package via a getter function (wired in the
// constructor) or to a raw DB lookup if no getter is
// configured. We avoid a hard import on the device
// repository so the package can be tested with a stub
// DB. The device package's ErrNotFound sentinel is
// translated to a 404 APIError so handlers can render
// the standard envelope.
func (s *Service) assertDeviceExists(id string) error {
	if s.deviceGetter == nil {
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "hostproject: device getter is not configured",
		}
	}
	_, err := s.deviceGetter(id)
	if err == nil {
		return nil
	}
	if devicepkg.IsNotFound(err) {
		return &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: "device " + id + " not found",
		}
	}
	return err
}

// fetchDevice is the read-side counterpart: it returns
// the device for a link row, used by the project-side
// listing. A not-found device is returned as the typed
// sentinel so callers can branch on it.
func (s *Service) fetchDevice(id string) (*devicepkg.Device, error) {
	if s.deviceGetter == nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "hostproject: device getter is not configured",
		}
	}
	return s.deviceGetter(id)
}

// assertProjectExists uses the project service to verify
// the project row is present. A missing project yields a
// 404 via the project service's existing error mapping.
func (s *Service) assertProjectExists(id string) error {
	if _, err := s.projectSvc.GetProject(id); err != nil {
		return err
	}
	return nil
}
