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
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	params.Set("end", fmt.Sprintf("%d", end.UnixNano()))
	params.Set("limit", fmt.Sprintf("%d", limit))

	u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", c.BaseURL, params.Encode())

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
	// Loki label selector for k8s pod logs
	query := fmt.Sprintf(`{pod="%s", namespace="%s", cluster="%s"}`, pod, namespace, cluster)

	result, err := c.QueryLogs(ctx, query, start, end, limit)
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, stream := range result.Data.Result {
		for _, value := range stream.Values {
			if len(value) >= 2 {
				logs = append(logs, value[1])
			}
		}
	}

	return logs, nil
}
