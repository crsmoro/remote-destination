package api

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"waze-remoto/internal/store"
)

func TestAgentEndToEndViaConnect(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "one", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	defer jwks.Close()
	connects := make(chan string, 2)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != jwks.Listener.Addr().String() {
			http.Error(w, "forbidden", 403)
			return
		}
		up, err := net.Dial("tcp", jwks.Listener.Addr().String())
		if err != nil {
			http.Error(w, "bad gateway", 502)
			return
		}
		client, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			up.Close()
			return
		}
		connects <- r.Host
		rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		rw.Flush()
		go func() { io.Copy(up, rw); up.Close() }()
		io.Copy(client, up)
		client.Close()
		up.Close()
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	tr := jwks.Client().Transport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyURL(proxyURL)
	defer tr.CloseIdleConnections()
	db, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &agentAuth{issuer: jwks.URL + "/", audience: "aud", clients: map[string]bool{"client": true}, httpClient: &http.Client{Transport: tr, Timeout: 5 * time.Second}}
	h := httptest.NewServer(&server{s: db, agent: a})
	defer h.Close()
	claims := map[string]any{"iss": a.issuer, "aud": "aud", "exp": time.Now().Unix() + 300, "azp": "client", "sub": "client@clients", "scope": "status:read destination:write"}
	token := testToken(t, key, claims)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{{"GET", "/agent/status", "", 200}, {"POST", "/agent/destinations", `{"input":"https://waze.com/ul?ll=1,2"}`, 201}} {
		req, _ := http.NewRequest(tc.method, h.URL+tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, resp.StatusCode, tc.want)
		}
	}

	// Agent submissions must be visible to the authenticated web status but not
	// added to the agent status contract.
	web := httptest.NewRecorder()
	h.Config.Handler.ServeHTTP(web, httptest.NewRequest("GET", "/api/status", nil))
	var ws map[string]any
	if err := json.Unmarshal(web.Body.Bytes(), &ws); err != nil || ws["display"] != "https://waze.com/ul?ll=1,2" {
		t.Fatalf("web status: %s (%v)", web.Body.String(), err)
	}
	req, _ := http.NewRequest("GET", h.URL+"/agent/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var as map[string]any
	err = json.NewDecoder(resp.Body).Decode(&as)
	resp.Body.Close()
	if err != nil || as["display"] != nil || as["uri"] != nil {
		t.Fatalf("agent status leaked input: %#v %v", as, err)
	}
	select {
	case <-connects:
	default:
		t.Fatal("JWKS did not use CONNECT")
	}
}

func TestAgentDisabledFailsClosed(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	w := httptest.NewRecorder()
	New(s, "device-key").ServeHTTP(w, httptest.NewRequest("GET", "/agent/status", nil))
	if w.Code != 401 {
		t.Fatalf("disabled agent status = %d, want 401", w.Code)
	}
}

func TestAgentAuthorizedStatus(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "one", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	defer jwks.Close()
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	auth := &agentAuth{issuer: jwks.URL + "/", audience: "test-aud", clients: map[string]bool{"client": true}, httpClient: jwks.Client()}
	h := &server{s: s, key: sha256.Sum256([]byte("device-key")), agent: auth}
	token := testToken(t, key, map[string]any{"iss": auth.issuer, "aud": auth.audience, "exp": time.Now().Unix() + 300, "azp": "client", "sub": "client@clients", "scope": "status:read destination:write"})
	r := httptest.NewRequest("GET", "/agent/status", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}
func testToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	enc := base64.RawURLEncoding.EncodeToString
	head, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "one", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	msg := enc(head) + "." + enc(payload)
	sum := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return msg + "." + enc(sig)
}

