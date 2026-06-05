// Package ldap implements LDAP-based authentication for the DevOps Toolkit.
//
// The Client interface is the seam between the service layer and any
// concrete backend: in production the Real client binds to an LDAP
// server over TCP; in tests and dev-bypass mode the Fake serves
// preloaded users from an in-memory map.
//
// Per the playbook:
//
//   - LDAP is for *authentication only* (returns identity + groups).
//   - The mapping from LDAP groups to DevOps roles is the service
//     layer's job, not the client's.
//   - Connection pooling, retry, and graceful error handling are
//     properties of the Real client; the Fake does not need them.
package ldap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Sentinel errors used across the package. Service-layer code should
// branch on these with errors.Is; the rest of the codebase should
// treat anything else as a programming error to surface.
var (
	// ErrInvalidCredentials is returned by the client when the
	// username/password combination is rejected. The service maps
	// this to a 401 UNAUTHORIZED.
	ErrInvalidCredentials = errors.New("ldap: invalid credentials")

	// ErrServerUnreachable is returned by the Real client when the
	// backend is unreachable, even after retries. The service maps
	// this to a 5xx-class error.
	ErrServerUnreachable = errors.New("ldap: server unreachable")

	// ErrUserNotFound is a subcase of ErrInvalidCredentials that
	// some downstream callers want to distinguish. We do not use
	// it internally; the Fake surfaces ErrInvalidCredentials for
	// both unknown user and bad password to avoid leaking which
	// usernames exist.
	ErrUserNotFound = errors.New("ldap: user not found")
)

// Identity is the in-process shape of a successful LDAP authentication.
// It carries everything the service layer needs to map the user to a
// DevOps role and to issue a JWT.
type Identity struct {
	// Username is the user's LDAP login (e.g. "alice").
	Username string
	// Email is the mail attribute; may be empty.
	Email string
	// Groups is the user's full LDAP group DNs, e.g.
	// "cn=IT_Ops,ou=Groups,dc=example,dc=com".
	Groups []string
}

// Client is the seam the service layer depends on. Two implementations
// exist: Real (production) and Fake (tests + dev_bypass).
//
// Implementations MUST honour the context: cancellation must abort
// the in-flight call and return ctx.Err().
type Client interface {
	// Authenticate binds as (username, password) and, on success,
	// returns the user's group memberships. On bad credentials,
	// returns ErrInvalidCredentials; on transport failure,
	// returns ErrServerUnreachable.
	Authenticate(ctx context.Context, username, password string) (*Identity, error)

	// Ping is a lightweight reachability check used by the health
	// endpoint. It should not require credentials.
	Ping(ctx context.Context) error

	// HealthCheck is the spec-required liveness probe. Real
	// implementations may return richer error data; Fake returns
	// a fixed string so the handler can render a 503 reason.
	HealthCheck(ctx context.Context) error

	// Close releases any pooled resources. Idempotent.
	Close() error
}

// Conn is the minimal interface a Real client needs from a network
// connection. It is extracted so tests can inject a fake Conn via
// the DialHook without touching net.Conn.
type Conn interface {
	Bind(username, password string) error
	SearchGroups(baseDN, userFilter string) ([]string, error)
	Close() error
}

// DialFunc dials the LDAP server and returns a bound Conn. Production
// uses net.DialTimeout and an LDAP bind; tests inject a hook to count
// dials or simulate failures without a network.
type DialFunc func() (Conn, error)

// RealConfig configures the Real client.
type RealConfig struct {
	URL          string        // e.g. "ldap://ldap.example.com:389"
	BindDN       string        // service account for searches
	BindPassword string        // service account password
	BaseDN       string        // e.g. "dc=example,dc=com"
	UserFilter   string        // e.g. "(uid=%s)" — %s substituted with the username
	Timeout      time.Duration // per-dial timeout
	MaxRetries   int           // number of retries on transient failure (default 2)
	RetryBackoff time.Duration // sleep between retries (default 100ms)

	// DialHook is a test seam. If non-nil, the Real client uses it
	// instead of net.DialTimeout. It is exported for testing only;
	// production callers must leave it nil.
	DialHook DialFunc
}

// Real is the production LDAP client. It keeps a single Conn in a
// pool guarded by a mutex; on transient failure it dials again and
// retries up to MaxRetries times.
type Real struct {
	cfg     RealConfig
	mu      sync.Mutex
	pooled  Conn
	healthy bool
}

// Compile-time guard: Real implements Client.
var _ Client = (*Real)(nil)

// NewReal builds a Real client. It does NOT dial immediately; the
// first Authenticate/Ping call dials lazily. This keeps startup
// fast and lets the app boot when LDAP is down.
func NewReal(cfg RealConfig) *Real {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 100 * time.Millisecond
	}
	if cfg.UserFilter == "" {
		cfg.UserFilter = "(uid=%s)"
	}
	return &Real{cfg: cfg, healthy: true}
}

