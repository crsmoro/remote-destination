package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticTextAndWazeQueryResolveButAmbiguousLinkRejected(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	var lookups []string
	a.geocode = func(_ context.Context, input string) ([]destination.Candidate, error) {
		lookups = append(lookups, input)
		return []destination.Candidate{{Label: "Rua A", Latitude: 1, Longitude: 2}}, nil
	}
	for _, input := range []string{"Rua A", "https://waze.com/ul?q=Rua+A"} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"`+input+`"}`)))
		if w.Code != 201 {
			t.Errorf("%s: got %d", input, w.Code)
		}
	}
	if len(lookups) != 2 || lookups[0] != "Rua A" || lookups[1] != "Rua A" {
		t.Fatalf("lookups: %#v", lookups)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"https://waze.com/ul?ll=1,2&q=x"}`)))
	if w.Code != 422 || len(lookups) != 2 {
		t.Fatalf("ambiguous URL: %d, lookups=%#v", w.Code, lookups)
	}
}

func TestReviewLongLabelsProduceUsableConfirmation(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		c := make([]destination.Candidate, 5)
		for i := range c {
			c[i] = destination.Candidate{Label: strings.Repeat("界", 2000), Latitude: 1, Longitude: 2}
		}
		return c, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"x"}`)))
	if w.Code != 200 {
		t.Fatalf("search %d", w.Code)
	}
	var v struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(w.Body.Bytes(), &v) != nil {
		t.Fatal("bad JSON")
	}
	body, _ := json.Marshal(map[string]any{"token": v.Token, "index": 0})
	if len(body) > 8192 {
		t.Fatalf("confirmation body %d exceeds limit", len(body))
	}
	w = httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode/confirm", bytes.NewReader(body)))
	if w.Code != 201 {
		t.Fatalf("confirm %d", w.Code)
	}
}
func TestReviewCityInTextKeepsProviderOrder(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Sao Paulo", City: "São Paulo", Latitude: -23.5, Longitude: -46.6}, {Label: "Joinville", City: "Joinville", Latitude: -26.3, Longitude: -48.85}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"padaria São Paulo"}`)))
	var v struct {
		Candidates []destination.Candidate `json:"candidates"`
	}
	json.Unmarshal(w.Body.Bytes(), &v)
	if w.Code != 200 || len(v.Candidates) != 2 || v.Candidates[0].Label != "Sao Paulo" {
		t.Fatalf("provider order changed: %s", w.Body.String())
	}
}
