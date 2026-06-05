package ldap

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Fake is an in-memory Client used by tests and by the dev-bypass
// configuration. It is safe for concurrent use.
//
// Behaviour is exactly what a well-behaved LDAP server would do
// from the caller's perspective:
//
//   - Right password returns Identity with groups.
//   - Wrong password / unknown user returns ErrInvalidCredentials.
//   - Health/Ping reflects the SetHealthy toggle.
//   - The context is checked on entry; cancellation returns ctx.Err().
type Fake struct {
	mu       sync.RWMutex
	users    map[string]devFakeEntry
	healthy  bool
	failNext error // if non-nil, Authenticate returns this on the next call
}

// devFakeEntry is a row in the Fake. We name it explicitly to keep
// test fixtures from colliding with future internal types.
type devFakeEntry struct {
	password string
	email    string
	groups   []string
}

// Compile-time guard.
var _ Client = (*Fake)(nil)

// NewFake returns an empty Fake with healthy=true. Add users with
// AddUser before calling Authenticate.
func NewFake() *Fake {
	return &Fake{users: map[string]devFakeEntry{}, healthy: true}
}

// AddUser registers a user with the Fake. Existing entries with the
// same username are overwritten; the test suite uses this freely.
func (f *Fake) AddUser(username, password, email string, groups []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[username] = devFakeEntry{
		password: password,
		email:    email,
		groups:   append([]string(nil), groups...),
	}
}

// SetHealthy toggles the result of Ping and HealthCheck. The
// production health probe maps unhealthy to a 503.
func (f *Fake) SetHealthy(h bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthy = h
}

// SetServerError, if non-nil, makes the next Authenticate call
// return this error verbatim (used to simulate server-down without
// rebuilding the Fake in each test). Cleared after one use.
func (f *Fake) SetServerError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext = err
}

// Authenticate returns the user on a matching password.
func (f *Fake) Authenticate(ctx context.Context, username, password string) (*Identity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	if f.failNext != nil {
		err := f.failNext
		f.failNext = nil
		f.mu.Unlock()
		return nil, err
	}
	e, ok := f.users[username]
	f.mu.Unlock()
	if !ok || e.password != password {
		return nil, ErrInvalidCredentials
	}
	return &Identity{
		Username: username,
		Email:    e.email,
		Groups:   append([]string(nil), e.groups...),
	}, nil
}

// Ping returns nil when healthy, an "unreachable" error otherwise.
func (f *Fake) Ping(_ context.Context) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.healthy {
		return nil
	}
	return fmt.Errorf("ldap: server unreachable")
}

// HealthCheck mirrors Ping with a wrapper message that the handler
// surfaces to the operator.
func (f *Fake) HealthCheck(_ context.Context) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.healthy {
		return nil
	}
	return fmt.Errorf("ldap unhealthy: server unreachable")
}

// Close is a no-op for the Fake; kept for interface conformance.
func (f *Fake) Close() error { return nil }

// Importing time keeps the linter from complaining if the file
// loses a time-typed argument in a future edit. Cheaper than
// fighting the linter per change.
var _ = time.Now
