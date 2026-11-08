package dbtest_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/platform/db/dbtest"
)

// A copy is the database a fresh migration builds: the same schema objects, the same rows in
// every table (goose's applied-at stamp aside), and the same pragmas once opened.
func TestCopyMatchesFreshMigration(t *testing.T) {
	ctx := context.Background()
	copied := dbtest.Open(t)
	fresh, err := db.Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fresh.Close() })
	if err := db.Migrate(ctx, fresh.Writer); err != nil {
		t.Fatal(err)
	}

	gotSchema, wantSchema := schema(t, copied.Reader), schema(t, fresh.Reader)
	if len(wantSchema) == 0 {
		t.Fatal("a fresh migration has no schema objects")
	}
	if !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Fatalf("sqlite_master differs:\ncopy  %d objects\nfresh %d objects\nfirst difference: %s", len(gotSchema), len(wantSchema), firstDifference(gotSchema, wantSchema))
	}

	if got, want := pragmaInt(t, copied.Reader, "user_version"), pragmaInt(t, fresh.Reader, "user_version"); got != want {
		t.Fatalf("user_version = %d, want %d", got, want)
	}
	for name, pool := range map[string]*sql.DB{"writer": copied.Writer, "reader": copied.Reader} {
		var mode string
		if err := pool.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if mode != "wal" {
			t.Fatalf("%s journal_mode = %q, want wal", name, mode)
		}
		if got := pragmaInt(t, pool, "foreign_keys"); got != 1 {
			t.Fatalf("%s foreign_keys = %d, want 1", name, got)
		}
	}
	violations, err := copied.Reader.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if violations.Next() {
		_ = violations.Close()
		t.Fatal("foreign_key_check reports a violation on the copy")
	}
	if err := violations.Close(); err != nil {
		t.Fatal(err)
	}

	tables := tableNames(t, fresh.Reader)
	if !reflect.DeepEqual(tableNames(t, copied.Reader), tables) {
		t.Fatal("the copy's tables differ from a fresh migration's")
	}
	for _, table := range tables {
		got, want := rows(t, copied.Reader, table), rows(t, fresh.Reader, table)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("table %s: copy has %d rows, fresh has %d; first difference: %s", table, len(got), len(want), firstDifference(got, want))
		}
		if table == "goose_db_version" && len(want) == 0 {
			t.Fatal("goose_db_version is empty: the comparison proves nothing")
		}
	}
}

// Each copy is its own file: a write through one is seen by neither a copy opened beside it
// nor one opened after it.
func TestCopiesAreIndependent(t *testing.T) {
	first := dbtest.Open(t)
	second := dbtest.Open(t)
	if _, err := first.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('dbtest-probe','hash','2026-11-09T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if got := probeUsers(t, first); got != 1 {
		t.Fatalf("the writing copy sees %d probe users, want 1", got)
	}
	if got := probeUsers(t, second); got != 0 {
		t.Fatalf("a second copy sees %d probe users, want 0", got)
	}
	if got := probeUsers(t, dbtest.Open(t)); got != 0 {
		t.Fatalf("a later copy sees %d probe users, want 0", got)
	}
}

func probeUsers(t *testing.T, handle *db.DB) int {
	t.Helper()
	var n int
	if err := handle.Reader.QueryRow(`SELECT COUNT(*) FROM users WHERE id='dbtest-probe'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func schema(t *testing.T, pool *sql.DB) []string {
	t.Helper()
	found, err := pool.Query(`SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer found.Close()
	var out []string
	for found.Next() {
		var kind, name, table, text string
		if err := found.Scan(&kind, &name, &table, &text); err != nil {
			t.Fatal(err)
		}
		out = append(out, strings.Join([]string{kind, name, table, text}, "\x1f"))
	}
	if err := found.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func pragmaInt(t *testing.T, pool *sql.DB, name string) int {
	t.Helper()
	var value int
	if err := pool.QueryRow("PRAGMA " + name).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func tableNames(t *testing.T, pool *sql.DB) []string {
	t.Helper()
	found, err := pool.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer found.Close()
	var out []string
	for found.Next() {
		var name string
		if err := found.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	if err := found.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// rows renders every row of table as one sorted string per row. Goose's tstamp is when a
// migration ran, which is the one value a copy cannot share with a later migration.
func rows(t *testing.T, pool *sql.DB, table string) []string {
	t.Helper()
	columns, err := pool.Query(`SELECT name FROM pragma_table_xinfo(?) WHERE hidden != 1 ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for columns.Next() {
		var name string
		if err := columns.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if table == "goose_db_version" && name == "tstamp" {
			continue
		}
		names = append(names, `"`+strings.ReplaceAll(name, `"`, `""`)+`"`)
	}
	if err := columns.Close(); err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatalf("table %s has no columns", table)
	}
	found, err := pool.Query(`SELECT ` + strings.Join(names, ", ") + ` FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
	if err != nil {
		t.Fatal(err)
	}
	defer found.Close()
	var out []string
	for found.Next() {
		values := make([]any, len(names))
		targets := make([]any, len(names))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := found.Scan(targets...); err != nil {
			t.Fatal(err)
		}
		cells := make([]string, len(values))
		for i, value := range values {
			cells[i] = fmt.Sprintf("%T:%v", value, value)
		}
		out = append(out, strings.Join(cells, "\x1f"))
	}
	if err := found.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func firstDifference(got, want []string) string {
	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			return fmt.Sprintf("copy %q, fresh %q", got[i], want[i])
		}
	}
	return fmt.Sprintf("lengths %d and %d", len(got), len(want))
}
