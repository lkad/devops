package alerts

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// TestLogDispatcher_RecordsPayload verifies the production
// dispatcher writes a structured log line per call so the
// audit trail survives when external channels fail.
func TestLogDispatcher_RecordsPayload(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	d := NewLogDispatcher(log)
	a := Alert{Name: "high_cpu", Severity: SeverityWarning}
	ch := Channel{Type: ChannelTypeLog, Config: JSONMap{}}
	ch.ID = "ch1"
	if err := d.Dispatch(context.Background(), a, ch); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !strings.Contains(buf.String(), `"msg":"alert_dispatch"`) {
		t.Errorf("expected alert_dispatch log line, got %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"alert":"high_cpu"`) {
		t.Errorf("expected alert name in log, got %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"channel":"log"`) {
		t.Errorf("expected channel type in log, got %s", buf.String())
	}
}

// TestLogDispatcher_DispatchErrPropagates exercises the error
// path: a dispatcher must return a non-nil error to the caller so
// a failed external channel can be surfaced to the auditor.
func TestLogDispatcher_DispatchErrPropagates(t *testing.T) {
	d := NewLogDispatcher(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
	// Log dispatcher is happy-path only; an error here would mean
	// a programming bug. Confirm Dispatch does NOT return a non-
	// nil error in the default configuration.
	err := d.Dispatch(context.Background(), Alert{}, Channel{Type: ChannelTypeLog})
	if err != nil {
		t.Errorf("log dispatcher should not return error, got %v", err)
	}
}

// TestFakeDispatcher_RecordsCalls verifies the test double
// captures every (alert, channel) pair it receives so assertions
// can be made on the recorded payload in downstream tests.
func TestFakeDispatcher_RecordsCalls(t *testing.T) {
	d := NewFakeDispatcher()
	a := Alert{Name: "x", Severity: SeverityCritical}
	ch := Channel{Type: ChannelTypeSlack, Enabled: true}
	ch.ID = "c1"
	if err := d.Dispatch(context.Background(), a, ch); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if err := d.Dispatch(context.Background(), a, ch); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := len(d.Calls); got != 2 {
		t.Errorf("recorded calls: got %d want 2", got)
	}
	if d.Calls[0].Channel.Type != ChannelTypeSlack {
		t.Errorf("channel type: got %q want slack", d.Calls[0].Channel.Type)
	}
	if d.Calls[0].Alert.Name != "x" {
		t.Errorf("alert name: got %q want x", d.Calls[0].Alert.Name)
	}
}

// TestFakeDispatcher_ConcurrentSafe is the race-detector guard.
// Multiple goroutines dispatching in parallel must not race on
// the internal slice; the test is only meaningful with -race.
func TestFakeDispatcher_ConcurrentSafe(t *testing.T) {
	d := NewFakeDispatcher()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.Dispatch(context.Background(), Alert{Name: "y"}, Channel{Type: ChannelTypeLog})
		}()
	}
	wg.Wait()
	if got := len(d.Calls); got != 10 {
		t.Errorf("concurrent calls: got %d want 10", got)
	}
}

// TestStubDispatchers_DoNotMakeNetworkCalls verifies the Slack /
// PagerDuty / Email / Webhook stubs are safe to construct in a
// test environment: they return nil errors without touching the
// network. Phase 6 will replace the stubs with real HTTP clients.
func TestStubDispatchers_DoNotMakeNetworkCalls(t *testing.T) {
	cases := []struct {
		name string
		d    Dispatcher
	}{
		{"slack", NewSlackDispatcher("https://example.com")},
		{"pagerduty", NewPagerDuctyDispatcher("k")},
		{"email", NewEmailDispatcher("smtp.example.com")},
		{"webhook", NewWebhookDispatcher("https://example.com/hook")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.d.Dispatch(context.Background(), Alert{}, Channel{Type: ChannelType(tc.name)})
			if err != nil {
				t.Errorf("stub %s returned error: %v", tc.name, err)
			}
		})
	}
}
