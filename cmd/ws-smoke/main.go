// ws-smoke: minimal WebSocket client that connects, subscribes
// to physical_host.state_change, and prints what arrives.
// Used in scripts/verify/verify-flow.sh to assert the
// monitor's device_event end-to-end.
//
// Usage:
//	go run ./cmd/ws-smoke -url ws://localhost:3000/api/v1/ws?token=$TOK -timeout 5s
//	go run ./cmd/ws-smoke -url wss://... -ca ./ca.crt -cert ./client.crt -key ./client.key
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	urlStr := flag.String("url", "", "ws URL (required)")
	timeout := flag.Duration("timeout", 5*time.Second, "max wait time")
	caFile := flag.String("ca", "", "CA bundle file (mTLS)")
	certFile := flag.String("cert", "", "client cert file (mTLS)")
	keyFile := flag.String("key", "", "client key file (mTLS)")
	flag.Parse()
	if *urlStr == "" {
		fmt.Fprintln(os.Stderr, "usage: ws-smoke -url ws://...")
		os.Exit(2)
	}
	u, err := url.Parse(*urlStr)
	if err != nil {
		log.Fatal(err)
	}
	dialer := websocket.DefaultDialer
	if u.Scheme == "wss" {
		tc := &tls.Config{}
		if *caFile != "" {
			pem, err := os.ReadFile(*caFile)
			if err != nil {
				log.Fatalf("read ca: %v", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				log.Fatalf("parse ca: invalid pem")
			}
			tc.RootCAs = pool
		}
		if *certFile != "" && *keyFile != "" {
			cli, err := tls.LoadX509KeyPair(*certFile, *keyFile)
			if err != nil {
				log.Fatalf("load client cert: %v", err)
			}
			tc.Certificates = []tls.Certificate{cli}
		}
		dialer.TLSClientConfig = tc
	}
	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// Subscribe to the device_event channel. The hub's protocol
	// uses {"action":"subscribe","channel":"..."}.
	if err := c.WriteJSON(map[string]string{"action": "subscribe", "channel": "physical_host.state_change"}); err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	fmt.Println("[ws-smoke] subscribed to physical_host.state_change")

	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		_ = c.SetReadDeadline(deadline)
		_, msg, err := c.ReadMessage()
		if err != nil {
			fmt.Printf("[ws-smoke] read done: %v\n", err)
			return
		}
		fmt.Printf("[ws-smoke] %s\n", string(msg))
	}
}
