package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleCommercialNameAndExplicitCityOrdering(t *testing.T) {
	for _, tc := range []struct{ body, query, coord string }{
		{`{"input":"Escola Joinville"}`, "Escola Joinville", "-26.3045%2C-48.8487"},
		{`{"input":"Loja, Curitiba"}`, "Loja, Curitiba", "-28%2C-50"},
		{`{"input":"Loja","city":"Curitiba"}`, "Loja, Curitiba", "-28%2C-50"},
		{`{"input":"Loja","city":""}`, "Loja", "-26.3045%2C-48.8487"},
		{`{"input":"  Loja  "}`, "  Loja  ", "-26.3045%2C-48.8487"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			t.Setenv("WAZE_POSITION_FILE", "")
			t.Setenv("WAZE_GEOCODING_PROVIDER", "google")
			s, err := store.Open(t.TempDir() + "/db")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(_ context.Context, query string) ([]destination.Candidate, error) {
				if query != tc.query {
					t.Errorf("query=%q want %q", query, tc.query)
				}
				return []destination.Candidate{{GoogleResult: true, Label: "Primeiro", City: "Curitiba", Latitude: -28, Longitude: -50}, {GoogleResult: true, Label: "Segundo", City: "Joinville", Latitude: -26.3045, Longitude: -48.8487}}, nil
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(tc.body)))
			p, err := s.Pending()
			if w.Code != 201 || err != nil || p == nil || !strings.Contains(p.URI, tc.coord) {
				t.Fatalf("status=%d body=%s pending=%+v err=%v", w.Code, w.Body.String(), p, err)
			}
		})
	}
}
