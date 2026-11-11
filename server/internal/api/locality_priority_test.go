package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestFallbackRanksNearbyButRetainsAllCandidates(t *testing.T) {
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		return []destination.Candidate{{Label: "far", Latitude: -23.5, Longitude: -46.6}, {Label: "near", Latitude: -26.3, Longitude: -48.85}}, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"padaria"}`)))
	var result struct {
		Reference  string                  `json:"reference"`
		Candidates []destination.Candidate `json:"candidates"`
		Token      string                  `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || result.Reference != "joinville" || len(result.Candidates) != 2 || result.Candidates[0].Label != "near" || result.Token == "" {
		t.Fatalf("unexpected: %s", w.Body.String())
	}
}
