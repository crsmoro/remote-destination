package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestStalePositionSnapshotFallsBack(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "position.json")
	if err := os.WriteFile(file, []byte(`{"available":true,"latitude":-23.5,"longitude":-46.6}`), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_POSITION_FILE", file)
	_, _, ref := positionReference()
	if ref != "joinville" {
		t.Fatalf("stale snapshot used: %s", ref)
	}
}

func TestGPSReferenceAndExplicitCity(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "position.json")
	if err := os.WriteFile(file, []byte(`{"available":true,"latitude":-23.5,"longitude":-46.6}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_POSITION_FILE", file)
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	var query string
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		query = q
		return []destination.Candidate{{Label: "joinville", Latitude: -26.3, Longitude: -48.85}, {Label: "sao paulo", Latitude: -23.5, Longitude: -46.6}}, nil
	}
	call := func(body string) (string, string) {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", bytes.NewBufferString(body)))
		var v struct {
			Reference  string                  `json:"reference"`
			Candidates []destination.Candidate `json:"candidates"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || w.Code != 200 || len(v.Candidates) != 2 {
			t.Fatalf("bad response %d", w.Code)
		}
		return v.Reference, v.Candidates[0].Label
	}
	reference, first := call(`{"input":"padaria"}`)
	if reference != "gps_unverified_age" || first != "sao paulo" {
		t.Fatalf("GPS ranking %s %s", reference, first)
	}
	reference, first = call(`{"input":"padaria","city":"Joinville SC"}`)
	if reference != "explicit_city" || query != "padaria, Joinville SC" || first != "joinville" {
		t.Fatalf("city priority %s %q %s", reference, query, first)
	}
}
