package destination

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}
type dialFunc func(context.Context, string, string) (net.Conn, error)

var blockedNetworks = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16"} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// PublicIP reports whether an address is suitable for an outbound destination.
func PublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, p := range blockedNetworks {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
func allowedGoogle(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "maps.app.goo.gl":
		return u.Path != "" && u.Path != "/"
	case "goo.gl":
		return strings.HasPrefix(u.Path, "/maps/")
	case "www.google.com":
		return u.Path == "/maps" || strings.HasPrefix(u.Path, "/maps/")
	case "maps.google.com":
		return u.Path == "/" || u.Path == "/maps" || strings.HasPrefix(u.Path, "/maps/")
	}
	return false
}
func secureDial(r resolver, dial dialFunc) dialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != "443" {
			return nil, errors.New("invalid destination")
		}
		ips, err := r.LookupNetIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("DNS resolution failed")
		}
		// Fail closed on mixed public/private DNS answers, and pin the checked IP.
		for _, ip := range ips {
			if !PublicIP(ip) {
				return nil, errors.New("non-public address")
			}
		}
		var last error
		for _, ip := range ips {
			var c net.Conn
			c, last = dial(ctx, network, net.JoinHostPort(ip.String(), port))
			if last == nil {
				return c, nil
			}
		}
		return nil, last
	}
}
func expand(input string, r resolver, dial dialFunc) (string, error) {
	start, err := url.Parse(input)
	if err != nil || !allowedGoogle(start) {
		return "", errors.New("invalid URL")
	}
	selectedDial := secureDial(r, dial)
	if os.Getenv("WAZE_EGRESS_PROXY") == "egress:3128" {
		// The relay resolves and pins the allowed hostname; TLS remains end-to-end.
		selectedDial = dial
	}
	tr := &http.Transport{Proxy: nil, DialContext: selectedDial, TLSHandshakeTimeout: 4 * time.Second, DisableKeepAlives: true}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 6 || !allowedGoogle(req.URL) {
			return errors.New("redirect blocked")
		}
		return nil
	}}
	req, err := http.NewRequest(http.MethodGet, input, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("expansion failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("expansion failed")
	}
	return resp.Request.URL.String(), nil
}
func expandGoogle(input string) (string, error) {
	d := &net.Dialer{Timeout: 4 * time.Second}
	return expand(input, net.DefaultResolver, configuredDial(d.DialContext))
}
