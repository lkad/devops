package k8s

import (
	"errors"
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

// countingDecrypter is the test fake. It implements
// the kubeconfigDecrypter interface and counts how
// many times DecryptKubeconfig was called.
type countingDecrypter struct {
	resultFor func(c *Cluster) (string, error)
	count     int
}

func (s *countingDecrypter) DecryptKubeconfig(c *Cluster) (string, error) {
	s.count++
	return s.resultFor(c)
}
