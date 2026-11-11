package destination

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Geocoder func(context.Context, string) ([]Candidate, error)

// ConfiguredGeocoder validates all configuration at startup, without making requests.
func ConfiguredGeocoder() (Geocoder, error) {
	switch os.Getenv("WAZE_GEOCODING_PROVIDER") {
	case "", "google":
		path := os.Getenv("GOOGLE_GEOCODING_KEY_FILE")
		if path == "" {
			return nil, errors.New("google key file required")
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, errors.New("cannot read google key file")
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("invalid google key file")
		}
		raw, err := io.ReadAll(io.LimitReader(f, 4097))
		key := strings.TrimSpace(string(raw))
		if err != nil || len(raw) > 4096 || key == "" || strings.ContainsAny(key, "\r\n\t ") {
			return nil, errors.New("invalid google key file")
		}
		proxy := os.Getenv("WAZE_EGRESS_PROXY")
		if proxy != "" && proxy != "egress:3128" {
			return nil, errors.New("invalid egress configuration")
		}
		daily, interval := 100, time.Second
		if v := os.Getenv("GOOGLE_GEOCODING_DAILY_LIMIT"); v != "" {
			var e error
			daily, e = strconv.Atoi(v)
			if e != nil || daily < 1 {
				return nil, errors.New("invalid google daily limit")
			}
		}
		if v := os.Getenv("GOOGLE_GEOCODING_INTERVAL"); v != "" {
			var e error
			interval, e = time.ParseDuration(v)
			if e != nil || interval < time.Millisecond {
				return nil, errors.New("invalid google interval")
			}
		}
		budgetPath := os.Getenv("GOOGLE_GEOCODING_LIMIT_DB")
		if budgetPath == "" {
			return nil, errors.New("google limiter database path required")
		}
		limiter, err := newGoogleLimiter(budgetPath, interval, daily)
		if err != nil {
			return nil, errors.New("cannot initialize google request limiter")
		}
		placesDaily, placesInterval := 100, time.Second
		if v := os.Getenv("GOOGLE_PLACES_DAILY_LIMIT"); v != "" {
			var e error
			placesDaily, e = strconv.Atoi(v)
			if e != nil || placesDaily < 1 {
				limiter.Close()
				return nil, errors.New("invalid google places daily limit")
			}
		}
		if v := os.Getenv("GOOGLE_PLACES_INTERVAL"); v != "" {
			var e error
			placesInterval, e = time.ParseDuration(v)
			if e != nil || placesInterval < time.Millisecond {
				limiter.Close()
				return nil, errors.New("invalid google places interval")
			}
		}
		placesLimiter, err := newGooglePlacesLimiter(budgetPath, placesInterval, placesDaily)
		if err != nil {
			limiter.Close()
			return nil, errors.New("cannot initialize google places request limiter")
		}
		return func(ctx context.Context, input string) ([]Candidate, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := limiter.allow(time.Now()); err != nil {
				if errors.Is(err, ErrGoogleLimit) {
					return nil, err
				}
				return nil, errors.New("google request limiter unavailable")
			}
			dial := (&net.Dialer{Timeout: 4 * time.Second}).DialContext
			selected := secureDial(net.DefaultResolver, dial)
			if proxy == "egress:3128" {
				selected = configuredDial(dial)
			}
			tr := &http.Transport{Proxy: nil, DialContext: selected, TLSHandshakeTimeout: 4 * time.Second, DisableKeepAlives: true}
			defer tr.CloseIdleConnections()
			return searchGoogleWithPlaces(ctx, &http.Client{Transport: tr}, key, input, func() error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := placesLimiter.allow(time.Now()); err != nil {
					if errors.Is(err, ErrGoogleLimit) {
						return err
					}
					return errors.New("google request limiter unavailable")
				}
				return nil
			})
		}, nil
	default:
		return nil, errors.New("invalid geocoding provider")
	}
}
