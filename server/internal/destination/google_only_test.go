package destination

import "testing"

func TestGoogleOnlyRejectsMissingKeyAndAlternative(t *testing.T) {
	t.Setenv("WAZE_GEOCODING_PROVIDER", "")
	t.Setenv("GOOGLE_GEOCODING_KEY_FILE", "")
	if _, err := ConfiguredGeocoder(); err == nil {
		t.Error("missing key accepted")
	}
	t.Setenv("WAZE_GEOCODING_PROVIDER", "photon")
	if _, err := ConfiguredGeocoder(); err == nil {
		t.Error("alternative provider accepted")
	}
}
