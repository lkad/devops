package logstream

import (
	"testing"
)

// TestBackpressureSink_DropsOldestWhenFull: the backpressure
// policy MUST drop the OLDEST line when the consumer falls
// behind. This matches the spec scenario "Stream message
// format" that requires the client to see a "dropped" count
// on the next frame.
func TestBackpressureSink_DropsOldestWhenFull(t *testing.T) {
	b := NewBackpressureSink(2)
	b.Push(LogLine{Line: "a", Stream: StreamStdout})
	b.Push(LogLine{Line: "b", Stream: StreamStdout})
	b.Push(LogLine{Line: "c", Stream: StreamStdout})
	if b.Snapshot() != 1 {
		t.Errorf("dropped = %d, want 1", b.Snapshot())
	}
	// Drain: the two remaining lines should be "b" and "c"
	// in order.
	if line, ok := b.Pop(); !ok || line.Line != "b" {
		t.Errorf("first pop: %+v ok=%v, want b", line, ok)
	}
	if line, ok := b.Pop(); !ok || line.Line != "c" {
		t.Errorf("second pop: %+v ok=%v, want c", line, ok)
	}
	if _, ok := b.Pop(); ok {
		t.Error("third pop should be empty")
	}
}

// TestBackpressureSink_DefaultMaxLines: a sink built with
// MaxLines <= 0 falls back to DefaultMaxBackpressureLines
// (1000) so a misconfigured caller doesn't get a 0-sized
// buffer that drops everything.
func TestBackpressureSink_DefaultMaxLines(t *testing.T) {
	b := NewBackpressureSink(0)
	if b.MaxLines != DefaultMaxBackpressureLines {
		t.Errorf("MaxLines = %d, want %d", b.MaxLines, DefaultMaxBackpressureLines)
	}
}

// TestBackpressureSink_Reset: a session-scoped counter MUST
// be re-zeroable so two back-to-back stream sessions don't
// carry state across each other.
func TestBackpressureSink_Reset(t *testing.T) {
	b := NewBackpressureSink(1)
	b.Push(LogLine{Line: "a", Stream: StreamStdout})
	b.Push(LogLine{Line: "b", Stream: StreamStdout})
	if b.Snapshot() != 1 {
		t.Errorf("pre-reset dropped = %d, want 1", b.Snapshot())
	}
	b.Reset()
	if b.Snapshot() != 0 {
		t.Errorf("post-reset dropped = %d, want 0", b.Snapshot())
	}
}
