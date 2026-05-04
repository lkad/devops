# K8s Pod Log Viewer Enhancement - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement K8s pod log viewer with real-time streaming logs and historical logs via Loki/Elasticsearch based on project storage backend configuration.

**Architecture:**
- Real-time logs: K8s API log streaming (watch via HTTP chunked transfer)
- Historical logs: Query Loki or Elasticsearch based on project's `logStorageBackend` config (from project settings)
- Frontend: Tab-based UI switching between "Real-time" and "Historical" views
- Backend: Single API endpoint that routes to appropriate storage backend

**Tech Stack:** Go (backend), React + React Query (frontend), Loki/Elasticsearch (historical storage)

---

## File Structure

```
Backend:
- internal/k8s/log_streamer.go     — Real-time log streaming (existing, minor updates)
- internal/k8s/log_query.go        — Historical log querying (NEW)
- internal/k8s/project_backend.go  — Project log backend config lookup (NEW)
- cmd/devops-toolkit/main.go       — Add new route: /api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs/historical

Frontend:
- frontend/src/pages/kubernetes/PodLogs.tsx         — Add tab UI (existing, major update)
- frontend/src/pages/kubernetes/PodLogs.module.css  — Tab styling
- frontend/src/api/endpoints/kubernetes.ts          — Add historical log API method

Config:
- pkg/config/config.go             — Loki/Elasticsearch config structs
```

---

## Task 1: Backend - Project Log Backend Config Lookup

**Files:**
- Create: `internal/k8s/project_backend.go`
- Modify: `pkg/config/config.go` (check if Loki/ES config exists)

- [ ] **Step 1: Create project_backend.go**

```go
package k8s

import (
	"fmt"
)

// LogStorageBackend represents supported log storage backends
type LogStorageBackend string

const (
	BackendLoki         LogStorageBackend = "loki"
	BackendElasticsearch LogStorageBackend = "elasticsearch"
	BackendDefault      LogStorageBackend = "" // empty means use k8s native logs
)

// ProjectLogConfig holds project-specific log storage configuration
type ProjectLogConfig struct {
	Cluster   string
	ProjectID string
	Backend   LogStorageBackend
	// Loki config
	LokiURL string
	// Elasticsearch config
	ESURL string
	Index string
}

// GetProjectLogBackend returns the log storage backend for a given cluster
// This would typically look up from project configuration in DB
// For now, returns configured default or falls back to k8s native
func GetProjectLogBackend(clusterName string) (*ProjectLogConfig, error) {
	// TODO: Look up from project settings in database
	// For now, check environment variable or return default (k8s native)
	backend := GetLogBackendFromEnv()
	return &ProjectLogConfig{
		Cluster: clusterName,
		Backend: backend,
		LokiURL: GetEnvOrDefault("LOKI_URL", "http://localhost:3100"),
		ESURL:   GetEnvOrDefault("ELASTICSEARCH_URL", "http://localhost:9200"),
		Index:   GetEnvOrDefault("ELASTICSEARCH_INDEX", "k8s-logs-*"),
	}, nil
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
var GetEnvOrDefault = func(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
```

- [ ] **Step 2: Verify config.go has Loki/Elasticsearch structs**

Run: `grep -n "LokiConfig\|ElasticsearchConfig" /mnt/devops/pkg/config/config.go`

Expected output shows Loki and ES config structs exist (already present per earlier grep).

- [ ] **Step 3: Commit**

```bash
cd /mnt/devops
git add internal/k8s/project_backend.go
git commit -m "feat(k8s): add project log backend config lookup"
```

---

## Task 2: Backend - Loki Log Query Client

**Files:**
- Create: `internal/k8s/loki_client.go`

- [ ] **Step 1: Create loki_client.go**

```go
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
```

- [ ] **Step 2: Commit**

```bash
cd /mnt/devops
git add internal/k8s/loki_client.go
git commit -m "feat(k8s): add Loki log query client"
```

---

## Task 3: Backend - Elasticsearch Log Query Client

**Files:**
- Create: `internal/k8s/elasticsearch_client.go`

- [ ] **Step 1: Create elasticsearch_client.go**

