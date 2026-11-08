package destination

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
)

func TestProxyConnectUsesValidatedIPAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	seen := make(chan string, 1)
	go func() {
		c, e := listener.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		line, _ := r.ReadString('\n')
		seen <- strings.TrimSpace(line)
		for {
			s, e := r.ReadString('\n')
			if e != nil || s == "\r\n" {
				break
			}
		}
		fmt.Fprint(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
	}()
	r := &fakeResolver{answers: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}
	c, err := secureDial(r, proxyDial(listener.Addr().String(), (&net.Dialer{}).DialContext))(context.Background(), "tcp", "maps.app.goo.gl:443")
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if got := <-seen; got != "CONNECT 8.8.8.8:443 HTTP/1.1" {
		t.Fatalf("unexpected CONNECT: %q", got)
	}
}
