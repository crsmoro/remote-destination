// Command egress is an experimental IP-pinned CONNECT relay for the private bridge.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"waze-remoto/internal/destination"
)

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

var allowedHosts = []string{"maps.app.goo.gl", "goo.gl", "www.google.com", "maps.google.com", "geocode.googleapis.com", "places.googleapis.com"}

func configuredHosts() ([]string, error) {
	hosts := append([]string(nil), allowedHosts...)
	issuer := os.Getenv("AUTH0_AGENT_ISSUER")
	if issuer == "" {
		return hosts, nil
	}
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(issuer, "/") {
		return nil, errors.New("invalid issuer")
	}
	return append(hosts, u.Hostname()), nil
}

// lookup retries only transient resolver errors, within the caller's total budget.
func lookup(ctx context.Context, r resolver, name string) ([]netip.Addr, error) {
	for attempt := 0; attempt < 2; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		ips, err := r.LookupNetIP(attemptCtx, "ip", name)
		cancel()
		if err == nil || attempt == 1 || ctx.Err() != nil {
			return ips, err
		}
		var dnsErr *net.DNSError
		var netErr net.Error
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil, err
		}
		if !errors.Is(err, context.DeadlineExceeded) && !(errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())) {
			return nil, err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	panic("unreachable")
}

func allowedTarget(ctx context.Context, r resolver, target string) bool {
	allowed, _ := resolveAllowedTarget(ctx, r, target)
	return allowed
}

// resolveAllowedTarget preserves fail-closed matching while reporting an
// indeterminate result when transient DNS failures prevent validation.
func resolveAllowedTarget(ctx context.Context, r resolver, target string) (bool, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil || port != "443" {
		return false, nil
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" || !destination.PublicIP(ip) {
		return false, nil
	}
	ip = ip.Unmap()
	var unavailable error
	for _, name := range allowedHosts {
		ips, err := lookup(ctx, r, name)
		if err != nil {
			var dnsErr *net.DNSError
			var netErr net.Error
			if !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()))) {
				unavailable = err
			}
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			continue
		}
		if len(ips) == 0 {
			continue
		}
		valid, found := true, false
		for _, a := range ips {
			if !destination.PublicIP(a) {
				valid = false
			}
			if a.Unmap() == ip {
				found = true
			}
		}
		if valid && found {
			return true, nil
		}
	}
	return false, unavailable
}

func serve(w http.ResponseWriter, req *http.Request) {
	serveWith(w, req, net.DefaultResolver, (&net.Dialer{}).DialContext)
}

func serveWith(w http.ResponseWriter, req *http.Request, r resolver, dial func(context.Context, string, string) (net.Conn, error)) {
	if req.Method != http.MethodConnect || req.Host == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 4*time.Second)
	defer cancel()
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil || port != "443" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var targets []string
	for _, name := range allowedHosts {
		if host == name {
			ips, e := lookup(ctx, r, host)
			if e != nil || len(ips) == 0 {
				http.Error(w, "unavailable", http.StatusBadGateway)
				return
			}
			for _, ip := range ips {
				if !destination.PublicIP(ip) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				targets = append(targets, net.JoinHostPort(ip.String(), port))
			}
			break
		}
	}
	// Preserve the restricted literal-IP protocol for older clients.
	if len(targets) == 0 {
		allowed, lookupErr := resolveAllowedTarget(ctx, r, req.Host)
		if allowed {
			targets = append(targets, req.Host)
		} else if lookupErr != nil {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
	}
	if len(targets) == 0 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var c net.Conn
	for _, target := range targets {
		c, err = dial(ctx, "tcp", target)
		if err == nil {
			break
		}
	}
	if err != nil {
		http.Error(w, "unavailable", http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		c.Close()
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}
	client, rw, err := hijacker.Hijack()
	if err != nil {
		c.Close()
		return
	}
	defer client.Close()
	defer c.Close()
	// Bound tunnel lifetime even when either side becomes idle.
	deadline := time.Now().Add(15 * time.Second)
	client.SetDeadline(deadline)
	c.SetDeadline(deadline)
	if _, err = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err = rw.Flush(); err != nil {
		return
	}
	done := make(chan struct{}, 1)
	go func() { io.Copy(c, rw); done <- struct{}{} }()
	io.Copy(client, c)
	c.Close()
	client.Close()
	<-done
}
func main() {
	var err error
	allowedHosts, err = configuredHosts()
	if err != nil {
		log.Fatal("invalid AUTH0_AGENT_ISSUER")
	}
	srv := &http.Server{Addr: ":3128", Handler: http.HandlerFunc(serve), ReadHeaderTimeout: 3 * time.Second, MaxHeaderBytes: 8192}
	log.Fatal(srv.ListenAndServe())
}
