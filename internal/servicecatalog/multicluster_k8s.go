package servicecatalog

import (
	"context"
	"errors"
)

// MultiClusterK8sSource walks every registered K8s
// cluster and aggregates the deployments whose name
// matches the service. P1.4 wiring: production main.go
// builds a client getter from the k8s package's
// per-cluster registry; tests substitute an in-memory
// fake.
//
// Aggregation rule: a single unhealthy pod in any
// cluster tips the service to degraded. The matching
// is "deployment name == service name" — the simplest
// convention that works for 80% of the world. A
// P2 enhancement adds an explicit k8s_deployment
// column on Service for the divergent case.
type MultiClusterK8sSource struct {
	getter    K8sClientGetter
	clusterIDs []string
	// namespace is the single namespace to list from
	// per cluster. P2 will pull this from the cluster
	// row (each cluster can have a different default
	// namespace).
	namespace string
}

// NewMultiClusterK8sSource builds a K8sSource that
// walks the given cluster IDs. A cluster ID that
// returns ErrK8sNoClient is silently skipped (dev
// mode, no cluster wired). Any other error is
// returned to the rollup, which will mark the
// service as k8s_query_failed.
func NewMultiClusterK8sSource(getter K8sClientGetter, clusterIDs []string, namespace string) *MultiClusterK8sSource {
	if namespace == "" {
		namespace = "default"
	}
	return &MultiClusterK8sSource{getter: getter, clusterIDs: clusterIDs, namespace: namespace}
}

// ListDeploymentsForService satisfies K8sSource. It
// walks every configured cluster, lists deployments in
// the configured namespace, and filters to those whose
// name matches the service.
func (m *MultiClusterK8sSource) ListDeploymentsForService(ctx context.Context, serviceName string) ([]K8sDeploymentHealth, error) {
	var out []K8sDeploymentHealth
	for _, cid := range m.clusterIDs {
		client, err := m.getter.ClientFor(cid)
		if err != nil {
			if errors.Is(err, ErrK8sNoClient) {
				// Dev path: this cluster has no
				// client wired. Skip silently.
				continue
			}
			// Other errors (decrypt failure, network
			// blip) are surfaced — the rollup turns
			// them into k8s_query_failed (never
			// degraded).
			return nil, err
		}
		deps, err := client.ListDeployments(ctx, m.namespace)
		if err != nil {
			return nil, err
		}
		for _, d := range deps {
			if d.Name != serviceName {
				continue
			}
			out = append(out, K8sDeploymentHealth{
				ClusterID: d.ClusterID,
				Namespace: d.Namespace,
				Name:      d.Name,
				Ready:     renderReady(d.Available, d.Replicas),
				Replicas:  d.Replicas,
				Available: d.Available,
			})
		}
	}
	return out, nil
}

// renderReady produces the "N/M" wire format the
// frontend already renders. Kept here so the catalog
// owns its own wire shape.
func renderReady(available, replicas int32) string {
	if replicas == 0 {
		return "0/0"
	}
	return fmtInt(available) + "/" + fmtInt(replicas)
}

func fmtInt(n int32) string {
	if n == 0 {
		return "0"
	}
	// Small-int formatting without strconv import.
	// The values are bounded by Kubernetes' max
	// replicas (~5000 in practice) so a 6-digit
	// buffer is plenty.
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
