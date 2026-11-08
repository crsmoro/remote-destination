package destination

import (
	"net/url"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, contains string
		warning      bool
		bad          bool
	}{
		{"Praça São João", "q=Pra%C3%A7a+S%C3%A3o+Jo%C3%A3o", false, false},
		{"https://ul.waze.com/ul?venue_id=123&overview=yes&navigate=no&navigate=no", "venue_id=123", false, false},
		{"https://www.google.com/maps/place/X/@1,2/data=!3d12.3456!4d23.4567", "ll=12.3456%2C23.4567", false, false},
		{"https://maps.app.goo.gl/abc", "q=https%3A%2F%2Fmaps.app.goo.gl%2Fabc", true, false},
		{"https://waze.com.evil.test/ul?q=x", "", false, true},
	}
	for _, tt := range tests {
		u, w, e := Normalize(tt.in)
		if tt.bad {
			if e == nil {
				t.Errorf("accepted %q", tt.in)
			}
			continue
		}
		if e != nil || !strings.Contains(u, tt.contains) || (w != "") != tt.warning {
			t.Errorf("%q: %s %q %v", tt.in, u, w, e)
		}
		p, _ := url.Parse(u)
		if len(p.Query()["navigate"]) != 1 || p.Query().Get("navigate") != "yes" {
			t.Errorf("navigate: %s", u)
		}
	}
}
