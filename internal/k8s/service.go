package k8s

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// CreateClusterInput is the request payload for Service.Create.
// The handler decodes the wire JSON into this struct; the
// service then validates, encrypts, and persists it.
type CreateClusterInput struct {
	Name       string
	Type       ClusterType
	APIServer  string
	Kubeconfig string
	InCluster  bool
}

// UpdateClusterInput is the request payload for Service.Update.
// Pointer fields are used so the service can distinguish
// "field omitted" from "field set to zero value".
type UpdateClusterInput struct {
	Name       *string
	Type       *ClusterType
	APIServer  *string
	Kubeconfig *string
	InCluster  *bool
}

// ProbeResult is the response shape for Service.Probe.
type ProbeResult struct {
	Status       ClusterStatus `json:"status"`
	APIServerURL string        `json:"api_server_url"`
	Message      string        `json:"message,omitempty"`
	CheckedAt    time.Time     `json:"checked_at"`
}

// Service is the business-logic layer for the k8s cluster
// management subsystem. It owns validation, kubeconfig
// encryption, orchestration between the repository and the
// client interface, and the per-cluster client registry
// that resolves clusterID → Client for in-cluster operations
// (pod exec, etc).
type Service struct {
	repo      *Repository
	client    Client
	cryptoKey []byte
	// registry is the per-cluster ClientResolver (P1.5).
	// Optional — nil means "no per-cluster registry wired"
	// (e.g. dev/test fixtures). Set via SetRegistry. The
	// handler reads it back via Registry() when the
	// per-cluster read path (logs, exec, fan-out) is
	// requested.
	registry ClientRegistry
}

// NewService builds a Service. The client is seam-able
// (FakeClient for tests, KubeClient in production). An empty
// cryptoKey panics — encryption must always be configured.
func NewService(repo *Repository, client Client, cryptoKey []byte) *Service {
	if len(cryptoKey) == 0 {
		panic("k8s: NewService requires a non-empty cryptoKey")
	}
	return &Service{
		repo:      repo,
		client:    client,
		cryptoKey: cryptoKey,
	}
}

// SetRegistry wires the per-cluster ClientRegistry. Optional —
// routes that need per-cluster access (logs, exec, fan-out)
// check Registry() and return a 502 if it is nil. The shared
// s.client stays as a fallback for read paths that don't
// require a cluster ID; today those are the same paths that
// pre-date P1.5.
func (s *Service) SetRegistry(reg ClientRegistry) { s.registry = reg }

// Registry returns the per-cluster ClientRegistry wired via
// SetRegistry, or nil when no registry was wired. The handler
// layer uses this to translate a cluster ID into the
// per-cluster Client that backs the apiserver calls.
func (s *Service) Registry() ClientRegistry { return s.registry }

// Create validates the input, encrypts the kubeconfig, and
// persists the cluster. The returned Cluster's
// KubeconfigEncrypted field is the ciphertext — callers
// wishing to read the plaintext must use DecryptKubeconfig.
func (s *Service) Create(in CreateClusterInput) (*Cluster, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "name is required",
		}
	}
	if !in.Type.Valid() {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: fmt.Sprintf("type %q is not a valid cluster type", in.Type),
		}
	}
	if strings.TrimSpace(in.Kubeconfig) == "" {
		return nil, &contracts.APIError{
			Code:    contracts.CodeValidation,
			Message: "kubeconfig is required",
		}
	}

	// Enforce the unique-name constraint at the service
	// layer so the APIError type matches the rest of the
	// validation surface.
	if _, err := s.repo.FindByName(strings.TrimSpace(in.Name)); err == nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeConflict,
			Message: fmt.Sprintf("cluster %q already exists", in.Name),
		}
	} else if !IsNotFound(err) {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to check cluster name",
			Cause:   err,
		}
	}

	ct, err := s.encryptKubeconfig(in.Kubeconfig)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to encrypt kubeconfig",
			Cause:   err,
		}
	}

	c := &Cluster{
		Name:                strings.TrimSpace(in.Name),
		Type:                in.Type,
		APIServerURL:        strings.TrimSpace(in.APIServer),
		KubeconfigEncrypted: ct,
		InCluster:           in.InCluster,
		Status:              ClusterStatusUnknown,
	}
	if err := s.repo.Create(c); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to create cluster",
			Cause:   err,
		}
	}
	return c, nil
}

// Get returns a single cluster or a 404 APIError. The
// KubeconfigEncrypted field is the ciphertext — use
// DecryptKubeconfig if the plaintext is required.
func (s *Service) Get(id string) (*Cluster, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("cluster %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load cluster",
			Cause:   err,
		}
	}
	return c, nil
}

