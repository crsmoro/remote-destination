package destination

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPlacesBudgetConcurrentProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.db")
	a, err := newGooglePlacesLimiter(path, time.Second, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := newGooglePlacesLimiter(path, time.Second, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	now := time.Now()
	var won atomic.Int32
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
				won.Add(1)
			} else if !errors.Is(err, ErrGoogleLimit) {
				t.Errorf("allow: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("allowed %d concurrent calls", won.Load())
	}
}