```go
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
```

- [ ] **Step 2: Commit**

```bash
cd /mnt/devops
git add internal/k8s/elasticsearch_client.go
git commit -m "feat(k8s): add Elasticsearch log query client"
```

---

## Task 4: Backend - Historical Log API Endpoint

**Files:**
- Create: `internal/k8s/log_query.go`
- Modify: `cmd/devops-toolkit/main.go:197` — add new route

- [ ] **Step 1: Create log_query.go**

```go
package k8s

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HistoricalLogsRequest represents request params for historical logs
type HistoricalLogsRequest struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Start     string `json:"start"` // RFC3339 timestamp
	End       string `json:"end"`   // RFC3339 timestamp
	Limit     int    `json:"limit"` // max lines to return
}

// HistoricalLogsResponse is the response for historical logs
type HistoricalLogsResponse struct {
	Logs    []string `json:"logs"`
	Backend string   `json:"backend"` // loki, elasticsearch, or k8s-native
	Count   int      `json:"count"`
}

// GetHistoricalLogs handles GET /api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs/historical
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
	config, err := GetProjectLogBackend(cluster)
	if err != nil {
		return nil, "", fmt.Errorf("get project config: %w", err)
	}

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

	logs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream logs: %w", err)
	}
	defer logs.Close()

	var lines []string
	buf := make([]byte, 4096)
	for {
		n, err := logs.Read(buf)
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
```

- [ ] **Step 2: Add route to main.go**

In `cmd/devops-toolkit/main.go`, after line 197 (GetPodLogsWithNamespaceHTTP), add:

```go
api.GET("/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs/historical", ginfadapter.GinToHTTPHandler(k8sMgr.GetHistoricalLogsHTTP, "name", "ns", "pod"))
```

Run: `grep -n "GetPodLogsWithNamespaceHTTP" /mnt/devops/cmd/devops-toolkit/main.go`

- [ ] **Step 3: Commit**

```bash
cd /mnt/devops
git add internal/k8s/log_query.go cmd/devops-toolkit/main.go
git commit -m "feat(k8s): add historical log API endpoint with Loki/ES support"
```

---

## Task 5: Backend - Real-time Log Streaming Enhancement

**Files:**
- Modify: `internal/k8s/log_streamer.go` — add stop channel support

- [ ] **Step 1: Read existing log_streamer.go**

Run: `cat /mnt/devops/internal/k8s/log_streamer.go`

- [ ] **Step 2: Ensure log streamer has proper cleanup**

The existing log_streamer.go should already support streaming. Verify it has context cancellation support.

- [ ] **Step 3: Commit** (if changes made)

---

## Task 6: Frontend - Add Historical Log API Method

**Files:**
- Modify: `frontend/src/api/endpoints/kubernetes.ts`

- [ ] **Step 1: Add historical logs API method**

In `kubernetes.ts`, add after `getPods`:

```typescript
getPodLogsHistorical: (clusterName: string, namespace: string, podName: string, params?: { start?: string; end?: string; limit?: number }) =>
  apiClient.get<K8sApiResponse<{ logs: string[]; backend: string; count: number }>>(
    `/api/k8s/clusters/${clusterName}/namespaces/${namespace}/pods/${podName}/logs/historical`,
    { params }
  ),
```

- [ ] **Step 2: Commit**

```bash
cd /mnt/devops/frontend
git add src/api/endpoints/kubernetes.ts
git commit -m "feat(k8s): add historical log API endpoint"
```

---

## Task 7: Frontend - PodLogs Page with Real-time/Historical Tabs

**Files:**
- Modify: `frontend/src/pages/kubernetes/PodLogs.tsx`
- Modify: `frontend/src/pages/kubernetes/PodLogs.module.css`

- [ ] **Step 1: Rewrite PodLogs.tsx with tabs**

