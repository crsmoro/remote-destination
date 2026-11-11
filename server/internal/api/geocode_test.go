package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGeocodeRejectsSchemeLessMapsURL(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	called := false
	a.geocode = func(_ context.Context, _ string) ([]destination.Candidate, error) { called = true; return nil, nil }
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", bytes.NewBufferString(`{"input":"maps.google.com/maps/place/X"}`)))
	if w.Code != 422 || called {
		t.Fatalf("status %d photon %v", w.Code, called)
	}
}

func TestSelectionTokenCannotCrossAgentBoundary(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, _ string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "x", Latitude: 1, Longitude: 2}}, nil
	}
	w := httptest.NewRecorder()
	a.web(w, httptest.NewRequest("POST", "/api/geocode", bytes.NewBufferString(`{"input":"x"}`)))
	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	body, _ := json.Marshal(map[string]any{"token": result.Token, "index": 0})
	r := httptest.NewRequest("POST", "/api/geocode/confirm", bytes.NewReader(body))
	r.Header.Set("Authorization", "bearer verified-agent")
	w = httptest.NewRecorder()
	a.web(w, r)
	if w.Code != 422 {
		t.Fatalf("cross-boundary confirmation %d", w.Code)
	}
}

func TestGeocodeGoogleURLExtractsAddressNotURL(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	got := ""
	a.geocode = func(_ context.Context, input string) ([]destination.Candidate, error) {
		got = input
		return []destination.Candidate{{Label: "Place", Latitude: 1, Longitude: 2}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", bytes.NewBufferString(`{"input":"https://www.google.com/maps/search/?api=1&query=Pra%C3%A7a"}`)))
	if w.Code != 200 || got != "Praça" {
		t.Fatalf("status %d search %q", w.Code, got)
	}
}

func TestAgentGeocodeRequiresAuthorization(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, path := range []string{"/agent/geocode", "/agent/geocode/confirm"} {
		w := httptest.NewRecorder()
		New(s, "key").ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewBufferString(`{}`)))
		if w.Code != 401 {
			t.Fatalf("%s = %d", path, w.Code)
		}
	}
}

func TestGeocodeRequiresSelectionAndRejectsReplay(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, _ string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "<b>Place</b>", Latitude: 1, Longitude: 2}}, nil
	}
	call := func(path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewBufferString(body)))
		return w
	}
	w := call("/api/geocode", `{"input":"place"}`)
	if w.Code != 200 {
		t.Fatalf("search: %d %s", w.Code, w.Body)
	}
	var result struct {
		Token      string                  `json:"token"`
		Candidates []destination.Candidate `json:"candidates"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Candidates) != 1 || result.Token == "" {
		t.Fatal(w.Body)
	}
	p, _ := s.Pending()
	if p != nil {
		t.Fatal("search queued command")
	}
	body, _ := json.Marshal(map[string]any{"token": result.Token, "index": 0})
	w = call("/api/geocode/confirm", string(body))
	if w.Code != 201 {
		t.Fatalf("confirm: %d %s", w.Code, w.Body)
	}
	p, _ = s.Pending()
	if p == nil || p.URI != "https://waze.com/ul?ll=1%2C2&navigate=yes" {
		t.Fatalf("pending %+v", p)
	}
	w = call("/api/geocode/confirm", string(body))
	if w.Code != 409 {
		t.Fatalf("replay: %d", w.Code)
	}
}
