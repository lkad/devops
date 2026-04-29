package k8s

import (
	"time"
)

// ClusterType constants
type ClusterType string

const (
	ClusterTypeK3d     ClusterType = "k3d"
	ClusterTypeKind     ClusterType = "kind"
	ClusterTypeStandard ClusterType = "standard"
)

// ClusterEnv constants
type ClusterEnv string

const (
	ClusterEnvDev  ClusterEnv = "dev"
	ClusterEnvTest ClusterEnv = "test"
	ClusterEnvUat  ClusterEnv = "uat"
	ClusterEnvProd ClusterEnv = "prod"
)

// ClusterStatus constants
type ClusterStatus string

const (
	ClusterStatusHealthy   ClusterStatus = "healthy"
	ClusterStatusUnhealthy ClusterStatus = "unhealthy"
	ClusterStatusUnknown   ClusterStatus = "unknown"
)

// GORMCluster is the GORM model for K8sCluster
type GORMCluster struct {
	ID         uint         `gorm:"primaryKey" json:"id"`
	Name       string       `gorm:"type:varchar(128);uniqueIndex;not null" json:"name"`
	Type       ClusterType  `gorm:"type:varchar(32);not null" json:"type"`
	Env        ClusterEnv   `gorm:"type:varchar(32);not null;default:'dev'" json:"environment"`
	Kubeconfig string       `gorm:"type:text;not null" json:"kubeconfig"`
	Status     ClusterStatus `gorm:"type:varchar(32);not null;default:'unknown'" json:"status"`
	Version    string       `gorm:"type:varchar(32)" json:"version"`
	CreatedAt  time.Time    `gorm:"type:timestamptz" json:"created_at"`
	UpdatedAt  time.Time    `gorm:"type:timestamptz" json:"updated_at"`
}

func (GORMCluster) TableName() string {
	return "k8s_clusters"
}
