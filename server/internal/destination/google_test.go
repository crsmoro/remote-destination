package destination

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type googleTestTransport struct {
	target string
	client *http.Client
}

func (tr googleTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme = "http"
	r.URL.Host = tr.target
	return tr.client.Transport.RoundTrip(r)
}

func TestGoogleAddressQueryHTTP(t *testing.T) {
	for _, query := range []string{"Loja, Curitiba", "Loja", "  Loja  "} {
		t.Run(query, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					AddressQuery struct {
						AddressQuery string `json:"addressQuery"`
					} `json:"addressQuery"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.AddressQuery.AddressQuery != query {
					t.Errorf("upstream query=%q want %q; err=%v", payload.AddressQuery.AddressQuery, query, err)
				}
				io.WriteString(w, `{}`)
			}))
			defer s.Close()
			client := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
			if _, err := searchGoogleDestinations(context.Background(), client, "synthetic-key", query); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGoogleAddressSynthetic(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v4/geocode/destinations" || r.Header.Get("X-Goog-Api-Key") != "synthetic-key" || r.URL.Query().Has("key") || r.URL.Query().Has("locationBias") {
			t.Error("unexpected request")
		}
		io.WriteString(w, `{"destinations":[{"primary":{"displayName":{"text":"Rua Exemplo"},"formattedAddress":"Rua Exemplo, Cidade","location":{"latitude":-23.5,"longitude":-46.6},"addressComponents":[{"longText":"Cidade","types":["locality"]}]}}]}`)
	}))
	defer s.Close()
	c := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
	got, err := searchGoogle(context.Background(), c, "synthetic-key", "Rua Exemplo")
	if err != nil || len(got) != 1 || got[0].City != "Cidade" || got[0].Latitude != -23.5 || got[0].GoogleName != "Rua Exemplo" {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestGoogleSyntheticFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wantErr    bool
	}{
		{"invalid-utf8", "{\"destinations\":[{\"primary\":{\"displayName\":{\"text\":\"\xff\"},\"location\":{\"latitude\":1,\"longitude\":2}}}]}", 200, true},
		{"empty", `{}`, 200, false},
		{"unauthorized", `{"error":{"message":"secret"}}`, 403, true},
		{"redirect", ``, 302, true},
		{"oversized", strings.Repeat("x", 1<<20), 200, true},
		{"malformed", `{`, 200, true},
		{"legacy-zero-results", `{"status":"ZERO_RESULTS","results":[]}`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "http://127.0.0.1/")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer s.Close()
			c := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
			got, err := searchGoogle(context.Background(), c, "synthetic-key", "Rua Exemplo")
			if (err != nil) != tc.wantErr || len(got) != 0 {
				t.Fatalf("got %#v, %v", got, err)
			}
			if err != nil && (strings.Contains(err.Error(), "synthetic-key") || strings.Contains(err.Error(), "Rua Exemplo") || strings.Contains(err.Error(), "secret")) {
				t.Fatal("sensitive error")
			}
		})
	}
}
func TestGoogleRejectsURLInput(t *testing.T) {
	for _, input := range []string{"https://evil.test/x", "//evil.test", ""} {
		_, err := searchGoogle(context.Background(), &http.Client{}, "synthetic-key", input)
		if err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
