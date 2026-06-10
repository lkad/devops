package k8s

import (
	"errors"
	"fmt"
	"sync"
)

// ClientRegistry is the per-cluster client resolver.
// P1.5 wiring: production main.go builds a
// ClientRegistry that walks every Cluster row,
// decrypts the kubeconfig, and builds a KubeClient
// per cluster ID. The client is cached for the
// lifetime of the registry (kubeconfig is decrypted
// at most once per cluster per process).
//
// The current Service has a single shared client
// (FakeClient in dev, one KubeClient in production).
// P1.5 introduces the per-cluster registry; the
// Service is unchanged for now and the registry
// is wired only into the servicecatalog's K8s
// health source. Future iterations can refactor
// the Service to use the same registry.
type ClientRegistry interface {
	// ClientFor returns the per-cluster Client. A
	// cache hit returns immediately; a cache miss
	// triggers a decrypt + client build. A cluster
	// that does not exist returns ErrNoSuchCluster.
	ClientFor(clusterID string) (Client, error)
}

// ErrNoSuchCluster is the typed sentinel returned by
// ClientFor when the cluster ID is not in the
// registry. Callers (the servicecatalog walker) treat
// this as a non-error: skip the cluster, no K8s data
// for it, no impact on the rollup.
var ErrNoSuchCluster = errors.New("k8s: no such cluster")

// kubeconfigDecrypter is the minimal interface the
// registry needs from the k8s.Service. Defined here
// (not exported) so the test can substitute a counter
// fake without constructing a full Service. The
// pointer receiver matches k8s.Service's real
// DecryptKubeconfig signature.
type kubeconfigDecrypter interface {
	DecryptKubeconfig(c *Cluster) (string, error)
}

// Exported alias so cmd/devops-toolkit/main.go can
// pass *k8s.Service without an import cycle. P1.5
// is the only place this is needed; once the k8s
// module exposes its own service-client seam the
// alias can be removed.
type KubeconfigDecrypter = kubeconfigDecrypter

// defaultRegistry is the production implementation:
//  1. enumerate every Cluster row in the repo
//  2. for ClientFor(clusterID), look up the cluster
//     and build a KubeClient from its decrypted
//     kubeconfig (with a per-process cache)
type defaultRegistry struct {
	repo      *Repository
	decrypter kubeconfigDecrypter

	mu    sync.Mutex
	cache map[string]cachedClient
}

type cachedClient struct {
	client Client
	err    error // sticky: a cluster whose client could not be built is "no client" forever
}

// NewClientRegistry builds a default registry backed
// by the given Repository + a DecryptKubeconfig
// implementation. The interface seam keeps the
// test fake decoupled from the full Service.
func NewClientRegistry(repo *Repository, decrypter kubeconfigDecrypter) ClientRegistry {
	return &defaultRegistry{repo: repo, decrypter: decrypter, cache: make(map[string]cachedClient)}
}

// ClientFor satisfies ClientRegistry.
func (r *defaultRegistry) ClientFor(clusterID string) (Client, error) {
	r.mu.Lock()
	if c, ok := r.cache[clusterID]; ok {
		r.mu.Unlock()
		return c.client, c.err
	}
	r.mu.Unlock()

	cluster, err := r.repo.Get(clusterID)
	if err != nil {
		r.cacheErr(clusterID, err)
		return nil, err
	}

	cfg, err := r.decrypter.DecryptKubeconfig(cluster)
	if err != nil {
		wrapped := fmt.Errorf("decrypt kubeconfig: %w", err)
		r.cacheErr(clusterID, wrapped)
		return nil, wrapped
	}

	// KubeClient is the production Client
	// implementation. Its methods are still stubs
	// (P1.5 only wires the registry; the real
	// client-go calls live in a follow-up). We
	// construct it with the on-disk path; the
	// future implementation will parse the
	// in-memory config string instead.
	client := NewKubeClient(nil, cfg)

	r.mu.Lock()
	r.cache[clusterID] = cachedClient{client: client}
	r.mu.Unlock()
	return client, nil
}

// cacheErr records a sticky error for a cluster. A
// cluster whose client could not be built is treated
// as "no client" for the rest of the process so we
// do not retry the decrypt on every health rollup.
func (r *defaultRegistry) cacheErr(clusterID string, err error) {
	r.mu.Lock()
	r.cache[clusterID] = cachedClient{err: err}
	r.mu.Unlock()
}
