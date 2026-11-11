package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticExplicitCityRequiresMunicipalityIndex(t *testing.T) {
	t.Setenv("WAZE_MUNICIPIOS_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Rua das Flores, 100, Campinas", City: "Campinas", State: "SP", Latitude: -22, Longitude: -47}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Rua das Flores 100, São Paulo","city":"Campinas"}`)))
	if w.Code != 422 {
		t.Fatalf("got %d want 422: %s", w.Code, w.Body.String())
	}
	if p, err := s.Pending(); err != nil || p != nil {
		t.Fatalf("pending=%+v err=%v", p, err)
	}
}
