package store

import (
	"path/filepath"
	"testing"
)

func TestAckConflictFailureAndRestart(t *testing.T) {
	p := filepath.Join(t.TempDir(), "db")
	s, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Put("old", "uri", ""); e != nil {
		t.Fatal(e)
	}
	if e = s.Put("new", "uri", "warning"); e != nil {
		t.Fatal(e)
	}
	if e = s.Ack("old", "intent_sent"); e != ErrConflict {
		t.Fatalf("stale ack: %v", e)
	}
	if e = s.Ack("new", "failed"); e != nil {
		t.Fatal(e)
	}
	if e = s.Ack("new", "failed"); e != nil {
		t.Fatal(e)
	}
	c, e := s.Pending()
	if e != nil || c != nil {
		t.Fatalf("failed command should not loop: %v %v", c, e)
	}
	st, e := s.Status()
	if e != nil || st.State != "failed" || st.Warning == nil {
		t.Fatal(st, e)
	}
	if e = s.Cancel(); e != nil {
		t.Fatal(e)
	}
	if e = s.Ack("new", "intent_sent"); e != ErrConflict {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	st, e = s.Status()
	if e != nil || st.State != "cancelled" || st.Warning != nil {
		t.Fatal(st, e)
	}
}
