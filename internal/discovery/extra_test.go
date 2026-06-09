package discovery

import (
	"context"
	"testing"
	"time"
)

// TestScannerConfig_DefaultsTo5sTimeout: the spec's "Scan
// with timeout" scenario mandates a bounded per-host
// timeout. The default is 5s; users override via env.
func TestScannerConfig_DefaultsTo5sTimeout(t *testing.T) {
	c := ScannerConfig{}
	if got := c.TimeoutOrDefault(); got != 5*time.Second {
		t.Errorf("default timeout = %v, want 5s", got)
	}
	custom := ScannerConfig{Timeout: 2 * time.Second}
	if got := custom.TimeoutOrDefault(); got != 2*time.Second {
		t.Errorf("custom timeout = %v, want 2s", got)
	}
}

// TestScanWithTimeout_RespectsCancellation: a scanner that
// blocks for 1s must abort when the parent context is
// cancelled after 50ms.
func TestScanWithTimeout_RespectsCancellation(t *testing.T) {
	slow := &slowScanner{delay: 1 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := slow.Scan(ctx, "10.0.0.0/24")
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("scan took %v, expected <500ms (context should cancel)", elapsed)
	}
	if err == nil {
		t.Error("expected context-cancelled error, got nil")
	}
}

// slowScanner blocks for d before returning (or until ctx
// is cancelled).
type slowScanner struct {
	delay time.Duration
}

func (s *slowScanner) Scan(ctx context.Context, _ string) ([]Host, error) {
	select {
	case <-time.After(s.delay):
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