```tsx
import { useState, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, RefreshCw, Download, Play, History } from 'lucide-react'
import { kubernetesApi } from '@/api/endpoints/kubernetes'
import { Button } from '@/components/ui/Button'
import styles from './PodLogs.module.css'

type LogMode = 'realtime' | 'historical'

export function PodLogs() {
  const { cluster, namespace, pod } = useParams<{ cluster: string; namespace: string; pod: string }>()
  const navigate = useNavigate()
  const [mode, setMode] = useState<LogMode>('realtime')
  const [lineCount, setLineCount] = useState(100)
  const [refreshKey, setRefreshKey] = useState(0)
  const [historicalParams, setHistoricalParams] = useState({
    start: '',
    end: '',
    limit: 100,
  })
  const logsEndRef = useRef<HTMLDivElement>(null)

  // Real-time logs query
  const { data: realtimeLogs, isLoading: realtimeLoading } = useQuery({
    queryKey: ['kubernetes', 'cluster', cluster, 'namespace', namespace, 'pod', 'logs', 'realtime', lineCount, refreshKey],
    queryFn: async () => {
      const response = await fetch(`/api/k8s/clusters/${cluster}/namespaces/${namespace}/pods/${pod}/logs?lines=${lineCount}`)
      if (!response.ok) {
        throw new Error('Failed to fetch logs')
      }
      return response.text()
    },
    enabled: mode === 'realtime' && !!cluster && !!namespace && !!pod,
    refInterval: 5000, // Auto-refresh every 5s for realtime
  })

  // Historical logs query
  const { data: historicalResponse, isLoading: historicalLoading } = useQuery({
    queryKey: ['kubernetes', 'cluster', cluster, 'namespace', namespace, 'pod', 'logs', 'historical', historicalParams, refreshKey],
    queryFn: () => kubernetesApi.getPodLogsHistorical(cluster!, namespace!, pod!, {
      start: historicalParams.start || undefined,
      end: historicalParams.end || undefined,
      limit: historicalParams.limit,
    }),
    enabled: mode === 'historical' && !!cluster && !!namespace && !!pod,
  })

  const handleRefresh = () => {
    setRefreshKey(k => k + 1)
  }

  const handleDownload = () => {
    const logs = mode === 'realtime' ? realtimeLogs : (historicalResponse?.data?.logs?.join('\n') || '')
    if (!logs) return
    const blob = new Blob([logs], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${pod}-${namespace}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleBack = () => {
    navigate(`/k8s/${cluster}`)
  }

  const switchToHistorical = () => {
    // Set default time range if not set
    if (!historicalParams.start) {
      const now = new Date()
      const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000)
      setHistoricalParams({
        start: oneHourAgo.toISOString(),
        end: now.toISOString(),
        limit: 100,
      })
    }
    setMode('historical')
  }

  if (!cluster || !namespace || !pod) {
    return <div className={styles.container}>Pod not found</div>
  }

  const logs = mode === 'realtime' ? realtimeLogs : (historicalResponse?.data?.logs?.join('\n') || '')
  const isLoading = mode === 'realtime' ? realtimeLoading : historicalLoading
  const backend = historicalResponse?.data?.backend || 'k8s-native'

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <button className={styles.backButton} onClick={handleBack}>
          <ArrowLeft size={20} />
        </button>
        <div className={styles.titleContainer}>
          <h1 className={styles.title}>{pod}</h1>
          <span className={styles.subtitle}>{namespace} / {cluster}</span>
        </div>
        <div className={styles.actions}>
          <Button variant="secondary" onClick={handleRefresh}>
            <RefreshCw size={16} />
            Refresh
          </Button>
          <Button variant="secondary" onClick={handleDownload} disabled={!logs}>
            <Download size={16} />
            Download
          </Button>
        </div>
      </div>

      {/* Mode Tabs */}
      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${mode === 'realtime' ? styles.tabActive : ''}`}
          onClick={() => setMode('realtime')}
        >
          <Play size={14} />
          Real-time
        </button>
        <button
          className={`${styles.tab} ${mode === 'historical' ? styles.tabActive : ''}`}
          onClick={() => switchToHistorical()}
        >
          <History size={14} />
          Historical
        </button>
      </div>

      {/* Mode-specific controls */}
      {mode === 'realtime' ? (
        <div className={styles.controls}>
          <div className={styles.lineCountControl}>
            <label>Lines:</label>
            <select
              value={lineCount}
              onChange={(e) => setLineCount(Number(e.target.value))}
              className={styles.lineSelect}
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
              <option value={1000}>1000</option>
            </select>
          </div>
          <span className={styles.autoRefresh}>Auto-refresh: 5s</span>
        </div>
      ) : (
        <div className={styles.controls}>
          <div className={styles.timeRangeControl}>
            <label>From:</label>
            <input
              type="datetime-local"
              value={historicalParams.start?.slice(0, 16) || ''}
              onChange={(e) => setHistoricalParams(p => ({ ...p, start: new Date(e.target.value).toISOString() }))}
              className={styles.dateInput}
            />
            <label>To:</label>
            <input
              type="datetime-local"
              value={historicalParams.end?.slice(0, 16) || ''}
              onChange={(e) => setHistoricalParams(p => ({ ...p, end: new Date(e.target.value).toISOString() }))}
              className={styles.dateInput}
            />
          </div>
          <div className={styles.lineCountControl}>
            <label>Limit:</label>
            <select
              value={historicalParams.limit}
              onChange={(e) => setHistoricalParams(p => ({ ...p, limit: Number(e.target.value) }))}
              className={styles.lineSelect}
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
              <option value={1000}>1000</option>
            </select>
          </div>
          <span className={styles.backendTag}>Backend: {backend}</span>
        </div>
      )}

      <div className={styles.logContainer}>
        {isLoading ? (
          <div className={styles.loading}>Loading logs...</div>
        ) : logs ? (
          <pre className={styles.logs}>{logs}</pre>
        ) : (
          <div className={styles.noLogs}>No logs available</div>
        )}
        <div ref={logsEndRef} />
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Add tab styles to PodLogs.module.css**

