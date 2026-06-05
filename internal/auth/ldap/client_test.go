// Package ldap implements LDAP-based authentication for the devops toolkit.
// The tests in this file drive the design (TDD): the Fake is what production
// callers actually depend on for behavior; the Real is a thin TCP-bind
// wrapper that is exercised only by manual integration runs, never by
// the unit suite, per the playbook's "no real network in tests" rule.
package ldap

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Compile-time check that Fake satisfies the Client interface.
var _ Client = (*Fake)(nil)

// newDevFake builds a Fake preloaded with a handful of users covering
// the role-mapping scenarios from the spec.
func newDevFake() *Fake {
	f := NewFake()
	// Each entry uses the dev-mode short password convention so the
	// test bodies stay readable; passwords themselves are not the SUT.
	f.AddUser("alice", "alice-pw", "alice@example.com", []string{"cn=IT_Ops,ou=Groups,dc=example,dc=com"})
	f.AddUser("bob", "bob-pw", "bob@example.com", []string{"cn=DevTeam_Payments,ou=Groups,dc=example,dc=com"})
	f.AddUser("carol", "carol-pw", "carol@example.com", []string{"cn=Security_Auditors,ou=Groups,dc=example,dc=com"})
	f.AddUser("dave", "dave-pw", "dave@example.com", []string{"cn=SRE_Lead,ou=Groups,dc=example,dc=com"})
	f.AddUser("eve", "eve-pw", "eve@example.com", []string{"cn=NoMatch,ou=Groups,dc=example,dc=com"})
	return f
}

// TestFake_Authenticate_ValidCredentials covers the spec's
// "Valid LDAP credentials" scenario at the client layer. The handler
// tests verify the JWT issuance on top of this.
func TestFake_Authenticate_ValidCredentials(t *testing.T) {
	f := newDevFake()

	got, err := f.Authenticate(context.Background(), "alice", "alice-pw")
	if err != nil {
		t.Fatalf("expected success for alice, got error: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("expected username alice, got %q", got.Username)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %q", got.Email)
	}
	if len(got.Groups) != 1 || got.Groups[0] != "cn=IT_Ops,ou=Groups,dc=example,dc=com" {
		t.Errorf("expected IT_Ops group, got %v", got.Groups)
	}
}

