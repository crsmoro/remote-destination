package destination

import (
	"net/url"
	"testing"
)

func TestMapsSearchCoordinateQuery(t *testing.T) {
	uri, warning, err := Normalize("https://www.google.com/maps/search/?api=1&query=3%2C4")
	if err != nil || warning != "" {
		t.Fatalf("normalize: %q %v", warning, err)
	}
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("ll") != "3,4" || u.Query().Get("q") != "" {
		t.Fatalf("not coordinates: %s", uri)
	}
}
