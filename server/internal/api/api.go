package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

type server struct {
	s         *store.Store
	key       [32]byte
	agent     *agentAuth
	geocode   func(context.Context, string) ([]destination.Candidate, error)
	tokenOnce sync.Once
	tokenKey  [32]byte
	tokenErr  error
}

// SetGeocoder must be called before serving requests.
func SetGeocoder(h http.Handler, lookup destination.Geocoder) error {
	a, ok := h.(*server)
	if !ok || lookup == nil {
		return errors.New("invalid geocoder")
	}
	a.geocode = lookup
	return nil
}

func New(s *store.Store, key string) http.Handler {
	return &server{s: s, key: sha256.Sum256([]byte(key))}
}
func (a *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if strings.HasPrefix(r.URL.Path, "/agent/") {
		a.agentRequest(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/device/") {
		h := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		valid := false
		if len(h) == 2 && h[0] == "Bearer" && h[1] != "" {
			sum := sha256.Sum256([]byte(h[1]))
			valid = subtle.ConstantTimeCompare(a.key[:], sum[:]) == 1
		}
		if !valid {
			http.Error(w, "unauthorized", 401)
			return
		}
		a.device(w, r)
		return
	}
	// A bearer credential is never an alternative to the upstream web login.
	// Reject ambiguous multiple headers and comma-joined credentials as well.
	authorizations := r.Header.Values("Authorization")
	if len(authorizations) > 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	if len(authorizations) == 1 {
		for _, value := range strings.Split(authorizations[0], ",") {
			fields := strings.Fields(value)
			if len(fields) > 0 && strings.EqualFold(fields[0], "Bearer") {
				http.Error(w, "unauthorized", 401)
				return
			}
		}
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "forbidden", 403)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || u.Host != r.Host || (u.Scheme != "https" && u.Scheme != "http") {
				http.Error(w, "forbidden", 403)
				return
			}
		}
	}
	a.web(w, r)
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func read(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		http.Error(w, "invalid JSON", 400)
		return false
	}
	var x any
	if d.Decode(&x) != io.EOF {
		http.Error(w, "invalid JSON", 400)
		return false
	}
	return true
}
func id() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
func fail(w http.ResponseWriter, e error) {
	if errors.Is(e, store.ErrConflict) {
		http.Error(w, "conflict", 409)
	} else {
		http.Error(w, "internal error", 500)
	}
}
func (a *server) device(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	switch r.URL.Path {
	case "/device/poll":
		var input struct{}
		if !read(w, r, &input) {
			return
		}
		if e := a.s.Heartbeat(); e != nil {
			fail(w, e)
			return
		}
		c, e := a.s.Pending()
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]any{"command": c})
	case "/device/claim":
		var in struct {
			ID string `json:"id"`
		}
		if !read(w, r, &in) {
			return
		}
		if in.ID == "" {
			http.Error(w, "invalid id", 422)
			return
		}
		if e := a.s.Claim(in.ID); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]string{"state": "pending"})
	case "/device/ack":
		var in struct {
			ID     string `json:"id"`
			Result string `json:"result"`
		}
		if !read(w, r, &in) {
			return
		}
		if in.ID == "" || (in.Result != "intent_sent" && in.Result != "failed") {
			http.Error(w, "invalid result", 422)
			return
		}
		if e := a.s.Ack(in.ID, in.Result); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]string{"state": in.Result})
	default:
		http.NotFound(w, r)
	}
}
func (a *server) web(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/geocode", "/api/geocode/confirm":
		a.geocodeRequest(w, r)
	case "/api/destinations":
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Path = "/api/geocode/auto"
		clone.URL = &u
		a.geocodeRequest(w, clone)
	case "/api/destinations/pending":
		if r.Method != "DELETE" {
			http.Error(w, "method not allowed", 405)
			return
		}
		if e := a.s.Cancel(); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]string{"state": "cancelled"})
	case "/api/status":
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		st, e := a.s.Status()
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, st)
	default:
		serveStatic(w, r)
	}
}
