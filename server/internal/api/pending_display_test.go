package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"waze-remoto/internal/store"
)

func TestWebPendingDisplaySurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	s, e := store.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	h := New(s, "key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"https://waze.com/ul?ll=1,2"}`)))
	if w.Code != 201 {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}
	s.Close()
	s, e = store.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	w = httptest.NewRecorder()
	New(s, "key").ServeHTTP(w, httptest.NewRequest("GET", "/api/status", nil))
	var status map[string]any
	if e = json.Unmarshal(w.Body.Bytes(), &status); e != nil {
		t.Fatal(e)
	}
	if status["display"] != "https://waze.com/ul?ll=1,2" {
		t.Fatalf("display after reopen = %#v", status)
	}
}

func TestPendingDisplayLifecycle(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := New(s, "key")
	call := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if strings.HasPrefix(path, "/device/") {
			r.Header.Set("Authorization", "Bearer key")
		}
		h.ServeHTTP(w, r)
		return w
	}
	for _, result := range []string{"cancel", "intent_sent", "failed"} {
		t.Run(result, func(t *testing.T) {
			w := call("POST", "/api/destinations", `{"input":"https://waze.com/ul?ll=1,2"}`)
			if w.Code != 201 {
				t.Fatal(w.Body.String())
			}
			var created map[string]any
			json.Unmarshal(w.Body.Bytes(), &created)
			poll := call("POST", "/device/poll", `{}`)
			if strings.Contains(poll.Body.String(), "display") {
				t.Fatalf("device leaked input: %s", poll.Body.String())
			}
			if result == "cancel" {
				w = call("DELETE", "/api/destinations/pending", "")
			} else {
				b, _ := json.Marshal(map[string]any{"id": created["id"], "result": result})
				w = call("POST", "/device/ack", string(b))
			}
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			w = call("GET", "/api/status", "")
			if strings.Contains(w.Body.String(), "display") || strings.Contains(w.Body.String(), "uri") {
				t.Fatalf("non-pending leaked: %s", w.Body.String())
			}
		})
	}
}
