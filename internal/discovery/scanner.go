// Package discovery implements the network-discovery subsystem.
// It owns the scan-and-promote flow that turns CIDR ranges into
// unified device rows. The package follows the project layering
// rules:
//
//	handler -> service -> scanner/prober/repository
//
// The Scanner and Prober are interfaces so tests can swap a Fake
// in for the real (ICMP/TCP/SNMP) implementation. Promoting a
// discovered host into a Device is delegated to the device
// package's Repository — this package never duplicates the
// Device model.
package discovery

import (
	"context"
	"net"
	"time"
)

// Host is the smallest addressable unit a scanner returns.
// Hostname may be empty when reverse-DNS is unavailable; the
// service layer treats empty hostnames as "name = IP" when it
// promotes the row.
type Host struct {
	IPAddress string
	Hostname  string
}

// Scanner enumerates the live hosts in a CIDR range. The real
// implementation will use ICMP echo + ARP fallback; the Fake is
// the test seam.
//
// The contract is "fire and forget": the scanner returns the
// hosts it can find, and returns an error if the scan was
// aborted (cancelled context) or the network was unreachable.
// A partial scan with N out of M hosts found is NOT an error —
// the service layer is responsible for filtering.
type Scanner interface {
	Scan(ctx context.Context, cidr string) ([]Host, error)
}

// ScannerConfig is the optional configuration block for the
// real network scanner. The spec's "Scan with timeout"
// scenario mandates a per-host ceiling; the default is 5s
// when Timeout is left zero. Concurrency caps the number of
// in-flight probes (default 64) so a /16 doesn't fan out
// to 65k goroutines.
type ScannerConfig struct {
	Timeout     time.Duration
	Concurrency int
}

// TimeoutOrDefault returns the configured Timeout or 5s.
// A zero value means "use the default"; a negative value
// is treated as zero (so callers can pass env-driven values
// without a nil check).
func (c ScannerConfig) TimeoutOrDefault() time.Duration {
	if c.Timeout <= 0 {
		return 5 * time.Second
	}
	return c.Timeout
}

// ConcurrencyOrDefault returns the configured Concurrency or 64.
func (c ScannerConfig) ConcurrencyOrDefault() int {
	if c.Concurrency <= 0 {
		return 64
	}
	return c.Concurrency
}

// CIDRRange is a small helper used by both the real scanner
// and the service layer. It returns every IP address (inclusive
// of network and broadcast — a /31 or /32 has no broadcast
// concept and is returned as-is).
//
// We do this in pure Go (not via net.ParseCIDR's IPNet) because
// we want to skip the broadcast and network addresses for
// traditional /24 ranges — those addresses are never real
// devices. The helper is exported so the service can pre-warm
// the result set in tests without invoking a real scanner.
func CIDRRange(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	// ipnet.IP is the network address; iterating it walks every
	// address in the range, including the network and broadcast
	// addresses for /24 and larger. We skip those.
	var out []string
	for cur := ip.Mask(ipnet.Mask); ipnet.Contains(cur); incIP(cur) {
		if cur.Equal(ipnet.IP) {
			continue
		}
		if cur.Equal(broadcastIP(ipnet)) {
			continue
		}
		out = append(out, cur.String())
	}
	return out, nil
}

// incIP increments ip in place by one. It treats ip as a big-
// endian integer; the function returns when the address
// overflows the subnet (handled by the caller via ipnet.Contains).
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}

// broadcastIP returns the broadcast address of the given network.
// For a /31 or /32 the broadcast equals the network itself; the
// caller skips both.
func broadcastIP(n *net.IPNet) net.IP {
	ip := make(net.IP, len(n.IP))
	copy(ip, n.IP)
	for i := range ip {
		ip[i] |= ^n.Mask[i]
	}
	return ip
}
