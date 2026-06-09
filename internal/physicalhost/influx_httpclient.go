package physicalhost

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClientPost is the production InfluxDB HTTP client. It
// uses the standard net/http transport with a 5s timeout so
// a slow or unreachable InfluxDB cannot stall the async
// writer's worker pool. The body is the InfluxDB v2 line
// protocol encoded by InfluxWriter.encodeMetricsLineProtocol.
//
// Errors are intentionally swallowed: a failed write increments
// the AsyncInfluxWriter's dropped counter via the worker
// loop, so the test surface stays "POST" without a transport
// seam. Production deployments can swap a metrics-emitter
// implementation in via SetClient for retry-with-backoff.
type httpClientPost struct {
	timeout time.Duration
	client  *http.Client
}

// NewHTTPClientPost returns a configured InfluxDB HTTP client.
// The supplied timeout is a per-call ceiling; the underlying
// transport is shared across calls so connections pool.
func NewHTTPClientPost(timeout time.Duration) *httpClientPost {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &httpClientPost{
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        16,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// Post sends the line-protocol body to InfluxDB /api/v2/write.
// Returns an error if the response status is not 2xx so the
// caller (async writer worker) can log + count as a drop.
func (h *httpClientPost) Post(url, token string, body []byte) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("influx client: new request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Authorization", "Token "+token)
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("influx client: post: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("influx client: status %d", resp.StatusCode)
	}
	return nil
}

// Compile-time check that httpClientPost implements influxHTTPClient.
var _ influxHTTPClient = (*httpClientPost)(nil)
