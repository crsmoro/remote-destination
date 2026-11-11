package destination

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPlacesPreservesResultsAndLocality(t *testing.T) {
	var items []string
	for i := 0; i < 6; i++ {
		items = append(items, fmt.Sprintf(`{"displayName":{"text":"Lugar"},"location":{"latitude":%d,"longitude":1},"addressComponents":[{"longText":"Joinville","types":["locality"]},{"longText":"Santa Catarina","shortText":"SC","types":["administrative_area_level_1"]}]}`, i))
	}
	client := &http.Client{Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.Header.Get("X-Goog-FieldMask"), "places.addressComponents") {
			t.Error("missing address components field")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"places":[` + strings.Join(items, ",") + `]}`)), Header: make(http.Header)}, nil
	})}
	got, err := searchPlaces(context.Background(), client, "synthetic", "lugar")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("got %d candidates, want 6", len(got))
	}
	if got[0].City != "Joinville" || got[0].State != "SC" {
		t.Fatalf("city/state: %q %q", got[0].City, got[0].State)
	}
}
