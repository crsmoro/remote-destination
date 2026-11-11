package destination

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoogleAcceptsCoordinatesWithoutLabels(t *testing.T) {
	for _, tc := range []struct {
		primary, label string
		valid          bool
	}{
		{`{"location":{"latitude":0,"longitude":0}}`, "Consulta original", true},
		{`{"displayName":{"text":"Nome"},"location":{"latitude":0,"longitude":0}}`, "Nome", true},
		{`{"formattedAddress":"Rua A","location":{"latitude":0,"longitude":0}}`, "Rua A", true},
		{`{"displayName":{"text":"Nome"}}`, "", false},
		{`{"location":{"latitude":0}}`, "", false},
		{`{"location":{"latitude":91,"longitude":0}}`, "", false},
	} {
		t.Run(tc.primary, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"destinations":[{"primary":` + tc.primary + `}]}`))
			}))
			defer s.Close()
			c := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
			got, err := searchGoogle(context.Background(), c, "key", "Consulta original")
			if tc.valid {
				if err != nil || len(got) != 1 || got[0].Label != tc.label {
					t.Fatalf("got %#v err %v", got, err)
				}
			} else if err == nil {
				t.Fatalf("expected error, got %#v", got)
			}
		})
	}
}
