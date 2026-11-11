package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestAutomaticSlowGeocodeCannotOverrideLaterOperation(t *testing.T) {
	for _, op := range []string{"send", "cancel"} {
		t.Run(op, func(t *testing.T) {
			s, e := store.Open(t.TempDir() + "/db")
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			entered := make(chan struct{})
			release := make(chan struct{})
			a.geocode = func(context.Context, string) ([]destination.Candidate, error) {
				close(entered)
				<-release
				return []destination.Candidate{{Label: "Old", Latitude: 1, Longitude: 2}}, nil
			}
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				w := httptest.NewRecorder()
				a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"Old"}`)))
				done <- w
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("geocode did not start")
			}
			w := httptest.NewRecorder()
			if op == "send" {
				a.ServeHTTP(w, httptest.NewRequest("POST", "/api/destinations", strings.NewReader(`{"input":"https://waze.com/ul?ll=3,4"}`)))
			} else {
				a.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/destinations/pending", nil))
			}
			if op == "send" && w.Code != 201 {
				t.Fatalf("new send: %d %s", w.Code, w.Body.String())
			}
			if op == "cancel" && w.Code != 409 {
				t.Fatalf("cancel: %d", w.Code)
			}
			close(release)
			old := <-done
			if old.Code != 409 {
				t.Fatalf("old status %d: %s", old.Code, old.Body.String())
			}
			p, e := s.Pending()
			if e != nil {
				t.Fatal(e)
			}
			if op == "send" && (p == nil || !strings.Contains(p.URI, "3%2C4")) {
				t.Fatalf("pending %#v", p)
			}
			if op == "cancel" && p != nil {
				t.Fatalf("pending %#v", p)
			}
		})
	}
}
