package safehttp

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsBlocked(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},      // loopback
		{"::1", true},            // loopback v6
		{"10.0.0.5", true},       // private
		{"192.168.1.10", true},   // private
		{"172.16.0.1", true},     // private
		{"169.254.169.254", true}, // link-local (cloud metadata)
		{"0.0.0.0", true},        // unspecified
		{"8.8.8.8", false},       // public
		{"1.1.1.1", false},       // public
	}
	for _, c := range cases {
		if got := isBlocked(net.ParseIP(c.ip)); got != c.blocked {
			t.Errorf("isBlocked(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

// TestGuardBlocksLoopbackDial proves the client refuses to connect to a real
// loopback server when the guard is on, and connects when allowPrivate is set.
func TestGuardBlocksLoopbackDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// guard ON → loopback dial must fail
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if _, err := NewClient(2*time.Second, false).Do(req); err == nil {
		t.Fatal("expected blocked dial to loopback, got success")
	} else if !strings.Contains(err.Error(), "blocked non-public address") {
		t.Fatalf("expected block error, got: %v", err)
	}

	// guard OFF → same dial must succeed
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := NewClient(2*time.Second, true).Do(req2)
	if err != nil {
		t.Fatalf("expected success with allowPrivate, got: %v", err)
	}
	resp.Body.Close()
}
