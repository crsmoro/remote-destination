package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGeocodeRejectsTamperingExpiryCrossSiteAndSuperseded(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, _ string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "x", Latitude: 1, Longitude: 2}}, nil
	}
	search := func() string {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", bytes.NewBufferString(`{"input":"x"}`)))
		var v struct {
			Token string `json:"token"`
		}
		json.Unmarshal(w.Body.Bytes(), &v)
		if v.Token == "" {
			t.Fatal(w.Body)
		}
		return v.Token
	}
	confirm := func(token string, cross bool) int {
		b, _ := json.Marshal(map[string]any{"token": token, "index": 0})
		r := httptest.NewRequest("POST", "/api/geocode/confirm", bytes.NewReader(b))
		if cross {
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	token := search()
	raw, _ := base64.RawURLEncoding.DecodeString(token)
	raw[len(raw)-1] ^= 1
	if got := confirm(base64.RawURLEncoding.EncodeToString(raw), false); got != 422 {
		t.Fatalf("tamper %d", got)
	}
	if got := confirm(token, true); got != 403 {
		t.Fatalf("cross site %d", got)
	}
	cipher, err := a.selectionCipher()
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, cipher.NonceSize())
	b, _ := json.Marshal(selection{Revision: 1, Expires: time.Now().Add(-time.Second).Unix(), Audience: "web", Candidates: []destination.Candidate{{Label: "x", Latitude: 1, Longitude: 2}}})
	expired := base64.RawURLEncoding.EncodeToString(cipher.Seal(nonce, nonce, b, nil))
	if got := confirm(expired, false); got != 422 {
		t.Fatalf("expired %d", got)
	}
	_ = search()
	if got := confirm(token, false); got != 409 {
		t.Fatalf("superseded %d", got)
	}
	p, err := s.Pending()
	if err != nil || p != nil {
		t.Fatalf("unexpected command %v %v", p, err)
	}
}
