package destination

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUnexpectedDestinationEnvelopeDoesNotTriggerPlaces(t *testing.T) {
	calls := 0
	c := &http.Client{Transport: testRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"unexpected":true}`))}, nil
	})}
	_, err := searchGoogle(context.Background(), c, "synthetic-key", "query")
	if err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
