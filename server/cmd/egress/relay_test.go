package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestRelayTunnelsOnlyApprovedLiteral(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	go func() {
		c, e := upstream.Accept()
		if e == nil {
			defer c.Close()
			io.Copy(c, c)
		}
	}()
	dialed := make(chan string, 1)
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed <- addr
		return (&net.Dialer{}).DialContext(ctx, network, upstream.Addr().String())
	}
	dns := fixedDNS{"maps.app.goo.gl": {netip.MustParseAddr("8.8.8.8")}}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveWith(w, r, dns, dial) }))
	defer proxy.Close()
	conn, err := net.Dial("tcp", proxy.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	fmt.Fprint(conn, "CONNECT 8.8.8.8:443 HTTP/1.1\r\nHost: 8.8.8.8:443\r\n\r\n")
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("response %v %v", resp, err)
	}
	fmt.Fprint(conn, "hello")
	data := make([]byte, 5)
	if _, err = io.ReadFull(reader, data); err != nil || string(data) != "hello" {
		t.Fatalf("tunnel %q: %v", data, err)
	}
	if got := <-dialed; got != "8.8.8.8:443" {
		t.Fatalf("dial %q", got)
	}
}