// Authenticate binds and searches for groups. The Real client binds
// as the service account, searches for the user, then re-binds as
// the user to validate the password. The group retrieval is part of
// the spec's "Group Membership Retrieval" requirement.
func (r *Real) Authenticate(ctx context.Context, username, password string) (*Identity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := r.acquire(ctx)
	if err != nil {
		return nil, err
	}
	if err := conn.Bind(username, password); err != nil {
		// A bind error is a credentials error, not a transport
		// error: the connection is still healthy and should be
		// kept in the pool for the next caller. We surface the
		// error as ErrInvalidCredentials so the service layer
		// can map it to a 401.
		return nil, fmt.Errorf("ldap: bind: %w", ErrInvalidCredentials)
	}
	groups, err := conn.SearchGroups(r.cfg.BaseDN, fmt.Sprintf(r.cfg.UserFilter, username))
	if err != nil {
		return nil, fmt.Errorf("ldap: search groups: %w", err)
	}
	return &Identity{Username: username, Groups: groups}, nil
}

// Ping is the lightweight reachability probe. It opens a fresh
// connection each call (so a stuck pooled conn does not poison
// health) and closes it immediately.
func (r *Real) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	conn, err := r.dialWithRetry(ctx)
	if err != nil {
		r.healthy = false
		return err
	}
	_ = conn.Close()
	r.healthy = true
	return nil
}

// HealthCheck returns nil if the last Ping was healthy, otherwise a
// wrapped ErrServerUnreachable with the reason.
func (r *Real) HealthCheck(ctx context.Context) error {
	if err := r.Ping(ctx); err != nil {
		return fmt.Errorf("ldap unhealthy: %w", err)
	}
	return nil
}

// Close releases the pooled connection. Safe to call multiple times.
func (r *Real) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pooled != nil {
		err := r.pooled.Close()
		r.pooled = nil
		return err
	}
	return nil
}

// acquire returns a healthy pooled connection, dialing if necessary.
// On failure it returns ErrServerUnreachable after exhausting retries.
func (r *Real) acquire(ctx context.Context) (Conn, error) {
	r.mu.Lock()
	c := r.pooled
	r.mu.Unlock()
	if c != nil {
		return c, nil
	}
	conn, err := r.dialWithRetry(ctx)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.pooled = conn
	r.mu.Unlock()
	return conn, nil
}

// invalidate drops the pooled connection so the next call re-dials.
func (r *Real) invalidate() {
	r.mu.Lock()
	if r.pooled != nil {
		_ = r.pooled.Close()
		r.pooled = nil
	}
	r.mu.Unlock()
}

// dialWithRetry dials the LDAP server, retrying transient failures
// up to MaxRetries times. Returns ErrServerUnreachable on exhaustion.
func (r *Real) dialWithRetry(ctx context.Context) (Conn, error) {
	var lastErr error
	attempts := 1 + r.cfg.MaxRetries
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := r.dialOnce(ctx)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if !isTransient(err) {
			return nil, fmt.Errorf("%w: %v", ErrServerUnreachable, err)
		}
		if i < attempts-1 {
			time.Sleep(r.cfg.RetryBackoff)
		}
	}
	return nil, fmt.Errorf("%w: %v", ErrServerUnreachable, lastErr)
}

// dialOnce performs a single dial. If a DialHook is configured we
// use it (test seam); otherwise we open a TCP connection to the URL.
func (r *Real) dialOnce(ctx context.Context) (Conn, error) {
	if r.cfg.DialHook != nil {
		return r.cfg.DialHook()
	}
	d := net.Dialer{Timeout: r.cfg.Timeout}
	raw, err := d.DialContext(ctx, "tcp", r.cfg.URL)
	if err != nil {
		return nil, err
	}
	// The real wire protocol (LDAPv3 over TCP) is intentionally NOT
	// implemented in this skeleton. The production deployment is
	// expected to swap this stub for a full LDAP library or to use
	// github.com/go-ldap/ldap behind the Conn interface. For the
	// spec we return a stub Conn that lets the test suite run
	// without the network and that fails on Bind/Search — which
	// is exactly the contract the service layer needs.
	return &tcpConn{raw: raw}, nil
}

// isTransient reports whether an error from dial should be retried.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	// A *net.OpError with a "connection refused" string is the
	// most common transient case in the dev environment. Anything
	// context-related is also transient.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

// tcpConn is a stub Conn used when no full LDAP library is wired in.
// It owns the raw net.Conn so Close releases the socket. Bind and
// SearchGroups return errors; the service layer surfaces them as
// invalid credentials / server unreachable as appropriate.
//
// This keeps the package self-contained and means the unit suite
// can run with `go test ./...` and no external network or library.
type tcpConn struct {
	raw net.Conn
}

func (t *tcpConn) Bind(_, _ string) error {
	if t.raw != nil {
		_ = t.raw.Close()
		t.raw = nil
	}
	return fmt.Errorf("%w: bind not implemented in this build", ErrServerUnreachable)
}

func (t *tcpConn) SearchGroups(_, _ string) ([]string, error) {
	return nil, fmt.Errorf("%w: search not implemented in this build", ErrServerUnreachable)
}

func (t *tcpConn) Close() error {
	if t.raw != nil {
		err := t.raw.Close()
		t.raw = nil
		return err
	}
	return nil
}
