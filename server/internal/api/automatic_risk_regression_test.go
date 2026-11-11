package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticRejectsWrongExplicitCity(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Farmacia Centro", City: "Joinville", Latitude: -26, Longitude: -48}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Farmacia Centro","city":"Curitiba"}`)))
	if w.Code != 422 {
		t.Fatalf("wrong city accepted: %d %s", w.Code, w.Body.String())
	}
}
func TestAutomaticRejectsSharedGenericCategory(t *testing.T) {
	c := destination.Candidate{Label: "Farmacia outra", City: "Joinville"}
	if plausibleAutomaticCandidate("Farmacia inexistente", c) {
		t.Fatal("generic category matched unrelated result")
	}
}
