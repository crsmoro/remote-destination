package destination

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGoogleLimiterRejectsNonFilePaths(t *testing.T) {
	for _, path := range []string{"", ":memory:", "file::memory:", "file:quota?mode=memory&cache=shared", "file:/tmp/quota.db?mode=memory", "relative.db"} {
		t.Run(path, func(t *testing.T) {
			limiter, err := newGoogleLimiter(path, time.Second, 1)
			if err == nil {
				limiter.Close()
				t.Fatalf("accepted non-persistent database path %q", path)
			}
		})
	}
}

func TestGoogleRequiresExplicitQuotaDatabase(t *testing.T) {
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("synthetic-key"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAZE_GEOCODING_PROVIDER", "google")
	t.Setenv("GOOGLE_GEOCODING_KEY_FILE", key)
	t.Setenv("GOOGLE_GEOCODING_LIMIT_DB", "")
	if _, err := ConfiguredGeocoder(); err == nil || err.Error() != "google limiter database path required" {
		t.Fatalf("missing explicit database path: %v", err)
	}
}
