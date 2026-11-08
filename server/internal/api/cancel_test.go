package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"waze-remoto/internal/store"
)

func TestCancelWithoutPendingReturnsConflict(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	w := httptest.NewRecorder()
	New(s, "key").ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/destinations/pending", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