// List returns a page of clusters plus the unfiltered total.
func (s *Service) List(f ListFilter) ([]Cluster, int64, error) {
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, 0, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list clusters",
			Cause:   err,
		}
	}
	return rows, total, nil
}

// Update applies a partial update. Pointer fields are
// honoured (nil = leave unchanged); Kubeconfig is re-encrypted
// when supplied.
func (s *Service) Update(id string, in UpdateClusterInput) (*Cluster, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("cluster %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load cluster",
			Cause:   err,
		}
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: "name cannot be blank",
			}
		}
		if trimmed != c.Name {
			// Enforce unique-name on rename.
			if existing, ferr := s.repo.FindByName(trimmed); ferr == nil && existing.ID != c.ID {
				return nil, &contracts.APIError{
					Code:    contracts.CodeConflict,
					Message: fmt.Sprintf("cluster %q already exists", trimmed),
				}
			} else if ferr != nil && !IsNotFound(ferr) {
				return nil, &contracts.APIError{
					Code:    contracts.CodeInternal,
					Message: "failed to check cluster name",
					Cause:   ferr,
				}
			}
		}
		c.Name = trimmed
	}
	if in.Type != nil {
		if !in.Type.Valid() {
			return nil, &contracts.APIError{
				Code:    contracts.CodeValidation,
				Message: fmt.Sprintf("type %q is not a valid cluster type", *in.Type),
			}
		}
		c.Type = *in.Type
	}
	if in.APIServer != nil {
		c.APIServerURL = strings.TrimSpace(*in.APIServer)
	}
	if in.InCluster != nil {
		c.InCluster = *in.InCluster
	}
	if in.Kubeconfig != nil {
		ct, err := s.encryptKubeconfig(*in.Kubeconfig)
		if err != nil {
			return nil, &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "failed to encrypt kubeconfig",
				Cause:   err,
			}
		}
		c.KubeconfigEncrypted = ct
	}
	if err := s.repo.Update(c); err != nil {
		if IsNotFound(err) {
			return nil, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("cluster %q not found", id),
			}
		}
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update cluster",
			Cause:   err,
		}
	}
	return c, nil
}

// Delete soft-deletes a cluster.
func (s *Service) Delete(id string) error {
	if err := s.repo.Delete(id); err != nil {
		if IsNotFound(err) {
			return &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("cluster %q not found", id),
			}
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to delete cluster",
			Cause:   err,
		}
	}
	return nil
}

// Probe pings the cluster through the Client seam. The
// persisted status is updated to connected/disconnected based
// on the result. The returned ProbeResult always has a
// non-zero CheckedAt.
func (s *Service) Probe(id string) (*ProbeResult, error) {
	c, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res := &ProbeResult{
		APIServerURL: c.APIServerURL,
		CheckedAt:    now,
	}
	if pingErr := s.client.Ping(context.Background()); pingErr != nil {
		res.Status = ClusterStatusDisconnected
		res.Message = pingErr.Error()
		_ = s.repo.UpdateStatus(c.ID, ClusterStatusDisconnected, &now)
		return res, &contracts.APIError{
			Code:    contracts.CodeInvalidState,
			Message: "cluster unreachable: " + pingErr.Error(),
			Cause:   pingErr,
		}
	}
	res.Status = ClusterStatusConnected
	if err := s.repo.UpdateStatus(c.ID, ClusterStatusConnected, &now); err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to persist probe result",
			Cause:   err,
		}
	}
	return res, nil
}

// ListPods is a thin pass-through to the Client seam. The 404
// is raised when the cluster does not exist; connectivity
// errors are returned as 502-ish (we re-use CodeInternal for
// now since the spec only requires a 4xx/5xx on failure).
func (s *Service) ListPods(id, namespace string) ([]Pod, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	pods, err := s.client.ListPods(context.Background(), namespace)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list pods",
			Cause:   err,
		}
	}
	return pods, nil
}

// ListDeployments is a thin pass-through to the Client seam.
func (s *Service) ListDeployments(id, namespace string) ([]Deployment, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	deps, err := s.client.ListDeployments(context.Background(), namespace)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list deployments",
			Cause:   err,
		}
	}
	return deps, nil
}

// ListServices is a thin pass-through to the Client seam.
func (s *Service) ListServices(id, namespace string) ([]ServiceEntry, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	svcs, err := s.client.ListServices(context.Background(), namespace)
	if err != nil {
		return nil, &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to list services",
			Cause:   err,
		}
	}
	return svcs, nil
}