// TestFake_Authenticate_WrongPassword covers the spec's
// "Invalid LDAP credentials" scenario. The Fake must surface an
// ErrInvalidCredentials so the service layer can map it to a 401.
func TestFake_Authenticate_WrongPassword(t *testing.T) {
	f := newDevFake()

	_, err := f.Authenticate(context.Background(), "alice", "WRONG")
	if err == nil {
		t.Fatalf("expected error for wrong password, got nil")
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

// TestFake_Authenticate_UnknownUser ensures the Fake rejects users
// it has no record of. The exact error class is less important than
// that it is *not* ErrInvalidCredentials? actually it IS that — both
// unknown user and wrong password are invalid credentials from the
// caller's perspective. We assert that here to document the contract.
func TestFake_Authenticate_UnknownUser(t *testing.T) {
	f := newDevFake()

	_, err := f.Authenticate(context.Background(), "ghost", "anything")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
}

// TestFake_Ping_Success and TestFake_Ping_Failure cover the spec's
// "LDAP healthy" and "LDAP unhealthy" health-check scenarios. The
// Fake lets the test toggle the result deterministically.
func TestFake_Ping_Success(t *testing.T) {
	f := NewFake()
	f.SetHealthy(true)
	if err := f.Ping(context.Background()); err != nil {
		t.Errorf("expected healthy ping, got %v", err)
	}
}

func TestFake_Ping_Failure(t *testing.T) {
	f := NewFake()
	f.SetHealthy(false)
	err := f.Ping(context.Background())
	if err == nil {
		t.Fatalf("expected unhealthy ping, got nil")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected error mentioning unreachable, got %q", err.Error())
	}
}

// TestFake_HealthCheckEndpointReady asserts the Fake implements the
// extra method the service layer relies on for the health endpoint.
// (Spec: "Health Check".)
func TestFake_HealthCheckEndpointReady(t *testing.T) {
	f := NewFake()
	f.SetHealthy(true)
	if err := f.HealthCheck(context.Background()); err != nil {
		t.Errorf("expected healthy healthcheck, got %v", err)
	}
	f.SetHealthy(false)
	if err := f.HealthCheck(context.Background()); err == nil {
		t.Errorf("expected unhealthy healthcheck, got nil")
	}
}

// TestFake_Authenticate_RespectsContext ensures cancellation propagates
// so callers can drop in-flight auth requests. The Fake checks the
// context on entry.
func TestFake_Authenticate_RespectsContext(t *testing.T) {
	f := newDevFake()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Authenticate(ctx, "alice", "alice-pw")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestReal_RejectsConstructionWithoutURL pins the Real client behavior:
// construction itself must not fail (we defer dialing), and dialing
// without a real server surfaces ErrServerUnreachable. This is the
// minimal contract the service layer needs from Real.
func TestReal_RejectsConstructionWithoutURL(t *testing.T) {
	c := NewReal(RealConfig{URL: "ldap://127.0.0.1:1", Timeout: 100 * time.Millisecond})
	if c == nil {
		t.Fatalf("NewReal returned nil")
	}
	_, err := c.Authenticate(context.Background(), "alice", "alice-pw")
	if !errors.Is(err, ErrServerUnreachable) {
		t.Errorf("expected ErrServerUnreachable, got %v", err)
	}
	if err := c.Ping(context.Background()); !errors.Is(err, ErrServerUnreachable) {
		t.Errorf("expected ErrServerUnreachable on ping, got %v", err)
	}
}

// TestReal_ReusesConnections is the spec's "Connection reuse" scenario.
// The Real client exposes a connection counter we can read in tests
// without touching the network. We set up a counter that is invoked
// on each dial; the second auth call must not increment it because
// the first connection should be reused.
func TestReal_ReusesConnections(t *testing.T) {
	// Use a hook to count "dial" invocations rather than letting the
	// real client reach the network. The hook is invoked by Real
	// instead of net.Dial when non-nil; tests inject it to observe
	// pool behavior. We never call the real network in unit tests.
	var dials int32
	hook := func() (Conn, error) {
		atomic.AddInt32(&dials, 1)
		return &nopConn{}, nil
	}
	c := NewReal(RealConfig{URL: "ldap://test/", Timeout: time.Second, DialHook: hook})

	// Do two authentications; the second must reuse the first
	// connection (so the dial hook fires only once).
	if _, err := c.Authenticate(context.Background(), "u", "p"); err != nil && !errors.Is(err, ErrServerUnreachable) {
		// We accept ErrServerUnreachable too because the fake conn
		// returns no real data; the point is the dial count.
		t.Logf("first auth returned %v (expected for fake conn)", err)
	}
	if _, err := c.Authenticate(context.Background(), "u", "p"); err != nil && !errors.Is(err, ErrServerUnreachable) {
		t.Logf("second auth returned %v (expected for fake conn)", err)
	}
	if got := atomic.LoadInt32(&dials); got != 1 {
		t.Errorf("expected exactly 1 dial across 2 auths (connection reuse), got %d", got)
	}
}

// TestReal_RetriesOnTransientFailure is the spec's "Retry on connection
// failure" scenario. The Real client retries on transient errors up to
// MaxRetries times; we verify that by injecting a hook that fails the
// first two dials and succeeds the third.
func TestReal_RetriesOnTransientFailure(t *testing.T) {
	var calls int32
	hook := func() (Conn, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			// Use a *net.OpError so isTransient() recognises it.
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
		}
		return &nopConn{}, nil
	}
	c := NewReal(RealConfig{URL: "ldap://test/", Timeout: time.Second, DialHook: hook, MaxRetries: 3, RetryBackoff: time.Microsecond})

	_, _ = c.Authenticate(context.Background(), "u", "p") // conn is nop, so bind will fail — that's fine
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 dial attempts (2 fail + 1 success), got %d", got)
	}
}

// TestReal_StopsAfterMaxRetries pins the upper bound on retries so a
// hung server cannot turn login into a slow hang. The Real client must
// surface ErrServerUnreachable after exhausting attempts.
func TestReal_StopsAfterMaxRetries(t *testing.T) {
	var calls int32
	hook := func() (Conn, error) {
		atomic.AddInt32(&calls, 1)
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("nope")}
	}
	c := NewReal(RealConfig{URL: "ldap://test/", Timeout: time.Second, DialHook: hook, MaxRetries: 2, RetryBackoff: time.Microsecond})

	_, err := c.Authenticate(context.Background(), "u", "p")
	if !errors.Is(err, ErrServerUnreachable) {
		t.Errorf("expected ErrServerUnreachable after exhausting retries, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 dial attempts (1 + 2 retries), got %d", got)
	}
}

// nopConn is a stand-in for a real net.Conn when the test injects a
// DialHook. It implements Conn well enough to be returned by the hook;
// bind/search calls on it will fail, which is exactly what tests that
// only care about dial counting rely on.
type nopConn struct{}

func (n *nopConn) Close() error { return nil }
func (n *nopConn) Bind(_, _ string) error {
	return errors.New("nopConn: bind not implemented")
}
func (n *nopConn) SearchGroups(_, _ string) ([]string, error) {
	return nil, errors.New("nopConn: search not implemented")
}
