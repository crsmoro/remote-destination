package destination

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoogleLimiterRejectsInvalidConfiguration(t *testing.T) {
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_GEOCODING_PROVIDER", "google")
	t.Setenv("GOOGLE_GEOCODING_KEY_FILE", key)
	t.Setenv("GOOGLE_GEOCODING_LIMIT_DB", filepath.Join(t.TempDir(), "limit.db"))
	t.Setenv("GOOGLE_GEOCODING_DAILY_LIMIT", "0")
	if _, err := ConfiguredGeocoder(); err == nil {
		t.Fatal("invalid daily limit accepted")
	}
}
