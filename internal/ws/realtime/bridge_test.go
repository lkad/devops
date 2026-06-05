package realtime

import (
	"context"
	"testing"
	"time"
)

// TestBridge_DeviceStateChanged asserts the canonical event shape and
// channel naming for device state changes.
func TestBridge_DeviceStateChanged(t *testing.T) {
	p := NewNoopPublisher()
	ctx := context.Background()

	if err := PublishDeviceStateChanged(ctx, p, "dev-42", "offline", "online"); err != nil {
		t.Fatalf("PublishDeviceStateChanged err = %v", err)
	}
	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d", len(events))
	}
	evt := events[0]
	if evt.Type != "device.state_changed" {
		t.Fatalf("Type = %q", evt.Type)
	}
	if evt.Channel != "device.state_changed" {
		t.Fatalf("Channel = %q", evt.Channel)
	}
	if evt.SourceID != "dev-42" {
		t.Fatalf("SourceID = %q", evt.SourceID)
	}
	if evt.Payload["device_id"] != "dev-42" {
		t.Fatalf("Payload[device_id] = %v", evt.Payload["device_id"])
	}
	if evt.Payload["from"] != "offline" || evt.Payload["to"] != "online" {
		t.Fatalf("from/to mismatch: %v", evt.Payload)
	}
	if _, ok := evt.Payload["occurred_at"]; !ok {
		t.Fatalf("Payload missing occurred_at")
	}
}

// TestBridge_AlertFired asserts the event shape and channel for fired alerts.
func TestBridge_AlertFired(t *testing.T) {
	p := NewNoopPublisher()
	if err := PublishAlertFired(context.Background(), p, "alert-7", "high cpu", "warning"); err != nil {
		t.Fatalf("err = %v", err)
	}
	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("len = %d", len(events))
	}
	evt := events[0]
	if evt.Type != "alerts.fired" {
		t.Fatalf("Type = %q", evt.Type)
	}
	if evt.Channel != "alerts.fired" {
		t.Fatalf("Channel = %q", evt.Channel)
	}
	if evt.Payload["alert_id"] != "alert-7" {
		t.Fatalf("Payload[alert_id] = %v", evt.Payload["alert_id"])
	}
	if evt.Payload["title"] != "high cpu" {
		t.Fatalf("Payload[title] = %v", evt.Payload["title"])
	}
	if evt.Payload["severity"] != "warning" {
		t.Fatalf("Payload[severity] = %v", evt.Payload["severity"])
	}
}

// TestBridge_AlertResolved asserts the channel for resolved alerts.
func TestBridge_AlertResolved(t *testing.T) {
	p := NewNoopPublisher()
	if err := PublishAlertResolved(context.Background(), p, "alert-7"); err != nil {
		t.Fatalf("err = %v", err)
	}
	evt := p.Events()[0]
	if evt.Type != "alerts.resolved" || evt.Channel != "alerts.resolved" {
		t.Fatalf("Type=%q Channel=%q", evt.Type, evt.Channel)
	}
	if evt.Payload["alert_id"] != "alert-7" {
		t.Fatalf("alert_id mismatch: %v", evt.Payload)
	}
}

// TestBridge_PipelineRunStatusChanged asserts the channel for pipeline status.
func TestBridge_PipelineRunStatusChanged(t *testing.T) {
	p := NewNoopPublisher()
	if err := PublishPipelineRunStatus(context.Background(), p, "run-9", "build", "success"); err != nil {
		t.Fatalf("err = %v", err)
	}
	evt := p.Events()[0]
	if evt.Type != "pipeline.run_status_changed" || evt.Channel != "pipeline.run_status_changed" {
		t.Fatalf("Type=%q Channel=%q", evt.Type, evt.Channel)
	}
	if evt.Payload["run_id"] != "run-9" || evt.Payload["status"] != "success" {
		t.Fatalf("payload mismatch: %v", evt.Payload)
	}
	if evt.Payload["stage"] != "build" {
		t.Fatalf("stage mismatch: %v", evt.Payload)
	}
}

// TestBridge_HostMaintenanceEnteredExited asserts both maintenance events
// use the same channel and distinguishable types.
func TestBridge_HostMaintenanceEnteredExited(t *testing.T) {
	p := NewNoopPublisher()
	if err := PublishHostMaintenance(context.Background(), p, "host-3", true); err != nil {
		t.Fatalf("err = %v", err)
	}
	if err := PublishHostMaintenance(context.Background(), p, "host-3", false); err != nil {
		t.Fatalf("err = %v", err)
	}
	events := p.Events()
	if len(events) != 2 {
		t.Fatalf("len = %d", len(events))
	}
	if events[0].Type != "physicalhost.maintenance_entered" || events[0].Channel != "physicalhost.maintenance_entered" {
		t.Fatalf("enter Type=%q Channel=%q", events[0].Type, events[0].Channel)
	}
	if events[1].Type != "physicalhost.maintenance_exited" || events[1].Channel != "physicalhost.maintenance_exited" {
		t.Fatalf("exit Type=%q Channel=%q", events[1].Type, events[1].Channel)
	}
	for i, e := range events {
		if e.Payload["host_id"] != "host-3" {
			t.Fatalf("event[%d] host_id mismatch: %v", i, e.Payload)
		}
	}
}

// TestBridge_MetricThresholdCrossed asserts the metric threshold event shape.
func TestBridge_MetricThresholdCrossed(t *testing.T) {
	p := NewNoopPublisher()
	if err := PublishMetricThreshold(context.Background(), p, "metric-1", 92.5, 90.0); err != nil {
		t.Fatalf("err = %v", err)
	}
	evt := p.Events()[0]
	if evt.Type != "metric.threshold_crossed" || evt.Channel != "metric.threshold_crossed" {
		t.Fatalf("Type=%q Channel=%q", evt.Type, evt.Channel)
	}
	if evt.Payload["metric_id"] != "metric-1" {
		t.Fatalf("metric_id mismatch: %v", evt.Payload)
	}
	if v, _ := evt.Payload["value"].(float64); v != 92.5 {
		t.Fatalf("value mismatch: %v", evt.Payload["value"])
	}
	if th, _ := evt.Payload["threshold"].(float64); th != 90.0 {
		t.Fatalf("threshold mismatch: %v", evt.Payload["threshold"])
	}
}

// TestBridge_PreservesOccurredAt ensures the event timestamp matches a
// caller-supplied time for deterministic downstream serialization.
func TestBridge_PreservesOccurredAt(t *testing.T) {
	p := NewNoopPublisher()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := PublishAlertFiredWithTime(context.Background(), p, ts, "a-1", "x", "info"); err != nil {
		t.Fatalf("err = %v", err)
	}
	evt := p.Events()[0]
	if !evt.OccurredAt.Equal(ts) {
		t.Fatalf("OccurredAt = %v, want %v", evt.OccurredAt, ts)
	}
}
