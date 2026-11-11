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

func TestGoogleAutomaticRanksAllReceivedCandidates(t *testing.T) {
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		var c []destination.Candidate
		for i := 0; i < 6; i++ {
			c = append(c, destination.Candidate{GoogleResult: true, Label: "distante", Latitude: 0, Longitude: 0})
		}
		c = append(c, destination.Candidate{GoogleResult: true, Label: "mais próximo", Latitude: -26.3045, Longitude: -48.8487})
		return c, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"busca"}`)))
	var result struct {
		Display string `json:"display"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 201 || result.Display != "mais próximo" {
		t.Fatalf("status=%d display=%q", w.Code, result.Display)
	}
}
