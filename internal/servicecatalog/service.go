package servicecatalog

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// ErrConflict is returned when a Create would violate a
// uniqueness constraint. The service layer wraps it in
// a 409 CONFLICT APIError.
var ErrConflict = errors.New("servicecatalog: conflict")

// IsConflict reports whether err is (or wraps) ErrConflict.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// ErrValidation is the typed sentinel for any input
// rejected by validateCreate / validateUpdate. The service
// wraps it in a 400 APIError with the underlying message.
var ErrValidation = errors.New("servicecatalog: validation")

// IsValidation reports whether err is (or wraps)
// ErrValidation.
func IsValidation(err error) bool { return errors.Is(err, ErrValidation) }

// CreateInput is the DTO for Service creation. The handler
// decodes the wire JSON into this struct; the service then
// validates and maps it onto a Service.
type CreateInput struct {
	Name          string
	Description   string
	Owner         string
	RepositoryURL string
	Tier          Tier
}

// UpdateInput is the partial-update DTO. A zero-value Tier
// means "do not change"; an empty string is rejected
// (callers must either leave the field at the zero value
// or supply a valid tier).
type UpdateInput struct {
	Name          *string
	Description   *string
	Owner         *string
	RepositoryURL *string
	Tier          *Tier
}

// Catalog is the business-logic layer for the service
// catalog. It owns validation, the rules around
// uniqueness + tier, and the translation from a Create
// error into a typed ErrConflict. It is framework-agnostic
// (no Gin) so it can be reused by gRPC handlers, CLI
// tools, or background workers in the future.
//
// Named Catalog (not Service) to avoid colliding with the
// data-model type also called Service in this package.
type Catalog struct {
	repo *Repository
}

// NewCatalog builds a Catalog. The repository is the only
// dependency.
func NewCatalog(repo *Repository) *Catalog {
	return &Catalog{repo: repo}
}

// Create validates in, then persists it. Returns the
// freshly-stored row (with ID, CreatedAt, UpdatedAt).
func (s *Catalog) Create(in CreateInput) (*Service, error) {
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	row := &Service{
		Name:          strings.TrimSpace(in.Name),
		Description:   strings.TrimSpace(in.Description),
		Owner:         strings.TrimSpace(in.Owner),
		RepositoryURL: strings.TrimSpace(in.RepositoryURL),
		Tier:          in.Tier,
	}
	if err := s.repo.Create(row); err != nil {
		// Surface unique-constraint failures as ErrConflict
		// so the handler can map them to 409. We do a string
		// match because GORM does not expose a typed
		// "unique violation" error code.
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return nil, err
	}
	return row, nil
}

// Get returns a single service by ID. 404 NOT_FOUND on
// missing.
func (s *Catalog) Get(id string) (*Service, error) {
	return s.repo.Get(id)
}

// List returns a page of services matching the filter.
func (s *Catalog) List(f ServiceFilter) ([]Service, error) {
	return s.repo.List(f)
}

// Update applies a partial update.
func (s *Catalog) Update(id string, in UpdateInput) (*Service, error) {
	if err := validateUpdate(in); err != nil {
		return nil, err
	}
	row, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: name cannot be blank", ErrValidation)
		}
		if len(trimmed) > 64 {
			return nil, fmt.Errorf("%w: name exceeds 64 chars", ErrValidation)
		}
		row.Name = trimmed
	}
	if in.Description != nil {
		row.Description = strings.TrimSpace(*in.Description)
	}
	if in.Owner != nil {
		row.Owner = strings.TrimSpace(*in.Owner)
	}
	if in.RepositoryURL != nil {
		row.RepositoryURL = strings.TrimSpace(*in.RepositoryURL)
	}
	if in.Tier != nil {
		row.Tier = *in.Tier
	}
	if err := s.repo.Update(row); err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return nil, err
	}
	return row, nil
}

// SoftDelete marks the row deleted. 404 on missing.
func (s *Catalog) SoftDelete(id string) error {
	return s.repo.SoftDelete(id)
}

