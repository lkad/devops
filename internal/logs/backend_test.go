package logs

import (
	"context"
	"testing"
	"time"
)

// fakeBackend is the smallest possible LogBackend impl, used to
// verify interface conformance and that Capabilities() is a pure
// accessor.
type fakeBackend struct {
	caps Capabilities
}

func (f *fakeBackend) Capabilities() Capabilities { return f.caps }
func (f *fakeBackend) Query(_ context.Context, _ Query) (Result, error) {
	return Result{Meta: Meta{Backend: f.caps.BackendName}}, nil
}
func (f *fakeBackend) Streams(_ context.Context) ([]Stream, error) {
	return []Stream{{Name: "fake"}}, nil
}

// TestLogBackend_InterfaceCompile is the compile-time assertion
// that fakeBackend implements LogBackend. A change to the interface
// signature breaks this test before runtime.
func TestLogBackend_InterfaceCompile(t *testing.T) {
	var _ LogBackend = (*fakeBackend)(nil)
}

// TestCapabilities_Fields pins the Capabilities field set. New
// fields are non-breaking; renames or removals would be a breaking
// change and must be deliberate.
func TestCapabilities_Fields(t *testing.T) {
	c := Capabilities{
		SupportsAggregation: true,
		MaxTimeRange:        30 * 24 * time.Hour,
		MaxQueryLength:      5000,
		BackendName:         "loki",
	}
	if !c.SupportsAggregation {
		t.Errorf("SupportsAggregation lost")
	}
	if c.MaxTimeRange != 30*24*time.Hour {
		t.Errorf("MaxTimeRange lost: %v", c.MaxTimeRange)
	}
	if c.MaxQueryLength != 5000 {
		t.Errorf("MaxQueryLength lost: %d", c.MaxQueryLength)
	}
	if c.BackendName != "loki" {
		t.Errorf("BackendName lost: %q", c.BackendName)
	}
}

// TestLogBackend_CapabilitiesIsPure ensures Capabilities() is a
// pure read (no side effects on repeated calls).
func TestLogBackend_CapabilitiesIsPure(t *testing.T) {
	want := Capabilities{BackendName: "local", MaxTimeRange: 7 * 24 * time.Hour}
	fb := &fakeBackend{caps: want}
	for i := 0; i < 3; i++ {
		got := fb.Capabilities()
		if got != want {
			t.Errorf("call %d: got %+v want %+v", i, got, want)
		}
	}
}

// TestLogBackend_StreamsReturns ensures the Streams call returns
// at least an empty slice (not nil panic in handler).
func TestLogBackend_StreamsReturns(t *testing.T) {
	fb := &fakeBackend{caps: Capabilities{BackendName: "local"}}
	streams, err := fb.Streams(context.Background())
	if err != nil {
		t.Fatalf("Streams: %v", err)
	}
	if len(streams) != 1 || streams[0].Name != "fake" {
		t.Errorf("streams: %+v", streams)
	}
}
