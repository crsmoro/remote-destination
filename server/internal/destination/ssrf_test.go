package destination

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"testing"
)

type fakeResolver struct {
	answers []netip.Addr
	calls   int
}

func (r *fakeResolver) LookupNetIP(_ context.Context, _, _ string) ([]netip.Addr, error) {
	r.calls++
	if r.calls > 1 {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
	return r.answers, nil
}
func TestDialPinsResolvedIPAgainstRebinding(t *testing.T) {
	r := &fakeResolver{answers: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}
	var address string
	dial := func(_ context.Context, _, addr string) (net.Conn, error) {
		address = addr
		return nil, errors.New("stop")
	}
	_, _ = secureDial(r, dial)(context.Background(), "tcp", "maps.app.goo.gl:443")
	if r.calls != 1 || address != "8.8.8.8:443" {
		t.Fatalf("resolution %d dial %q", r.calls, address)
	}
}
func TestDialRejectsPrivateAndMixedAnswers(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.64.0.1", "::1", "::ffff:192.168.1.1", "fc00::1", "2001:db8::1"} {
		t.Run(ip, func(t *testing.T) {
			r := &fakeResolver{answers: []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(ip)}}
			called := false
			d := func(context.Context, string, string) (net.Conn, error) { called = true; return nil, nil }
			_, err := secureDial(r, d)(context.Background(), "tcp", "maps.app.goo.gl:443")
			if err == nil || called {
				t.Fatal("private/mixed answer reached dial")
			}
		})
	}
}
func TestGoogleURLAllowlist(t *testing.T) {
	for _, raw := range []string{"http://www.google.com/maps/place/x", "https://www.google.com.evil.test/maps/x", "https://www.google.com:444/maps/x", "https://user@www.google.com/maps/x", "https://www.google.com/mapsbad", "https://127.0.0.1/maps/x"} {
		u, _ := url.Parse(raw)
		if allowedGoogle(u) {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestRouteCoordinatesAreNotAssumedDestination(t *testing.T) {
	got, warning, err := Normalize("https://www.google.com/maps/dir/A/B/data=!3d1!4d2")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("ll") != "" || warning == "" {
		t.Fatalf("got %q warning %q", got, warning)
	}
}
