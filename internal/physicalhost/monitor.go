package physicalhost

import (
	"context"
	"errors"
	"time"

	"github.com/devops-toolkit/backend/internal/ws/realtime"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

// MonitorConfig bundles the dependencies of MonitorService. Kept
// as a struct so new dependencies (clock, metrics emitter) can
// be added without changing call sites.
type MonitorConfig struct {
	Repo                *Repository
	Prober              Prober
	ConsecutiveFailures int
	CheckInterval       time.Duration
}

// MetricsSink is the seam the monitor uses to push a fresh
// metrics snapshot after every Check. Production wires the
// AsyncInfluxWriter; tests inject a fake that just records.
// The signature is the minimum needed — the hostID and the
// snapshot — so other sinks (e.g. Prometheus pushgateway) can
// be added without touching the monitor.
type MetricsSink interface {
	Enqueue(hostID string, m Metrics)
}

// MonitorService owns the periodic health-check loop and the
// state-transition rules. State transitions:
//
//	online       -> monitoring_issue    (1st failed probe)
//	monitoring_issue -> monitoring_issue (failed, below threshold)
//	monitoring_issue -> offline         (Nth consecutive failure)
//	offline      -> online              (1st successful probe)
//	any          -> maintenance        (via MaintenanceService, blocked here)
//	*            -> *                   (no auto-transition while in maintenance)
//
// The monitor does NOT enter / exit maintenance itself; that is
// the MaintenanceService's job so the audit row is guaranteed
// to be emitted. The monitor short-circuits Check() while
// MaintenanceStartedAt is non-nil, per the spec's
// "auto state change blocked during maintenance" scenario.
type MonitorService struct {
	repo        *Repository
	prober      Prober
	threshold   int
	interval    time.Duration
	now         func() time.Time
	maintenance *MaintenanceService
	publisher   realtime.Publisher
	metricsSink MetricsSink
	collector   MetricsSource
}

// NewMonitorService builds a MonitorService with sane defaults
// for the threshold and interval if the caller leaves them zero.
func NewMonitorService(cfg MonitorConfig) *MonitorService {
	threshold := cfg.ConsecutiveFailures
	if threshold <= 0 {
		threshold = 3
	}
	interval := cfg.CheckInterval
	if interval <= 0 {
		interval = time.Minute
	}
	return &MonitorService{
		repo:      cfg.Repo,
		prober:    cfg.Prober,
		threshold: threshold,
		interval:  interval,
		now:       time.Now,
	}
}

// SetMaintenance wires the MaintenanceService the monitor
// delegates to for the Enter/Exit plumbing. Split from the
// constructor to avoid a circular factory call; tests wire it
// after both services are built.
func (m *MonitorService) SetMaintenance(s *MaintenanceService) {
	m.maintenance = s
}

// SetPublisher wires the realtime hub publisher used to
// broadcast device_event on state transitions. A nil publisher
// turns this into a no-op so existing tests / non-realtime
// deployments can opt out.
func (m *MonitorService) SetPublisher(p realtime.Publisher) {
	m.publisher = p
}

// SetMetricsSink wires the optional async metrics writer
// (typically AsyncInfluxWriter). A nil sink disables the
// "push every Check" path; the metrics endpoint / cache
// continues to work via the cache's own GetOrCollect.
func (m *MonitorService) SetMetricsSink(s MetricsSink) { m.metricsSink = s }

// SetCollector wires the optional metrics collector so the
// monitor can produce a fresh snapshot during each Check.
// nil disables the "push every Check" path even if a sink is
// set (the sink has nothing to write).
func (m *MonitorService) SetCollector(c MetricsSource) { m.collector = c }

// Check runs one probe against the given host. The public entry
// point is the method the monitoring loop calls; tests call it
// directly.
//
// State transitions:
//
//	online  + success           -> online  (counter reset, LastCheckAt bumped)
//	online  + fail              -> monitoring_issue (counter=1)
//	monitoring_issue + success  -> online  (counter reset)
//	monitoring_issue + fail     -> monitoring_issue
//	                           -> offline  (counter >= threshold)
//	offline + success           -> online  (counter reset)
//	offline + fail              -> offline
//	*  + (in maintenance)       -> unchanged (probe still runs, counter may grow)
func (m *MonitorService) Check(ctx context.Context, hostID string) error {
	p, err := m.repo.Get(hostID)
	if err != nil {
		if IsNotFound(err) {
			return ErrNotFound
		}
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to load physical host",
			Cause:   err,
		}
	}

	now := m.now().UTC()
	p.LastCheckAt = &now
	next := now.Add(m.interval)
	p.NextCheckAt = &next

	// Auto state change is blocked while the host is in
	// maintenance. The probe still runs and the counter /
	// LastCheckAt are still updated so the post-maintenance
	// review can see the history.
	if p.InMaintenance() {
		if err := m.repo.Update(p); err != nil {
			return &contracts.APIError{
				Code:    contracts.CodeInternal,
				Message: "failed to update physical host",
				Cause:   err,
			}
		}
		return nil
	}

	res, _ := m.prober.Ping(ctx, Host{
		IPAddress: p.IPAddress,
		SSHPort:   p.SSHPort,
		SSHUser:   p.SSHUser,
	})
	// A probe error is itself a negative result; the structured
	// PingResult.Reachable flag is what the state machine
	// branches on. We deliberately do not return the error
	// here — it has been recorded in LastCheckAt.

	prevState := p.State
	if res.Reachable {
		p.State = StateOnline
		p.ConsecutiveFails = 0
	} else {
		p.ConsecutiveFails++
		switch p.State {
		case StateOnline:
			// First failure: enter monitoring_issue.
			p.State = StateMonitoringIssue
		case StateMonitoringIssue:
			if p.ConsecutiveFails >= m.threshold {
				p.State = StateOffline
			}
		case StateOffline:
			// Already offline; counter grows but state stays.
		}
	}

	if err := m.repo.Update(p); err != nil {
		return &contracts.APIError{
			Code:    contracts.CodeInternal,
			Message: "failed to update physical host",
			Cause:   err,
		}
	}

	// Broadcast a device_event when (and only when) the state
	// actually changed. The channel name is the canonical
	// "device_event" so the frontend can subscribe to every
	// device domain in one place.
	if prevState != p.State {
		m.emitStateChange(ctx, p.ID, string(prevState), string(p.State))
	}

	// Best-effort: push a fresh metrics snapshot to the
	// async writer. We run the collector synchronously here
	// (in the monitor's goroutine) so the sink sees the same
	// data the cache would. A nil collector or sink is a no-op.
	if m.collector != nil && m.metricsSink != nil {
		if snap, err := m.collector.Collect(ctx, Host{
			IPAddress: p.IPAddress, SSHPort: p.SSHPort, SSHUser: p.SSHUser,
		}); err == nil && snap.DataStatus != DataStatusUnavailable {
			m.metricsSink.Enqueue(p.ID, snap)
		}
	}
	return nil
}

