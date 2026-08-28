// Package db owns the SQLite handle. It has no business meaning: contexts get a *DB
// injected from cmd/api and never open their own connection.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// modernc.org/sqlite is the pure-Go driver. A cgo driver would force CGO_ENABLED=1
	// and with it a libc-carrying base image, breaking the distroless/static runtime.
	_ "modernc.org/sqlite"
)

// busyTimeoutMillis is how long a blocked statement waits for the write lock before
// returning SQLITE_BUSY. Reads never block writers under WAL, so this only covers the
// brief overlap when the single writer is mid-transaction.
const busyTimeoutMillis = 5000

// DB holds the two handles a SQLite process needs. Splitting them is the point: SQLite
// serializes writes at the file level, so a pool of writers just converts contention
// into SQLITE_BUSY errors. Writer is capped at one connection (ARCHITECTURE §2.4) and
// Reader keeps the default pool, which WAL lets read concurrently with a live write.
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
}

// Open prepares the database file's directory and returns the writer/reader pair with
// WAL, foreign keys, and a busy timeout applied to every connection in both pools.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory %s: %w", dir, err)
		}
	}

	writer, err := open(path)
	if err != nil {
		return nil, err
	}
	// One connection = one writer. Every write in the process queues here instead of
	// racing for the file lock.
	writer.SetMaxOpenConns(1)

	reader, err := open(path)
	if err != nil {
		writer.Close()
		return nil, err
	}

	return &DB{Writer: writer, Reader: reader}, nil
}

// Close releases both pools, reporting the first failure.
func (d *DB) Close() error {
	werr := d.Writer.Close()
	rerr := d.Reader.Close()
	if werr != nil {
		return werr
	}
	return rerr
}

func open(path string) (*sql.DB, error) {
	// The pragmas ride the DSN so they apply to every connection the pool opens, not
	// just the first — a pragma run once as a statement would be lost when the pool
	// reconnects. modernc's `_pragma=name(value)` form takes the value in parentheses.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(%d)",
		path, busyTimeoutMillis,
	)

	handle, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// sql.Open is lazy — without this the first real failure (unwritable directory,
	// corrupt file) would surface deep inside a request instead of at boot.
	if err := handle.Ping(); err != nil {
		handle.Close()
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	return handle, nil
}
