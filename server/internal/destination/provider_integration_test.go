package destination

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProviderConfigurationFailsClosed(t *testing.T) {
	t.Setenv("WAZE_GEOCODING_PROVIDER", "")
	t.Setenv("GOOGLE_GEOCODING_KEY_FILE", "")
	if _, err := ConfiguredGeocoder(); err == nil {
		t.Fatal("missing key must fail closed")
	}
	t.Setenv("WAZE_GEOCODING_PROVIDER", "google")
	if _, err := ConfiguredGeocoder(); err == nil {
		t.Fatal("google without a key file must fail")
	}
	t.Setenv("WAZE_GEOCODING_PROVIDER", "unknown")
	if _, err := ConfiguredGeocoder(); err == nil {
		t.Fatal("unknown provider must fail")
	}
	t.Setenv("WAZE_GEOCODING_PROVIDER", "google")
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("synthetic-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_GEOCODING_KEY_FILE", path)
	t.Setenv("GOOGLE_GEOCODING_LIMIT_DB", filepath.Join(t.TempDir(), "budget.db"))
	if _, err := ConfiguredGeocoder(); err != nil {
		t.Fatal(err)
	}
}
