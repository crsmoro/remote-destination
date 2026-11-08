package destination

import (
	"net/url"
	"testing"
)

func TestAmbiguousPlaceCoordinatesDoNotChooseFirst(t *testing.T) {
	input := "https://www.google.com/maps/place/X/data=!3d1!4d2!3d3!4d4"
	got, warning, err := Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("ll") != "" || warning == "" {
		t.Fatalf("ambiguous result %q warning %q", got, warning)
	}
}
