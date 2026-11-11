package main

import (
	"net/http"
	"testing"
	"waze-remoto/internal/api"
)

func TestConfigureGeocoderRejectsUnknownProvider(t *testing.T) {
	t.Setenv("WAZE_GEOCODING_PROVIDER", "unknown")
	var h http.Handler = api.New(nil, "synthetic")
	if err := configureGeocoder(h); err == nil {
		t.Fatal("unknown provider accepted")
	}
}
