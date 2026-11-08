package destination

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// configuredDial connects only to the explicitly configured private relay.
// Relay mode sends the Google hostname; the relay validates DNS and pins the IP.
// TLS hostname verification remains with the backend.
func configuredDial(dial dialFunc) dialFunc {
	switch os.Getenv("WAZE_EGRESS_PROXY") {
	case "":
		return dial
	case "egress:3128":
		return proxyDial("egress:3128", dial)
	default:
		return func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("invalid egress configuration")
		}
	}
}

func proxyDial(proxy string, dial dialFunc) dialFunc {
	return func(ctx context.Context, network, target string) (net.Conn, error) {
		c, err := dial(ctx, "tcp", proxy)
		if err != nil {
			return nil, err
		}
		ok := false
		defer func() {
			if !ok {
				c.Close()
			}
		}()
		deadline := time.Now().Add(4 * time.Second)
		if d, has := ctx.Deadline(); has && d.Before(deadline) {
			deadline = d
		}
		c.SetDeadline(deadline)
		stop := context.AfterFunc(ctx, func() { c.Close() })
		defer stop()
		if _, err = fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
			return nil, err
		}
		br := bufio.NewReader(c)
		resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
		if err != nil || resp.StatusCode != http.StatusOK || br.Buffered() != 0 {
			return nil, errors.New("proxy CONNECT failed")
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		c.SetDeadline(time.Time{})
		ok = true
		return c, nil
	}
}