// Exec is a thin pass-through that resolves the per-cluster
// Client via the registry and calls its ExecInPod method.
// The handler layer is the right place for HTTP-level concerns
// (wire-shape validation, error code mapping), so this
// function is intentionally small: it exists so the
// handler can use the Service as its single seam into the
// subsystem (matching the rest of the k8s handler code).
//
// Errors returned by the underlying Client are wrapped in
// *contracts.APIError so the handler can render the spec's
// error envelope without re-mapping. An invalid client
// (empty command/container/namespace) is surfaced as
// INVALID_EXEC_REQUEST; a network / RBAC failure becomes
// APISERVER_UNREACHABLE; a context-deadline becomes TIMEOUT;
// a non-zero exit is NOT an error (the result carries the
// exit code). The "pod not found" / "container not found"
// distinction is left to the apiserver's error message — the
// client does not surface it as a typed sentinel today.
func (s *Service) Exec(ctx context.Context, id, namespace, pod, container string, command []string, timeout time.Duration) (PodExecResult, error) {
	if len(command) == 0 {
		return PodExecResult{}, &contracts.APIError{
			Code:    contracts.CodeInvalidExecRequest,
			Message: "command is required and must be a non-empty array",
		}
	}
	if container == "" {
		return PodExecResult{}, &contracts.APIError{
			Code:    contracts.CodeInvalidExecRequest,
			Message: "container is required",
		}
	}
	reg := s.Registry()
	if reg == nil {
		return PodExecResult{}, &contracts.APIError{
			Code:    contracts.CodeAPIServerUnreachable,
			Message: "no per-cluster client registry wired",
		}
	}
	client, err := reg.ClientFor(id)
	if err != nil {
		// Map "cluster not found" to the 404 envelope;
		// anything else (decrypt, parse, build) is a
		// 502 — the cluster is registered but its client
		// is unreachable.
		if IsNotFound(err) {
			return PodExecResult{}, &contracts.APIError{
				Code:    contracts.CodeNotFound,
				Message: fmt.Sprintf("cluster %q not found", id),
			}
		}
		return PodExecResult{}, &contracts.APIError{
			Code:    contracts.CodeAPIServerUnreachable,
			Message: fmt.Sprintf("cluster %q has no usable client: %v", id, err),
			Cause:   err,
		}
	}
	result, execErr := client.ExecInPod(ctx, namespace, pod, container, command, timeout)
	if execErr != nil {
		// Distinguish a context-deadline (TIMEOUT, 504)
		// from a generic apiserver / RBAC failure
		// (APISERVER_UNREACHABLE, 502). The fake client
		// already wraps ErrInvalidExecRequest; we re-
		// surface that as INVALID_EXEC_REQUEST to match
		// the spec.
		if errors.Is(execErr, ErrInvalidExecRequest) {
			return PodExecResult{}, &contracts.APIError{
				Code:    contracts.CodeInvalidExecRequest,
				Message: execErr.Error(),
				Cause:   execErr,
			}
		}
		if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
			return PodExecResult{}, &contracts.APIError{
				Code:    contracts.CodeTimeout,
				Message: "exec timed out",
				Cause:   execErr,
			}
		}
		return PodExecResult{}, &contracts.APIError{
			Code:    contracts.CodeAPIServerUnreachable,
			Message: fmt.Sprintf("exec failed: %v", execErr),
			Cause:   execErr,
		}
	}
	return result, nil
}

// DecryptKubeconfig returns the plaintext kubeconfig for a
// cluster. The caller is responsible for not logging or
// persisting the result.
func (s *Service) DecryptKubeconfig(c *Cluster) (string, error) {
	return s.decryptKubeconfig(c.KubeconfigEncrypted)
}

// encryptKubeconfig wraps the plaintext in AES-GCM and
// returns a base64(nonce || ciphertext) string. The nonce is
// 12 bytes (the AES-GCM standard).
func (s *Service) encryptKubeconfig(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.cryptoKey)
	if err != nil {
		return "", fmt.Errorf("k8s: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("k8s: cipher.NewGCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("k8s: read nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// decryptKubeconfig reverses encryptKubeconfig. Returns an
// error when the input is not a valid base64 blob of the
// right size, or when the AES-GCM tag does not verify.
func (s *Service) decryptKubeconfig(b64ct string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64ct)
	if err != nil {
		return "", fmt.Errorf("k8s: base64 decode: %w", err)
	}
	block, err := aes.NewCipher(s.cryptoKey)
	if err != nil {
		return "", fmt.Errorf("k8s: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("k8s: cipher.NewGCM: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("k8s: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("k8s: gcm.Open: %w", err)
	}
	return string(pt), nil
}
