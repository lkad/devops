// Package k8s implements the multi-cluster Kubernetes management
// subsystem. It owns the Cluster model, the GORM repository, the
// K8s API client abstraction (with a Fake for tests), and the
// layered handler/service/repository code that exposes cluster
// CRUD, connectivity probes, and per-cluster read APIs over
// /api/v1/k8s/clusters.
//
// The package follows the project-wide layering rules:
//
//	handler -> service -> repository -> model
//
// The Client interface is a seam: production code wires a real
// client backed by k8s.io/client-go, tests wire a Fake that
// returns canned responses. Kubeconfig is encrypted at rest
// using AES-GCM (see service.go).
package k8s

import (
	"time"

	"gorm.io/gorm"

	"github.com/devops-toolkit/backend/internal/database"
)

// ClusterType is the categorical kind of cluster registration.
// The string values are part of the public API contract — clients
// branch on them, so renaming a value is a breaking change.
type ClusterType string

const (
	ClusterTypeK3d      ClusterType = "k3d"
	ClusterTypeKind     ClusterType = "kind"
	ClusterTypeStandard ClusterType = "standard"
)

// Valid reports whether t is one of the recognised cluster types.
func (t ClusterType) Valid() bool {
	switch t {
	case ClusterTypeK3d, ClusterTypeKind, ClusterTypeStandard:
		return true
	}
	return false
}

// ClusterStatus is the most-recent connectivity status. The
// values mirror the spec's "active / error" wording but are
// extended to support the probe-based health check.
type ClusterStatus string

const (
	ClusterStatusUnknown      ClusterStatus = "unknown"
	ClusterStatusConnected    ClusterStatus = "connected"
	ClusterStatusDisconnected ClusterStatus = "disconnected"
)

// Valid reports whether s is one of the recognised statuses.
func (s ClusterStatus) Valid() bool {
	switch s {
	case ClusterStatusUnknown, ClusterStatusConnected, ClusterStatusDisconnected:
		return true
	}
	return false
}

// Cluster is the persisted registration of a Kubernetes
// cluster. The kubeconfig is NEVER stored in plaintext — see
// service.encryptKubeconfig for the AES-GCM wrapper.
type Cluster struct {
	database.BaseModel

	// Name is the human-readable label, unique within the
	// system. Required.
	Name string `gorm:"column:name;size:128;not null;uniqueIndex" json:"name"`

	// Type is one of ClusterTypeK3d / ClusterTypeKind /
	// ClusterTypeStandard. Required.
	Type ClusterType `gorm:"column:type;size:32;not null;index" json:"type"`

	// APIServerURL is the API server endpoint (e.g.
	// "https://k8s.example.com:6443"). Required for non
	// in-cluster registrations.
	APIServerURL string `gorm:"column:api_server_url;size:512" json:"api_server"`

	// KubeconfigEncrypted is the AES-GCM ciphertext of the
	// supplied kubeconfig. The plaintext is never persisted
	// and never returned in API responses.
	KubeconfigEncrypted string `gorm:"column:kubeconfig_encrypted;type:text" json:"-"`

	// InCluster marks a cluster that the platform is itself
	// running in (uses the in-cluster service account).
	InCluster bool `gorm:"column:in_cluster;default:false" json:"in_cluster"`

	// Status is the most-recent connectivity status. Updated
	// by Service.Probe.
	Status ClusterStatus `gorm:"column:status;size:32;not null;default:unknown;index" json:"status"`

	// LastCheckedAt is the timestamp of the most-recent
	// connectivity probe. Nil means "never probed".
	LastCheckedAt *time.Time `gorm:"column:last_checked_at;index" json:"last_checked_at,omitempty"`
}

// TableName pins the GORM-generated table name.
func (Cluster) TableName() string { return "k8s_clusters" }

// BeforeCreate wires the GORM hook. We default the Status to
// "unknown" so a service-layer caller can leave it blank, and
// explicitly call the embedded BaseModel's BeforeCreate so the
// UUID PK is assigned — GORM does not invoke promoted hooks
// from embedded structs when a top-level method of the same
// name exists.
func (c *Cluster) BeforeCreate(tx *gorm.DB) error {
	if c.Status == "" {
		c.Status = ClusterStatusUnknown
	}
	return c.BaseModel.BeforeCreate(tx)
}

// AllModels returns every GORM model this package owns.
func AllModels() []any {
	return []any{
		&Cluster{},
	}
}

// ListFilter narrows the result of a Repository.List call.
// All fields are optional; the zero value returns every
// cluster.
type ListFilter struct {
	// Type filters by ClusterType (exact match).
	Type ClusterType
	// Limit caps the page size; 0 falls back to the default
	// (20, per contracts.defaultPageSize).
	Limit int
	// Offset is the number of rows to skip; 0 means start at
	// the beginning of the result set.
	Offset int
}
