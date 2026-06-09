package prober

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshTestServer is an in-process SSH server for tests. Each
// command requested via the "exec" channel is looked up in
// scripted (map of "cmd" -> stdout) and the result is returned
// verbatim. If the cmd is not scripted, the server returns
// "unknown command" as stdout and exit-status 1 — useful for
// asserting error paths.
type sshTestServer struct {
	listener net.Listener
	signer   ssh.Signer
	scripted map[string]string
	calls    map[string]int
	mu       sync.Mutex
	wg       sync.WaitGroup
	stop     chan struct{}
}

// startSSHTestServer spins up an in-process SSH server on a
// random localhost port. The returned host carries a public
// key that the SSH prober will be configured with, plus the
// scripted command table for assertions.
func startSSHTestServer(t *testing.T) (*sshTestServer, string, ssh.Signer) {
	t.Helper()

	// 1. Generate the server host key (the one the server
	//    presents to clients during handshake).
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	// 2. Generate the client key the prober will authenticate
	//    with. The server whitelists only this public key.
	clientPub, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("client key: %v", err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatalf("client signer: %v", err)
	}
	allowedPubKey := clientSigner.PublicKey()
	_ = clientPub

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if keysEqual(k, allowedPubKey) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unknown key")
		},
	}
	cfg.AddHostKey(hostSigner)

	// 3. Listen on a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &sshTestServer{
		listener: ln,
		signer:   hostSigner,
		scripted: map[string]string{},
		calls:    map[string]int{},
		stop:     make(chan struct{}),
	}
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			srv.wg.Add(1)
			go func(c net.Conn) {
				defer srv.wg.Done()
				srv.handleConn(c, cfg)
			}(conn)
		}
	}()

	addr := ln.Addr().String()
	t.Cleanup(func() {
		close(srv.stop)
		_ = ln.Close()
		srv.wg.Wait()
	})
	return srv, addr, clientSigner
}

func (s *sshTestServer) handleConn(c net.Conn, cfg *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "only session")
			continue
		}
		ch, requests, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(ch, requests)
	}
}

func (s *sshTestServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "exec":
			// Payload format: uint32 len, then bytes of command.
			if len(req.Payload) < 4 {
				_ = req.Reply(false, nil)
				continue
			}
			cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
			if uint32(len(req.Payload)) < 4+cmdLen {
				_ = req.Reply(false, nil)
				continue
			}
			cmd := string(req.Payload[4 : 4+cmdLen])
			s.mu.Lock()
			s.calls[cmd]++
			out, ok := s.scripted[cmd]
			s.mu.Unlock()
			_ = req.Reply(true, nil)
			if !ok {
				out = "unknown command"
			}
			_, _ = io.WriteString(ch, out)
			// Send exit-status 0 for known commands, 1 for
			// unknown. Clients don't have to act on it.
			status := uint32(0)
			if !ok {
				status = 1
			}
			payload := make([]byte, 4)
			binary.BigEndian.PutUint32(payload, status)
			_, _ = ch.SendRequest("exit-status", false, payload)
			return
		case "shell", "pty-req":
			_ = req.Reply(false, nil)
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func (s *sshTestServer) Script(cmd, out string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripted[cmd] = out
}

func (s *sshTestServer) Calls(cmd string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[cmd]
}

func keysEqual(a, b ssh.PublicKey) bool {
	return strings.EqualFold(string(ssh.MarshalAuthorizedKey(a)), string(ssh.MarshalAuthorizedKey(b)))
}

// TestSSHProber_SSHExec_HappyPath is the integration test for
// the SSH implementation. It stands up a local SSH server,
// configures the prober with the matching client key, and
// asserts that SSHExec returns the scripted stdout.
func TestSSHProber_SSHExec_HappyPath(t *testing.T) {
	srv, addr, clientSigner := startSSHTestServer(t)
	srv.Script("echo hello", "hello\n")

	host, port := splitHostPort(t, addr)
	prober := NewSSHProber(SSHProberConfig{
		Signer:  clientSigner,
		Timeout: 5 * time.Second,
	})

	out, err := prober.SSHExec(context.Background(), Host{IPAddress: host, SSHPort: port, SSHUser: "root"}, "echo hello")
	if err != nil {
		t.Fatalf("SSHExec: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("out = %q, want %q", out, "hello\n")
	}
	if srv.Calls("echo hello") != 1 {
		t.Errorf("server saw %d calls, want 1", srv.Calls("echo hello"))
	}
}

// TestSSHProber_SSHExec_BadKey rejects a prober that does not
// present the server's whitelisted key.
func TestSSHProber_SSHExec_BadKey(t *testing.T) {
	_, addr, _ := startSSHTestServer(t)
	// Generate a different client key — server will reject.
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	otherSigner, _ := ssh.NewSignerFromKey(otherPriv)

	host, port := splitHostPort(t, addr)
	prober := NewSSHProber(SSHProberConfig{
		Signer:  otherSigner,
		Timeout: 2 * time.Second,
	})
	_, err := prober.SSHExec(context.Background(), Host{IPAddress: host, SSHPort: port, SSHUser: "root"}, "echo")
	if err == nil {
		t.Fatal("expected auth failure, got nil")
	}
	if !strings.Contains(err.Error(), "ssh") {
		t.Errorf("err = %v, want ssh-related", err)
	}
}

// TestSSHProber_Ping_LiveSSHSucceeds pins the "ping" path: a
// live SSH server should report Reachable=true via the
// TCP-equivalent reachability check (the prober first tries a
// quick SSH banner grab; we test the SSH-up branch here).
func TestSSHProber_Ping_LiveSSHSucceeds(t *testing.T) {
	_, addr, _ := startSSHTestServer(t)
	host, port := splitHostPort(t, addr)
	prober := NewSSHProber(SSHProberConfig{Timeout: 2 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := prober.Ping(ctx, Host{IPAddress: host, SSHPort: port, SSHUser: "root"})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if !res.Reachable {
		t.Errorf("Reachable = false, want true; err = %v", res.Err)
	}
}

// TestSSHProber_Ping_ClosedPort confirms the negative path:
// a closed port returns Reachable=false (within the timeout).
func TestSSHProber_Ping_ClosedPort(t *testing.T) {
	// Bind a port and immediately close it so we get a free
	// port nobody is listening on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	host, port := splitHostPort(t, addr)
	prober := NewSSHProber(SSHProberConfig{Timeout: 500 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := prober.Ping(ctx, Host{IPAddress: host, SSHPort: port, SSHUser: "root"})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if res.Reachable {
		t.Errorf("Reachable = true on closed port, want false")
	}
}

// splitHostPort breaks "127.0.0.1:34567" into ("127.0.0.1", 34567).
func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	port := 0
	for _, c := range portStr {
		if c < '0' || c > '9' {
			t.Fatalf("non-numeric port: %s", portStr)
		}
		port = port*10 + int(c-'0')
	}
	return host, port
}
