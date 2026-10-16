// SPDX-License-Identifier: AGPL-3.0-only

package sqlitedb

import (
	"database/sql"
	"errors"
	"net/url"

	ncsqlite3 "github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/driver" // registers "sqlite3"
	msqlite "modernc.org/sqlite"             // registers "sqlite"
	msqlite3 "modernc.org/sqlite/lib"
)

// Driver selects the cgo-free SQLite driver.
type Driver string

const (
	Modernc Driver = "modernc" // modernc.org/sqlite (C transpiled to Go)
	Ncruces Driver = "ncruces" // github.com/ncruces/go-sqlite3 (Wasm translated to Go)
)

// Open opens path (a file path, or ":memory:") with the pragmas every
// connection needs. Both drivers accept the same _pragma / _txlock keys.
func Open(drv Driver, path string) (*DB, error) { return OpenTxLock(drv, path, "immediate") }

// OpenTxLock is Open with an explicit BEGIN mode (deferred / immediate / exclusive).
func OpenTxLock(drv Driver, path, txlock string) (*DB, error) {
	q := url.Values{}
	// Order matters for ncruces: busy_timeout first.
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_txlock", txlock)
	memory := path == ":memory:"
	if !memory {
		q.Add("_pragma", "journal_mode(wal)")
		q.Add("_pragma", "synchronous(normal)")
	}
	dsn := "file:" + path + "?" + q.Encode()

	var (
		db  *sql.DB
		err error
		cls func(error) bool
	)
	switch drv {
	case Modernc:
		db, err = sql.Open("sqlite", dsn)
		cls = moderncRetryable
	case Ncruces:
		db, err = sql.Open("sqlite3", dsn)
		cls = ncrucesRetryable
	default:
		return nil, errors.New("sqlitedb: unknown driver " + string(drv))
	}
	if err != nil {
		return nil, err
	}
	if memory {
		// Each connection to :memory: is a separate database; pin to one.
		db.SetMaxOpenConns(1)
	}
	return New(db, cls), nil
}

func moderncRetryable(err error) bool {
	var e *msqlite.Error
	if !errors.As(err, &e) {
		return false
	}
	switch e.Code() & 0xff { // primary result code
	case msqlite3.SQLITE_BUSY, msqlite3.SQLITE_LOCKED:
		return true
	}
	return false
}

func ncrucesRetryable(err error) bool {
	return errors.Is(err, ncsqlite3.BUSY) || errors.Is(err, ncsqlite3.LOCKED)
}
