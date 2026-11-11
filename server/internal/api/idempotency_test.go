package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func sameReplay(first, replay *httptest.ResponseRecorder) bool {
	var a, b map[string]any
	if json.Unmarshal(first.Body.Bytes(), &a) != nil || json.Unmarshal(replay.Body.Bytes(), &b) != nil || b["replayed"] != true {
		return false
	}
	delete(a, "replayed")
	delete(b, "replayed")
	return reflect.DeepEqual(a, b)
}

func TestDestinationIdempotencyPersistsAfterDelivery(t *testing.T) {
	path := t.TempDir() + "/state.db"
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := New(s, "key")
	call := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/destinations", strings.NewReader(body))
		r.Header.Set("Idempotency-Key", "test-key")
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"input":"https://waze.com/ul?ll=1,2"}`
	first := call(body)
	if first.Code != 201 {
		t.Fatal(first.Code, first.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p == nil {
		t.Fatal(p, err)
	}
	if err = s.Ack(p.ID, "intent_sent"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h = New(s, "key")
	second := call(body)
	if second.Code != 201 || !sameReplay(first, second) {
		t.Fatalf("replay got %d %s, want %s", second.Code, second.Body.String(), first.Body.String())
	}
	if pending, _ := s.Pending(); pending != nil {
		t.Fatal("replay created command", pending)
	}
	conflict := call(`{"input":"https://waze.com/ul?ll=3,4"}`)
	if conflict.Code != 409 {
		t.Fatal(conflict.Code, conflict.Body.String())
	}
}

func TestIdempotencyConcurrentReservationDoesNotSupersede(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := New(s, "key")
	started := make(chan struct{})
	unblock := make(chan struct{})
	if err := SetGeocoder(h, func(ctx context.Context, input string) ([]destination.Candidate, error) {
		close(started)
		<-unblock
		return []destination.Candidate{{Label: "Mercado", Latitude: 1, Longitude: 2}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	call := func(key, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/destinations", strings.NewReader(body))
		r.Header.Set("Idempotency-Key", key)
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"input":"Mercado"}`
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- call("one", body) }()
	<-started
	dupe := call("one", body)
	if dupe.Code != 409 {
		t.Fatal(dupe.Code, dupe.Body.String())
	}
	close(unblock)
	first := <-done
	if first.Code != 201 {
		t.Fatal(first.Code, first.Body.String())
	}
	replay := call("one", body)
	if replay.Code != 201 || !sameReplay(first, replay) {
		t.Fatal(replay.Code, replay.Body.String())
	}
	later := call("two", `{"input":"https://waze.com/ul?ll=3,4"}`)
	if later.Code != 201 {
		t.Fatal(later.Code, later.Body.String())
	}
	prior := call("one", body)
	if prior.Code != 201 || !sameReplay(first, prior) {
		t.Fatal(prior.Code, prior.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p == nil || !strings.Contains(p.URI, "3%2C4") {
		t.Fatal(p, err)
	}
}

func TestIdempotencyIsScopedToVerifiedAgentSubject(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	auth := &agentAuth{issuer: "https://issuer/", audience: "aud", clients: map[string]bool{"a": true, "b": true}, keys: map[string]*rsa.PublicKey{"one": &key.PublicKey}, expires: time.Now().Add(time.Hour)}
	h := &server{s: s, agent: auth}
	body := `{"input":"https://waze.com/ul?ll=1,2"}`
	call := func(client string) *httptest.ResponseRecorder {
		token := testToken(t, key, map[string]any{"iss": auth.issuer, "aud": auth.audience, "exp": time.Now().Unix() + 300, "azp": client, "sub": client + "@clients", "scope": "destination:write"})
		r := httptest.NewRequest("POST", "/agent/destinations", strings.NewReader(body))
		r.Header.Set("Authorization", "bearer "+token)
		r.Header.Set("Idempotency-Key", "shared")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	a := call("a")
	b := call("b")
	again := call("a")
	if a.Code != 201 || b.Code != 201 || again.Code != 201 || !sameReplay(a, again) || a.Body.String() == b.Body.String() {
		t.Fatalf("a=%d %s b=%d %s replay=%d %s", a.Code, a.Body, b.Code, b.Body, again.Code, again.Body)
	}
}
