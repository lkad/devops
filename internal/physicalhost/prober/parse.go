// Parse helpers. The SSH prober runs four commands on the host
// and hands the raw stdout to these functions; they are pure and
// have no I/O, so they are exercised as unit tests with fixture
// strings. The "real" shape of the metrics struct lives in the
// physicalhost package; this file only returns plain Go types so
// the prober has no GORM / model dependency.
package prober

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// UptimeMetrics is the parsed form of `uptime -p` plus the first
// field of /proc/uptime. Seconds is the canonical numeric value;
// Formatted is the human-readable string from uptime(1).
type UptimeMetrics struct {
	Seconds   int64  `json:"seconds"`
	Formatted string `json:"formatted"`
}

// ParseUptime extracts the uptime value. uptimeOutput is the
// stdout of `uptime -p` ("up 10 days, ...") and procUptime is
// the first whitespace-separated field of /proc/uptime. Either
// may be malformed; the function returns whatever it can.
func ParseUptime(uptimeOutput, procUptime string) UptimeMetrics {
	out := UptimeMetrics{Formatted: strings.TrimSpace(uptimeOutput)}
	if fields := strings.Fields(procUptime); len(fields) > 0 {
		if f, err := strconv.ParseFloat(fields[0], 64); err == nil {
			out.Seconds = int64(f)
		}
	}
	return out
}

// MemoryMetrics is the parsed form of `free -m`. All values are
// MiB; UsagePercent is computed.
type MemoryMetrics struct {
	TotalMiB     int64   `json:"total_mib"`
	UsedMiB      int64   `json:"used_mib"`
	UsagePercent float64 `json:"usage_percent"`
}

var memLineRe = regexp.MustCompile(`^Mem:\s+(\d+)\s+(\d+)\s+\d+\s+\d+\s+\d+\s+\d+`)

// ParseMemory scans `free -m` output for the "Mem:" line and
// extracts total/used. Malformed input yields zero values.
func ParseMemory(freeOutput string) MemoryMetrics {
	scanner := bufio.NewScanner(strings.NewReader(freeOutput))
	for scanner.Scan() {
		m := memLineRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		total, _ := strconv.ParseInt(m[1], 10, 64)
		used, _ := strconv.ParseInt(m[2], 10, 64)
		out := MemoryMetrics{TotalMiB: total, UsedMiB: used}
		if total > 0 {
			out.UsagePercent = float64(used) / float64(total) * 100.0
		}
		return out
	}
	return MemoryMetrics{}
}

// DiskMetrics is one row of `df -BG`. All values are GB.
type DiskMetrics struct {
	Mount        string  `json:"mount"`
	Filesystem   string  `json:"filesystem"`
	SizeGB       int64   `json:"size_gb"`
	UsedGB       int64   `json:"used_gb"`
	UsagePercent float64 `json:"usage_percent"`
}

// DiskResult is the parsed result for the whole df output.
type DiskResult struct {
	Disks []DiskMetrics `json:"disks"`
}

// ParseDisk scans df -BG output. Header row is skipped; any
// row that doesn't have at least 6 columns is silently dropped.
func ParseDisk(dfOutput string) DiskResult {
	var out DiskResult
	scanner := bufio.NewScanner(strings.NewReader(dfOutput))
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			// Skip the "Filesystem ... Mounted on" header.
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// Layout: Filesystem 1G-blocks Used Available Use% Mounted on
		filesystem := fields[0]
		sizeGB, _ := strconv.ParseInt(strings.TrimSuffix(fields[1], "G"), 10, 64)
		usedGB, _ := strconv.ParseInt(strings.TrimSuffix(fields[2], "G"), 10, 64)
		usePctStr := strings.TrimSuffix(fields[4], "%")
		usePct, _ := strconv.ParseFloat(usePctStr, 64)
		mount := fields[5]
		out.Disks = append(out.Disks, DiskMetrics{
			Filesystem:   filesystem,
			Mount:        mount,
			SizeGB:       sizeGB,
			UsedGB:       usedGB,
			UsagePercent: usePct,
		})
	}
	return out
}

// CPUMetrics is the parsed form of `nproc` + `top -bn1`.
type CPUMetrics struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usage_percent"`
}

// topCPURe captures the "12.5 us,  3.2 sy,  ..." line from top.
// It is forgiving about whitespace because the column count
// varies between coreutils versions.
var topCPURe = regexp.MustCompile(`([\d.]+)\s*us,\s*([\d.]+)\s*sy`)

// ParseCPU extracts cores (from nproc) and usage% (from top).
// The usage is user+system; iowait/steal/nice are excluded
// because they don't represent CPU pressure from a workload.
func ParseCPU(nprocOutput, topOutput string) CPUMetrics {
	out := CPUMetrics{}
	if c, err := strconv.Atoi(strings.TrimSpace(nprocOutput)); err == nil {
		out.Cores = c
	}
	if m := topCPURe.FindStringSubmatch(topOutput); m != nil {
		us, _ := strconv.ParseFloat(m[1], 64)
		sy, _ := strconv.ParseFloat(m[2], 64)
		out.UsagePercent = us + sy
	}
	return out
}
