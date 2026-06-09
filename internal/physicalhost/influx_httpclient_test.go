package physicalhost

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPClientPost_HappyPath confirms a 2xx response is
// considered a success (no error returned to the writer).
func TestHTTPClientPost_HappyPath(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewHTTPClientPost(2 * 1_000_000_000) // 2s, avoid touching time import
	if err := c.Post(srv.URL, "my-token", []byte("m,v=1")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if captured != "Token my-token" {
		t.Errorf("auth header = %q, want %q", captured, "Token my-token")
	}
}

// TestHTTPClientPost_Non2xxIsError: InfluxDB returns 400 for
// malformed line protocol. The client must surface that so the
// async writer can drop + log.
func TestHTTPClientPost_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid: missing measurement"))
	}))
	defer srv.Close()

	c := NewHTTPClientPost(0)
	err := c.Post(srv.URL, "tok", []byte("bad"))
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("err = %v, want 400 in message", err)
	}
}

// TestHTTPClientPost_UnreachableServer: a closed server returns
// a transport error (timeout / connection refused), not nil.
func TestHTTPClientPost_UnreachableServer(t *testing.T) {
	c := NewHTTPClientPost(0)
	err := c.Post("http://127.0.0.1:1", "tok", []byte("body"))
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
