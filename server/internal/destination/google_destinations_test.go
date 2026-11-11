package destination

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchDestinationsBusiness(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Method != "POST" || r.URL.Path != "/v4/geocode/destinations" || r.Header.Get("X-Goog-Api-Key") != "synthetic-key" || r.Header.Get("X-Goog-FieldMask") == "" || json.NewDecoder(r.Body).Decode(&body) != nil || func() bool {
			q, ok := body["addressQuery"].(map[string]any)
			return !ok || q["addressQuery"] != "zx armazem"
		}() {
			t.Error("invalid SearchDestinations request")
		}
		w.Write([]byte(`{"destinations":[{"primary":{"displayName":{"text":"ZX ARMAZEM"},"formattedAddress":"Rua Nebulosa, 10, Cidade Ficticia","location":{"latitude":0.0,"longitude":0.0},"addressComponents":[{"longText":"Cidade Ficticia","types":["administrative_area_level_2","political"]}]}}]}`))
	}))
	defer s.Close()
	c := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
	got, err := searchGoogle(context.Background(), c, "synthetic-key", "zx armazem")
	if err != nil || len(got) != 1 || got[0].City != "Cidade Ficticia" || !strings.Contains(got[0].Label, "ZX ARMAZEM") {
		t.Fatalf("got %#v, %v", got, err)
	}
}
