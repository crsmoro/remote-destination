package destination

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
)

func TestProxyRefusalFailsClosed(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, e := l.Accept()
		if e == nil {
			defer c.Close()
			fmt.Fprint(c, "HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n")
		}
	}()
	r := &fakeResolver{answers: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}
	c, err := secureDial(r, proxyDial(l.Addr().String(), (&net.Dialer{}).DialContext))(context.Background(), "tcp", "maps.app.goo.gl:443")
	if err == nil {
		c.Close()
		t.Fatal("proxy refusal accepted")
	}
}
func TestProxyNotContactedForMixedDNS(t *testing.T) {
	r := &fakeResolver{answers: []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}}
	called := false
	dial := func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, fmt.Errorf("unexpected dial")
	}
	_, err := secureDial(r, proxyDial("egress:3128", dial))(context.Background(), "tcp", "maps.app.goo.gl:443")
	if err == nil || called {
		t.Fatal("mixed answer reached proxy")
	}
}
