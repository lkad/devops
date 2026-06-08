// Boundary / edge-case tests for PhysicalHost.
//
// These tests exercise the validate() function and the monitor path
// to make sure the system degrades gracefully on partial or malformed
// input. A user pointed out "主机没有IP" as the canonical example;
// the other edge cases below are the natural neighbours.
package physicalhost

import (
	"net/http"
	"strings"
	"testing"
)

// assertValidationCode is a small helper for the boundary tests
// that all expect a 400 with a VALIDATION_ERROR envelope.
func assertValidationCode(t *testing.T, body map[string]any) {
	t.Helper()
	errEnv, _ := body["error"].(map[string]any)
	if errEnv == nil || errEnv["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR envelope, got %v", body)
	}
}

// ─── validate() : empty / whitespace field rejection ───────────

func TestBoundary_RejectsEmptyIPAddress(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	assertValidationCode(t, body)
}

func TestBoundary_RejectsWhitespaceOnlyIPAddress(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "   ", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	assertValidationCode(t, body)
}

func TestBoundary_RejectsEmptyDeviceID(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	assertValidationCode(t, body)
}

func TestBoundary_RejectsEmptySSHUser(t *testing.T) {
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "", "ssh_port": 22,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	assertValidationCode(t, body)
}

// ─── validate() : SSH port boundary cases ────────────────────────

func TestBoundary_PortZeroDefaultsToTwentyTwo(t *testing.T) {
	// The handler's toModel() treats ssh_port=0 as "not provided"
	// and falls back to the default 22. This pins the current
	// behaviour — callers that genuinely need to disable SSH on a
	// host must clear the field elsewhere.
	r := handlerFixture(t)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 0,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("port=0 should be accepted; status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if body["ssh_port"].(float64) != 22 {
		t.Errorf("ssh_port = %v, want 22 (default from 0)", body["ssh_port"])
	}
}

func TestBoundary_AcceptsPortMaxValid(t *testing.T) {
	// 65535 is the highest legal TCP port; must be accepted.
	r := handlerFixture(t)
	rr, _ := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 65535,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("port=65535 should be accepted; status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestBoundary_RejectsPortAboveMax(t *testing.T) {
	r := handlerFixture(t)
	rr, _ := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 65536,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("port=65536 should be rejected; status = %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestBoundary_RejectsNegativePort(t *testing.T) {
	r := handlerFixture(t)
	rr, _ := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": "d1", "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": -1,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("port=-1 should be rejected; status = %d", rr.Code)
	}
}

// ─── validate() : over-long / pathological strings ──────────────

func TestBoundary_AcceptsOverlongDeviceID(t *testing.T) {
	// We do not currently cap string length; a very long device_id
	// is accepted at the API surface and stored verbatim. SQLite has
	// no VARCHAR limit so the row persists. This test pins that
	// behaviour — if we add a length cap later, this will start
	// failing and that is the right time to revisit.
	r := handlerFixture(t)
	longID := strings.Repeat("a", 1024)
	rr, body := doPHRequest(t, r, "POST", "/api/v1/physical-hosts", map[string]any{
		"device_id": longID, "ip_address": "10.0.0.1", "ssh_user": "root", "ssh_port": 22,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("1KB device_id should be accepted; status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if body["device_id"].(string) != longID {
		t.Errorf("device_id was not round-tripped verbatim")
	}
}

// ─── monitor() : probe on host with empty IP ───────────────────
//
// The most-asked edge case: a host row exists but its IP is empty.
// The monitor should NOT crash, NOT raise a Go panic, and should
// return a clean error that the API can surface to the caller.

func TestBoundary_ProbeOnHostWithoutIPFailsCleanly(t *testing.T) {
	r := handlerFixture(t)
	// Bypass the handler's validate() to persist a row that
	// mirrors what a migration script or future bulk import might
	// produce — a device FK satisfied but the IP left blank.
	repo := NewRepository(openDB(t))
	host := &PhysicalHost{
		DeviceID:  "device-no-ip",
		IPAddress: "",
		SSHPort:   22,
		SSHUser:   "root",
		State:     StateOnline,
	}
	if err := repo.Create(host); err != nil {
		t.Fatalf("repo.Create returned %v", err)
	}

	// Now hit the probe endpoint. The handler delegates to the
	// Monitor which uses the FakeProber by default; the probe should
	// return an error rather than crash, and the API should map
	// that to a 4xx/5xx.
	rr, _ := doPHRequest(t, r, "POST", "/api/v1/physical-hosts/"+host.ID+"/probe", nil)
	if rr.Code < 400 || rr.Code >= 600 {
		t.Errorf("probe with empty IP: status = %d, want 4xx/5xx", rr.Code)
	}
}
