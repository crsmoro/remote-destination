package api

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"waze-remoto/internal/store"
)

func TestWebSecurityAndLimits(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := New(s, "secret")
	tests := []struct {
		method, path, body, auth, origin string
		code                             int
	}{
		{"POST", "/device/poll", "{}", "Bearer incorrect", "", 401},
		{"POST", "/api/destinations", `{"input":" "}`, "", "", 422},
		{"POST", "/api/destinations", `{"input":"` + strings.Repeat("a", 9000) + `"}`, "", "", 400},
		{"POST", "/api/destinations", `{"input":"abc"}`, "", "https://attacker.invalid", 403},
		{"GET", "/", "", "", "", 200},
		{"GET", "/api/status", "", "Bearer secret", "", 401},
	}
	for _, v := range tests {
		r := httptest.NewRequest(v.method, v.path, bytes.NewBufferString(v.body))
		r.Header.Set("Authorization", v.auth)
		r.Header.Set("Origin", v.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != v.code {
			t.Errorf("%s %s: got %d want %d", v.method, v.path, w.Code, v.code)
		}
	}
}
