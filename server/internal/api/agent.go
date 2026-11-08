package api

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"waze-remoto/internal/store"
)

type agentAuth struct {
	issuer, audience   string
	clients            map[string]bool
	httpClient         *http.Client
	mu                 sync.Mutex
	keys               map[string]*rsa.PublicKey
	expires, lastFetch time.Time
}

// NewWithAgent enables the agent endpoints only for the configured Auth0 client.
func NewWithAgent(s *store.Store, key, clientID string) (http.Handler, error) {
	if strings.TrimSpace(clientID) != clientID || clientID == "" || strings.ContainsAny(clientID, " \t\r\n,") {
		return nil, errors.New("invalid agent client ID")
	}
	issuer, audience := os.Getenv("AUTH0_AGENT_ISSUER"), os.Getenv("AUTH0_AGENT_AUDIENCE")
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(issuer, "/") || strings.TrimSpace(issuer) != issuer || strings.TrimSpace(audience) != audience || audience == "" {
		return nil, errors.New("invalid agent issuer or audience")
	}
	transport := &http.Transport{TLSHandshakeTimeout: 4 * time.Second}
	switch os.Getenv("WAZE_EGRESS_PROXY") {
	case "": // Direct access, without implicit environment proxy settings.
	case "egress:3128":
		transport.Proxy = http.ProxyURL(&url.URL{Scheme: "http", Host: "egress:3128"})
	default:
		return nil, errors.New("invalid egress configuration")
	}
	return &server{s: s, key: sha256.Sum256([]byte(key)), agent: &agentAuth{issuer: issuer, audience: audience, clients: map[string]bool{clientID: true}, httpClient: &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}}, nil
}

// strictJSON rejects duplicate members at any nesting depth and trailing data.
func strictJSON(data []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var walk func() error
	walk = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := []string{}
			for d.More() {
				if len(seen) >= 256 {
					return errors.New("too many members")
				}
				x, e := d.Token()
				if e != nil {
					return e
				}
				k, ok := x.(string)
				if !ok {
					return errors.New("invalid member")
				}
				for _, previous := range seen {
					if strings.EqualFold(previous, k) {
						return errors.New("duplicate member")
					}
				}
				seen = append(seen, k)
				if e = walk(); e != nil {
					return e
				}
			}
			_, e = d.Token()
			return e
		case '[':
			for d.More() {
				if e = walk(); e != nil {
					return e
				}
			}
			_, e = d.Token()
			return e
		}
		return errors.New("bad JSON")
	}
	if e := walk(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return json.Unmarshal(data, out)
}

func (a *agentAuth) publicKey(kid string) (*rsa.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if now.Before(a.expires) {
		if k := a.keys[kid]; k != nil {
			return k, nil
		}
	}
	if now.Sub(a.lastFetch) < 30*time.Second {
		return nil, errors.New("unknown key")
	}
	a.lastFetch = now
	u, err := url.Parse(a.issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid issuer")
	}
	resp, err := a.httpClient.Get(a.issuer + ".well-known/jwks.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("JWKS fetch failed")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || len(body) >= 1<<20 {
		return nil, errors.New("invalid JWKS")
	}
	var set struct {
		Keys []struct {
			Kty    string   `json:"kty"`
			Kid    string   `json:"kid"`
			Alg    string   `json:"alg"`
			Use    string   `json:"use"`
			N      string   `json:"n"`
			E      string   `json:"e"`
			KeyOps []string `json:"key_ops"`
		} `json:"keys"`
	}
	if strictJSON(body, &set) != nil || len(set.Keys) == 0 {
		return nil, errors.New("invalid JWKS")
	}
	keys := map[string]*rsa.PublicKey{}
	seen := map[string]bool{}
	for _, v := range set.Keys {
		if v.Kid == "" || seen[v.Kid] {
			return nil, errors.New("duplicate key ID")
		}
		seen[v.Kid] = true
		if v.Kty != "RSA" || (v.Alg != "" && v.Alg != "RS256") || (v.Use != "" && v.Use != "sig") {
			continue
		}
		if len(v.KeyOps) > 0 {
			allowed := false
			for _, op := range v.KeyOps {
				if op == "verify" {
					allowed = true
				}
			}
			if !allowed {
				continue
			}
		}
		n, e1 := base64.RawURLEncoding.DecodeString(v.N)
		eb, e2 := base64.RawURLEncoding.DecodeString(v.E)
		if e1 != nil || e2 != nil || len(n) < 256 || len(n) > 1024 || len(eb) == 0 || len(eb) > 4 {
			continue
		}
		exponent := new(big.Int).SetBytes(eb).Int64()
		if exponent < 3 || exponent > 2147483647 || exponent%2 == 0 {
			continue
		}
		keys[v.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exponent)}
	}
	a.keys = keys
	a.expires = now.Add(5 * time.Minute)
	if k := keys[kid]; k != nil {
		return k, nil
	}
	return nil, errors.New("unknown key")
}