// validateCreate is the shared field-level rules.
func validateCreate(in CreateInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if len(in.Name) > 64 {
		return fmt.Errorf("%w: name exceeds 64 chars", ErrValidation)
	}
	switch in.Tier {
	case TierCritical, TierImportant, TierStandard:
		// ok
	case "":
		// default to standard
	default:
		return fmt.Errorf("%w: tier %q is not valid", ErrValidation, in.Tier)
	}
	return nil
}

// validateUpdate enforces the same rules on the
// partial-update DTO. nil fields are skipped; non-nil
// fields are validated.
func validateUpdate(in UpdateInput) error {
	if in.Name != nil {
		if len(*in.Name) > 64 {
			return fmt.Errorf("%w: name exceeds 64 chars", ErrValidation)
		}
	}
	if in.Tier != nil {
		switch *in.Tier {
		case TierCritical, TierImportant, TierStandard:
			// ok
		default:
			return fmt.Errorf("%w: tier %q is not valid", ErrValidation, *in.Tier)
		}
	}
	return nil
}

// isUniqueViolation sniffs the GORM error string for a
// unique-constraint failure. SQLite and Postgres both
// produce messages containing "UNIQUE constraint failed"
// (SQLite) or "duplicate key" (Postgres); we check the
// common substring.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") ||
		strings.Contains(s, "duplicate key")
}

// apiErrFrom returns the right *contracts.APIError for a
// known sentinel, or a generic 500 for unknown errors.
// Kept here so the handler stays thin.
func apiErrFrom(err error) *contracts.APIError {
	switch {
	case IsNotFound(err):
		return &contracts.APIError{
			Code:    contracts.CodeNotFound,
			Message: "service not found",
			Cause:   err,
		}
	case IsConflict(err):
		return &contracts.APIError{
			Code:    contracts.CodeConflict,
			Message: "service name already in use",
			Cause:   err,
		}
	case IsValidation(err):
		return &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: err.Error(),
			Cause:   err,
		}
	default:
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "internal error",
			Cause:   err,
		}
	}
}

// OnCallScope selects whether an on-call rotation entry
// is per-service or a global fallback. Empty scope
// defaults to "service" so the common case is a single
// keystroke less.
type OnCallScope string

const (
	OnCallScopeService OnCallScope = "service"
	OnCallScopeGlobal  OnCallScope = "global"
)

// CreateOnCallInput is the DTO for new on-call rotation
// rows. User is the rotation participant (the LDAP
// account the on-call is paged to); it is the on-call's
// identity, not the actor creating the row — the actor
// is captured separately from the JWT subject so the
// audit trail is forgery-proof (P0 #3).
type CreateOnCallInput struct {
	User       string
	ShiftStart time.Time
	ShiftEnd   time.Time
	Scope      OnCallScope
	// Actor is the authenticated user creating the row.
	// Held in the input so the service layer can pass it
	// to the eventual audit emit (P0 #3); the on-call row
	// itself has no created_by column today, so Actor is
	// not persisted on the entity.
	Actor string
}

// CreateRunbookInput is the DTO for new runbook entries.
// The body is plain text; the frontend renders newlines
// verbatim (see OnCallBlock / RunbookBlock in
// Services.tsx).
type CreateRunbookInput struct {
	Title string
	Body  string
	// Actor follows the same JWT-subject rule as
	// CreateOnCallInput.Actor.
	Actor string
}