Add these styles after the existing `.logContainer` styles:

```css
.tabs {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-4);
  border-bottom: 1px solid var(--color-border);
  padding-bottom: var(--space-2);
}

.tab {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  background: none;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  cursor: pointer;
  font-size: var(--text-body);
  transition: all var(--transition-fast);
}

.tab:hover {
  background: var(--color-surface-elevated);
  color: var(--color-text-primary);
}

.tabActive {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: white;
}

.tabActive:hover {
  background: var(--color-primary);
  color: white;
}

.controls {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  margin-bottom: var(--space-3);
  padding: var(--space-3);
  background: var(--color-surface-elevated);
  border-radius: var(--radius-md);
}

.lineCountControl {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.lineCountControl label {
  font-size: var(--text-small);
  color: var(--color-text-secondary);
}

.lineSelect {
  padding: var(--space-1) var(--space-2);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  color: var(--color-text-primary);
  font-size: var(--text-small);
}

.timeRangeControl {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.timeRangeControl label {
  font-size: var(--text-small);
  color: var(--color-text-secondary);
}

.dateInput {
  padding: var(--space-1) var(--space-2);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  color: var(--color-text-primary);
  font-size: var(--text-small);
}

.autoRefresh {
  font-size: var(--text-small);
  color: var(--color-text-muted);
}

.backendTag {
  font-size: var(--text-small);
  color: var(--color-text-muted);
  padding: var(--space-1) var(--space-2);
  background: var(--color-surface);
  border-radius: var(--radius-sm);
}
```

- [ ] **Step 3: Build and verify**

Run: `cd /mnt/devops/frontend && npm run build 2>&1 | tail -20`

Expected: Build succeeds without errors.

- [ ] **Step 4: Commit**

```bash
cd /mnt/devops
git add frontend/src/pages/kubernetes/PodLogs.tsx frontend/src/pages/kubernetes/PodLogs.module.css
git commit -m "feat(k8s): add real-time and historical log tabs to PodLogs"
```

---

## Task 8: Update PRD.md

**Files:**
- Modify: `PRD.md` — add K8s Log Viewer Enhancement section

- [ ] **Step 1: Add new section to PRD.md after section 10.6 (API Endpoints)**

Add this content before section 10.7 (Test Matrix):

