package store

import (
	"errors"
	"testing"
	"time"
)

func TestCancelWithoutCommandConflicts(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e := s.Cancel(); !errors.Is(e, ErrConflict) {
		t.Fatalf("cancel without command: %v", e)
	}
}
func TestAckSuccessRetainsWarning(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Put("id", "uri", "warning"); e != nil {
		t.Fatal(e)
	}
	if e = s.Ack("id", "intent_sent"); e != nil {
		t.Fatal(e)
	}
	st, e := s.Status()
	if e != nil || st.Warning == nil || *st.Warning != "warning" {
		t.Fatalf("status %#v: %v", st, e)
	}
}
func TestFutureHeartbeatOffline(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.db.Exec("INSERT INTO heartbeat VALUES(1,?)", time.Now().Add(time.Hour).Unix()); e != nil {
		t.Fatal(e)
	}
	st, e := s.Status()
	if e != nil || st.Online {
		t.Fatalf("status %#v: %v", st, e)
	}
}
func TestSecureDeleteEnabled(t *testing.T) {
	s, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var enabled int
	if e = s.db.QueryRow("PRAGMA secure_delete").Scan(&enabled); e != nil || enabled != 1 {
		t.Fatalf("secure_delete=%d: %v", enabled, e)
	}
}
