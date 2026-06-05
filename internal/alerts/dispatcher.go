package alerts

import (
	"context"
	"log/slog"
	"sync"
)

// Dispatcher is the seam the Service uses to send an alert to a
// channel. The interface is small on purpose: one method, one
// return value. Implementations:
//
//   - LogDispatcher:     writes a structured log line. Always
//                        available, used in dev / test.
//   - FakeDispatcher:    records every (alert, channel) pair in a
//                        slice for assertion in tests.
//   - Slack/PagerDuty/Email/WebhookDispatcher: stubs that do
//                        nothing; Phase 6 will replace them with
//                        real HTTP clients.
//
// Dispatch must be safe to call concurrently from multiple
// goroutines.
type Dispatcher interface {
	Dispatch(ctx context.Context, alert Alert, channel Channel) error
}

// LogDispatcher writes a structured log line per dispatch. It is
// the default dispatcher in dev / test environments and the
// "always on" channel behind the spec's log-only channel.
type LogDispatcher struct {
	log *slog.Logger
}

// NewLogDispatcher builds a LogDispatcher with the supplied
// logger. A nil logger falls back to slog.Default so the
// constructor never panics.
func NewLogDispatcher(log *slog.Logger) *LogDispatcher {
	if log == nil {
		log = slog.Default()
	}
	return &LogDispatcher{log: log}
}

// Dispatch writes a single info-level log line carrying the
// alert name, severity, and channel type. The error return is
// always nil today; the signature is preserved for future
// log-dispatcher failures (e.g. stderr closed).
func (d *LogDispatcher) Dispatch(_ context.Context, a Alert, ch Channel) error {
	d.log.Info("alert_dispatch",
		"alert", a.Name,
		"alert_id", a.ID,
		"severity", a.Severity,
		"channel", ch.Type,
		"channel_id", ch.ID,
		"suppressed", a.Suppressed,
	)
	return nil
}

// DispatchCall is a single (alert, channel) pair recorded by the
// FakeDispatcher. Tests assert on the slice contents.
type DispatchCall struct {
	Alert   Alert
	Channel Channel
}

// FakeDispatcher is a test double that records every dispatch
// call. It is safe for concurrent use so the race-detector
// guards the service-level test paths.
type FakeDispatcher struct {
	mu    sync.Mutex
	Calls []DispatchCall
}

// NewFakeDispatcher returns an empty FakeDispatcher.
func NewFakeDispatcher() *FakeDispatcher { return &FakeDispatcher{} }

// Dispatch appends the call to the recorded slice and returns
// nil. The recording happens under a mutex so the test that
// fires N goroutines does not race.
func (f *FakeDispatcher) Dispatch(_ context.Context, a Alert, ch Channel) error {
	f.mu.Lock()
	f.Calls = append(f.Calls, DispatchCall{Alert: a, Channel: ch})
	f.mu.Unlock()
	return nil
}

// SlackDispatcher is a stub for the Slack webhook integration.
// Phase 6 will replace the body with a real HTTP POST. The
// constructor stores the webhook URL for that future use.
type SlackDispatcher struct {
	webhookURL string
}

// NewSlackDispatcher returns a stub SlackDispatcher. The URL is
// retained for the future HTTP client; the stub itself does not
// make any network call.
func NewSlackDispatcher(webhookURL string) *SlackDispatcher {
	return &SlackDispatcher{webhookURL: webhookURL}
}

// Dispatch is a no-op for now. Phase 6 will post the alert to
// the configured webhook URL.
func (s *SlackDispatcher) Dispatch(_ context.Context, _ Alert, _ Channel) error {
	return nil
}

// PagerDutyDispatcher is a stub for the PagerDuty Events API v2
// integration. Phase 6 will replace the body with a real HTTP
// POST to events.pagerduty.com.
type PagerDutyDispatcher struct {
	integrationKey string
}

// NewPagerDuctyDispatcher returns a stub PagerDutyDispatcher.
// (The function name keeps the constructor symmetrical with the
// other channels even though "PagerDuty" is two words.)
func NewPagerDuctyDispatcher(integrationKey string) *PagerDutyDispatcher {
	return &PagerDutyDispatcher{integrationKey: integrationKey}
}

// Dispatch is a no-op for now.
func (p *PagerDutyDispatcher) Dispatch(_ context.Context, _ Alert, _ Channel) error {
	return nil
}

// EmailDispatcher is a stub for the SMTP integration. Phase 6
// will replace the body with a real smtp.SendMail call.
type EmailDispatcher struct {
	smtpHost string
}

// NewEmailDispatcher returns a stub EmailDispatcher.
func NewEmailDispatcher(smtpHost string) *EmailDispatcher {
	return &EmailDispatcher{smtpHost: smtpHost}
}

// Dispatch is a no-op for now.
func (e *EmailDispatcher) Dispatch(_ context.Context, _ Alert, _ Channel) error {
	return nil
}

// WebhookDispatcher is a stub for the generic webhook
// integration. Phase 6 will replace the body with a real HTTP
// POST to the configured URL.
type WebhookDispatcher struct {
	url string
}

// NewWebhookDispatcher returns a stub WebhookDispatcher.
func NewWebhookDispatcher(url string) *WebhookDispatcher {
	return &WebhookDispatcher{url: url}
}

// Dispatch is a no-op for now.
func (w *WebhookDispatcher) Dispatch(_ context.Context, _ Alert, _ Channel) error {
	return nil
}
