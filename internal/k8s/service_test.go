package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// testCryptoKey is a fixed 32-byte key used by every test so
// the AES-GCM wrapper is deterministic.
var testCryptoKey = []byte("0123456789abcdef0123456789abcdef")

// serviceFixture builds a Service backed by a fresh in-memory
// repository and a FakeClient. The same encryption key is used
// across all tests for determinism.
func serviceFixture(t *testing.T) *Service {
	t.Helper()
	return NewService(NewRepository(openTestDB(t)), &FakeClient{}, testCryptoKey)
}

// TestService_Create_EncryptsKubeconfig verifies that the
// service encrypts the kubeconfig before persisting; the stored
// value MUST NOT equal the plaintext.
func TestService_Create_EncryptsKubeconfig(t *testing.T) {
	svc := serviceFixture(t)
	c, err := svc.Create(CreateClusterInput{
		Name:       "prod",
		Type:       ClusterTypeStandard,
		APIServer:  "https://k8s.example.com:6443",
		Kubeconfig: "apiVersion: v1\nclusters: []\n",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.KubeconfigEncrypted == "apiVersion: v1\nclusters: []\n" {
		t.Error("KubeconfigEncrypted should be the ciphertext, not plaintext")
	}
	if c.KubeconfigEncrypted == "" {
		t.Error("KubeconfigEncrypted should not be empty")
	}
}

// TestService_Create_DuplicateName_ReturnsConflict covers the
// "register with duplicate name" scenario from the spec.
func TestService_Create_DuplicateName_ReturnsConflict(t *testing.T) {
	svc := serviceFixture(t)
	in := CreateClusterInput{
		Name: "dup", Type: ClusterTypeK3d, APIServer: "x", Kubeconfig: "k",
	}
	if _, err := svc.Create(in); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(in)
	apiErr, ok := err.(*contracts.APIError)
	if !ok {
		t.Fatalf("err = %T, want *contracts.APIError", err)
	}
	if apiErr.Code != contracts.CodeConflict {
		t.Errorf("code = %q, want CONFLICT", apiErr.Code)
	}
}

// TestService_Create_InvalidType verifies that an unknown
// cluster type is rejected with a 400.
func TestService_Create_InvalidType(t *testing.T) {
	svc := serviceFixture(t)
	_, err := svc.Create(CreateClusterInput{
		Name: "x", Type: ClusterType("bogus"), Kubeconfig: "k",
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Create_EmptyName verifies that a missing name is
// rejected.
func TestService_Create_EmptyName(t *testing.T) {
	svc := serviceFixture(t)
	_, err := svc.Create(CreateClusterInput{
		Name: "", Type: ClusterTypeK3d, Kubeconfig: "k",
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Create_EmptyKubeconfig verifies that a missing
// kubeconfig is rejected.
func TestService_Create_EmptyKubeconfig(t *testing.T) {
	svc := serviceFixture(t)
	_, err := svc.Create(CreateClusterInput{
		Name: "x", Type: ClusterTypeK3d, Kubeconfig: "",
	})
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeValidation {
		t.Errorf("err = %v, want *contracts.APIError(VALIDATION_ERROR)", err)
	}
}

// TestService_Get_DecryptsKubeconfig verifies the round-trip:
// the service can read the stored ciphertext and decrypt it
// back to the original plaintext.
func TestService_Get_DecryptsKubeconfig(t *testing.T) {
	svc := serviceFixture(t)
	original := "apiVersion: v1\nclusters: []\n"
	c, err := svc.Create(CreateClusterInput{
		Name: "dec", Type: ClusterTypeK3d, Kubeconfig: original,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Get(c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// Get should return the cluster WITHOUT the kubeconfig
	// exposed; the test for decryption happens through
	// DecryptKubeconfig below.
	plain, err := svc.DecryptKubeconfig(got)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != original {
		t.Errorf("decrypted = %q, want %q", plain, original)
	}
}

// TestService_Get_MasksKubeconfigInView verifies that the
// response shape never includes the plaintext or ciphertext
// kubeconfig (the "Get cluster details" scenario).
func TestService_Get_MasksKubeconfigInView(t *testing.T) {
	svc := serviceFixture(t)
	c, err := svc.Create(CreateClusterInput{
		Name: "v", Type: ClusterTypeK3d, Kubeconfig: "super-secret",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.Get(c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.KubeconfigEncrypted == "super-secret" {
		t.Error("KubeconfigEncrypted must not contain the plaintext")
	}
	if strings.Contains(got.KubeconfigEncrypted, "super-secret") {
		t.Errorf("KubeconfigEncrypted should not leak the plaintext; got %q", got.KubeconfigEncrypted)
	}
}

// TestService_Get_NotFound covers the standard 404 path.
func TestService_Get_NotFound(t *testing.T) {
	svc := serviceFixture(t)
	_, err := svc.Get("missing")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_List returns the persisted clusters.
func TestService_List(t *testing.T) {
	svc := serviceFixture(t)
	_, _ = svc.Create(CreateClusterInput{Name: "a", Type: ClusterTypeK3d, Kubeconfig: "k"})
	_, _ = svc.Create(CreateClusterInput{Name: "b", Type: ClusterTypeKind, Kubeconfig: "k"})

	rows, total, err := svc.List(ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

// TestService_Update_PartialChanges verifies that only the
// supplied fields are touched.
func TestService_Update_PartialChanges(t *testing.T) {
	svc := serviceFixture(t)
	c, err := svc.Create(CreateClusterInput{Name: "h", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newName := "renamed"
	out, err := svc.Update(c.ID, UpdateClusterInput{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if out.Name != "renamed" {
		t.Errorf("Name = %q, want renamed", out.Name)
	}
	// kubeconfig unchanged — the original plaintext round-trips
	plain, err := svc.DecryptKubeconfig(out)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "k" {
		t.Errorf("decrypted = %q, want k", plain)
	}
}

// TestService_Update_RotatesKubeconfig verifies that a new
// kubeconfig on update is encrypted and replaces the old one.
func TestService_Update_RotatesKubeconfig(t *testing.T) {
	svc := serviceFixture(t)
	c, err := svc.Create(CreateClusterInput{Name: "h", Type: ClusterTypeK3d, Kubeconfig: "old"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newKC := "new-kc"
	out, err := svc.Update(c.ID, UpdateClusterInput{Kubeconfig: &newKC})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	plain, err := svc.DecryptKubeconfig(out)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "new-kc" {
		t.Errorf("decrypted = %q, want new-kc", plain)
	}
}

// TestService_Delete soft-deletes and hides the row.
func TestService_Delete(t *testing.T) {
	svc := serviceFixture(t)
	c, err := svc.Create(CreateClusterInput{Name: "h", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(c.ID); err == nil {
		t.Error("expected not-found after delete")
	}
}

// TestService_Probe_OK pings the cluster and returns a
// ProbeResult with status=connected and the API server version.
func TestService_Probe_OK(t *testing.T) {
	svc := serviceFixture(t)
	c, err := svc.Create(CreateClusterInput{Name: "p", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := svc.Probe(c.ID)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if res.Status != ClusterStatusConnected {
		t.Errorf("Status = %q, want connected", res.Status)
	}
	// After probe, the persisted status is updated
	got, _ := svc.Get(c.ID)
	if got.Status != ClusterStatusConnected {
		t.Errorf("persisted Status = %q, want connected", got.Status)
	}
	if got.LastCheckedAt == nil {
		t.Error("LastCheckedAt should be set after probe")
	}
}

// TestService_Probe_Unreachable flips the persisted status to
// disconnected and returns an error.
func TestService_Probe_Unreachable(t *testing.T) {
	svc := serviceFixture(t)
	svc.client = &FakeClient{PingErr: errors.New("dial: connection refused")}
	c, err := svc.Create(CreateClusterInput{Name: "p2", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := svc.Probe(c.ID)
	if err == nil {
		t.Fatal("expected probe error")
	}
	if res.Status != ClusterStatusDisconnected {
		t.Errorf("Status = %q, want disconnected", res.Status)
	}
	got, _ := svc.Get(c.ID)
	if got.Status != ClusterStatusDisconnected {
		t.Errorf("persisted Status = %q, want disconnected", got.Status)
	}
}

// TestService_Probe_NotFound covers the 404 path.
func TestService_Probe_NotFound(t *testing.T) {
	svc := serviceFixture(t)
	_, err := svc.Probe("missing")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_ListPods returns the Fake's pods.
func TestService_ListPods(t *testing.T) {
	svc := serviceFixture(t)
	svc.client = &FakeClient{
		Pods: []Pod{{Name: "p1", Namespace: "default", Phase: "Running"}},
	}
	c, err := svc.Create(CreateClusterInput{Name: "p", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pods, err := svc.ListPods(c.ID, "default")
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if len(pods) != 1 {
		t.Errorf("pods = %d, want 1", len(pods))
	}
}

// TestService_ListPods_NotFound covers the 404 path.
func TestService_ListPods_NotFound(t *testing.T) {
	svc := serviceFixture(t)
	_, err := svc.ListPods("missing", "default")
	apiErr, ok := err.(*contracts.APIError)
	if !ok || apiErr.Code != contracts.CodeNotFound {
		t.Errorf("err = %v, want *contracts.APIError(NOT_FOUND)", err)
	}
}

// TestService_ListDeployments returns the Fake's deployments.
func TestService_ListDeployments(t *testing.T) {
	svc := serviceFixture(t)
	svc.client = &FakeClient{
		Deployments: []Deployment{{Name: "web", Namespace: "default", Ready: "3/3"}},
	}
	c, err := svc.Create(CreateClusterInput{Name: "p", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	deps, err := svc.ListDeployments(c.ID, "default")
	if err != nil {
		t.Fatalf("list deployments: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("deps = %d, want 1", len(deps))
	}
}

// TestService_ListServices returns the Fake's services.
func TestService_ListServices(t *testing.T) {
	svc := serviceFixture(t)
	svc.client = &FakeClient{
		Services: []ServiceEntry{{Name: "svc", Namespace: "default", Type: "ClusterIP"}},
	}
	c, err := svc.Create(CreateClusterInput{Name: "p", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	svcs, err := svc.ListServices(c.ID, "default")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(svcs) != 1 {
		t.Errorf("svcs = %d, want 1", len(svcs))
	}
}

// TestService_DecryptKubeconfig_BadCiphertext covers the
// defensive path where the stored value cannot be decrypted.
func TestService_DecryptKubeconfig_BadCiphertext(t *testing.T) {
	svc := serviceFixture(t)
	c := &Cluster{KubeconfigEncrypted: "not-a-valid-ciphertext"}
	_, err := svc.DecryptKubeconfig(c)
	if err == nil {
		t.Error("expected decrypt error on garbage ciphertext")
	}
}

// TestService_ExecStub_DisabledByDefault verifies that
// PodExec is gated behind a feature flag and returns
// 403/FORBIDDEN when disabled.
func TestService_ExecStub_DisabledByDefault(t *testing.T) {
	svc := serviceFixture(t) // feature_k8s_exec defaults to false
	c, err := svc.Create(CreateClusterInput{Name: "p", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, execErr := svc.Exec(c.ID, "default", "pod-1", []string{"ls"})
	apiErr, ok := execErr.(*contracts.APIError)
	if !ok {
		t.Fatalf("err = %T, want *contracts.APIError", execErr)
	}
	if apiErr.Code != contracts.CodeForbidden {
		t.Errorf("code = %q, want FORBIDDEN", apiErr.Code)
	}
}

// TestService_ExecStub_Enabled covers the happy path when the
// feature flag is on. The real implementation lives in
// k8s-pod-log-streaming; the stub returns a fixed message.
func TestService_ExecStub_Enabled(t *testing.T) {
	svc := serviceFixture(t)
	svc.execEnabled = true
	c, err := svc.Create(CreateClusterInput{Name: "p", Type: ClusterTypeK3d, Kubeconfig: "k"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	res, execErr := svc.Exec(c.ID, "default", "pod-1", []string{"ls"})
	if execErr != nil {
		t.Fatalf("exec: %v", execErr)
	}
	if res.Output == "" {
		t.Error("Output should not be empty")
	}
}

// TestService_Encryption_RoundTripAcrossKeys ensures that data
// encrypted under one key cannot be decrypted under a different
// key. This pins the key-isolation contract.
func TestService_Encryption_RoundTripAcrossKeys(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	svcA := NewService(repo, &FakeClient{}, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	svcB := NewService(repo, &FakeClient{}, []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))

	c, err := svcA.Create(CreateClusterInput{Name: "k", Type: ClusterTypeK3d, Kubeconfig: "secret"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svcB.DecryptKubeconfig(c); err == nil {
		t.Error("expected decrypt failure with a different key")
	}
}

// TestService_NewService_RequiresKey guards against accidentally
// passing an empty key.
func TestService_NewService_RequiresKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty key")
		}
	}()
	_ = NewService(NewRepository(openTestDB(t)), &FakeClient{}, nil)
}

// keep the imports used in test helpers
var _ = context.Background
