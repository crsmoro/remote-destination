package store

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestConfirmRevisionOnlyOnceUnderConcurrency(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rev, err := s.BeginSubmit()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.ConfirmIfCurrent(rev, "id", "https://waze.com/ul?ll=1,2&navigate=yes", "chosen")
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else if e != ErrConflict {
			t.Fatal(e)
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successes", successes)
	}
}
