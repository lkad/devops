// Package server provides TLS / mTLS configuration for the
// devops-toolkit HTTP server. Production deployments must run
// behind mTLS; the certs come from a deployment-managed
// secret store (cert-manager, Vault, etc.) and are mounted
// as files referenced by TLS_CERT_FILE / TLS_KEY_FILE /
// TLS_CA_FILE env vars.
//
// Tests generate self-signed certs in-process via
// crypto/x509 + crypto/rsa so the build is hermetic.
package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// TLSConfig bundles the file paths the production server
// reads at startup. A nil CAFile disables client-cert
// verification (mTLS off) — only safe in dev.
type TLSConfig struct {
	CertFile string // server cert (PEM)
	KeyFile  string // server key (PEM)
	CAFile   string // client CA bundle (PEM); empty = no mTLS
}

// LoadServerTLS builds a *tls.Config the devops-toolkit
// http.Server uses. If caFile is non-empty the server is
// configured for mTLS (ClientCAs + RequireAndVerifyClientCert).
// A missing server cert pair is a fatal error: fail fast on
// misconfiguration rather than start an HTTP-only server.
func LoadServerTLS(cfg TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("server: load cert/key: %w", err)
	}
	tc := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("server: read CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("server: parse CA bundle")
		}
		tc.ClientCAs = pool
		tc.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tc, nil
}

// GenerateSelfSignedCert creates a one-off CA + server cert +
// client cert pair and writes them to tmp. Used by the smoke
// test and the dev tier (config-dev.yaml references these
// files via TLS_CERT_FILE etc).
//
// The certs are valid for 24h, with the server's CN pointing
// at 127.0.0.1 + localhost so a client doing IP-based TLS
// verification passes. The CA signs both server and client
// so the same root can verify mTLS handshakes.
func GenerateSelfSignedCert(dir string) (serverCert, serverKey, caCert, clientCert, clientKey string, err error) {
	// CA
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", "", "", err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "devops-toolkit-test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", "", "", err
	}

	// Server cert
	srvKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", "", "", err
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "devops-toolkit-server"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	caCertTmpl, _ := x509.ParseCertificate(caDER)
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCertTmpl, &srvKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", "", "", err
	}

	// Client cert
	cliKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", "", "", err
	}
	cliTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "devops-toolkit-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	cliDER, err := x509.CreateCertificate(rand.Reader, cliTmpl, caCertTmpl, &cliKey.PublicKey, caKey)
	if err != nil {
		return "", "", "", "", "", err
	}

	// Encode + write. The server cert + key are written as
	// the pair LoadServerTLS expects; the CA + client cert +
	// client key are also written so mTLS test clients can
	// present the matching client cert.
	certFile := dir + "/server.crt"
	keyFile := dir + "/server.key"
	caFile := dir + "/ca.crt"
	clientCertFile := dir + "/client.crt"
	clientKeyFile := dir + "/client.key"
	if err := writePEM(certFile, "CERTIFICATE", srvDER); err != nil {
		return "", "", "", "", "", err
	}
	if err := writePEM(keyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(srvKey)); err != nil {
		return "", "", "", "", "", err
	}
	if err := writePEM(caFile, "CERTIFICATE", caDER); err != nil {
		return "", "", "", "", "", err
	}
	if err := writePEM(clientCertFile, "CERTIFICATE", cliDER); err != nil {
		return "", "", "", "", "", err
	}
	if err := writePEM(clientKeyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(cliKey)); err != nil {
		return "", "", "", "", "", err
	}
	return certFile, keyFile, caFile, clientCertFile, clientKeyFile, nil
}

func writePEM(path, blockType string, der []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
}
