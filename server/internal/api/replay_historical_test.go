package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"waze-remoto/internal/store"
)

func TestReplayMarksHistoricalResponse(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := New(s, "key")
	call := func() map[string]any {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"https://waze.com/ul?ll=1,2"}`))
		r.Header.Set("Idempotency-Key", "retry")
		h.ServeHTTP(w, r)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var v map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	first := call()
	if first["replayed"] != false {
		t.Fatalf("new command replay marker: %v", first)
	}
	p, e := s.Pending()
	if e != nil || p == nil {
		t.Fatal(p, e)
	}
	if e = s.Ack(p.ID, "intent_sent"); e != nil {
		t.Fatal(e)
	}
	replay := call()
	if replay["replayed"] != true || replay["id"] != first["id"] || replay["state"] != "pending" {
		t.Fatalf("historical replay: %v", replay)
	}
	if pending, _ := s.Pending(); pending != nil {
		t.Fatal("replay created command")
	}
}
