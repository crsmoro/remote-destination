package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestPlacesEgressExactAndPublic(t *testing.T) {
	for _, tc := range []struct {
		host string
		ips  []netip.Addr
		want string
	}{
		{"places.googleapis.com:443", []netip.Addr{netip.MustParseAddr("8.8.4.4")}, "8.8.4.4:443"},
		{"places.googleapis.com:443", []netip.Addr{netip.MustParseAddr("8.8.4.4"), netip.MustParseAddr("127.0.0.1")}, ""},
		{"places.googleapis.com.evil.test:443", []netip.Addr{netip.MustParseAddr("8.8.4.4")}, ""},
		{"photon.komoot.io:443", []netip.Addr{netip.MustParseAddr("8.8.4.4")}, ""},
	} {
		t.Run(tc.host, func(t *testing.T) {
			dns := fixedDNS{"places.googleapis.com": tc.ips, "photon.komoot.io": tc.ips}
			got := ""
			dial := func(_ context.Context, _, addr string) (net.Conn, error) {
				got = addr
				return nil, context.DeadlineExceeded
			}
			r := httptest.NewRequest(http.MethodConnect, "http://example.test/", nil)
			r.Host = tc.host
			serveWith(httptest.NewRecorder(), r, dns, dial)
			if got != tc.want {
				t.Fatalf("dial=%q want=%q", got, tc.want)
			}
		})
	}
}
