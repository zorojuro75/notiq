// Package safehttp provides an *http.Client hardened against SSRF.
//
// Outbound webhook URLs are user-controlled, so a naive client could be coerced
// into reaching internal services or the cloud metadata endpoint
// (169.254.169.254). The dialer's Control hook runs AFTER DNS resolution with
// the concrete IP about to be dialed, so it also defends against DNS-rebinding.
package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrBlockedAddress is returned when a connection targets a non-public address.
type ErrBlockedAddress struct{ Addr string }

func (e *ErrBlockedAddress) Error() string {
	return fmt.Sprintf("blocked non-public address: %s", e.Addr)
}

// isBlocked reports whether an IP must not be dialed for outbound webhooks.
func isBlocked(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() || // 127.0.0.0/8, ::1
		ip.IsPrivate() || // RFC1918 + ULA fc00::/7
		ip.IsLinkLocalUnicast() || // 169.254.0.0/16 (incl. cloud metadata), fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() // 0.0.0.0, ::
}

// NewClient returns an http.Client whose dialer refuses to connect to
// loopback, private, link-local, or unspecified addresses, caps redirects,
// and applies the given total timeout.
//
// allowPrivate disables the address guard. It exists ONLY for local
// development/testing against loopback receivers and must never be enabled in
// production — doing so re-opens the SSRF hole the guard closes.
func NewClient(timeout time.Duration, allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if isBlocked(ip) {
				return &ErrBlockedAddress{Addr: address}
			}
			return nil
		},
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			return nil
		},
	}
}
