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

func TestMunicipalityBusinessContext(t *testing.T) {
	file := t.TempDir() + "/index.json"
	rows := []municipality{{2908205, "Central", "BA"}, {4106902, "Curitiba", "PR"}, {4209102, "Joinville", "SC"}, {3500001, "Cidade", "SP"}, {3500002, "São José", "SP"}, {4200001, "São José", "SC"}, {3500003, "Domingo", "SP"}}
	if err := os.WriteFile(file, syntheticSnapshot(rows), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", file)
	cases := []struct {
		input         string
		candidates    []destination.Candidate
		matched, fail bool
	}{
		{"Mercado Central", []destination.Candidate{{City: "Joinville"}}, false, false},
		{"Shopping Cidade", []destination.Candidate{{City: "Joinville"}}, false, false},
		{"Sao Jose", []destination.Candidate{{City: "Joinville"}}, false, false},
		{"Mercado Domingo", []destination.Candidate{{City: "Joinville"}}, false, false},
		{"Mercado, Central", []destination.Candidate{{City: "Joinville"}}, true, true},
		{"Mercado - Central", []destination.Candidate{{City: "Joinville"}}, true, true},
		{"Mercado Central BA", []destination.Candidate{{City: "Joinville"}}, true, true},
		{"Mercado Curitiba", []destination.Candidate{{City: "Joinville"}}, true, true},
		{"Mercado Central", []destination.Candidate{{City: "Central", State: "BA"}, {City: "Joinville"}}, true, false},
		{"Sao Jose", []destination.Candidate{{City: "São José", State: "SC"}}, true, true},
		{"Sao Jose SC", []destination.Candidate{{City: "São José", State: "SC"}}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			_, matched, err := constrainMunicipality(tc.input, tc.candidates)
			if matched != tc.matched || (err != nil) != tc.fail {
				t.Fatalf("matched=%v err=%v", matched, err)
			}
		})
	}
}

func TestAutomaticBusinessMunicipalityWordNearest(t *testing.T) {
	file := t.TempDir() + "/index.json"
	if err := os.WriteFile(file, syntheticSnapshot([]municipality{{2908205, "Central", "BA"}, {4106902, "Curitiba", "PR"}, {4209102, "Joinville", "SC"}}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", file)
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Mercado Central Curitiba", City: "Curitiba", Latitude: -25.43, Longitude: -49.27}, {Label: "Mercado Central Joinville", City: "Joinville", Latitude: -26.3044, Longitude: -48.8464}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado Central"}`)))
	if w.Code != 201 {
		t.Fatalf("expected nearest business, got %d: %s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p == nil || !strings.Contains(p.URI, "-26.3044%2C-48.8464") {
		t.Fatalf("pending=%+v err=%v", p, err)
	}
	position := t.TempDir() + "/position.json"
	if err := os.WriteFile(position, []byte(`{"available":true,"latitude":-25.43,"longitude":-49.27}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_POSITION_FILE", position)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado Central"}`)))
	if w.Code != 201 {
		t.Fatalf("GPS status %d: %s", w.Code, w.Body.String())
	}
	p, err = s.Pending()
	if err != nil || p == nil || !strings.Contains(p.URI, "-25.43%2C-49.27") {
		t.Fatalf("GPS pending=%+v err=%v", p, err)
	}
	previous := p.ID
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) { return nil, nil }
	w = httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado Central"}`)))
	p, err = s.Pending()
	if w.Code != 422 || err != nil || p == nil || p.ID != previous {
		t.Fatalf("empty status=%d pending=%+v err=%v", w.Code, p, err)
	}
}
