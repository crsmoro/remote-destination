package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"waze-remoto/internal/store"
)

func TestDeviceFlow(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "x.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := New(s, "device-key")
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("POST", "/device/poll", "{}", ""); w.Code != 401 {
		t.Fatalf("auth %d", w.Code)
	}
	if w := call("POST", "/api/destinations", `{"input":"X"}`, "device-key"); w.Code != 401 {
		t.Fatalf("device key user access %d", w.Code)
	}
	w := call("POST", "/api/destinations", `{"input":"https://waze.com/ul?ll=1,2"}`, "")
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	w = call("POST", "/device/poll", "{}", "device-key")
	if w.Code != 200 {
		t.Fatalf("poll %d %s", w.Code, w.Body)
	}
	var poll struct {
		Command *store.Command `json:"command"`
	}
	json.Unmarshal(w.Body.Bytes(), &poll)
	if poll.Command == nil || poll.Command.ID != created.ID {
		t.Fatal(w.Body)
	}
	w = call("POST", "/device/claim", `{"id":"`+created.ID+`"}`, "device-key")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	body := `{"id":"` + created.ID + `","result":"intent_sent"}`
	for i := 0; i < 2; i++ {
		w = call("POST", "/device/ack", body, "device-key")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	w = call("POST", "/device/claim", `{"id":"`+created.ID+`"}`, "device-key")
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	w = call("GET", "/api/status", "", "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"online":true`)) {
		t.Fatal(w.Code, w.Body)
	}
}
