package servicecatalog

import (
	"testing"
	"time"
)

// TestOnCall_CurrentForService_PicksActiveWindow pins
// the "now is inside a shift" rule. When the OnCall
// table has one shift covering [t-1h, t+1h] and the
// service is the same, CurrentOnCall returns the
// assigned user.
func TestOnCall_CurrentForService_PicksActiveWindow(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now()
	shift := &OnCall{
		ServiceID:   "svc-x",
		User:       "alice@example.com",
		ShiftStart: now.Add(-1 * time.Hour),
		ShiftEnd:   now.Add(1 * time.Hour),
	}
	if err := repo.CreateOnCall(shift); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.CurrentOnCall("svc-x", now)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if got == nil {
		t.Fatalf("expected an active shift, got nil")
	}
	if got.User != "alice@example.com" {
		t.Errorf("user = %q, want alice", got.User)
	}
}

// TestOnCall_CurrentForService_OutsideWindow pins the
// "no active shift" branch: a shift in the past is not
// returned, the helper reports nil so the page can
// show "no one on call".
func TestOnCall_CurrentForService_OutsideWindow(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now()
	past := &OnCall{
		ServiceID:   "svc-x",
		User:       "alice@example.com",
		ShiftStart: now.Add(-2 * time.Hour),
		ShiftEnd:   now.Add(-1 * time.Hour),
	}
	if err := repo.CreateOnCall(past); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.CurrentOnCall("svc-x", now)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

// TestOnCall_GlobalRotation_FallsThroughToGlobal
// pins the "service-specific rotation overrides global"
// rule. When no per-service shift is active but a
// global (service_id empty) one is, CurrentOnCall
// returns the global.
func TestOnCall_GlobalRotation_FallsThroughToGlobal(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now()
	global := &OnCall{
		ServiceID:   "",
		User:       "platform@example.com",
		ShiftStart: now.Add(-1 * time.Hour),
		ShiftEnd:   now.Add(1 * time.Hour),
	}
	if err := repo.CreateOnCall(global); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.CurrentOnCall("svc-x", now)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if got == nil || got.User != "platform@example.com" {
		t.Errorf("expected global rotation, got %+v", got)
	}
}
