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

func TestMobileQFtidSearchDoesNotSend(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		if q != "Mercado São João, Joinville" {
			t.Errorf("wrong query: %q", q)
		}
		return []destination.Candidate{{GoogleResult: true, GoogleName: "Mercado São João", Label: "Mercado São João, Joinville", City: "Joinville", Latitude: 3, Longitude: 4}}, nil
	}
	body, _ := json.Marshal(map[string]string{"input": "https://maps.google.com/maps?q=Mercado+S%C3%A3o+Jo%C3%A3o%2C+Joinville&ftid=0xabc:0x123&g_st=ic"})
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(string(body))))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Mercado São João") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p != nil {
		t.Fatalf("unexpected pending: %+v %v", p, err)
	}
}

func TestMobileQFtidAutomaticUsesGoogleTarget(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "key").(*server)
	a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
		if q != "Mercado São João, Joinville" {
			t.Errorf("unexpected query %q", q)
		}
		return []destination.Candidate{{GoogleResult: true, GoogleName: "Mercado São João", Label: "Mercado São João, Joinville", City: "Joinville", Latitude: 3, Longitude: 4}}, nil
	}
	body, _ := json.Marshal(map[string]string{"input": "https://maps.google.com/maps?q=Mercado+S%C3%A3o+Jo%C3%A3o%2C+Joinville&ftid=0xabc:0x123"})
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(string(body))))
	if w.Code != 201 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	p, err := s.Pending()
	if err != nil || p == nil || p.URI != "https://waze.com/ul?ll=3%2C4&navigate=yes" {
		t.Fatalf("wrong local destination: %+v %v", p, err)
	}
}
