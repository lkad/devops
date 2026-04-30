package k8s

import (
	"os"
)

// LogStorageBackend represents supported log storage backends
type LogStorageBackend string

const (
	BackendLoki          LogStorageBackend = "loki"
	BackendElasticsearch LogStorageBackend = "elasticsearch"
	BackendDefault       LogStorageBackend = "" // empty means use k8s native logs
)

// ProjectLogConfig holds project-specific log storage configuration
type ProjectLogConfig struct {
	Cluster string
	Backend LogStorageBackend
	LokiURL string
	ESURL   string
	Index   string
}

// GetProjectLogBackend returns the log storage backend for a given cluster
func GetProjectLogBackend(clusterName string) *ProjectLogConfig {
	backend := GetLogBackendFromEnv()
	return &ProjectLogConfig{
		Cluster: clusterName,
		Backend: backend,
		LokiURL: GetEnvOrDefault("LOKI_URL", "http://localhost:3100"),
		ESURL:   GetEnvOrDefault("ELASTICSEARCH_URL", "http://localhost:9200"),
		Index:   GetEnvOrDefault("ELASTICSEARCH_INDEX", "k8s-logs-*"),
	}
}

// GetLogBackendFromEnv returns configured log backend from environment
func GetLogBackendFromEnv() LogStorageBackend {
	backend := GetEnvOrDefault("LOG_STORAGE_BACKEND", "")
	switch backend {
	case "loki":
		return BackendLoki
	case "elasticsearch":
		return BackendElasticsearch
	default:
		return BackendDefault
	}
}

// GetEnvOrDefault is a helper to get env var or default
func GetEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}