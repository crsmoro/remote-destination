package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"waze-remoto/internal/destination"
	"waze-remoto/internal/store"
)

func TestMobileTextReachesGeocoderUnchanged(t *testing.T) {
	for _, text := range []string{"123", "Avenida 50% Legal", "  Praça São João  "} {
		t.Run(text, func(t *testing.T) {
			s, err := store.Open(t.TempDir() + "/db")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s, "key").(*server)
			var got string
			a.geocode = func(_ context.Context, q string) ([]destination.Candidate, error) {
				got = q
				return []destination.Candidate{{GoogleResult: true, GoogleName: "Lugar", Label: "Lugar", Latitude: 3, Longitude: 4}}, nil
			}
			b, _ := json.Marshal(map[string]string{"input": "https://maps.google.com/maps?ftid=0xa:0xb&q=" + url.QueryEscape(text)})
			w := httptest.NewRecorder()
			a.ServeHTTP(w, httptest.NewRequest("POST", "/api/geocode", strings.NewReader(string(b))))
			if got != text {
				t.Fatalf("query %q, want %q (status %d: %s)", got, text, w.Code, w.Body.String())
			}
		})
	}
}
