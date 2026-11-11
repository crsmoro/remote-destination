package destination

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGoogleZeroDestinationsFallsBackToPlaces(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{}`
		if calls == 1 {
			if r.URL.Host != "geocode.googleapis.com" {
				t.Errorf("wrong first host: %s", r.URL.Host)
			}
		} else {
			if r.Method != "POST" || r.URL.String() != "https://places.googleapis.com/v1/places:searchText" || r.Header.Get("X-Goog-Api-Key") != "synthetic-key" || r.Header.Get("X-Goog-FieldMask") != "places.displayName,places.formattedAddress,places.location,places.addressComponents" {
				t.Errorf("invalid Places request: %s %s", r.Method, r.URL)
			}
			data, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(data), `"textQuery":"empresa ficticia vila imaginária"`) {
				t.Errorf("invalid request body: %s", data)
			}
			body = `{"places":[{"displayName":{"text":"EMPRESA FICTICIA"},"formattedAddress":"Vila Imaginária, SC","location":{"latitude":-26.3,"longitude":-48.8}}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	got, err := searchGoogle(context.Background(), client, "synthetic-key", "empresa ficticia vila imaginária")
	if err != nil || calls != 2 || len(got) != 1 || got[0].Label != "EMPRESA FICTICIA, Vila Imaginária, SC" {
		t.Fatalf("calls=%d got=%+v err=%v", calls, got, err)
	}
}
