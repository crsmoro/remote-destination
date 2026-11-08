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

var allowedHosts = []string{"maps.app.goo.gl", "goo.gl", "www.google.com", "maps.google.com"}

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

func allowedTarget(ctx context.Context, r resolver, target string) bool {
	host, port, err := net.SplitHostPort(target)
	if err != nil || port != "443" {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" || !destination.PublicIP(ip) {
		return false
	}
	ip = ip.Unmap()
	for _, name := range allowedHosts {
		ips, err := r.LookupNetIP(ctx, "ip", name)
		if err != nil || len(ips) == 0 {
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
			return true
		}
	}
	return false
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
			ips, e := r.LookupNetIP(ctx, "ip", host)
			if e == nil && len(ips) > 0 {
				for _, ip := range ips {
					if !destination.PublicIP(ip) {
						targets = nil
						break
					}
					targets = append(targets, net.JoinHostPort(ip.String(), port))
				}
			}
			break
		}
	}
	// Preserve the restricted literal-IP protocol for older clients.
	if len(targets) == 0 && allowedTarget(ctx, r, req.Host) {
		targets = append(targets, req.Host)
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
