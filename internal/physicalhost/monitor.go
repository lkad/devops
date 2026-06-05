package physicalhost

import (
	"context"
	"errors"
	"time"

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
	return nil
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
