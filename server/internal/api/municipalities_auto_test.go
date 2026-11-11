package api

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestMunicipalityConflictDoesNotQueue(t *testing.T) {
	file := t.TempDir() + "/municipios.json"
	if err := os.WriteFile(file, syntheticSnapshot([]municipality{{4106902, "Curitiba", "PR"}, {4209102, "Joinville", "SC"}}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", file)
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Mercado Curitiba", City: "Joinville", Latitude: -26, Longitude: -48}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado Curitiba"}`)))
	if w.Code != 422 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p != nil {
		t.Fatalf("pending=%+v err=%v", p, err)
	}
}

func TestMunicipalityCorruptionBlocksExplicitCity(t *testing.T) {
	file := t.TempDir() + "/index.json"
	if err := os.WriteFile(file, syntheticSnapshot(nil), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", file)
	if err := ValidateMunicipalityIndex(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`[]`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Farmacia Centro", City: "Curitiba", State: "PR", Latitude: -25, Longitude: -49}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Farmacia Centro","city":"Curitiba"}`)))
	if w.Code != 422 {
		t.Fatalf("corrupt index accepted: %d", w.Code)
	}
	pending, err := s.Pending()
	if err != nil || pending != nil {
		t.Fatal("destination queued")
	}
}
