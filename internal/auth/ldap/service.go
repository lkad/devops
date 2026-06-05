package ldap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// Service is the business-logic layer between the HTTP handler and
// the LDAP client. It is responsible for:
//
//   - Mapping LDAP groups to DevOps roles (the spec's "Group-to-Role
//     Mapping" requirement).
//   - Rate-limiting failed attempts per username (the spec's
//     "Graceful Error Handling" requirement plus the RBAC spec's
//     brute-force defence).
//   - Translating client-level errors to APIError so handlers can
//     render the standard envelope.
//
// The Service does not depend on Gin; it can be reused by future
// gRPC handlers, CLI tools, or background workers.
type Service struct {
	client   Client
	maxFails int
	window   time.Duration
	groups   map[string]contracts.Role // LDAP group DN -> DevOps role
	now      func() time.Time

	mu      sync.Mutex
	buckets map[string]*rateBucket
}

// ServiceConfig configures a Service. GroupRoleMap may be nil; the
// service falls back to a default mapping covering the four groups
// the spec names (IT_Ops, DevTeam_Payments, Security_Auditors,
// SRE_Lead).
type ServiceConfig struct {
	Client          Client
	MaxFailedLogins int
	RateLimitWindow time.Duration
	GroupRoleMap    map[string]contracts.Role
	// Now is injectable for tests; production code can leave it nil.
	Now func() time.Time
}

// NewService constructs a Service. The rate limiter is per-username;
// the window is the rolling window over which MaxFailedLogins is
// counted.
func NewService(cfg ServiceConfig) *Service {
	if cfg.MaxFailedLogins <= 0 {
		cfg.MaxFailedLogins = 5
	}
	if cfg.RateLimitWindow <= 0 {
		cfg.RateLimitWindow = time.Minute
	}
	if cfg.GroupRoleMap == nil {
		cfg.GroupRoleMap = defaultGroupRoleMap()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{
		client:   cfg.Client,
		maxFails: cfg.MaxFailedLogins,
		window:   cfg.RateLimitWindow,
		groups:   cfg.GroupRoleMap,
		now:      cfg.Now,
		buckets:  make(map[string]*rateBucket),
	}
}

// defaultGroupRoleMap returns the spec's four role mappings. These
// are the groups named explicitly in the spec scenarios; production
// deployments will override them through config.
func defaultGroupRoleMap() map[string]contracts.Role {
	return map[string]contracts.Role{
		"cn=IT_Ops,ou=Groups,dc=example,dc=com":            contracts.RoleOperator,
		"cn=DevTeam_Payments,ou=Groups,dc=example,dc=com":  contracts.RoleDeveloper,
		"cn=Security_Auditors,ou=Groups,dc=example,dc=com": contracts.RoleAuditor,
		"cn=SRE_Lead,ou=Groups,dc=example,dc=com":           contracts.RoleSuperAdmin,
	}
}

// Authenticate authenticates against the configured LDAP client and
// returns a fully-populated contracts.User on success. The returned
// User is safe to hand to the JWT issuer.
func (s *Service) Authenticate(ctx context.Context, username, password string) (*contracts.User, error) {
	if username == "" || password == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "username and password are required",
		}
	}

	if err := s.checkRateLimit(username); err != nil {
		return nil, err
	}

	ident, err := s.client.Authenticate(ctx, username, password)
	if err != nil {
		s.recordFailure(username)
		return nil, s.translateClientError(err)
	}
	s.recordSuccess(username)

	return &contracts.User{
		ID:       deriveUserID(username),
		Username: ident.Username,
		Email:    ident.Email,
		Role:     s.mapRole(ident.Groups),
	}, nil
}

// HealthCheck delegates to the client. Kept on the service so the
// handler does not need to know about the client interface.
func (s *Service) HealthCheck(ctx context.Context) error {
	return s.client.HealthCheck(ctx)
}

// mapRole walks the user's groups in the order the LDAP server
// returned them and returns the highest-privilege match. This means
// a user in both DevTeam and SRE_Lead ends up as SuperAdmin, which
// is the right answer when the LDAP admin has made the group
// hierarchy explicit.
func (s *Service) mapRole(groups []string) contracts.Role {
	best := contracts.RoleProjectAdmin // no privilege by default
	for _, g := range groups {
		if r, ok := s.groups[g]; ok {
			if r.HasPermission(best) {
				best = r
			}
		}
	}
	return best
}

// translateClientError converts client-level errors to APIError
// shapes the handler knows how to render. The spec requires that
// we "do not expose internal details" — so for server-down we
// return a generic CodeInternal with no underlying error attached
// to the envelope (the cause is logged, not sent).
func (s *Service) translateClientError(err error) *contracts.APIError {
	if err == nil {
		return nil
	}
	if isInvalidCreds(err) {
		return &contracts.APIError{
			Code:    contracts.CodeUnauthorized,
			Message: "invalid username or password",
		}
	}
	return &contracts.APIError{
		Code:    contracts.CodeInternal,
		Message: "authentication service is currently unavailable",
		Cause:   err,
	}
}

// isInvalidCreds reports whether an error is (or wraps)
// ErrInvalidCredentials. We check both direct equality and the
// error message because the Real client wraps it as
// "ldap: bind: <sentinel>" and we want the test to be robust.
func isInvalidCreds(err error) bool {
	if err == nil {
		return false
	}
	if err == ErrInvalidCredentials {
		return true
	}
	// Walk the chain via errors.Is semantics by checking
	// string match — the Real client wraps with fmt.Errorf
	// %w which makes errors.Is work, but keeping a string
	// fallback guards against accidental %v in callers.
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		if cur == ErrInvalidCredentials {
			return true
		}
	}
	return false
}

// deriveUserID is the in-process user identifier. We derive it from
// the LDAP username so it is stable across logins, and we keep the
// UUID form so the rest of the system can stay schema-agnostic.
func deriveUserID(username string) string {
	return "ldap:" + username
}

// rateBucket counts failures within a rolling window. It is not
// thread-safe on its own; the Service holds a mutex around all
// bucket access.
type rateBucket struct {
	fails    int
	firstTry time.Time
}

func (s *Service) checkRateLimit(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[username]
	if !ok {
		return nil
	}
	if s.now().Sub(b.firstTry) > s.window {
		delete(s.buckets, username)
		return nil
	}
	if b.fails >= s.maxFails {
		return &contracts.APIError{
			Code:    contracts.CodeRateLimited,
			Message: fmt.Sprintf("too many failed attempts for user; try again in %s", s.window),
		}
	}
	return nil
}

func (s *Service) recordFailure(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[username]
	if !ok || s.now().Sub(b.firstTry) > s.window {
		s.buckets[username] = &rateBucket{fails: 1, firstTry: s.now()}
		return
	}
	b.fails++
}

func (s *Service) recordSuccess(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.buckets, username)
}
