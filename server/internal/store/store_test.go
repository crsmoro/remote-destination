package store

import (
	"path/filepath"
	"testing"
)

func TestReplaceCancelAndPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "db.sqlite")
	s, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Put("a", "https://waze.com/ul?q=a", ""); e != nil {
		t.Fatal(e)
	}
	if e = s.Put("b", "https://waze.com/ul?q=b", ""); e != nil {
		t.Fatal(e)
	}
	if e = s.Claim("a"); e == nil {
		t.Fatal("superseded command claim succeeded")
	}
	s.Close()
	s, e = Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	c, e := s.Pending()
	if e != nil || c == nil || c.ID != "b" {
		t.Fatalf("pending: %#v %v", c, e)
	}
	if e = s.Cancel(); e != nil {
		t.Fatal(e)
	}
	if e = s.Claim("b"); e == nil {
		t.Fatal("cancelled command claim succeeded")
	}
}
