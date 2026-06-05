package hub

import (
	"testing"
)

// TestDefaultLimits returns a non-zero Limits struct with sensible defaults.
// Per the spec: 30s ping, 10s write, 60s read; max connections, channels, message
// bytes, and publish payload bytes are all positive integers.
func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()

	if l.MaxConnections <= 0 {
		t.Errorf("MaxConnections must be > 0, got %d", l.MaxConnections)
	}
	if l.MaxChannelsPerConn <= 0 {
		t.Errorf("MaxChannelsPerConn must be > 0, got %d", l.MaxChannelsPerConn)
	}
	if l.MaxMessageBytes <= 0 {
		t.Errorf("MaxMessageBytes must be > 0, got %d", l.MaxMessageBytes)
	}
	if l.MaxPayloadBytes <= 0 {
		t.Errorf("MaxPayloadBytes must be > 0, got %d", l.MaxPayloadBytes)
	}
	if l.PingIntervalSec <= 0 {
		t.Errorf("PingIntervalSec must be > 0, got %d", l.PingIntervalSec)
	}
	if l.WriteTimeoutSec <= 0 {
		t.Errorf("WriteTimeoutSec must be > 0, got %d", l.WriteTimeoutSec)
	}
	if l.ReadTimeoutSec <= 0 {
		t.Errorf("ReadTimeoutSec must be > 0, got %d", l.ReadTimeoutSec)
	}
	// Spec mandates these specific values as safe defaults.
	if l.PingIntervalSec != 30 {
		t.Errorf("PingIntervalSec default must be 30, got %d", l.PingIntervalSec)
	}
	if l.WriteTimeoutSec != 10 {
		t.Errorf("WriteTimeoutSec default must be 10, got %d", l.WriteTimeoutSec)
	}
	if l.ReadTimeoutSec != 60 {
		t.Errorf("ReadTimeoutSec default must be 60, got %d", l.ReadTimeoutSec)
	}
}

// TestLimits_Validate applies the validation rules. Negative values and zero
// are rejected for every numeric field; the OutBufferSize is implicitly part
// of Limits via spec note (256 for Send channel).
func TestLimits_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(l *Limits)
		wantErr bool
	}{
		{
			name:   "valid defaults pass",
			mutate: func(l *Limits) {},
		},
		{
			name:    "negative max connections",
			mutate:  func(l *Limits) { l.MaxConnections = -1 },
			wantErr: true,
		},
		{
			name:    "zero max channels per conn",
			mutate:  func(l *Limits) { l.MaxChannelsPerConn = 0 },
			wantErr: true,
		},
		{
			name:    "negative max message bytes",
			mutate:  func(l *Limits) { l.MaxMessageBytes = -5 },
			wantErr: true,
		},
		{
			name:    "negative max payload bytes",
			mutate:  func(l *Limits) { l.MaxPayloadBytes = -1 },
			wantErr: true,
		},
		{
			name:    "zero ping interval",
			mutate:  func(l *Limits) { l.PingIntervalSec = 0 },
			wantErr: true,
		},
		{
			name:    "zero write timeout",
			mutate:  func(l *Limits) { l.WriteTimeoutSec = 0 },
			wantErr: true,
		},
		{
			name:    "zero read timeout",
			mutate:  func(l *Limits) { l.ReadTimeoutSec = 0 },
			wantErr: true,
		},
		{
			name:    "negative out buffer",
			mutate:  func(l *Limits) { l.OutBufferSize = -1 },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := DefaultLimits()
			tt.mutate(&l)
			err := l.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDefaultOutBufferSize is the spec-mandated constant for the per-client
// Send channel buffer.
func TestDefaultOutBufferSize(t *testing.T) {
	if DefaultOutBufferSize != 256 {
		t.Errorf("DefaultOutBufferSize must be 256 per spec, got %d", DefaultOutBufferSize)
	}
}
