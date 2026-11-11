package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestIdempotencyRetryAfterGeocodingFailure(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := New(s, "key")
	calls := 0
	if err := SetGeocoder(h, func(context.Context, string) ([]destination.Candidate, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("offline")
		}
		return []destination.Candidate{{Label: "Mercado", Latitude: 1, Longitude: 2}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	call := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Mercado"}`))
		r.Header.Set("Idempotency-Key", "recover")
		h.ServeHTTP(w, r)
		return w
	}
	if w := call(); w.Code != 502 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call(); w.Code != 201 {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
