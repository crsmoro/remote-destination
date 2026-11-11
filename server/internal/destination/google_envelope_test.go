package destination

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testRoundTrip func(*http.Request) (*http.Response, error)

func (f testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGoogleInvalidResultsAreNotNoResults(t *testing.T) {
	for _, body := range []string{`null`, `{"results":[{"formattedAddress":"synthetic"}]}`} {
		c := &http.Client{Transport: testRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
		if _, err := searchGoogle(context.Background(), c, "synthetic-key", "synthetic"); err == nil {
			t.Fatal("invalid response accepted as no results")
		}
	}
}

func TestGoogleRejectsErrorEnvelopeOnSuccess(t *testing.T) {
	c := &http.Client{Transport: testRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"synthetic-key private-query"}}`)), Header: make(http.Header)}, nil
	})}
	_, err := searchGoogle(context.Background(), c, "synthetic-key", "private-query")
	if err == nil {
		t.Fatal("error envelope mistaken for empty results")
	}
	if strings.Contains(err.Error(), "synthetic-key") || strings.Contains(err.Error(), "private-query") {
		t.Fatal("error leaked sensitive data")
	}
}
