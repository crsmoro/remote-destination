package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticStreetIdentity(t *testing.T) {
	for _, tc := range []struct {
		input, label string
		status       int
	}{
		{"Rua Correta 123", "Rua Errada, 123, Campinas", 422},
		{"Rua Correta 123", "Rua Correta Errada, 123, Campinas", 422},
		{"Rua Correta 123", "Rua Correta, 999, Campinas", 422},
		{"Rua Correta, 123", "Rua Correta, 999, Campinas", 422},
		{"Av. das Acácias 123", "Avenida Acacias, 123, Campinas", 201},
		{"Rua das Flores 100", "Avenida das Flores, 100, Campinas", 422},
		{"Avenida das Flores 100", "Rua das Flores, 100, Campinas", 422},
		{"R. das Flores 100", "Rua das Flores, 100, Campinas", 201},
		{"Rua das Flores 100", "R. das Flores, 100, Campinas", 201},
		{"Rua das Flores 100", "Flores, 100, Campinas", 422},
	} {
		t.Run(tc.input+"/"+tc.label, func(t *testing.T) {
			s, err := store.Open(t.TempDir() + "/db")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
				return []destination.Candidate{{AddressOnlyGoogle: true, Label: tc.label, City: "Campinas", Latitude: -22, Longitude: -47}}, nil
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"`+tc.input+`"}`)))
			if w.Code != tc.status {
				t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 422 {
				if pending, err := s.Pending(); err != nil || pending != nil {
					t.Fatalf("pending=%+v err=%v", pending, err)
				}
			}
		})
	}
}
