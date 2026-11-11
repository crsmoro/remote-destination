package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleBusinessCityMismatchRejected(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{AddressOnlyGoogle: true, Label: "Rua das Acácias, 123, Curitiba", City: "Curitiba", Latitude: -25, Longitude: -49}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"xy atacado","city":"Joinville"}`)))
	if w.Code != 422 {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p != nil {
		t.Fatalf("pending=%+v err=%v", p, err)
	}
}