// CreateOnCall validates in, resolves the scope (global
// rotations leave ServiceID empty so CurrentOnCall's
// fallback branch picks them up), and persists. Returns
// 404 when the service is unknown, 400 on bad shape, 409
// on an overlapping shift for the same service.
//
// TODO(audit): emit audit.Service.RecordAction here once
// P0 #3's audit wiring lands. The Actor field is the
// authenticated subject (never the request body) so the
// emit will be forgery-proof.
func (s *Catalog) CreateOnCall(serviceID string, in CreateOnCallInput) (*OnCall, error) {
	if err := validateOnCall(in); err != nil {
		return nil, err
	}
	if _, err := s.repo.Get(serviceID); err != nil {
		return nil, err
	}
	overlap, err := s.repo.HasOverlappingOnCall(effectiveServiceID(in.Scope, serviceID), in.ShiftStart, in.ShiftEnd)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("%w: shift overlaps an existing on-call rotation", ErrConflict)
	}
	row := &OnCall{
		ServiceID:  effectiveServiceID(in.Scope, serviceID),
		User:       strings.TrimSpace(in.User),
		ShiftStart: in.ShiftStart,
		ShiftEnd:   in.ShiftEnd,
	}
	if err := s.repo.CreateOnCall(row); err != nil {
		return nil, err
	}
	return row, nil
}

// DeleteOnCall removes a rotation row. Returns 404 if
// either the service is missing or the row id is unknown;
// the handler does not distinguish the two.
func (s *Catalog) DeleteOnCall(serviceID, shiftID string) error {
	if _, err := s.repo.Get(serviceID); err != nil {
		return err
	}
	return s.repo.DeleteOnCall(shiftID)
}

// CreateRunbook validates in, then persists. Returns
// 404 on unknown service, 400 on bad shape.
func (s *Catalog) CreateRunbook(serviceID string, in CreateRunbookInput) (*RunbookEntry, error) {
	if err := validateRunbook(in); err != nil {
		return nil, err
	}
	if _, err := s.repo.Get(serviceID); err != nil {
		return nil, err
	}
	row := &RunbookEntry{
		ServiceID: serviceID,
		Title:     strings.TrimSpace(in.Title),
		Body:      in.Body,
	}
	if err := s.repo.CreateRunbook(row); err != nil {
		return nil, err
	}
	return row, nil
}

// DeleteRunbook removes a runbook entry. Returns 404 on
// unknown service or unknown entry id.
func (s *Catalog) DeleteRunbook(serviceID, entryID string) error {
	if _, err := s.repo.Get(serviceID); err != nil {
		return err
	}
	return s.repo.DeleteRunbook(entryID)
}

// validateOnCall enforces the request shape:
//   - User must be non-empty after trim
//   - ShiftStart and ShiftEnd must be valid timestamps
//   - ShiftStart < ShiftEnd
//   - Scope, if non-empty, must be "service" or "global"
func validateOnCall(in CreateOnCallInput) error {
	if strings.TrimSpace(in.User) == "" {
		return fmt.Errorf("%w: user_email is required", ErrValidation)
	}
	if in.ShiftStart.IsZero() {
		return fmt.Errorf("%w: shift_start is required", ErrValidation)
	}
	if in.ShiftEnd.IsZero() {
		return fmt.Errorf("%w: shift_end is required", ErrValidation)
	}
	if !in.ShiftStart.Before(in.ShiftEnd) {
		return fmt.Errorf("%w: shift_start must be before shift_end", ErrValidation)
	}
	if in.Scope != "" && in.Scope != OnCallScopeService && in.Scope != OnCallScopeGlobal {
		return fmt.Errorf("%w: scope %q is not valid", ErrValidation, in.Scope)
	}
	return nil
}

// validateRunbook enforces the request shape:
//   - Title is non-empty after trim and <= 256 chars
//   - Body is optional (empty allowed) but capped at
//     64 KiB to keep the page responsive
func validateRunbook(in CreateRunbookInput) error {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if len(title) > 256 {
		return fmt.Errorf("%w: title exceeds 256 chars", ErrValidation)
	}
	if len(in.Body) > 64*1024 {
		return fmt.Errorf("%w: body exceeds 64 KiB", ErrValidation)
	}
	return nil
}

// effectiveServiceID returns the value that ends up in
// the row's service_id column: the URL param for a
// per-service shift, the empty string for a global
// rotation. The overlap check and the eventual INSERT
// both feed through this helper so the two stay in
// sync.
func effectiveServiceID(scope OnCallScope, urlParam string) string {
	if scope == OnCallScopeGlobal {
		return ""
	}
	return urlParam
}
