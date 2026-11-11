package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleAddressOnlyCannotVerifyBusiness(t *testing.T) {
	for _, input := range []string{"loja inexistente", "loja Campinas"} {
		t.Run(input, func(t *testing.T) {
			s, err := store.Open(t.TempDir() + "/db")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
				return []destination.Candidate{{AddressOnlyGoogle: true, Label: "Rua Qualquer, 123, Campinas", City: "Campinas", Latitude: -22, Longitude: -47}}, nil
			}
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"`+input+`"}`)))
			if w.Code != 422 {
				t.Fatalf("got %d want 422: %s", w.Code, w.Body.String())
			}
			p, err := s.Pending()
			if err != nil || p != nil {
				t.Fatalf("pending=%+v err=%v", p, err)
			}
		})
	}
}
