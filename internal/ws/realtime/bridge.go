package realtime

import (
	"context"
	"time"
)

// Canonical event types and their channel names. Channel names use the
// "<domain>.<event>" convention, all lower-case.
const (
	TypeDeviceStateChanged   = "device.state_changed"
	TypeAlertFired           = "alerts.fired"
	TypeAlertResolved        = "alerts.resolved"
	TypePipelineRunStatusChg = "pipeline.run_status_changed"
	TypeHostMaintEntered     = "physicalhost.maintenance_entered"
	TypeHostMaintExited      = "physicalhost.maintenance_exited"
	TypeMetricThresholdCross = "metric.threshold_crossed"
)

// PublishDeviceStateChanged emits a device.state_changed event on the
// "device.state_changed" channel.
func PublishDeviceStateChanged(ctx context.Context, p Publisher, deviceID, from, to string) error {
	return p.Publish(ctx, NewEvent(TypeDeviceStateChanged, deviceID, time.Now().UTC(), map[string]any{
		"device_id":   deviceID,
		"from":        from,
		"to":          to,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}))
}

// PublishAlertFired emits an alerts.fired event on the "alerts.fired"
// channel.
func PublishAlertFired(ctx context.Context, p Publisher, alertID, title, severity string) error {
	return PublishAlertFiredWithTime(ctx, p, time.Now().UTC(), alertID, title, severity)
}

// PublishAlertFiredWithTime is the time-deterministic variant used by
// tests and by callers that have already captured an OccurredAt.
func PublishAlertFiredWithTime(ctx context.Context, p Publisher, occurredAt time.Time, alertID, title, severity string) error {
	return p.Publish(ctx, NewEvent(TypeAlertFired, alertID, occurredAt, map[string]any{
		"alert_id":    alertID,
		"title":       title,
		"severity":    severity,
		"occurred_at": occurredAt.UTC().Format(time.RFC3339Nano),
	}))
}

// PublishAlertResolved emits an alerts.resolved event on the
// "alerts.resolved" channel.
func PublishAlertResolved(ctx context.Context, p Publisher, alertID string) error {
	return p.Publish(ctx, NewEvent(TypeAlertResolved, alertID, time.Now().UTC(), map[string]any{
		"alert_id":    alertID,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}))
}

// PublishPipelineRunStatus emits a pipeline.run_status_changed event on
// the "pipeline.run_status_changed" channel.
func PublishPipelineRunStatus(ctx context.Context, p Publisher, runID, stage, status string) error {
	return p.Publish(ctx, NewEvent(TypePipelineRunStatusChg, runID, time.Now().UTC(), map[string]any{
		"run_id":      runID,
		"stage":       stage,
		"status":      status,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}))
}

// PublishHostMaintenance emits either a maintenance_entered or
// maintenance_exited event on the corresponding channel based on `enter`.
func PublishHostMaintenance(ctx context.Context, p Publisher, hostID string, enter bool) error {
	typ := TypeHostMaintExited
	if enter {
		typ = TypeHostMaintEntered
	}
	return p.Publish(ctx, NewEvent(typ, hostID, time.Now().UTC(), map[string]any{
		"host_id":     hostID,
		"enter":       enter,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}))
}

// PublishMetricThreshold emits a metric.threshold_crossed event on the
// "metric.threshold_crossed" channel.
func PublishMetricThreshold(ctx context.Context, p Publisher, metricID string, value, threshold float64) error {
	return p.Publish(ctx, NewEvent(TypeMetricThresholdCross, metricID, time.Now().UTC(), map[string]any{
		"metric_id":   metricID,
		"value":       value,
		"threshold":   threshold,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}))
}
