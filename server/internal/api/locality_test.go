package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestGeocodeRanksFallbackWithoutExplicitReference(t *testing.T) {
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "longe", Latitude: -23.5, Longitude: -46.6}, {Label: "perto", Latitude: -26.3, Longitude: -48.85}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", bytes.NewBufferString(`{"input":"padaria"}`)))
	var result struct {
		Candidates []destination.Candidate `json:"candidates"`
		Reference  string                  `json:"reference"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(result.Candidates) != 2 || result.Candidates[0].Label != "perto" || result.Reference != "joinville" {
		t.Fatalf("unexpected result: %s", w.Body.String())
	}
}