func TestAgentSecurityAndSeparation(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	serverTLS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "one", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	defer serverTLS.Close()
	s, e := store.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	auth := &agentAuth{issuer: serverTLS.URL + "/", audience: "aud", clients: map[string]bool{"client": true}, httpClient: serverTLS.Client()}
	h := &server{s: s, key: sha256.Sum256([]byte("device-key")), agent: auth}
	base := map[string]any{"iss": auth.issuer, "aud": []string{"other", "aud"}, "exp": time.Now().Unix() + 300, "sub": "client@clients", "azp": "client", "scope": "status:read destination:write"}
	makeClaims := func(k string, v any) map[string]any {
		m := map[string]any{}
		for a, b := range base {
			m[a] = b
		}
		m[k] = v
		return m
	}
	tests := []struct {
		name, method, path, body string
		claims                   map[string]any
		token                    string
		want                     int
	}{
		{"valid status", "GET", "/agent/status", "", base, "", 200},
		{"valid write", "POST", "/agent/destinations", `{"input":"https://waze.com/ul?ll=1,2"}`, base, "", 201},
		{"missing token", "GET", "/agent/status", "", nil, "", 401},
		{"device key", "GET", "/agent/status", "", nil, "device-key", 401},
		{"wrong issuer", "GET", "/agent/status", "", makeClaims("iss", "https://evil/"), "", 401},
		{"wrong audience", "GET", "/agent/status", "", makeClaims("aud", "else"), "", 401},
		{"expired", "GET", "/agent/status", "", makeClaims("exp", time.Now().Unix()-1), "", 401},
		{"future nbf", "GET", "/agent/status", "", makeClaims("nbf", time.Now().Unix()+300), "", 401},
		{"wrong client", "GET", "/agent/status", "", makeClaims("azp", "evil"), "", 401},
		{"human subject", "GET", "/agent/status", "", makeClaims("sub", "auth0|user"), "", 401},
		{"wrong scope", "POST", "/agent/destinations", `{"input":"X"}`, makeClaims("scope", "status:read"), "", 401},
		{"scope substring", "GET", "/agent/status", "", makeClaims("scope", "status:readmore"), "", 401},
		{"extra unauthorized scope", "GET", "/agent/status", "", makeClaims("scope", "status:read destination:write other:scope"), "", 401},
		{"cannot cancel", "DELETE", "/agent/destinations/pending", "", base, "", 404},
		{"no UI access", "GET", "/api/status", "", base, "", 401},
		{"no device access", "POST", "/device/poll", "{}", base, "", 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			token := tc.token
			if tc.claims != nil {
				token = testToken(t, key, tc.claims)
			}
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestStrictJSONRejectsExcessiveMembers(t *testing.T) {
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < 257; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"k%d":0`, i)
	}
	b.WriteByte('}')
	var v any
	if strictJSON([]byte(b.String()), &v) == nil {
		t.Fatal("accepted excessive object members")
	}
}

func TestStrictJSONRejectsCaseFoldedDuplicates(t *testing.T) {
	var claims struct {
		Issuer string `json:"iss"`
	}
	if strictJSON([]byte(`{"iss":"trusted","ISS":"attacker"}`), &claims) == nil {
		t.Fatalf("accepted ambiguous issuer: %q", claims.Issuer)
	}
}

func TestStrictJSONRejectsDuplicates(t *testing.T) {
	for _, s := range []string{`{"iss":"a","iss":"b"}`, `{"aud":{"x":1,"x":2}}`, `{"x":1} {"x":2}`} {
		var v any
		if strictJSON([]byte(s), &v) == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
func TestAgentRejectsStringNumericDates(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "one", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	defer jwks.Close()
	a := &agentAuth{issuer: jwks.URL + "/", audience: "aud", clients: map[string]bool{"client": true}, httpClient: jwks.Client()}
	claims := map[string]any{"iss": a.issuer, "aud": "aud", "exp": "9999999999", "azp": "client", "sub": "client@clients", "scope": "status:read"}
	r := httptest.NewRequest("GET", "/agent/status", nil)
	r.Header.Set("Authorization", "Bearer "+testToken(t, key, claims))
	if a.authorize(r, "status:read") {
		t.Fatal("accepted string-valued exp")
	}
}

func TestAgentJWKSUsesRestrictedRelay(t *testing.T) {
	t.Setenv("WAZE_EGRESS_PROXY", "egress:3128")
	t.Setenv("AUTH0_AGENT_ISSUER", "https://tenant.example.com/")
	t.Setenv("AUTH0_AGENT_AUDIENCE", "https://api.example.com/agent")
	h, err := NewWithAgent(nil, "key", "client")
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := h.(*server).agent.httpClient.Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatal("JWKS client does not use the configured relay")
	}
	req := httptest.NewRequest("GET", "https://tenant.example.com/.well-known/jwks.json", nil)
	proxy, err := tr.Proxy(req)
	if err != nil || proxy == nil || proxy.String() != "http://egress:3128" {
		t.Fatalf("unexpected proxy: %v %v", proxy, err)
	}
}

func TestAgentConfigurationRequiresClientID(t *testing.T) {
	for _, client := range []string{"", " client", "one,two"} {
		if _, e := NewWithAgent(nil, "key", client); e == nil {
			t.Errorf("accepted %q", client)
		}
	}
}

func TestAgentRejectsTamperingAndUntrustedJWKS(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "one", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	}))
	defer srv.Close()
	a := &agentAuth{issuer: srv.URL + "/", audience: "aud", clients: map[string]bool{"client": true}, httpClient: srv.Client()}
	claims := map[string]any{"iss": a.issuer, "aud": "aud", "exp": time.Now().Unix() + 300, "azp": "client", "sub": "client@clients", "scope": "status:read"}
	valid := testToken(t, key, claims)
	badSig := testToken(t, other, claims)
	pieces := strings.Split(valid, ".")
	badAlg := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","kid":"one"}`)) + "." + pieces[1] + "." + pieces[2]
	duplicate := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","alg":"RS256","kid":"one"}`)) + "." + pieces[1] + "." + pieces[2]
	call := func(auth *agentAuth, token string) bool {
		r := httptest.NewRequest("GET", "/agent/status", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		return auth.authorize(r, "status:read")
	}
	if !call(a, valid) {
		t.Fatal("valid token rejected")
	}
	for _, v := range []string{badSig, badAlg, duplicate, valid + "x"} {
		if call(a, v) {
			t.Error("accepted tampered token")
		}
	}
	untrusted := &agentAuth{issuer: a.issuer, audience: a.audience, clients: a.clients, httpClient: &http.Client{Timeout: time.Second}}
	if call(untrusted, valid) {
		t.Fatal("accepted untrusted TLS JWKS")
	}
}

func TestAgentJWKSRotationAndDuplicateKids(t *testing.T) {
	first, _ := rsa.GenerateKey(rand.Reader, 2048)
	second, _ := rsa.GenerateKey(rand.Reader, 2048)
	var mu sync.Mutex
	current := first
	duplicate := false
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		k := map[string]any{"kty": "RSA", "kid": "one", "n": base64.RawURLEncoding.EncodeToString(current.N.Bytes()), "e": "AQAB"}
		keys := []any{k}
		if duplicate {
			keys = append(keys, k)
		}
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	defer jwks.Close()
	auth := &agentAuth{issuer: jwks.URL + "/", audience: "aud", clients: map[string]bool{"client": true}, httpClient: jwks.Client()}
	claims := map[string]any{"iss": auth.issuer, "aud": "aud", "exp": time.Now().Unix() + 300, "azp": "client", "sub": "client@clients", "scope": "status:read"}
	check := func(key *rsa.PrivateKey) bool {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer "+testToken(t, key, claims))
		return auth.authorize(r, "status:read")
	}
	if !check(first) {
		t.Fatal("first key rejected")
	}
	mu.Lock()
	current = second
	mu.Unlock()
	auth.mu.Lock()
	auth.expires = time.Time{}
	auth.lastFetch = time.Time{}
	auth.mu.Unlock()
	if !check(second) {
		t.Fatal("rotated key rejected")
	}
	mu.Lock()
	duplicate = true
	mu.Unlock()
	auth.mu.Lock()
	auth.expires = time.Time{}
	auth.lastFetch = time.Time{}
	auth.mu.Unlock()
	if check(second) {
		t.Fatal("duplicate JWKS key accepted")
	}
}
