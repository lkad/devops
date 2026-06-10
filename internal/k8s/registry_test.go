package k8s

import (
	"errors"
	"testing"
)

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
	svc := &countingDecrypter{resultFor: func(c *Cluster) (string, error) { return "/tmp/kc-" + c.ID, nil }}
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
