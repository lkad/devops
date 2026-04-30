package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/api/core/v1"
)

// HistoricalLogsResponse is the response for historical logs
type HistoricalLogsResponse struct {
	Logs    []string `json:"logs"`
	Backend string   `json:"backend"` // loki, elasticsearch, or k8s-native
	Count   int      `json:"count"`
}

// GetHistoricalLogsHTTP handles GET /api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs/historical
func (m *ClusterManager) GetHistoricalLogsHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cluster := vars["name"]
	namespace := vars["ns"]
	pod := vars["pod"]

	if cluster == "" || namespace == "" || pod == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing required params"})
		return
	}

	// Parse query params
	startStr := r.URL.Query().Get("start")
	if startStr == "" {
		startStr = time.Now().Add(-1*time.Hour).Format(time.RFC3339)
	}
	endStr := r.URL.Query().Get("end")
	if endStr == "" {
		endStr = time.Now().Format(time.RFC3339)
	}
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "100"
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		start = time.Now().Add(-1 * time.Hour)
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		end = time.Now()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	logs, backend, err := m.GetHistoricalLogs(ctx, cluster, namespace, pod, start, end, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HistoricalLogsResponse{
		Logs:    logs,
		Backend: backend,
		Count:   len(logs),
	})
}

// GetHistoricalLogs retrieves historical logs based on project backend config
func (m *ClusterManager) GetHistoricalLogs(ctx context.Context, cluster, namespace, pod string, start, end time.Time, limit int) ([]string, string, error) {
	// Get project log backend config
	config := GetProjectLogBackend(cluster)

	switch config.Backend {
	case BackendLoki:
		client := NewLokiClient(config.LokiURL)
		logs, err := client.QueryPodLogs(ctx, pod, namespace, cluster, start, end, limit)
		if err != nil {
			return nil, "loki", fmt.Errorf("query loki: %w", err)
		}
		return logs, "loki", nil

	case BackendElasticsearch:
		client := NewESClient(config.ESURL, config.Index)
		logs, err := client.QueryPodLogs(ctx, pod, namespace, cluster, start, end, limit)
		if err != nil {
			return nil, "elasticsearch", fmt.Errorf("query elasticsearch: %w", err)
		}
		return logs, "elasticsearch", nil

	default:
		// Fall back to k8s native logs (current behavior)
		logs, err := m.getPodLogsFromK8s(ctx, cluster, namespace, pod, limit)
		if err != nil {
			return nil, "k8s-native", fmt.Errorf("query k8s: %w", err)
		}
		return logs, "k8s-native", nil
	}
}

// getPodLogsFromK8s retrieves logs directly from K8s API
func (m *ClusterManager) getPodLogsFromK8s(ctx context.Context, cluster, namespace, pod string, limit int) ([]string, error) {
	clientset, err := m.buildClient(cluster)
	if err != nil {
		return nil, err
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(pod, &v1.PodLogOptions{
		TailLines: int64Ptr(int64(limit)),
	})

	logsStream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream logs: %w", err)
	}
	defer logsStream.Close()

	data, err := io.ReadAll(logsStream)
	if err != nil {
		return nil, fmt.Errorf("read log stream: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	// Filter empty lines
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

// int64Ptr is a helper to get int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}