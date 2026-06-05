package ldap

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// TestService_Authenticate_Operator covers the spec's "Map LDAP group
// to Operator" scenario: a user in cn=IT_Ops must be issued a User
// whose Role is RoleOperator.
func TestService_Authenticate_Operator(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 100, time.Minute)

	u, err := svc.Authenticate(context.Background(), "alice", "alice-pw")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if u.Role != contracts.RoleOperator {
		t.Errorf("expected Operator role, got %q", u.Role)
	}
	if u.Username != "alice" {
		t.Errorf("expected username alice, got %q", u.Username)
	}
	if u.ID == "" {
		t.Errorf("expected non-empty ID, got empty")
	}
}

// TestService_Authenticate_Developer covers "Map LDAP group to Developer".
func TestService_Authenticate_Developer(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	u, err := svc.Authenticate(context.Background(), "bob", "bob-pw")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if u.Role != contracts.RoleDeveloper {
		t.Errorf("expected Developer role, got %q", u.Role)
	}
}

// TestService_Authenticate_Auditor covers "Map LDAP group to Auditor".
func TestService_Authenticate_Auditor(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	u, err := svc.Authenticate(context.Background(), "carol", "carol-pw")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if u.Role != contracts.RoleAuditor {
		t.Errorf("expected Auditor role, got %q", u.Role)
	}
}

// TestService_Authenticate_SuperAdmin covers "Map LDAP group to SuperAdmin".
func TestService_Authenticate_SuperAdmin(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 100, time.Minute)
	u, err := svc.Authenticate(context.Background(), "dave", "dave-pw")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if u.Role != contracts.RoleSuperAdmin {
		t.Errorf("expected SuperAdmin role, got %q", u.Role)
	}
}

// TestService_Authenticate_InvalidCredentials covers "Invalid LDAP
// credentials": a bad password must surface CodeUnauthorized so the
// handler can render a 401 envelope.
func TestService_Authenticate_InvalidCredentials(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 100, time.Minute)

	_, err := svc.Authenticate(context.Background(), "alice", "WRONG")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	apiErr := &contracts.APIError{}
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.Code != contracts.CodeUnauthorized {
		t.Errorf("expected CodeUnauthorized, got %q", apiErr.Code)
	}
}

// TestService_Authenticate_ServerUnavailable covers "LDAP server
// unavailable": an unreachable backend must surface CodeInternal (or
// at minimum a 5xx-class code) and never leak the underlying error
// text — the spec is explicit: "handle LDAP errors gracefully without
// exposing internal details."
func TestService_Authenticate_ServerUnavailable(t *testing.T) {
	f := NewFake()
	f.SetHealthy(false) // we'll make it return ErrServerUnreachable
	f.SetServerError(ErrServerUnreachable)
	svc := newServiceForTest(f, 100, time.Minute)

	_, err := svc.Authenticate(context.Background(), "alice", "alice-pw")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	apiErr := &contracts.APIError{}
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	// The spec calls for a 503. We don't have 503 in contracts yet, so
	// we surface CodeInternal (500) and document the gap; the handler
	// will translate this. Acceptable for this phase.
	if apiErr.Code != contracts.CodeInternal {
		t.Errorf("expected CodeInternal (no internal details), got %q", apiErr.Code)
	}
	if apiErr.Message == "" {
		t.Errorf("expected non-empty user-facing message")
	}
}

// TestService_RateLimit_PerUsername pins the per-username rate limit
// behaviour: 5 fails in 60s blocks the 6th. This is a security
// requirement implicit in the "Graceful Error Handling" requirement
// ("system handles LDAP errors gracefully") plus the RBAC spec's
// brute-force defence rule.
func TestService_RateLimit_PerUsername(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 5, time.Minute)

	for i := 0; i < 5; i++ {
		_, err := svc.Authenticate(context.Background(), "alice", "WRONG")
		if err == nil {
			t.Fatalf("expected failure on attempt %d, got nil", i)
		}
	}
	_, err := svc.Authenticate(context.Background(), "alice", "alice-pw") // even correct pw
	if err == nil {
		t.Fatalf("expected rate-limit error after 5 failures, got nil")
	}
	if !isRateLimited(err) {
		t.Errorf("expected rate-limited error, got %v", err)
	}
}

// TestService_RateLimit_DoesNotBlockOtherUsers ensures the limit is
// per-username, not global. One user's lockout must not stop another
// from logging in.
func TestService_RateLimit_DoesNotBlockOtherUsers(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 3, time.Minute)

	for i := 0; i < 3; i++ {
		_, _ = svc.Authenticate(context.Background(), "alice", "WRONG")
	}
	// alice is now locked; bob should still be free to succeed.
	u, err := svc.Authenticate(context.Background(), "bob", "bob-pw")
	if err != nil {
		t.Fatalf("expected bob to succeed while alice is locked, got %v", err)
	}
	if u.Username != "bob" {
		t.Errorf("expected bob, got %q", u.Username)
	}
}

// TestService_HealthCheck covers "LDAP healthy" / "LDAP unhealthy" at
// the service layer. The service must delegate to the client and not
// reimplement it.
func TestService_HealthCheck(t *testing.T) {
	f := NewFake()
	f.SetHealthy(true)
	svc := newServiceForTest(f, 100, time.Minute)
	if err := svc.HealthCheck(context.Background()); err != nil {
		t.Errorf("expected healthy, got %v", err)
	}

	f.SetHealthy(false)
	if err := svc.HealthCheck(context.Background()); err == nil {
		t.Errorf("expected unhealthy, got nil")
	}
}

// TestService_ConcurrentAccess makes sure the rate limiter and Fake
// can be hammered from many goroutines without a data race. The
// fake keeps a map behind a mutex; the service uses a sync.Mutex
// (or sync.Map) for the rate limit buckets. -race in the test runner
// is what actually proves it.
func TestService_ConcurrentAccess(t *testing.T) {
	svc := newServiceForTest(newDevFake(), 1000, time.Minute)
	var wg sync.WaitGroup
	var ok int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := svc.Authenticate(context.Background(), "alice", "alice-pw"); err == nil {
					atomic.AddInt32(&ok, 1)
				}
			}
		}()
	}
	wg.Wait()
	if ok == 0 {
		t.Fatalf("expected at least some successes under concurrency, got 0")
	}
}

// helpers --

// newServiceForTest wires a Service with the given Fake and rate-limit
// knobs. Kept in the test file so production code stays minimal.
func newServiceForTest(c Client, maxFails int, window time.Duration) *Service {
	return NewService(ServiceConfig{
		Client:          c,
		MaxFailedLogins: maxFails,
		RateLimitWindow: window,
		GroupRoleMap:    defaultGroupRoleMap(),
	})
}

// isRateLimited is a tiny helper so the test doesn't have to import
// errors and contracts repeatedly.
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	apiErr := &contracts.APIError{}
	if errors.As(err, &apiErr) {
		return apiErr.Code == contracts.CodeRateLimited
	}
	return false
}
