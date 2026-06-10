package servicecatalog

import (
	"errors"
	"fmt"
	"strings"

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
