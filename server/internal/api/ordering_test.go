package api

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
	"waze-remoto/internal/store"
)

func TestSlowGoogleResolutionCannotOverrideNewerOperations(t *testing.T) {
	for _, operation := range []string{"submit", "cancel", "cancel-empty"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db")
			s, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			other, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if operation == "cancel" {
				if err := s.Put("existing", "uri", ""); err != nil {
					t.Fatal(err)
				}
			}
			entered, release := make(chan struct{}, 1), make(chan struct{})
			old := net.DefaultResolver
			net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				select {
				case entered <- struct{}{}:
				default:
				}
				select {
				case <-release:
				case <-ctx.Done():
				}
				return nil, errors.New("blocked DNS")
			}}
			defer func() { net.DefaultResolver = old }()
			h := New(s, "key")
			call := func(body string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/destinations", bytes.NewBufferString(body)))
				return w
			}
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- call(`{"input":"https://maps.app.goo.gl/slow"}`) }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				close(release)
				t.Fatal("DNS not entered")
			}
			newer := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				w := httptest.NewRecorder()
				if operation == "submit" {
					New(other, "key").ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/destinations", bytes.NewBufferString(`{"input":"https://waze.com/ul?ll=1,2"}`)))
				} else {
					New(other, "key").ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/destinations/pending", nil))
				}
				newer <- w
			}()
			select {
			case w := <-newer:
				want := 201
				if operation == "cancel" {
					want = 200
				}
				if operation == "cancel-empty" {
					want = 409
				}
				if w.Code != want {
					close(release)
					t.Fatalf("new submit: %d", w.Code)
				}
			case <-time.After(2 * time.Second):
				close(release)
				t.Fatal("new submit blocked by DNS")
			}
			close(release)
			if w := <-done; w.Code != 422 {
				t.Errorf("old submit status = %d, want 422", w.Code)
			}
			c, err := s.Pending()
			if err != nil {
				t.Fatal(err)
			}
			if operation == "submit" {
				if c == nil || c.URI != "https://waze.com/ul?ll=1%2C2&navigate=yes" {
					t.Fatalf("pending = %#v", c)
				}
				if err = s.Claim(c.ID); err != nil {
					t.Fatal(err)
				}
			} else if c != nil {
				t.Fatalf("unexpected pending = %#v", c)
			}
			if operation == "cancel" {
				if err = s.Claim("existing"); !errors.Is(err, store.ErrConflict) {
					t.Fatalf("cancelled claim: %v", err)
				}
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			after, err := reopened.Pending()
			if err != nil {
				t.Fatal(err)
			}
			if operation == "submit" {
				if after == nil || after.ID != c.ID {
					t.Fatalf("reopened pending = %#v", after)
				}
			} else if after != nil {
				t.Fatalf("reopened pending = %#v", after)
			}
		})
	}
}
