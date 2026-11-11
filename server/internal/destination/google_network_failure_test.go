package destination

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestGoogleNetworkFailureDoesNotFallbackToPlaces(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "geocode.googleapis.com" {
			t.Errorf("unexpected fallback host: %s", r.URL.Host)
		}
		return nil, errors.New("simulated network failure")
	})}
	_, err := searchGoogle(context.Background(), client, "synthetic-key", "empresa ficticia vila imaginária")
	if err == nil || calls != 1 {
		t.Fatalf("expected one failed geocoding request without fallback; calls=%d err=%v", calls, err)
	}
}
