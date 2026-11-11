package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticSendSelectsNearestWithinExplicitCity(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{
			{Label: "Far", City: "Joinville", Latitude: -27, Longitude: -49},
			{Label: "Near", City: "Joinville", Latitude: -26.3045, Longitude: -48.8487},
			{Label: "Wrong city", City: "Outra", Latitude: -26.3045, Longitude: -48.8487},
		}, nil
	}
	t.Setenv("WAZE_POSITION_FILE", "")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado Joinville"}`)))
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"display":"Near"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
