package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestAuth0HostnameRejectsMixedDNS(t *testing.T) {
	allowedHosts = append(allowedHosts, "tenant.example.com")
	t.Cleanup(func() { allowedHosts = allowedHosts[:len(allowedHosts)-1] })
	dns := fixedDNS{"tenant.example.com": {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}}
	dialed := false
	dial := func(_ context.Context, _, _ string) (net.Conn, error) {
		dialed = true
		return nil, context.DeadlineExceeded
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "http://example.com/", nil)
	req.Host = "tenant.example.com:443"
	serveWith(w, req, dns, dial)
	if dialed || w.Code != http.StatusForbidden {
		t.Fatalf("dialed %v status %d", dialed, w.Code)
	}
}

func TestAuth0HostnameConnectPinsPublicDNS(t *testing.T) {
	allowedHosts = append(allowedHosts, "tenant.example.com")
	t.Cleanup(func() { allowedHosts = allowedHosts[:len(allowedHosts)-1] })
	dns := fixedDNS{"tenant.example.com": {netip.MustParseAddr("8.8.8.8")}}
	dialed := ""
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = addr
		return nil, context.DeadlineExceeded
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "http://example.com/", nil)
	req.Host = "tenant.example.com:443"
	serveWith(w, req, dns, dial)
	if dialed != "8.8.8.8:443" || w.Code != http.StatusBadGateway {
		t.Fatalf("dialed %q status %d", dialed, w.Code)
	}
}

func TestHostnameConnectPinsPublicDNS(t *testing.T) {
	allowedHosts = append(allowedHosts, "tenant.example.com")
	t.Cleanup(func() { allowedHosts = allowedHosts[:len(allowedHosts)-1] })
	dns := fixedDNS{"maps.app.goo.gl": {netip.MustParseAddr("8.8.8.8")}}
	dialed := ""
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = addr
		return nil, context.DeadlineExceeded
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "http://example.com/", nil)
	req.Host = "maps.app.goo.gl:443"
	serveWith(w, req, dns, dial)
	if dialed != "8.8.8.8:443" || w.Code != http.StatusBadGateway {
		t.Fatalf("dialed %q status %d", dialed, w.Code)
	}
}

func TestHostnameConnectRejectsMixedDNS(t *testing.T) {
	allowedHosts = append(allowedHosts, "tenant.example.com")
	t.Cleanup(func() { allowedHosts = allowedHosts[:len(allowedHosts)-1] })
	dns := fixedDNS{"maps.app.goo.gl": {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}}
	dialed := false
	dial := func(_ context.Context, _, _ string) (net.Conn, error) {
		dialed = true
		return nil, context.DeadlineExceeded
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "http://example.com/", nil)
	req.Host = "maps.app.goo.gl:443"
	serveWith(w, req, dns, dial)
	if dialed || w.Code != http.StatusForbidden {
		t.Fatalf("dialed %v status %d", dialed, w.Code)
	}
}
