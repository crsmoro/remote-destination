package destination

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoogleLimiterConcurrentPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "limit.db")
	a, err := newGoogleLimiter(path, time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := newGoogleLimiter(path, time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l := a
			if i%2 == 0 {
				l = b
			}
			err := l.allow(now)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrGoogleLimit) {
				t.Errorf("allow: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successes = %d, want 1", successes.Load())
	}
	if err := b.allow(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := a.allow(now.Add(2 * time.Second)); !errors.Is(err, ErrGoogleLimit) {
		t.Fatalf("daily limit: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := newGoogleLimiter(path, time.Second, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if err := restarted.allow(now.Add(3 * time.Second)); !errors.Is(err, ErrGoogleLimit) {
		t.Fatalf("restart lost daily limit: %v", err)
	}
	if err := b.allow(now.Add(24 * time.Hour)); err != nil {
		t.Fatalf("next UTC day: %v", err)
	}
}