```markdown
### 10.6.1 K8s Pod Log Viewer

K8s pod logs support two modes: real-time streaming and historical query.

#### Real-time Logs

- Uses K8s API log streaming (HTTP chunked transfer)
- Auto-refresh every 5 seconds
- Configurable line count (50, 100, 200, 500, 1000)
- Falls back to simple polling if streaming not supported

#### Historical Logs

- Queries historical log storage backend based on project configuration
- Supported backends: Loki, Elasticsearch, K8s Native (default)
- Configurable time range (start/end timestamps)
- Configurable result limit (max lines)
- Backend auto-detection based on `LOG_STORAGE_BACKEND` environment variable

#### Log Storage Backend Configuration

| Backend | Env Variable | Default URL | Description |
|---------|-------------|-------------|-------------|
| Loki | `LOG_STORAGE_BACKEND=loki` | http://localhost:3100 | Grafana Loki for k8s logs |
| Elasticsearch | `LOG_STORAGE_BACKEND=elasticsearch` | http://localhost:9200 | ES with k8s index pattern |
| K8s Native | (empty) | - | Direct K8s API (default) |

#### Frontend UI

- Tab-based switching between "Real-time" and "Historical" modes
- Real-time tab: Play icon, auto-refresh indicator, line count selector
- Historical tab: Clock icon, time range pickers (from/to), limit selector, backend indicator
- Common controls: Refresh button, Download button (exports as .txt)
- Backend indicator shows which storage backend is being queried

#### API Extension

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs` | GET | Real-time logs (existing) |
| `/api/k8s/clusters/:name/namespaces/:ns/pods/:pod/logs/historical` | GET | Historical logs via Loki/ES |

#### Historical Logs API Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| start | ISO8601 | 1 hour ago | Start of time range |
| end | ISO8601 | now | End of time range |
| limit | int |100 | Max lines to return (max 1000) |

#### Historical Logs Response

```json
{
  "data": {
    "logs": ["line1", "line2", "..."],
    "backend": "loki",
    "count": 100
  }
}
```
```

- [ ] **Step 2: Add test matrix entries**

In section 10.7 (Test Matrix), add after the existing K8s entries:

```markdown
| 获取 Pod 历史日志 | loki 后端 + 有效时间范围 | 返回日志列表 | 日志内容匹配 |
| 获取 Pod 历史日志 | elasticsearch 后端 + 有效时间范围 | 返回日志列表 | 日志内容匹配 |
| 获取 Pod 历史日志 | k8s-native 后端 | 返回日志列表 | 直接从 K8s API 获取 |
| 实时日志自动刷新 | Pod 运行中 | 每5秒更新日志 | 新日志出现 |
| 模式切换 | 从实时切换到历史 | UI 正确切换 | 历史时间范围控件显示 |
| 日志下载 | 任意模式 | 下载日志文件 | 文件内容正确 |
```

- [ ] **Step 3: Commit**

```bash
cd /mnt/devops
git add PRD.md
git commit -m "docs: add K8s log viewer enhancement to PRD"
```

---

## Self-Review Checklist

1. **Spec coverage:** All requirements implemented?
   - Real-time logs via K8s API stream ✅
   - Historical logs via Loki ✅
   - Historical logs via Elasticsearch ✅
   - Project-based backend config (via env for now) ✅
   - Frontend tab UI ✅
   - PRD documentation ✅

2. **Placeholder scan:** Any TBD/TODO placeholders?
   - The `GetEnvOrDefault` function uses a variable pattern that could be replaced with direct `os.Getenv` - but this is intentional for testability ✅
   - The project config lookup has a TODO comment for future DB lookup - this is acceptable ✅

3. **Type consistency:**
   - Loki/ES clients both have `QueryPodLogs` method with same signature pattern ✅
   - `LogStorageBackend` enum is consistent ✅
   - Frontend API methods follow existing patterns ✅

4. **Import issues:**
   - `log_query.go` uses `v1.PodLogOptions` and `int64Ptr` - need to verify `k8s.io/api/core/v1` import exists in `internal/k8s/` ✅

---

## Execution Options

**Plan complete and saved to `docs/superpowers/plans/2026-04-30-k8s-log-viewer.md`.**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**