package server_test

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devops-toolkit/backend/internal/server"
)

// TestGenerateSelfSignedCert_LoadsInTLSConfig is the
// round-trip: generate certs, load them via LoadServerTLS,
// start a tls httptest.Server, connect with a client that
// trusts the same CA AND presents a matching client cert,
// expect a clean handshake.
func TestGenerateSelfSignedCert_LoadsInTLSConfig(t *testing.T) {
	dir := t.TempDir()
	cert, key, ca, clientCert, clientKey, err := server.GenerateSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	for _, p := range []string{cert, key, ca, clientCert, clientKey} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("cert file %s missing: %v", p, err)
		}
	}
	tc, err := server.LoadServerTLS(server.TLSConfig{CertFile: cert, KeyFile: key, CAFile: ca})
	if err != nil {
		t.Fatalf("LoadServerTLS: %v", err)
	}
	if tc.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("ClientAuth = %v, want RequireAndVerifyClientCert", tc.ClientAuth)
	}

	// Stand up an HTTPS server that demands mTLS and a route
	// handler. The httptest server can serve our *tls.Config
	// directly via NewUnstartedServer + Listener.
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mTLS works"))
	}))
	https.TLS = tc
	https.StartTLS()
	defer https.Close()

	// Build a client that trusts the same CA AND presents the
	// matching client cert. Without the client cert, the
	// server's RequireAndVerifyClientCert kicks in and the
	// handshake fails (which is what we want in prod).
	caPEM, err := os.ReadFile(ca)
	if err != nil {
		t.Fatalf("read ca: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatal("failed to parse CA")
	}
	clientTLSCert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("load client cert: %v", err)
	}
	cli := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientTLSCert},
	}}}
	req, _ := http.NewRequest("GET", https.URL, nil)
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("https client: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// TestLoadServerTLS_NoClientCA covers the dev tier: the
// server starts with mTLS off (no ClientCAs). The Load call
// must succeed.
func TestLoadServerTLS_NoClientCA(t *testing.T) {
	dir := t.TempDir()
	cert, key, _, _, _, err := server.GenerateSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	tc, err := server.LoadServerTLS(server.TLSConfig{CertFile: cert, KeyFile: key})
	if err != nil {
		t.Fatalf("LoadServerTLS: %v", err)
	}
	if tc.ClientAuth != tls.NoClientCert {
		t.Errorf("ClientAuth = %v, want NoClientCert (dev mode)", tc.ClientAuth)
	}
}

// TestMTLS_RejectsClientWithoutCert: a client that does NOT
// present a cert must be rejected when the server requires
// one. The TLS handshake itself fails before any HTTP
// response, so Do() returns an error.
func TestMTLS_RejectsClientWithoutCert(t *testing.T) {
	dir := t.TempDir()
	cert, key, ca, _, _, err := server.GenerateSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	tc, err := server.LoadServerTLS(server.TLSConfig{CertFile: cert, KeyFile: key, CAFile: ca})
	if err != nil {
		t.Fatalf("LoadServerTLS: %v", err)
	}
	https := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	https.TLS = tc
	https.StartTLS()
	defer https.Close()

	caPEM, err := os.ReadFile(ca)
	if err != nil {
		t.Fatalf("read ca: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	cli := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	_, err = cli.Get(https.URL)
	if err == nil {
		t.Fatal("expected handshake failure when client presents no cert")
	}
}
