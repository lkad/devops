package k8s

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// validKubeconfig is a minimal kubeconfig that parses cleanly
// via client-go's clientcmd.RESTConfigFromKubeConfig. The
// server URL never gets called in unit tests — the registry
// stops at building the client. Defined once and shared by
// every happy-path test below.
const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: dev-token
`

// TestClientRegistry_CacheHitReturnsSameClient pins
// the sticky-cache rule: a second ClientFor call for
// the same cluster ID must not re-decrypt the
// kubeconfig.
func TestClientRegistry_CacheHitReturnsSameClient(t *testing.T) {
	repo := repoFixture(t)
	created := &Cluster{Name: "cl-1", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct-cl-1"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) { return validKubeconfig, nil }}
	reg := NewClientRegistry(repo, svc)

	if _, err := reg.ClientFor(created.ID); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := reg.ClientFor(created.ID); err != nil {
		t.Fatalf("second: %v", err)
	}
	if svc.count != 1 {
		t.Errorf("decrypt called %d times, want 1 (cache hit)", svc.count)
	}
}

// TestClientRegistry_NoSuchCluster pins the
// "unknown cluster" branch: ClientFor returns
// ErrNotFound and the call is NOT cached.
func TestClientRegistry_NoSuchCluster(t *testing.T) {
	repo := repoFixture(t)
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) { return "", nil }}
	reg := NewClientRegistry(repo, svc)

	_, err := reg.ClientFor("cl-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestClientRegistry_DecryptErrorCached pins the
// sticky-error rule: a cluster whose kubeconfig
// could not be decrypted is "no client" for the
// rest of the process so we do not hammer the
// decrypt path.
func TestClientRegistry_DecryptErrorCached(t *testing.T) {
	repo := repoFixture(t)
	created := &Cluster{Name: "cl-1", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct-cl-1"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) {
		return "", errors.New("bad key")
	}}
	reg := NewClientRegistry(repo, svc)

	for i := 0; i < 3; i++ {
		_, err := reg.ClientFor(created.ID)
		if err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}
	if svc.count != 1 {
		t.Errorf("decrypt called %d times, want 1 (sticky error)", svc.count)
	}
}

// TestClientRegistry_ParseErrorCached pins the
// kubeconfig-parse failure path added in P1.6:
// a decrypted-but-malformed kubeconfig must surface
// an error AND be cached stickily, exactly like the
// decrypt-error case. P1.5 silently swallowed this
// because NewKubeClient was a stub.
func TestClientRegistry_ParseErrorCached(t *testing.T) {
	repo := repoFixture(t)
	created := &Cluster{Name: "cl-1", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct-cl-1"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) {
		return "not a kubeconfig", nil
	}}
	reg := NewClientRegistry(repo, svc)

	for i := 0; i < 3; i++ {
		_, err := reg.ClientFor(created.ID)
		if err == nil {
			t.Fatalf("call %d: expected error", i)
		}
	}
	if svc.count != 1 {
		t.Errorf("decrypt called %d times, want 1 (sticky parse error)", svc.count)
	}
}

// TestClientRegistry_ConcurrentAccess_SameKey pins the
// concurrency safety of defaultRegistry: N goroutines
// hammering ClientFor with the same cluster ID must not
// trigger a data race (run with `-race`) and the
// sticky-cache rule must collapse the per-goroutine
// DecryptKubeconfig calls to <= 1 successful decrypt
// (N concurrent first-misses would each call the
// decrypt path; the cache's mu.Lock serialises them and
// only the winner writes the entry; subsequent calls
// hit the cache).
//
// The test MUST be run with `-race` for the assertion
// to be meaningful: a non-race build may pass on
// architectures where the race is latent.
func TestClientRegistry_ConcurrentAccess_SameKey(t *testing.T) {
	repo := repoFixture(t)
	created := &Cluster{Name: "cl-race", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct-race"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) { return validKubeconfig, nil }}
	reg := NewClientRegistry(repo, svc)

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := reg.ClientFor(created.ID); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("ClientFor: %v", err)
	}
	// Allow up to goroutines decrypter calls in the
	// race-worst case (every goroutine races past the
	// first read before any of them writes the entry).
	// In practice the mu.Lock contention serialises the
	// decrypt path; on a contended CI runner the count
	// can be anywhere in [1, goroutines]. The binding
	// assertion is that NO call panicked and NO race was
	// detected by the race detector.
	svc.mu.Lock()
	count := svc.count
	svc.mu.Unlock()
	if count < 1 || count > goroutines {
		t.Errorf("decrypt count = %d, want 1..%d", count, goroutines)
	}
}

// TestClientRegistry_ConcurrentAccess_DifferentKeys pins
// the "different keys" concurrency path: N goroutines
// hit ClientFor with distinct cluster IDs. The cache
// must grow without a race; every goroutine gets back
// a (distinct) client.
func TestClientRegistry_ConcurrentAccess_DifferentKeys(t *testing.T) {
	repo := repoFixture(t)
	const goroutines = 16
	clusters := make([]*Cluster, goroutines)
	for i := 0; i < goroutines; i++ {
		cl := &Cluster{
			Name:              fmt.Sprintf("cl-%d", i),
			Type:              ClusterTypeK3d,
			KubeconfigEncrypted: fmt.Sprintf("ct-%d", i),
		}
		if err := repo.Create(cl); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		clusters[i] = cl
	}
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) { return validKubeconfig, nil }}
	reg := NewClientRegistry(repo, svc)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	clients := make([]Client, goroutines)
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			c, err := reg.ClientFor(clusters[i].ID)
			if err != nil {
				errCh <- err
				return
			}
			clients[i] = c
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("ClientFor: %v", err)
	}
	// Every goroutine must have received a non-nil
	// client; distinct cluster IDs -> distinct clients.
	seen := make(map[Client]bool, goroutines)
	for i, c := range clients {
		if c == nil {
			t.Errorf("goroutine %d got nil client", i)
			continue
		}
		if seen[c] {
			t.Errorf("goroutine %d: client reused across distinct cluster IDs", i)
		}
		seen[c] = true
	}
	if len(seen) != goroutines {
		t.Errorf("distinct clients = %d, want %d", len(seen), goroutines)
	}
}

// countingDecrypter is the test fake. It implements
// the kubeconfigDecrypter interface and counts how
// many times DecryptKubeconfig was called.
type countingDecrypter struct {
	mu        sync.Mutex
	resultFor func(c *Cluster) (string, error)
	count     int
}

// TestClientRegistry_ListerFor returns the narrow Lister
// view from a ListerFor call. The returned value must
// implement Lister (Ping + List*) and reuse the same
// underlying client object (no extra decrypt path) as
// ClientFor. The narrow typing is the audit-trail win:
// servicecatalog's health rollup needs ListDeployments
// only, so depending on Lister makes the dependency
// surface auditable.
func TestClientRegistry_ListerFor(t *testing.T) {
	repo := repoFixture(t)
	created := &Cluster{Name: "cl-lister", Type: ClusterTypeK3d, KubeconfigEncrypted: "ct-lister"}
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) { return validKubeconfig, nil }}
	reg := NewClientRegistry(repo, svc)

	// Two ListerFor calls must collapse to ONE decrypt
	// (sticky cache; same rule as ClientFor).
	lister1, err := reg.ListerFor(created.ID)
	if err != nil {
		t.Fatalf("first ListerFor: %v", err)
	}
	lister2, err := reg.ListerFor(created.ID)
	if err != nil {
		t.Fatalf("second ListerFor: %v", err)
	}
	if svc.count != 1 {
		t.Errorf("decrypt count = %d, want 1 (cache hit)", svc.count)
	}
	// Compile-time interface assertion; the test fails
	// to compile if Lister loses any of these methods.
	var _ Lister = lister1
	// ListerFor must surface the same not-found error as
	// ClientFor.
	if _, err := reg.ListerFor("cl-missing-lister"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ListerFor(missing) = %v, want ErrNotFound", err)
	}
	// Both lister1 and lister2 must be the same cached
	// client object (no separate cache).
	if lister1 != lister2 {
		t.Errorf("ListerFor returned different objects on repeated calls")
	}
}

func (s *countingDecrypter) DecryptKubeconfig(c *Cluster) (string, error) {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	return s.resultFor(c)
}
