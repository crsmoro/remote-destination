package destination

import (
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ErrGoogleLimit means the local Google request budget is exhausted.
var ErrGoogleLimit = errors.New("google geocoding request limit reached")

type googleLimiter struct {
	db       *sql.DB
	interval time.Duration
	daily    int
	table    string
}

func newGooglePlacesLimiter(path string, interval time.Duration, daily int) (*googleLimiter, error) {
	return newGoogleLimiterTable(path, interval, daily, "google_places_budget")
}

func newGoogleLimiter(path string, interval time.Duration, daily int) (*googleLimiter, error) {
	return newGoogleLimiterTable(path, interval, daily, "google_budget")
}

func newGoogleLimiterTable(path string, interval time.Duration, daily int, table string) (*googleLimiter, error) {
	// Plain absolute paths exclude SQLite URI parameters and in-memory databases.
	// A deployment must separately ensure the filesystem is durable.
	if !filepath.IsAbs(path) || interval <= 0 || daily <= 0 {
		return nil, errors.New("invalid google limiter configuration")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, query := range []string{`PRAGMA busy_timeout=5000`, `CREATE TABLE IF NOT EXISTS ` + table + ` (id INTEGER PRIMARY KEY CHECK(id=1), day TEXT NOT NULL, count INTEGER NOT NULL, last_ns INTEGER NOT NULL)`, `INSERT OR IGNORE INTO ` + table + ` VALUES (1,'',0,0)`} {
		if _, err = db.Exec(query); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &googleLimiter{db, interval, daily, table}, nil
}
func (l *googleLimiter) Close() error { return l.db.Close() }
func (l *googleLimiter) allow(now time.Time) error {
	day := now.UTC().Format("2006-01-02")
	ns := now.UnixNano()
	// A single conditional update is atomic across processes sharing this database.
	result, err := l.db.Exec(`UPDATE `+l.table+` SET count=CASE WHEN day < ? THEN 1 ELSE count+1 END, day=?, last_ns=? WHERE id=1 AND day <= ? AND (day < ? OR count < ?) AND last_ns <= ?`, day, day, ns, day, day, l.daily, ns-int64(l.interval))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrGoogleLimit
	}
	return nil
}
