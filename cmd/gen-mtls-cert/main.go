// gen-mtls-cert: small CLI that generates a self-signed
// CA + server cert + client cert into a chosen directory.
// Used by the smoke test + dev tier to enable mTLS without
// spinning up a real PKI.
//
// Usage:
//	go run ./cmd/gen-mtls-cert ./deploy/mtls
package main

import (
	"fmt"
	"os"

	"github.com/devops-toolkit/backend/internal/server"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	cert, key, ca, clientCert, clientKey, err := server.GenerateSelfSignedCert(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("TLS_CERT_FILE=" + cert)
	fmt.Println("TLS_KEY_FILE=" + key)
	fmt.Println("TLS_CA_FILE=" + ca)
	fmt.Println("TLS_CLIENT_CERT=" + clientCert)
	fmt.Println("TLS_CLIENT_KEY=" + clientKey)
}
