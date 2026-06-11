package physicalhost

import (
	"context"

	"github.com/devops-toolkit/backend/internal/audit"
)

// ServiceConfig bundles the dependencies of Service. The
// Service hides the joins between Repository and audit.Repository
// from the handler — the handler is a thin parse/render shim
// and never reaches for h.repo / h.auditRepo directly.
//
// All fields are positional names that map 1:1 to the
// dependencies the handler used to keep on its own struct; the
// refactor is byte-identical at the wire level.
type ServiceConfig struct {
	Repo      *Repository
	AuditRepo *AuditRepo
	// ProjectIDsForHost is the optional seam the
	// per-project access middleware uses to resolve a
	// host's project set (the host is project-scoped
	// via the host_project_links table — see
	// internal/hostproject.Service.ProjectIDsForDevice
	// for the canonical implementation). A nil
	// resolver means "per-project access is not
	// configured"; the handler's middleware treats
	// that as fail-closed (every host-scoped route
	// returns 403 for non-SuperAdmin callers) so a
	// misconfigured deploy does not accidentally
	// expose cross-tenant host data.
	ProjectIDsForHost func(hostID string) ([]string, error)
}

// Service is the framework-agnostic orchestration layer for
// the physical-host HTTP surface. It owns the "read with
// joined device name", the "look up + update in place" flow
// used by PUT, and the maintenance-history read against the
// cross-module audit table. CRUD and the maintenance-history
// query are pure pass-throughs to the repository today, but
// they belong here (not in the handler) so future logic —
// caching, event emission, observability hooks — has one
// place to land.
//
// Service does NOT call MonitorService.Check or
// MaintenanceService.Enter/Exit. Those are owned by the
// monitor loop / the maintenance service respectively; the
// handler reaches them through their own typed handles (see
// handler.go's Handler struct). Service is the
// "list-with-joins and basic CRUD" companion to those.
type Service struct {
	repo               *Repository
	auditRepo          *AuditRepo
	projectIDsForHost  func(hostID string) ([]string, error)
}

// NewService builds a Service. A nil AuditRepo is tolerated —
// the maintenance-history route returns 500 only at call time
// so a misconfigured deploy fails loudly at the route, not at
// boot. This mirrors the original handler's optional-dep
// semantics. A nil ProjectIDsForHost is also tolerated: the
// per-project access middleware in handler.Register reads it
// and refuses every host-scoped request in that mode
// (fail-closed).
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		repo:              cfg.Repo,
		auditRepo:         cfg.AuditRepo,
		projectIDsForHost: cfg.ProjectIDsForHost,
	}
}

// ProjectIDsForHost returns the set of project IDs the host
// is currently linked to. It is a thin pass-through to the
// ProjectIDsForHost resolver wired into ServiceConfig; the
// handler's per-project access middleware calls this once
// per request (the membership cache in the caller package
// keeps the actual membership check cheap).
//
// A nil resolver returns nil, []. The middleware treats
// nil/empty as "host is not linked to any project" and
// fail-closes (403).
func (s *Service) ProjectIDsForHost(ctx context.Context, hostID string) ([]string, error) {
	if s.projectIDsForHost == nil {
		return nil, nil
	}
	return s.projectIDsForHost(hostID)
}

// ListWithDevice returns a page of HostListItem (PhysicalHost
// + the joined device name) plus the unfiltered total. It
// hides the LEFT JOIN onto the devices table from the
// handler — the JOIN lives in Repository.ListWithDevice and
// is the only reason this wrapper exists.
func (s *Service) ListWithDevice(_ context.Context, f ListFilter) ([]HostListItem, int64, error) {
	return s.repo.ListWithDevice(f)
}

// Get returns the host with the given ID, or
// physicalhost.ErrNotFound if no such row exists. It is a
// pass-through to Repository.Get today; the seam is here so
// a future "preload the last state-transition row" or
// "annotate with the latest metrics snapshot" hook has one
// place to land.
func (s *Service) Get(_ context.Context, id string) (*PhysicalHost, error) {
	return s.repo.Get(id)
}

// Create inserts a new physical_hosts row. ID is filled in
// by BaseModel.BeforeCreate; the caller can read p.ID
// immediately after Create returns.
func (s *Service) Create(_ context.Context, p *PhysicalHost) error {
	return s.repo.Create(p)
}

// Update persists the entire record. The row must already
// exist; an Update on a missing ID returns ErrNotFound.
func (s *Service) Update(_ context.Context, p *PhysicalHost) error {
	return s.repo.Update(p)
}

// Delete soft-deletes the host. A missing row returns
// ErrNotFound.
func (s *Service) Delete(_ context.Context, id string) error {
	return s.repo.Delete(id)
}

// ListMaintenanceHistory returns the audit rows for a given
// physical host in DESC occurred_at order. It is the
// maintenance-history route's read path: the audit_logs table
// is owned by the audit package, so the query goes through
// audit.Repository. The Service is the boundary that keeps the
// handler free of audit-package imports for this read.
func (s *Service) ListMaintenanceHistory(_ context.Context, hostID string, limit int) ([]audit.AuditEvent, int64, error) {
	if s.auditRepo == nil {
		return nil, 0, nil
	}
	return s.auditRepo.List(AuditFilter{
		ResourceType: AuditResourcePhysicalHost,
		ResourceID:   hostID,
		Limit:        limit,
	})
}

// HasAuditRepo reports whether the service was wired with an
// audit repository. The handler uses this to render a clean
// 500 when the route is hit in a deploy that did not wire
// the audit package (mirrors the original handler's nil check).
func (s *Service) HasAuditRepo() bool { return s.auditRepo != nil }