// emitStateChange publishes a realtime event to the device_event
// channel. A nil publisher is a no-op so test fixtures and
// non-realtime deployments can opt out.
func (m *MonitorService) emitStateChange(ctx context.Context, hostID, prev, next string) {
	if m.publisher == nil {
		return
	}
	evt := realtime.NewEvent("physical_host.state_change", hostID, m.now().UTC(), map[string]any{
		"host_id":        hostID,
		"previous_state": prev,
		"new_state":      next,
	})
	_ = m.publisher.Publish(ctx, evt)
}

// EnterMaintenanceForTest is a thin wrapper exposed to the test
// package so monitor_test.go can drive the maintenance path
// without the HTTP layer. Production code goes through the
// MaintenanceService directly.
func (m *MonitorService) EnterMaintenanceForTest(hostID, reason, userID string) error {
	if m.maintenance == nil {
		return errors.New("monitor: maintenance service not wired")
	}
	_, _, err := m.maintenance.EnterMaintenance(context.Background(), hostID, reason, userID)
	return err
}

// ExitMaintenanceForTest mirrors EnterMaintenanceForTest for the
// exit path.
func (m *MonitorService) ExitMaintenanceForTest(hostID, userID string) error {
	if m.maintenance == nil {
		return errors.New("monitor: maintenance service not wired")
	}
	_, _, err := m.maintenance.ExitMaintenance(context.Background(), hostID, userID)
	return err
}
