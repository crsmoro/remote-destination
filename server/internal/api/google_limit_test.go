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

func TestGoogleLimitReturns429(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) { return nil, destination.ErrGoogleLimit }
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"test"}`)))
	if w.Code != 429 || !strings.Contains(w.Body.String(), "limit") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
}
