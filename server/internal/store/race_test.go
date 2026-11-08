package store

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestAckRaceDoesNotAlterReplacement(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "race.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for i := 0; i < 50; i++ {
		if e = s.Put("old", "old", ""); e != nil {
			t.Fatal(e)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _ = s.Ack("old", "intent_sent") }()
		go func() {
			defer wg.Done()
			if e := s.Put("new", "new", ""); e != nil {
				t.Error(e)
			}
		}()
		wg.Wait()
		c, e := s.Pending()
		if e != nil || c == nil || c.ID != "new" {
			t.Fatalf("new command altered: %+v %v", c, e)
		}
	}
}
