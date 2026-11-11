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

func TestTextCityMatchPrioritizesWithoutDiscardingOthers(t *testing.T) {
	t.Setenv("WAZE_POSITION_FILE", "")
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
		var c []destination.Candidate
		json.Unmarshal([]byte(`[{"label":"local","city":"Joinville","latitude":-26.3,"longitude":-48.85},{"label":"requested","city":"São Paulo","latitude":-23.5,"longitude":-46.6}]`), &c)
		return c, nil
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(`{"input":"padaria em SAO PAULO"}`)))
	var result struct {
		Reference  string                  `json:"reference"`
		Candidates []destination.Candidate `json:"candidates"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || result.Reference != "possible_text_city" || len(result.Candidates) != 2 || result.Candidates[0].Label != "requested" {
		t.Fatalf("unexpected: %s", w.Body.String())
	}
}
