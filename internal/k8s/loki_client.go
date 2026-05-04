package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// LokiClient queries logs from Loki
type LokiClient struct {
	BaseURL string
	Client  *http.Client
}

// LokiLogResponse is the response from Loki log endpoint
type LokiLogResponse struct {
	Status     string `json:"status"`
	Version    string `json:"version,omitempty"`
	Data       struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [[timestamp, line], ...]
		} `json:"result"`
		Stats struct {
			Lines  int64 `json:"lines"`
			Bytes  int64 `json:"bytes"`
			ExecMs int64 `json:"execMs"`
		} `json:"stats"`
	} `json:"data"`
}

// NewLokiClient creates a new Loki client
func NewLokiClient(baseURL string) *LokiClient {
	return &LokiClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// QueryLogs queries Loki for logs matching label selectors
func (c *LokiClient) QueryLogs(ctx context.Context, query string, start, end time.Time, limit int) (*LokiLogResponse, error) {
	// Use raw query to avoid double-encoding of LogQL special chars
	u := c.BaseURL + "/loki/api/v1/query_range?query=" + url.QueryEscape(query) + "&start=" + fmt.Sprintf("%d", start.UnixNano()) + "&end=" + fmt.Sprintf("%d", end.UnixNano()) + "&limit=" + fmt.Sprintf("%d", limit)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("create loki request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute loki request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki returned status %d", resp.StatusCode)
	}

	var result LokiLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode loki response: %w", err)
	}

	return &result, nil
}

// QueryPodLogs queries logs for a specific pod using Loki
func (c *LokiClient) QueryPodLogs(ctx context.Context, pod, namespace, cluster string, start, end time.Time, limit int) ([]string, error) {
	// Query by cluster and namespace first
	// The pod label may not exist in Loki due to Promtail relabeling issues,
	// so we filter by filename containing the pod name
	query := fmt.Sprintf(`{cluster="%s", namespace="%s"}`, cluster, namespace)

	result, err := c.QueryLogs(ctx, query, start, end, limit*10)
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, stream := range result.Data.Result {
		// Get filename from stream labels to filter by pod
		filename := stream.Stream["filename"]
		if filename == "" {
			continue
		}

		// Filename format: /var/log/pods/{namespace}_{pod_name}_{uid}/log-tester/0.log
		// The pod name in filename contains the deployment name (e.g., "log-tester-768899844-5zfjm")
		// We check if the filename contains the pod name as a substring
		if !containsString(filename, pod) {
			continue
		}

		for _, value := range stream.Values {
			if len(value) >= 2 {
				logs = append(logs, value[1])
				if len(logs) >= limit {
					break
				}
			}
		}
		if len(logs) >= limit {
			break
		}
	}

	return logs, nil
}

// containsString checks if substr exists in s
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
