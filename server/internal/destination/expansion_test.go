package destination

import (
	"net/url"
	"os"
	"testing"
)

func TestExpandRealShortLink(t *testing.T) {
	if os.Getenv("TEST_LIVE_GOOGLE") != "1" {
		t.Skip("set TEST_LIVE_GOOGLE=1 for live Google integration")
	}
	got, warning, err := Normalize("https://maps.app.goo.gl/LmQsu81HjubAbeYf9")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("ll") != "12.3456,23.4567" || warning != "" {
		t.Fatalf("got %q warning %q", got, warning)
	}
}
