// Package dbtest hands DB-backed tests a private, fully migrated SQLite file without
// replaying every migration per test.
//
// The schema is built once per test binary by the real db.Migrate on a db.Open-ed file;
// each caller then gets its own byte copy of that file. A copy, not a shared connection:
// every test keeps its own file, so writer serialization, WAL and file-level behaviour stay
// what a freshly migrated database gives.
//
// This is test support. Only _test.go files import it; production code never does, and it
// imports nothing but internal/platform/db and the standard library.
package dbtest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/postpilot/backend/internal/platform/db"
)

var (
	templateOnce  sync.Once
	templateBytes []byte
	templateErr   error
)

// Open returns a fresh migrated database in its own file under t.TempDir(), closed when the
// test cleans up.
func Open(t testing.TB) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	WriteMigrated(t, path)
	handle, err := db.Open(path)
	if err != nil {
		t.Fatalf("dbtest: open migrated copy: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	return handle
}

// WriteMigrated writes a migrated database file at path for a test that opens, reopens or
// backs up the file by its path. The path must not exist yet: a copy never replaces a file
// a test already holds. Its parent directory is created the way db.Open creates it.
func WriteMigrated(t testing.TB, path string) {
	t.Helper()
	data, err := migratedTemplate()
	if err != nil {
		t.Fatalf("dbtest: migrated template: %v", err)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("dbtest: create db directory %s: %v", dir, err)
		}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatalf("dbtest: create %s: %v", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		t.Fatalf("dbtest: write %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("dbtest: close %s: %v", path, err)
	}
}

// migratedTemplate builds the template once per binary. A failure is kept and returned to
// every later caller, so each test fails with the original error rather than a nil copy.
func migratedTemplate() ([]byte, error) {
	templateOnce.Do(func() { templateBytes, templateErr = buildTemplate() })
	return templateBytes, templateErr
}

// buildTemplate migrates a file in its own temporary directory, folds the WAL back into the
// main file, closes both pools and keeps the main file's bytes; the directory is removed.
func buildTemplate() ([]byte, error) {
	dir, err := os.MkdirTemp("", "dbtest-template-")
	if err != nil {
		return nil, fmt.Errorf("create template directory: %w", err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "template.db")

	handle, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		_ = handle.Close()
		return nil, err
	}
	// TRUNCATE moves every WAL frame into the main file and empties the log; busy=1 would
	// mean a reader held frames back and the main file is not the whole schema.
	var busy, logFrames, checkpointed int
	if err := handle.Writer.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("checkpoint template: %w", err)
	}
	if busy != 0 {
		_ = handle.Close()
		return nil, fmt.Errorf("checkpoint template: busy (log=%d checkpointed=%d)", logFrames, checkpointed)
	}
	if err := handle.Close(); err != nil {
		return nil, fmt.Errorf("close template: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); err == nil {
			return nil, fmt.Errorf("template %s file left after checkpoint and close", suffix)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("stat template %s: %w", suffix, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}
	return data, nil
}
