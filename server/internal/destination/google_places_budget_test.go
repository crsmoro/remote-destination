package destination

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestPlacesBudgetIsSeparateAndPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.db")
	geo, e := newGoogleLimiter(path, time.Second, 1)
	if e != nil {
		t.Fatal(e)
	}
	defer geo.Close()
	places, e := newGooglePlacesLimiter(path, time.Second, 1)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	if e = geo.allow(now); e != nil {
		t.Fatal(e)
	}
	if e = places.allow(now); e != nil {
		t.Fatalf("places shares geocoding quota: %v", e)
	}
	if e = places.Close(); e != nil {
		t.Fatal(e)
	}
	places, e = newGooglePlacesLimiter(path, time.Second, 1)
	if e != nil {
		t.Fatal(e)
	}
	defer places.Close()
	if e = places.allow(now.Add(2 * time.Second)); !errors.Is(e, ErrGoogleLimit) {
		t.Fatalf("budget not persistent: %v", e)
	}
}
