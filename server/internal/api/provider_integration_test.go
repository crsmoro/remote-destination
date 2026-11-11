package api

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestConfiguredGeocoderIsUsedAndRequiresConfirmation(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s := New(db, "key")
	called := false
	err := SetGeocoder(s, func(_ context.Context, input string) ([]destination.Candidate, error) {
		called = true
		return []destination.Candidate{{Label: "synthetic place", Latitude: 1, Longitude: 2}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"synthetic"}`)))
	if !called || w.Code != 200 || !strings.Contains(w.Body.String(), `"token"`) {
		t.Fatal("provider not used or confirmation token missing")
	}
}
