package api

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"waze-remoto/internal/store"
)

func TestUserRoutesRejectBearerInAnyAuthorizationHeader(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := httptest.NewRequest("GET", "/api/status", nil)
	r.Header.Add("Authorization", "Basic ignored")
	r.Header.Add("Authorization", "Bearer agent-token")
	w := httptest.NewRecorder()
	New(s, "device-key").ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("got %d, want 401", w.Code)
	}
}
