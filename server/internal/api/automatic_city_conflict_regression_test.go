package api

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticExplicitCityConflicts(t *testing.T) {
	file := t.TempDir() + "/municipios.json"
	if err := os.WriteFile(file, syntheticSnapshot([]municipality{{3550308, "São Paulo", "SP"}, {3509502, "Campinas", "SP"}}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_MUNICIPIOS_FILE", file)
	for _, tc := range []struct {
		input, city, candidateCity, state string
		status                            int
	}{
		{"Rua das Flores 100, São Paulo", "Campinas", "Campinas", "SP", 422},
		{"Rua das Flores 100, Campinas", "Campinas", "Campinas", "SP", 201},
		{"Rua das Flores 100, Campinas - RJ", "Campinas", "Campinas", "SP", 422},
	} {
		t.Run(tc.input, func(t *testing.T) {
			s, err := store.Open(t.TempDir() + "/db")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
				return []destination.Candidate{{Label: "Rua das Flores, 100, Campinas", City: tc.candidateCity, State: tc.state, Latitude: -22, Longitude: -47}}, nil
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(fmt.Sprintf(`{"input":%q,"city":%q}`, tc.input, tc.city))))
			if w.Code != tc.status {
				t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 422 {
				if p, e := s.Pending(); e != nil || p != nil {
					t.Fatalf("pending=%+v err=%v", p, e)
				}
			}
		})
	}
}
