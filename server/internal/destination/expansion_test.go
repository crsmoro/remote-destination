package destination

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type expansionResolver struct{}

func (expansionResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
}

// A subprocess isolates the test CA from Go's cached system certificate pool.
func TestExpandShortLinkLocally(t *testing.T) {
	if address := os.Getenv("EXPANSION_TEST_ADDRESS"); address != "" {
		dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}
		expanded, err := expand("https://maps.app.goo.gl/synthetic-fixture", expansionResolver{}, dial)
		if err != nil {
			t.Fatal(err)
		}
		got, warning, err := Normalize(expanded)
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(got)
		if err != nil {
			t.Fatal(err)
		}
		if u.Query().Get("ll") != "12.3456,23.4567" || warning != "" {
			t.Fatalf("got %q warning %q", got, warning)
		}
		return
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"maps.app.goo.gl", "www.google.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cert, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "maps.app.goo.gl":
			if r.URL.Path != "/synthetic-fixture" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "https://www.google.com/maps/place/Ficticio/data=!3d12.3456!4d23.4567", http.StatusFound)
		case "www.google.com":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	certFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestExpandShortLinkLocally$")
	cmd.Env = append(os.Environ(), "EXPANSION_TEST_ADDRESS="+server.Listener.Addr().String(), "SSL_CERT_FILE="+certFile, "WAZE_EGRESS_PROXY=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, output)
	}
}
