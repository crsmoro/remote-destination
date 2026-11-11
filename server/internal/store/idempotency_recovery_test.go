package store

import (
	"testing"
	"time"
)

func TestExpiredReservationCanBeRecoveredWithoutOldOwnerCommit(t *testing.T) {
	s, err := Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	old, _, err := s.BeginIdempotent("p", "k", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE idempotency SET created=?`, time.Now().Add(-2*time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	recovered, response, err := s.BeginIdempotent("p", "k", "hash")
	if err != nil || response != nil || recovered == old {
		t.Fatalf("recovery: %d %s %v", recovered, response, err)
	}
	if err = s.ConfirmIdempotent(old, "old", "uri", "", "p", "k", []byte("old")); err == nil {
		t.Fatal("expired owner committed")
	}
	if err = s.ConfirmIdempotent(recovered, "new", "uri", "", "p", "k", []byte("new")); err != nil {
		t.Fatal(err)
	}
}
