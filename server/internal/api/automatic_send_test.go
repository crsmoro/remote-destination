package api

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticSendTextQueuesNearestCandidate(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, input string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "Mercado Distante", Latitude: 0, Longitude: 0}, {Label: "Mercado Joinville", Latitude: -26.3044, Longitude: -48.8464}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", bytes.NewBufferString(`{"input":"Mercado"}`)))
	if w.Code != 201 {
		t.Fatalf("expected automatic send, got %d: %s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p == nil || p.URI != "https://waze.com/ul?ll=-26.3044%2C-48.8464&navigate=yes" {
		t.Fatalf("pending=%+v err=%v", p, err)
	}
}