func (a *agentAuth) authorize(r *http.Request, scope string) bool {
	headers := r.Header.Values("Authorization")
	if len(headers) != 1 {
		return false
	}
	parts := strings.Split(headers[0], " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 16384 {
		return false
	}
	pieces := strings.Split(parts[1], ".")
	if len(pieces) != 3 {
		return false
	}
	decode := base64.RawURLEncoding.DecodeString
	hb, e := decode(pieces[0])
	if e != nil {
		return false
	}
	pb, e := decode(pieces[1])
	if e != nil {
		return false
	}
	sig, e := decode(pieces[2])
	if e != nil {
		return false
	}
	var h struct {
		Alg  string          `json:"alg"`
		Kid  string          `json:"kid"`
		Crit json.RawMessage `json:"crit"`
		B64  *bool           `json:"b64"`
	}
	if strictJSON(hb, &h) != nil || h.Alg != "RS256" || h.Kid == "" || len(h.Kid) > 256 || len(h.Crit) > 0 || h.B64 != nil {
		return false
	}
	var c struct {
		Issuer   string          `json:"iss"`
		Audience json.RawMessage `json:"aud"`
		Exp      json.RawMessage `json:"exp"`
		Nbf      json.RawMessage `json:"nbf"`
		Azp      string          `json:"azp"`
		Scope    string          `json:"scope"`
		Subject  string          `json:"sub"`
	}
	if strictJSON(pb, &c) != nil || c.Issuer != a.issuer || !a.clients[c.Azp] || c.Subject != c.Azp+"@clients" {
		return false
	}
	exp, e := strconv.ParseInt(string(c.Exp), 10, 64)
	if e != nil || time.Now().Unix() >= exp {
		return false
	}
	if len(c.Nbf) != 0 {
		nbf, e := strconv.ParseInt(string(c.Nbf), 10, 64)
		if e != nil || time.Now().Unix() < nbf {
			return false
		}
	}
	var aud string
	var auds []string
	if json.Unmarshal(c.Audience, &aud) == nil {
		auds = []string{aud}
	} else if json.Unmarshal(c.Audience, &auds) != nil {
		return false
	}
	found := false
	for _, v := range auds {
		if v == a.audience {
			found = true
		}
	}
	if !found {
		return false
	}
	found = false
	for _, v := range strings.Split(c.Scope, " ") {
		if v != "status:read" && v != "destination:write" {
			return false
		}
		if v == scope {
			found = true
		}
	}
	if !found {
		return false
	}
	key, e := a.publicKey(h.Kid)
	if e != nil {
		return false
	}
	sum := sha256.Sum256([]byte(pieces[0] + "." + pieces[1]))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) == nil
}

func (a *server) agentRequest(w http.ResponseWriter, r *http.Request) {
	var method, scope, path string
	switch r.URL.Path {
	case "/agent/destinations":
		method, scope, path = "POST", "destination:write", "/api/destinations"
	case "/agent/status":
		method, scope, path = "GET", "status:read", "/api/status"
	default:
		http.NotFound(w, r)
		return
	}
	if a.agent == nil || !a.agent.authorize(r, scope) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != method {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if path == "/api/status" {
		st, err := a.s.Status()
		if err != nil {
			fail(w, err)
			return
		}
		st.Display = ""
		st.URI = ""
		respond(w, 200, st)
		return
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	u.Path = path
	clone.URL = &u
	a.web(w, clone)
}
