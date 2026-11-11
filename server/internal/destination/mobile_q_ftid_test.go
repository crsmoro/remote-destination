package destination

import (
	"net/url"
	"testing"
)

func TestMobileQFtid(t *testing.T) {
	for _, tc := range []struct {
		raw, key, value string
		valid           bool
	}{
		{"https://maps.google.com/maps?q=Mercado+S%C3%A3o+Jo%C3%A3o%2C+Joinville&ftid=0xabc:0x123&entry=ttu&g_st=ic", "q", "Mercado São João, Joinville", true},
		{"https://www.google.com/maps?q=-26.3%2C-48.8&ftid=0xa:0xb", "ll", "-26.3,-48.8", true},
		{"https://www.google.com/maps/@1,2?q=Mercado&ftid=0xa:0xb", "", "", false},
		{"https://www.google.com/maps?q=Mercado&q=Outro&ftid=0xa:0xb", "", "", false},
		{"https://www.google.com/maps?q=Mercado&ftid=bad", "", "", false},
		{"https://www.google.com/maps?q=Mercado&ftid=0xa:0xb&query=Outro", "", "", false},
		{"https://www.google.com/maps?q=https%3A%2F%2Fevil.test&ftid=0xa:0xb", "", "", false},
		{"https://www.google.com/maps?q=Mercado%ZZ&ftid=0xa:0xb", "", "", false},
		{"https://www.google.com/maps?q=91%2C2&ftid=0xa:0xb", "", "", false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, warn, err := Normalize(tc.raw)
			if !tc.valid {
				if err == nil && warn == "" {
					t.Fatalf("unsafe accepted: %s", got)
				}
				return
			}
			if err != nil || warn != "" {
				t.Fatalf("rejected: %v %q", err, warn)
			}
			u, e := url.Parse(got)
			if e != nil || u.Query().Get(tc.key) != tc.value {
				t.Fatalf("wrong destination: %s %v", got, e)
			}
		})
	}
}
