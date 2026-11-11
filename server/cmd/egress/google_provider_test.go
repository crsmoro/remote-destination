package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestGoogleConnectExactAndPublic(t *testing.T) {
	for _, tc := range []struct {
		host string
		ips  []netip.Addr
		want string
	}{
		{"geocode.googleapis.com:443", []netip.Addr{netip.MustParseAddr("8.8.4.4")}, "8.8.4.4:443"},
		{"geocode.googleapis.com:443", []netip.Addr{netip.MustParseAddr("8.8.4.4"), netip.MustParseAddr("127.0.0.1")}, ""},
		{"geocode.googleapis.com.evil.test:443", []netip.Addr{netip.MustParseAddr("8.8.4.4")}, ""},
		{"geocode.googleapis.com:444", []netip.Addr{netip.MustParseAddr("8.8.4.4")}, ""},
	} {
		dns := fixedDNS{"geocode.googleapis.com": tc.ips}
		got := ""
		dial := func(_ context.Context, _, addr string) (net.Conn, error) {
			got = addr
			return nil, context.DeadlineExceeded
		}
		req := httptest.NewRequest(http.MethodConnect, "http://example.test/", nil)
		req.Host = tc.host
		w := httptest.NewRecorder()
		serveWith(w, req, dns, dial)
		if got != tc.want {
			t.Fatalf("host %s dial %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestGoogleGeocoderExactEgressPublicDNS(t *testing.T) {
	r := fixedDNS{"geocode.googleapis.com": {netip.MustParseAddr("8.8.4.4")}}
	if !allowedTarget(context.Background(), r, "8.8.4.4:443") {
		t.Fatal("Google geocoding target not allowed")
	}
}
