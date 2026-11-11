package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGoogleBusinessExplicitCityDoesNotVerifyName(t *testing.T) {
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{
			{AddressOnlyGoogle: true, Label: "Rua Distante, Joinville", City: "Joinville", Latitude: -28, Longitude: -50},
			{AddressOnlyGoogle: true, Label: "Rua Perto, Joinville", City: "Joinville", Latitude: -26.3045, Longitude: -48.8487},
		}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"xy atacado","city":"Joinville"}`)))
	if w.Code != 422 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if pending, err := s.Pending(); err != nil || pending != nil {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
}
