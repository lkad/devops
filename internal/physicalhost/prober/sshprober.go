package prober

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHProberConfig is the constructor input for NewSSHProber. The
// only required dependency is the client Signer; Timeout is a
// per-command ceiling that defaults to 5s.
//
// Password-based auth is intentionally NOT supported: the
// physicalhost table stores an SSH user but no credential, and
// the production deployment model is "key in vault, signer
// materialised at boot". Tests construct a fresh keypair with
// crypto/ed25519 and pass the signer in.
type SSHProberConfig struct {
	Signer  ssh.Signer
	Timeout time.Duration
}

// SSHProber is the production Prober for the physical-host
// monitor. It satisfies Prober (via Ping and SSHExec) and
// opens a fresh SSH connection per call. The monitor's state
// machine only needs a positive/negative answer for Ping, so we
// do NOT pool connections here — pooling belongs to a separate
// optimisation phase and is out of scope for the spec.
type SSHProber struct {
	signer  ssh.Signer
	timeout time.Duration
}

// NewSSHProber builds an SSHProber. A nil Signer is a programming
// error; we return one but every call will fail.
func NewSSHProber(cfg SSHProberConfig) *SSHProber {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SSHProber{signer: cfg.Signer, timeout: timeout}
}

// clientConfig builds the per-call *ssh.ClientConfig. The
// signer is the one supplied at construction; we only ever
// authenticate with the public key the server whitelists.
func (p *SSHProber) clientConfig(user string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(p.signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Tier 3 hardening: known_hosts
		Timeout:         p.timeout,
	}
}

// Ping implements Prober. The check is "can we open a TCP
// connection to host:port within the timeout?". A bare TCP dial
// is the cheapest reliable signal for "is the daemon up"; the
// real SSH handshake is exercised by SSHExec.
func (p *SSHProber) Ping(ctx context.Context, host Host) (PingResult, error) {
	dialer := &net.Dialer{Timeout: p.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host.IPAddress, host.SSHPort))
	if err != nil {
		return PingResult{Reachable: false, Err: err}, nil
	}
	_ = conn.Close()
	return PingResult{Reachable: true}, nil
}

// SSHExec opens a fresh SSH connection, opens a session, runs
// cmd, and returns the combined stdout. stderr is intentionally
// discarded: callers that need it should call SSHExec again
// with a `2>&1` redirection command.
func (p *SSHProber) SSHExec(ctx context.Context, host Host, cmd string) ([]byte, error) {
	if p.signer == nil {
		return nil, errors.New("prober.SSHProber: nil signer")
	}
	cfg := p.clientConfig(host.SSHUser)
	addr := fmt.Sprintf("%s:%d", host.IPAddress, host.SSHPort)
	client, err := dialWithContext(ctx, addr, cfg, p.timeout)
	if err != nil {
		return nil, fmt.Errorf("prober.SSHProber dial %s: %w", addr, err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("prober.SSHProber session: %w", err)
	}
	defer sess.Close()

	// Honour ctx by closing the session early on cancel.
	done := make(chan struct{})
	var out []byte
	var runErr error
	go func() {
		defer close(done)
		out, runErr = sess.Output(cmd) // stdout only, returns combined output
	}()
	select {
	case <-ctx.Done():
		_ = sess.Close()
		<-done
		return nil, ctx.Err()
	case <-done:
		if runErr != nil {
			return out, fmt.Errorf("prober.SSHProber exec: %w", runErr)
		}
		return out, nil
	}
}

// dialWithContext races ssh.Dial against a context. ssh.Dial
// respects cfg.Timeout but not ctx, so we drive the cancel by
// simply dropping the in-flight dial — the goroutine exits
// once Dial returns and its result is read by the cleanup
// goroutine.
func dialWithContext(ctx context.Context, addr string, cfg *ssh.ClientConfig, max time.Duration) (*ssh.Client, error) {
	type result struct {
		c   *ssh.Client
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ssh.Dial("tcp", addr, cfg)
		ch <- result{c, err}
	}()
	select {
	case r := <-ch:
		return r.c, r.err
	case <-ctx.Done():
		// The dial will still return via cfg.Timeout; we let
		// the goroutine finish in the background and discard.
		go func() { <-ch }()
		return nil, ctx.Err()
	case <-time.After(max + 500*time.Millisecond):
		return nil, errors.New("prober.SSHProber: dial exceeded soft cap")
	}
}

// Compile-time check that *SSHProber implements Prober.
var _ Prober = (*SSHProber)(nil)
