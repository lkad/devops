package prober

import (
	"strings"
	"testing"
)

// fixtures used by every parser test. Kept in this file (not a
// testdata dir) because they are short and need to be readable
// alongside the assertions.
const fixtureUptimeOutput = "up 10 days, 3 hours, 42 minutes"
const fixtureUptimeSeconds = "943320.00 1234567.89"

const fixtureFreeOutput = `              total        used        free      shared  buff/cache   available
Mem:          16384        8192        4096         256        4096        7584
Swap:          2048         512        1536
`

const fixtureDFOutput = `Filesystem     1G-blocks  Used Available Use% Mounted on
/dev/sda1             100G    50G        50G  50% /
/dev/sdb1             500G   200G       300G  40% /data
tmpfs                  10G     0G        10G   0% /tmp
`

const fixtureNprocOutput = "8\n"

const fixtureTopCPUOutput = `top - 10:00:00 up 10 days,  3:42,  1 user,  load average: 0.05, 0.10, 0.15
Tasks: 100 total,   1 running,  99 sleeping
%Cpu(s): 12.5 us,  3.2 sy,  0.0 ni, 84.3 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
MiB Mem :  16384.0 total,   8192.0 free,   4096.0 used,   4096.0 buff/cache
`

// TestParseUptime pins the uptime parser. The spec says uptime
// returns "value (seconds) and formatted string". The test
// covers both pieces from the two real /proc and `uptime -p`
// outputs.
func TestParseUptime(t *testing.T) {
	got := ParseUptime(fixtureUptimeOutput, fixtureUptimeSeconds)
	if !strings.HasPrefix(got.Formatted, "up 10 days") {
		t.Errorf("Formatted = %q, want prefix %q", got.Formatted, "up 10 days")
	}
	if got.Seconds < 943300 || got.Seconds > 943400 {
		t.Errorf("Seconds = %d, want ~943320", got.Seconds)
	}
}

// TestParseUptime_TrimsWhitespace covers the "real shell echoes
// include a trailing newline" case the parser must tolerate.
func TestParseUptime_TrimsWhitespace(t *testing.T) {
	got := ParseUptime("  up 1 hour  \n\n", "3600.00 0.00\n")
	if got.Seconds != 3600 {
		t.Errorf("Seconds = %d, want 3600", got.Seconds)
	}
}

// TestParseMemory covers the free parser. Total/used in MiB,
// usagePercent computed.
func TestParseMemory(t *testing.T) {
	got := ParseMemory(fixtureFreeOutput)
	if got.TotalMiB != 16384 {
		t.Errorf("TotalMiB = %d, want 16384", got.TotalMiB)
	}
	if got.UsedMiB != 8192 {
		t.Errorf("UsedMiB = %d, want 8192", got.UsedMiB)
	}
	// (8192 / 16384) * 100 = 50.0
	if got.UsagePercent < 49.9 || got.UsagePercent > 50.1 {
		t.Errorf("UsagePercent = %f, want ~50.0", got.UsagePercent)
	}
}

// TestParseMemory_Malformed returns zero values + a parser
// signal; we do not want metrics collection to crash on a
// weird `free` output (some BusyBox versions omit the header).
func TestParseMemory_Malformed(t *testing.T) {
	got := ParseMemory("not free output at all")
	if got.TotalMiB != 0 || got.UsedMiB != 0 {
		t.Errorf("malformed input should yield zero, got %+v", got)
	}
}

// TestParseDisk covers df. The header row is skipped, the
// tmpfs row is still parsed (per spec, all block devices
// are returned).
func TestParseDisk(t *testing.T) {
	got := ParseDisk(fixtureDFOutput)
	if len(got.Disks) != 3 {
		t.Fatalf("len(Disks) = %d, want 3 (sda1, sdb1, tmpfs); got %+v", len(got.Disks), got.Disks)
	}
	if got.Disks[0].Mount != "/" {
		t.Errorf("Disks[0].Mount = %q, want /", got.Disks[0].Mount)
	}
	if got.Disks[0].SizeGB != 100 || got.Disks[0].UsedGB != 50 {
		t.Errorf("Disks[0] size/used = %d/%d, want 100/50", got.Disks[0].SizeGB, got.Disks[0].UsedGB)
	}
	if got.Disks[1].Mount != "/data" || got.Disks[1].SizeGB != 500 {
		t.Errorf("Disks[1] = %+v, want mount=/data size=500", got.Disks[1])
	}
}

// TestParseCPU covers both pieces: cores (from nproc) and
// usage% (from top -bn1). The spec lists "使用率、核心数".
func TestParseCPU(t *testing.T) {
	got := ParseCPU(fixtureNprocOutput, fixtureTopCPUOutput)
	if got.Cores != 8 {
		t.Errorf("Cores = %d, want 8", got.Cores)
	}
	// top shows "12.5 us" + "3.2 sy" — usage = user+sys = 15.7
	if got.UsagePercent < 15.5 || got.UsagePercent > 16.0 {
		t.Errorf("UsagePercent = %f, want ~15.7", got.UsagePercent)
	}
}

// TestParseCPU_NoTopRow ensures we tolerate `top` variants that
// don't include the %Cpu(s) line (some Docker images run a
// stripped busybox). In that case usage% falls back to 0 and
// the host is still considered "fresh" — the operator can
// interpret the missing value.
func TestParseCPU_NoTopRow(t *testing.T) {
	got := ParseCPU("4\n", "no top header here")
	if got.Cores != 4 {
		t.Errorf("Cores = %d, want 4", got.Cores)
	}
	if got.UsagePercent != 0 {
		t.Errorf("UsagePercent = %f, want 0 (no data)", got.UsagePercent)
	}
}
