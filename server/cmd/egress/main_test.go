package main

import (
	"context"
	"net/netip"
	"testing"
)

type fixedDNS map[string][]netip.Addr

func (f fixedDNS) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return f[host], nil
}
func TestDestinationOnlyMatchesGooglePublicDNS(t *testing.T) {
	r := fixedDNS{"maps.app.goo.gl": {netip.MustParseAddr("8.8.8.8")}, "www.google.com": {netip.MustParseAddr("127.0.0.1")}}
	for _, tc := range []struct {
		target string
		want   bool
	}{{"8.8.8.8:443", true}, {"8.8.4.4:443", false}, {"127.0.0.1:443", false}, {"8.8.8.8:80", false}, {"maps.app.goo.gl:443", false}} {
		if got := allowedTarget(context.Background(), r, tc.target); got != tc.want {
			t.Errorf("%s: %v", tc.target, got)
		}
	}
}
