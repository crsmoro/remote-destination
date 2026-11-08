package destination

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestPinnedProxyPreservesTLSIdentity(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "maps.app.goo.gl"}, DNSNames: []string{"maps.app.goo.gl"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	sni := make(chan string, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sni <- r.TLS.ServerName; w.WriteHeader(204) }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
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
		reader := bufio.NewReader(c)
		req, e := http.ReadRequest(reader)
		if e != nil {
			return
		}
		seen <- req.Host
		upstream, e := net.Dial("tcp", server.Listener.Addr().String())
		if e != nil {
			return
		}
		defer upstream.Close()
		io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
		done := make(chan struct{})
		go func() { io.Copy(upstream, reader); upstream.Close(); close(done) }()
		io.Copy(c, upstream)
		c.Close()
		<-done
	}()
	roots := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots.AddCert(parsed)
	r := &fakeResolver{answers: []netip.Addr{netip.MustParseAddr("8.8.8.8")}}
	tr := &http.Transport{Proxy: nil, DialContext: secureDial(r, proxyDial(listener.Addr().String(), (&net.Dialer{}).DialContext)), TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}
	resp, err := client.Get("https://maps.app.goo.gl/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := <-seen; got != "8.8.8.8:443" {
		t.Fatalf("CONNECT %q", got)
	}
	if got := <-sni; got != "maps.app.goo.gl" {
		t.Fatalf("SNI %q", got)
	}
}
