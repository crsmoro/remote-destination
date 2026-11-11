package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticPendingRetainsOriginalText(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Mercado Exemplo, Joinville", City: "Joinville", Latitude: -26, Longitude: -48}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado Exemplo"}`)))
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	status, err := s.Status()
	if err != nil || status.Display != "Mercado Exemplo" {
		t.Fatalf("status %+v %v", status, err)
	}
}
