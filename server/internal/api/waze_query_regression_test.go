package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestWazeQueryIsDecodedAndRequiresSelection(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	var query string
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		query = q
		return []destination.Candidate{{Label: "Rua A", Latitude: 1, Longitude: 2}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"https://waze.com/ul?q=Pra%C3%A7a+A&navigate=yes"}`)))
	if w.Code != 200 || query != "Praça A" || !strings.Contains(w.Body.String(), `"token"`) {
		t.Fatalf("status=%d query=%q body=%s", w.Code, query, w.Body.String())
	}
}

func TestLongLabelRetainsAddressSuffix(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: strings.Repeat("A", 140) + ", Rua das Flores 123", Latitude: 1, Longitude: 2}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"local"}`)))
	var v struct {
		Candidates []destination.Candidate `json:"candidates"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || len(v.Candidates) != 1 {
		t.Fatalf("bad response: %s", w.Body.String())
	}
	if !strings.Contains(v.Candidates[0].Label, "Rua das Flores 123") {
		t.Fatalf("address lost: %q", v.Candidates[0].Label)
	}
}

func TestWazeQueryRejectsAmbiguousAndUnsafe(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	calls := 0
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) { calls++; return nil, nil }
	for _, input := range []string{"https://waze.com/ul?q=x&ll=1,2", "https://waze.com/ul?q=x&q=y", "https://waze.com/ul?q=http%3A%2F%2F127.0.0.1", "https://waze.com/ul?q=%ZZ", "https://waze.com.evil.test/ul?q=x", "https://waze.com/ul?q=x&venue_id=1"} {
		body, _ := json.Marshal(map[string]string{"input": input})
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(string(body))))
		if w.Code != 422 {
			t.Errorf("%s: %d", input, w.Code)
		}
	}
	if calls != 0 {
		t.Fatalf("unexpected lookups %d", calls)
	}
}
