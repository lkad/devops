package physicalhost

import (
	"context"
	"sync"

	"github.com/devops-toolkit/backend/internal/physicalhost/prober"
)

// Prober is the seam the monitor talks to. Re-exported from the
// prober subpackage so service-layer code can keep using
// physicalhost.Prober without a transitive import. The concrete
// implementations (TCP, SSH, Fake) all live in the prober
// subpackage; this file is the in-package test fake.
type Prober = prober.Prober

// Host is the value the prober accepts. Re-exported for the
// same reason as Prober.
type Host = prober.Host

// PingResult is the structured outcome of a reachability probe.
type PingResult = prober.PingResult

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
	pingCounts map[string]int
	sshCounts  map[string]int
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
