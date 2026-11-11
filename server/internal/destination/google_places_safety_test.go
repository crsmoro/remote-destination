package destination

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFallbackOnlyOnEmptySuccessfulDestinations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantCalls int
		wantError bool
	}{
		{"unauthorized", 403, `{}`, 1, true}, {"quota", 429, `{}`, 1, true}, {"server error", 500, `{}`, 1, true}, {"redirect", 302, `{}`, 1, true},
		{"result", 200, `{"destinations":[{"primary":{"formattedAddress":"A","location":{"latitude":1,"longitude":2}}}]}`, 1, false},
		{"malformed", 200, `{`, 1, true}, {"empty both", 200, `{}`, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: testRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				body := tc.body
				status := tc.status
				if calls > 1 {
					body = `{}`
					status = 200
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://evil.example/"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			_, err := searchGoogle(context.Background(), client, "synthetic-key", "example")
			if calls != tc.wantCalls || (err != nil) != tc.wantError {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestPlacesLimiterBlocksFallback(t *testing.T) {
	calls := 0
	c := &http.Client{Transport: testRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	_, err := searchGoogleWithPlaces(context.Background(), c, "synthetic-key", "example", func() error { return ErrGoogleLimit })
	if calls != 1 || !errors.Is(err, ErrGoogleLimit) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestPlacesRedirectAndErrorsFailClosed(t *testing.T) {
	for _, status := range []int{302, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			c := &http.Client{Transport: testRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://evil.example/"}}, Body: io.NopCloser(strings.NewReader(`secret`))}, nil
			})}
			_, err := searchGoogle(context.Background(), c, "synthetic-key", "example")
			if calls != 2 || err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
