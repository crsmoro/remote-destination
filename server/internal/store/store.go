package store

import (
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"strings"
	"time"
)

var ErrConflict = errors.New("command is no longer pending")

type Command struct {
	ID      string  `json:"id"`
	URI     string  `json:"uri"`
	Warning *string `json:"warning"`
}
type Status struct {
	ID       string     `json:"id,omitempty"`
	State    string     `json:"state,omitempty"`
	Warning  *string    `json:"warning"`
	LastSeen *time.Time `json:"last_seen"`
	Online   bool       `json:"online"`
	Display  string     `json:"display,omitempty"`
	URI      string     `json:"uri,omitempty"`
}
type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA secure_delete=ON; CREATE TABLE IF NOT EXISTS current (slot INTEGER PRIMARY KEY CHECK(slot=1), id TEXT NOT NULL, state TEXT NOT NULL, uri TEXT NOT NULL DEFAULT '', warning TEXT NOT NULL DEFAULT '', updated INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS heartbeat (slot INTEGER PRIMARY KEY CHECK(slot=1), seen INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS ordering (slot INTEGER PRIMARY KEY CHECK(slot=1), revision INTEGER NOT NULL); INSERT OR IGNORE INTO ordering VALUES(1,0);`)
	if e != nil {
		db.Close()
		return nil, e
	}
	// ALTER is safe for existing databases and concurrent startup; duplicate means migrated.
	if _, e = db.Exec(`ALTER TABLE current ADD COLUMN display TEXT NOT NULL DEFAULT ''`); e != nil && !strings.Contains(e.Error(), "duplicate column name: display") {
		db.Close()
		return nil, e
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Put(id, uri, warning string) error {
	revision, err := s.BeginSubmit()
	if err != nil {
		return err
	}
	return s.PutIfCurrent(revision, id, uri, warning)
}

// BeginSubmit reserves an order before potentially slow destination resolution.
func (s *Store) BeginSubmit() (int64, error) {
	var revision int64
	err := s.db.QueryRow(`UPDATE ordering SET revision=revision+1 WHERE slot=1 RETURNING revision`).Scan(&revision)
	return revision, err
}

// PutIfCurrent commits only if no later submission or cancellation was received.
func (s *Store) PutIfCurrent(revision int64, id, uri, warning string, display ...string) error {
	original := ""
	if len(display) > 0 {
		original = display[0]
	}
	r, err := s.db.Exec(`INSERT INTO current(slot,id,state,uri,warning,updated,display) SELECT 1,?,'pending',?,?,?,? FROM ordering WHERE slot=1 AND revision=? ON CONFLICT(slot) DO UPDATE SET id=excluded.id,state='pending',uri=excluded.uri,warning=excluded.warning,updated=excluded.updated,display=excluded.display`, id, uri, warning, time.Now().Unix(), original, revision)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Cancel() error {
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`UPDATE ordering SET revision=revision+1 WHERE slot=1`); e != nil {
		return e
	}
	r, e := tx.Exec(`UPDATE current SET state='cancelled',uri='',warning='',display='',updated=? WHERE slot=1 AND state IN ('pending','failed')`, time.Now().Unix())
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Claim(id string) error {
	var n int
	e := s.db.QueryRow(`SELECT count(*) FROM current WHERE slot=1 AND id=? AND state='pending'`, id).Scan(&n)
	if e != nil {
		return e
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Pending() (*Command, error) {
	var c Command
	var w string
	e := s.db.QueryRow(`SELECT id,uri,warning FROM current WHERE slot=1 AND state='pending'`).Scan(&c.ID, &c.URI, &w)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if w != "" {
		c.Warning = &w
	}
	return &c, nil
}
func (s *Store) Heartbeat() error {
	_, e := s.db.Exec(`INSERT INTO heartbeat VALUES(1,?) ON CONFLICT(slot) DO UPDATE SET seen=excluded.seen`, time.Now().Unix())
	return e
}
func (s *Store) Ack(id, result string) error {
	if result != "intent_sent" && result != "failed" {
		return errors.New("invalid result")
	}
	r, e := s.db.Exec(`UPDATE current SET state=?, uri=CASE WHEN ?='intent_sent' THEN '' ELSE uri END, display='', updated=? WHERE slot=1 AND id=? AND (state='pending' OR state=?)`, result, result, time.Now().Unix(), id, result)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Status() (Status, error) {
	var st Status
	var w string
	e := s.db.QueryRow(`SELECT id,state,warning,CASE WHEN state='pending' THEN display ELSE '' END,CASE WHEN state='pending' THEN uri ELSE '' END FROM current WHERE slot=1`).Scan(&st.ID, &st.State, &w, &st.Display, &st.URI)
	if e != nil && e != sql.ErrNoRows {
		return st, e
	}
	if w != "" {
		st.Warning = &w
	}
	var seen int64
	e = s.db.QueryRow(`SELECT seen FROM heartbeat WHERE slot=1`).Scan(&seen)
	if e != nil && e != sql.ErrNoRows {
		return st, e
	}
	if e == nil {
		v := time.Unix(seen, 0).UTC()
		st.LastSeen = &v
		age := time.Since(v)
		st.Online = age >= 0 && age <= 45*time.Second
	}
	return st, nil
}
