package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLegacyMigrationAndDisplayErasure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE current (slot INTEGER PRIMARY KEY CHECK(slot=1),id TEXT NOT NULL,state TEXT NOT NULL,uri TEXT NOT NULL DEFAULT '',warning TEXT NOT NULL DEFAULT '',updated INTEGER NOT NULL); INSERT INTO current VALUES(1,'old','pending','https://waze.com/ul?ll=1,2','','1')`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	st, e := s.Status()
	if e != nil || st.URI != "https://waze.com/ul?ll=1,2" || st.Display != "" {
		t.Fatalf("legacy: %#v %v", st, e)
	}
	rev, e := s.BeginSubmit()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.PutIfCurrent(rev, "new", "uri", "", "secret"); e != nil {
		t.Fatal(e)
	}
	if e = s.Cancel(); e != nil {
		t.Fatal(e)
	}
	var display, uri string
	if e = s.db.QueryRow(`SELECT display,uri FROM current`).Scan(&display, &uri); e != nil || display != "" || uri != "" {
		t.Fatalf("not erased: %q %q %v", display, uri, e)
	}
}
