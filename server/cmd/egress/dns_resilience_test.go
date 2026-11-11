package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

type scriptedDNS struct {
	calls   int
	answers [][]netip.Addr
	errs    []error
}

func (s *scriptedDNS) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	i := s.calls
	s.calls++
	if i >= len(s.errs) {
		return nil, &net.DNSError{Err: "not found", IsNotFound: true}
	}
	return s.answers[i], s.errs[i]
}
func TestRelayRetriesTemporaryDNS(t *testing.T) {
	dns := &scriptedDNS{answers: [][]netip.Addr{nil, {netip.MustParseAddr("8.8.8.8")}}, errs: []error{&net.DNSError{Err: "timeout", IsTimeout: true}, nil}}
	dialed := ""
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed = addr
		return nil, context.DeadlineExceeded
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "https://places.googleapis.com:443", nil)
	req.Host = "places.googleapis.com:443"
	serveWith(w, req, dns, dial)
	if dns.calls != 2 || dialed != "8.8.8.8:443" || w.Code != http.StatusBadGateway {
		t.Fatalf("calls=%d dialed=%q status=%d", dns.calls, dialed, w.Code)
	}
}

func TestRelayDNSFailClosed(t *testing.T) {
	timeout := &net.DNSError{Err: "timeout", IsTimeout: true}
	nxdomain := &net.DNSError{Err: "not found", IsNotFound: true}
	public := netip.MustParseAddr("8.8.8.8")
	private := netip.MustParseAddr("10.0.0.1")
	cases := []struct {
		name          string
		answers       [][]netip.Addr
		errs          []error
		calls, status int
	}{
		{"exhausted", [][]netip.Addr{nil, nil}, []error{timeout, timeout}, 2, 502},
		{"nxdomain", [][]netip.Addr{nil}, []error{nxdomain}, 1, 502},
		{"private", [][]netip.Addr{{private}, {public}}, []error{nil, nil}, 1, 403},
		{"mixed", [][]netip.Addr{{public, private}}, []error{nil}, 1, 403},
		{"retry_private", [][]netip.Addr{nil, {private}}, []error{timeout, nil}, 2, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dns := &scriptedDNS{answers: tc.answers, errs: tc.errs}
			dialed := false
			dial := func(context.Context, string, string) (net.Conn, error) {
				dialed = true
				return nil, context.DeadlineExceeded
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodConnect, "https://places.googleapis.com:443", nil)
			req.Host = "places.googleapis.com:443"
			serveWith(w, req, dns, dial)
			if dns.calls != tc.calls || w.Code != tc.status || dialed {
				t.Fatalf("calls=%d status=%d dialed=%v", dns.calls, w.Code, dialed)
			}
		})
	}
}

func TestRelayRevalidatesDNSOnEachRequest(t *testing.T) {
	dns := &scriptedDNS{answers: [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}, {netip.MustParseAddr("127.0.0.1")}}, errs: []error{nil, nil}}
	calls := 0
	dial := func(context.Context, string, string) (net.Conn, error) { calls++; return nil, context.DeadlineExceeded }
	for i, status := range []int{502, 403} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodConnect, "https://places.googleapis.com:443", nil)
		req.Host = "places.googleapis.com:443"
		serveWith(w, req, dns, dial)
		if w.Code != status {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	if calls != 1 || dns.calls != 2 {
		t.Fatalf("dial calls %d DNS calls %d", calls, dns.calls)
	}
}

func TestLookupHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dns := &scriptedDNS{answers: [][]netip.Addr{nil}, errs: []error{context.DeadlineExceeded}}
	_, err := lookup(ctx, dns, "places.googleapis.com")
	if err == nil || dns.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, dns.calls)
	}
}

type blockingDNS struct{}

func (blockingDNS) LookupNetIP(ctx context.Context, _, _ string) ([]netip.Addr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestLookupBoundsBlockedResolver(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := lookup(ctx, blockingDNS{}, "places.googleapis.com")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("err=%v elapsed=%v", err, time.Since(started))
	}
}
func TestRelayRejectsHostAndPortBeforeDNS(t *testing.T) {
	for _, target := range []string{"places.googleapis.com.evil.example:443", "places.googleapis.com:80", "127.0.0.1:443"} {
		dns := &scriptedDNS{}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodConnect, "https://example.com", nil)
		req.Host = target
		serveWith(w, req, dns, func(context.Context, string, string) (net.Conn, error) { t.Fatal("unexpected dial"); return nil, nil })
		if w.Code != 403 || dns.calls != 0 {
			t.Fatalf("%s: status=%d calls=%d", target, w.Code, dns.calls)
		}
	}
}
