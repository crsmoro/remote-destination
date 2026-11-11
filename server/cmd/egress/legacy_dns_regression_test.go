package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

type legacyRetryDNS struct {
	calls int
	names []string
}

func (d *legacyRetryDNS) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	d.calls++
	d.names = append(d.names, host)
	if host != "maps.app.goo.gl" {
		return nil, &net.DNSError{Err: "not found", IsNotFound: true}
	}
	if d.calls == 1 {
		return nil, &net.DNSError{Err: "timeout", IsTimeout: true}
	}
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
}

func TestLegacyLiteralRetriesTransientDNS(t *testing.T) {
	dns := &legacyRetryDNS{}
	dialed := ""
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com", nil)
	req.Host = "8.8.8.8:443"
	serveWith(w, req, dns, func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = addr
		return nil, context.DeadlineExceeded
	})
	if dns.calls != 2 || dialed != req.Host || w.Code != http.StatusBadGateway {
		t.Fatalf("calls=%d dialed=%q status=%d", dns.calls, dialed, w.Code)
	}
}

type legacyUniformDNS struct {
	calls  map[string]int
	answer []netip.Addr
	err    error
}

func (d *legacyUniformDNS) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	d.calls[host]++
	return d.answer, d.err
}

func TestLegacyLiteralDNSFailClosed(t *testing.T) {
	public := netip.MustParseAddr("8.8.8.8")
	private := netip.MustParseAddr("10.0.0.1")
	other := netip.MustParseAddr("8.8.4.4")
	cases := []struct {
		name          string
		answer        []netip.Addr
		err           error
		calls, status int
	}{
		{"exhausted", nil, &net.DNSError{Err: "timeout", IsTimeout: true}, 2, 502},
		{"nxdomain", nil, &net.DNSError{Err: "not found", IsNotFound: true}, 1, 403},
		{"private", []netip.Addr{private}, nil, 1, 403},
		{"mixed", []netip.Addr{public, private}, nil, 1, 403},
		{"mismatch", []netip.Addr{other}, nil, 1, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dns := &legacyUniformDNS{calls: map[string]int{}, answer: tc.answer, err: tc.err}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodConnect, "https://example.com", nil)
			req.Host = "8.8.8.8:443"
			serveWith(w, req, dns, func(context.Context, string, string) (net.Conn, error) { t.Fatal("unexpected dial"); return nil, nil })
			if w.Code != tc.status || len(dns.calls) != len(allowedHosts) {
				t.Fatalf("status=%d calls=%v", w.Code, dns.calls)
			}
			for _, host := range allowedHosts {
				if dns.calls[host] != tc.calls {
					t.Errorf("host=%s calls=%d want=%d", host, dns.calls[host], tc.calls)
				}
			}
		})
	}
}

type legacyMixedDNS struct{ calls map[string]int }

func (d *legacyMixedDNS) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	d.calls[host]++
	if host == "maps.app.goo.gl" {
		return nil, &net.DNSError{Err: "timeout", IsTimeout: true}
	}
	if host == "geocode.googleapis.com" {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	return nil, &net.DNSError{Err: "not found", IsNotFound: true}
}
func TestLegacyLiteralOtherAllowedMappingAfterDNSFailure(t *testing.T) {
	dns := &legacyMixedDNS{calls: map[string]int{}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com", nil)
	req.Host = "8.8.8.8:443"
	dialed := ""
	serveWith(w, req, dns, func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = addr
		return nil, context.DeadlineExceeded
	})
	if dialed != req.Host || dns.calls["maps.app.goo.gl"] != 2 || dns.calls["geocode.googleapis.com"] != 1 {
		t.Fatalf("dialed=%q calls=%v", dialed, dns.calls)
	}
}

func TestLegacyLiteralDNSDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodConnect, "https://example.com", nil).WithContext(ctx)
	req.Host = "8.8.8.8:443"
	w := httptest.NewRecorder()
	started := time.Now()
	serveWith(w, req, blockingDNS{}, func(context.Context, string, string) (net.Conn, error) { t.Fatal("unexpected dial"); return nil, nil })
	if w.Code != http.StatusBadGateway || time.Since(started) > time.Second {
		t.Fatalf("status=%d elapsed=%v", w.Code, time.Since(started))
	}
}
