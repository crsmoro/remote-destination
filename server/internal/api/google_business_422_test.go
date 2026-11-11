package api

import (
	"context"
	"net/http/httptest"

	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleBusinessFormattedAddressRequiresVerifiedName(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		c := destination.Candidate{AddressOnlyGoogle: true, Label: "Rua das Acácias, 123, Campinas, Brasil", City: "Campinas", Latitude: -22.9, Longitude: -47.1}
		return []destination.Candidate{c}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"xy atacado"}`)))
	if w.Code != 422 {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	if pending, err := s.Pending(); err != nil || pending != nil {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}
