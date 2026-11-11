package destination

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchDestinationsMissingPrimaryAndPostalFallback(t *testing.T) {
	for _, tc := range []struct {
		body    string
		wantErr bool
		city    string
	}{
		{`{"destinations":[{"navigationPoints":[{}]}]}`, true, ""},
		{`{"destinations":[{"primary":{"formattedAddress":"Rua A","location":{"latitude":1,"longitude":2}}}]}`, false, ""},
		{`{"destinations":[{"primary":{"displayName":{"text":"Nome"},"formattedAddress":"Rua A","postalAddress":{"locality":"Joinville"},"location":{"latitude":1,"longitude":2}}}]}`, false, "Joinville"},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(tc.body)) }))
		c := &http.Client{Transport: googleTestTransport{strings.TrimPrefix(s.URL, "http://"), s.Client()}}
		got, err := searchGoogle(context.Background(), c, "key", "Nome")
		s.Close()
		if (err != nil) != tc.wantErr || (!tc.wantErr && (len(got) != 1 || got[0].City != tc.city)) {
			t.Errorf("got %#v %v", got, err)
		}
	}
}
