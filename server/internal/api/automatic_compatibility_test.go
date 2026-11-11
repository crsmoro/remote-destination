package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticRejectsClearlyUnrelatedResults(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Loja, Brasília, Brasil", City: "Brasília", Latitude: -15.79, Longitude: -47.88}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"ZX Armazem"}`)))
	if w.Code != 422 {
		t.Fatalf("expected rejection, got %d: %s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p != nil {
		t.Fatalf("pending=%+v err=%v", p, err)
	}
}
