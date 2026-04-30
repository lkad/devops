package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ESClient queries logs from Elasticsearch
type ESClient struct {
	BaseURL string
	Index   string
	Client  *http.Client
}

// ESSearchResponse is the response from ES search
type ESSearchResponse struct {
	Took int `json:"took"`
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source struct {
				Timestamp string `json:"@timestamp"`
				Message   string `json:"message"`
				Stream    string `json:"stream"`
				Pod       string `json:"kubernetes.pod_name"`
				Namespace string `json:"kubernetes.namespace_name"`
				Cluster   string `json:"kubernetes.cluster_name"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// NewESClient creates a new Elasticsearch client
func NewESClient(baseURL, index string) *ESClient {
	return &ESClient{
		BaseURL: baseURL,
		Index:   index,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// QueryPodLogs queries logs for a specific pod using Elasticsearch
func (c *ESClient) QueryPodLogs(ctx context.Context, pod, namespace, cluster string, start, end time.Time, limit int) ([]string, error) {
	// Build ES query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"kubernetes.pod_name": pod}},
					{"term": map[string]interface{}{"kubernetes.namespace_name": namespace}},
					{"term": map[string]interface{}{"kubernetes.cluster_name": cluster}},
				},
				"filter": []map[string]interface{}{
					{"range": map[string]interface{}{
						"@timestamp": map[string]interface{}{
							"gte": start.Format(time.RFC3339),
							"lte": end.Format(time.RFC3339),
						},
					}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"@timestamp": map[string]interface{}{"order": "desc"}},
		},
		"size": limit,
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal es query: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.BaseURL, c.Index)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create es request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute es request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("es returned status %d", resp.StatusCode)
	}

	var result ESSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode es response: %w", err)
	}

	var logs []string
	for _, hit := range result.Hits.Hits {
		logs = append(logs, hit.Source.Message)
	}

	return logs, nil
}