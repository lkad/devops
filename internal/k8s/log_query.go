package k8s

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/api/core/v1"
)

// HistoricalLogsResponse is the response for historical logs
type HistoricalLogsResponse struct {
	Logs    []string `json:"logs"`
	Backend string   `json:"backend"` // loki, elasticsearch, or k8s-native
	Count   int      `json:"count"`
}

// GetHistoricalLogsHTTP handles GET /api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs/historical
func (m *ClusterManager) GetHistoricalLogsHTTP(c *gin.Context) {
	cluster := c.Param("name")
	namespace := c.Param("ns")
	pod := c.Param("pod")

	if cluster == "" || namespace == "" || pod == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required params"})
		return
	}

	// Parse query params
	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))
	limitStr := c.DefaultQuery("limit", "100")

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logs, backend, err := m.GetHistoricalLogs(ctx, cluster, namespace, pod, start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, HistoricalLogsResponse{
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
		TailLines: int64Ptr(limit),
	})

	logsStream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream logs: %w", err)
	}
	defer logsStream.Close()

	var lines []string
	buf := make([]byte, 4096)
	for {
		n, err := logsStream.Read(buf)
		if n > 0 {
			lines = append(lines, string(buf[:n]))
		}
		if err != nil {
			break
		}
	}

	return lines, nil
}

// int64Ptr is a helper to get int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}