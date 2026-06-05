package audit

import (
	"context"
	"errors"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ServiceConfig bundles the dependencies of Service. Kept as a
// struct (not positional args) so new dependencies (clock,
// alternate read replica) can be added without changing call
// sites.
type ServiceConfig struct {
	// Repo is required. Read paths (List, Get) hit the repo
	// directly. The Emitter is used by RecordAction; the
	// repository is not consulted on the write path because
	// the production wiring is BufferedEmitter(DBEmitter)
	// which itself uses the repo.
	Repo *Repository
	// Emitter is the seam every other module's service layer
	// talks to. A nil Emitter makes RecordAction a no-op
	// (helpful in unit tests that exercise calling modules
	// without an audit row in the DB).
	Emitter Emitter
	// Now is the clock the service uses to stamp OccurredAt.
	// Defaults to time.Now so unit tests can substitute a
	// fixed clock.
	Now func() time.Time
}

// Service is the business-logic layer for the audit subsystem.
// It owns the read API (List, Get) and the RecordAction
// convenience used by every other module's service layer. The
// repository is GORM-only; the emitter is the seam for the
// production stack.
type Service struct {
	repo    *Repository
	emitter Emitter
	now     func() time.Time
}

// NewService builds a Service. A nil Emitter is tolerated
// (RecordAction becomes a no-op) so unit tests of the calling
// modules can wire a Service without an audit row in the DB.
func NewService(cfg ServiceConfig) *Service {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		repo:    cfg.Repo,
		emitter: cfg.Emitter,
		now:     now,
	}
}

// RecordActionInput is the convenience DTO for the cross-module
// RecordAction call. Every field is optional; the service fills
// in OccurredAt and a sensible default for the IP/UA when
// blank.
type RecordActionInput struct {
	Action       AuditAction
	ResourceType AuditResourceType
	ResourceID   string
	// ActorID / ActorName identify who performed the action.
	// Either may be blank (system / scheduled jobs); the
	// column is nullable in the audit log.
	ActorID   string
	ActorName string
	// Metadata is a free-form key/value bag the calling module
	// can use to record before / after snapshots. Nil is
	// stored as an empty map so the column never has to deal
	// with NULL vs empty-object ambiguity.
	Metadata JSONMap
	// IPAddress / UserAgent are read from the request scope by
	// the middleware helper (see middleware.go). Either may be
	// blank.
	IPAddress string
	UserAgent string
}

// RecordAction stamps the supplied event with OccurredAt and
// hands it to the configured emitter. It is the cross-module
// convenience: callers should NOT construct an AuditEvent by
// hand, they should fill in RecordActionInput and let the
// service shape the rest.
//
// The method is intentionally non-blocking: the production
// emitter is a BufferedEmitter, so a slow DB write never
// stalls the calling request. A nil emitter is a clean no-op.
func (s *Service) RecordAction(ctx context.Context, in RecordActionInput) {
	if s == nil || s.emitter == nil {
		return
	}
	md := in.Metadata
	if md == nil {
		md = JSONMap{}
	}
	evt := AuditEvent{
		Action:        in.Action,
		ActorID:       in.ActorID,
		ActorUsername: in.ActorName,
		ResourceType:  in.ResourceType,
		ResourceID:    in.ResourceID,
		Metadata:      md,
		IPAddress:     in.IPAddress,
		UserAgent:     in.UserAgent,
		OccurredAt:    s.now(),
	}
	s.emitter.Emit(ctx, evt)
}

// List returns a page of events plus the unfiltered total. All
// filter fields are optional; the zero value returns every
// event in DESC occurred_at order.
func (s *Service) List(f AuditFilter) ([]AuditEvent, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "audit service: missing repository",
		}
	}
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list audit events",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Get returns a single event or a 404 APIError. The handler
// renders the APIError as the standard error envelope.
func (s *Service) Get(id string) (*AuditEvent, error) {
	if s == nil || s.repo == nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "audit service: missing repository",
		}
	}
	e, err := s.repo.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: "audit event " + id + " not found",
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load audit event",
			Cause:   err,
		}
	}
	return e, nil
}
