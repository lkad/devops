package physicalhost

import (
	"context"
	"sync"
	"time"
)

// Host is the input to the Prober. It is a value type (no
// pointers) so the prober cannot mutate the persisted record
// during a probe. Fields are limited to what the prober needs;
// everything else (state, maintenance columns, etc.) lives on
// the PhysicalHost row and is not exposed here.
type Host struct {
	IPAddress string
	SSHPort   int
	SSHUser   string
}

// PingResult is the structured outcome of a reachability probe.
// Reachable=true means the host answered the TCP/ICMP probe;
// LatencyMs is the round-trip time in milliseconds. The Err
// field is populated on connection refused / timeout so the
// monitor can branch on error vs. clean negative.
type PingResult struct {
	Reachable bool
	LatencyMs int64
	Err       error
}

// Prober is the seam the monitor talks to. The interface is
// deliberately small so the Fake implementation is trivial and
// the real SSH/TCP client (a future phase) does not have to be
// mocked at a method-by-method level.
//
// All methods take a context so the monitor can apply a per-host
// timeout without exposing it through the interface.
type Prober interface {
	// Ping returns whether the host is reachable. Implementations
	// are expected to honour ctx.Deadline().
	Ping(ctx context.Context, host Host) (PingResult, error)

	// SSHExec runs a single command on the host and returns its
	// stdout. Implementations are expected to honour ctx.Deadline().
	SSHExec(ctx context.Context, host Host, cmd string) ([]byte, error)
}

// Fake is the in-test Prober. It does not open any sockets, run
// any commands, or depend on the network. Tests script the
// behaviour with ScriptPing / ScriptSSHExec; the default
// behaviour is "ping succeeds, ssh returns empty stdout".
//
// Fake is safe for concurrent use; the test surface is single-
// threaded in practice, but the mutexes are cheap and protect
// future parallel test runs.
type Fake struct {
	mu sync.Mutex

	// scripted map: host -> PingResult. A scripted result takes
	// precedence over the default (Reachable=true).
	pingScripted map[string]PingResult

	// sshScripted map: host|cmd -> (stdout, err).
	sshScripted map[string]sshScriptedResult

	// counters: keyed by host (ping) or host|cmd (ssh).
	pingCounts  map[string]int
	sshCounts   map[string]int
}

type sshScriptedResult struct {
	Out []byte
	Err error
}

// NewFake builds a Fake whose default behaviour is "ping
// succeeds, ssh returns empty stdout". Tests can override
// individual (host, cmd) pairs with ScriptPing / ScriptSSHExec.
func NewFake() *Fake {
	return &Fake{
		pingScripted: map[string]PingResult{},
		sshScripted:  map[string]sshScriptedResult{},
		pingCounts:   map[string]int{},
		sshCounts:    map[string]int{},
	}
}

// ScriptPing pre-programs a Ping result for one IP. Subsequent
// Ping calls for that IP return the scripted result verbatim
// (including Err).
func (f *Fake) ScriptPing(ip string, res PingResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pingScripted[ip] = res
}

// ScriptSSHExec pre-programs an SSH exec result for one
// (host, cmd) pair.
func (f *Fake) ScriptSSHExec(ip, cmd string, out []byte, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sshScripted[ip+"\x00"+cmd] = sshScriptedResult{Out: out, Err: err}
}

// PingCount returns the number of Ping calls made for the given
// IP. Used by tests to assert that the monitor did the work.
func (f *Fake) PingCount(ip string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pingCounts[ip]
}

// SSHExecCount returns the number of SSHExec calls made for the
// given (ip, cmd) pair.
func (f *Fake) SSHExecCount(ip, cmd string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sshCounts[ip+"\x00"+cmd]
}

// Ping implements Prober. If the IP is scripted, the scripted
// result wins; otherwise the default is "reachable, 1ms".
func (f *Fake) Ping(ctx context.Context, host Host) (PingResult, error) {
	f.mu.Lock()
	f.pingCounts[host.IPAddress]++
	scripted, ok := f.pingScripted[host.IPAddress]
	f.mu.Unlock()
	if ok {
		return scripted, scripted.Err
	}
	return PingResult{Reachable: true, LatencyMs: 1}, nil
}

// SSHExec implements Prober. If (ip, cmd) is scripted, the
// scripted result wins; otherwise the default is empty stdout.
func (f *Fake) SSHExec(ctx context.Context, host Host, cmd string) ([]byte, error) {
	f.mu.Lock()
	f.sshCounts[host.IPAddress+"\x00"+cmd]++
	scripted, ok := f.sshScripted[host.IPAddress+"\x00"+cmd]
	f.mu.Unlock()
	if ok {
		return scripted.Out, scripted.Err
	}
	return []byte{}, nil
}

// Compile-time check that *Fake implements Prober.
var _ Prober = (*Fake)(nil)

// _ = time.Now is referenced so the import survives even if a
// future refactor removes every direct usage of time.Time below.
var _ = time.Now
