package destination

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync/atomic"
    "testing"
)

func TestGoogleDoesNotFollowRedirect(t *testing.T) {
    var first, redirected atomic.Int32
    s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/redirected" { redirected.Add(1); w.WriteHeader(http.StatusOK); return }
        first.Add(1)
        http.Redirect(w, r, "/redirected", http.StatusFound)
    }))
    defer s.Close()
    client := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
    _, err := searchGoogle(context.Background(), client, "synthetic-key", "Rua Exemplo")
    if err == nil || first.Load() != 1 || redirected.Load() != 0 {
        t.Fatalf("expected rejected redirect, first=%d redirected=%d error=%v", first.Load(), redirected.Load(), err)
    }
}
