package destination

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPlacesRejectsUnexpectedEnvelope(t *testing.T) {
	c := &http.Client{Transport: testRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"unexpected":true}`))}, nil
	})}
	if _, err := searchPlaces(context.Background(), c, "synthetic-key", "query"); err == nil {
		t.Fatal("unexpected envelope accepted")
	}
}
