package db

import (
	"io/fs"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

// QUOTA-14, QUOTA-59: on a seeded ledger, 0131 charges nothing for unfinished pre-FX work —
// every credit an open admission without a frozen rate held returns to its own lot, never past
// the grant, and the admission goes with its hold and eligible-lot rows — while FX, settled
// and all-free admissions stay as they are; every still-open monthly lot without a coverage
// ends at the migration and every other lot keeps its expiry.
func TestMigration0131ClosesThePreFXLedgerOnce(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(t.Context(), 129); err != nil {
		t.Fatal(err)
	}
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := h.Writer.Exec(stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	const at = "2026-10-01T00:00:00.000000000Z"
	future := time.Now().UTC().Add(20 * 24 * time.Hour).Format("2006-01-02T15:04:05.000000000Z07:00")
	past := "2026-01-01T00:00:00.000000000Z"
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?),('root','hash',?)`, at, at)
	lot := func(id, kind string, granted, remaining int, expires any, coverage any) {
		t.Helper()
		exec(`INSERT INTO credit_lots(id,user_id,kind,granted,remaining,expires_at,created_at,coverage_id) VALUES(?,?,?,?,?,?,?,?)`,
			id, "alice", kind, granted, remaining, expires, at, coverage)
	}
	// The bonus lot's remainder already lacks what the holds below took from it. The purchased
	// lot holds 7 of 10 although 5 are held from it: returning all 5 would pass its grant.
	lot("bonus", "bonus", 100, 100-5-4, nil, nil)
	lot("purchased", "purchased", 10, 7, nil, nil)
	lot("legacy-open", "monthly", 50, 50, future, nil)
	lot("legacy-lapsed", "monthly", 50, 50, past, nil)
	lot("covered", "monthly", 50, 50, future, "coverage-1")
	admission := func(job, user string, hold int, settled, fx bool) {
		t.Helper()
		var settledAt, source, published, reference, applied any
		if settled {
			settledAt = at
		}
		if fx {
			source, published, reference, applied = "korea-eximbank", "2026-09-30", 13_600_000, 13_600_000
		}
		exec(`INSERT INTO usage_admissions(user_id,kind,job_id,hold_credits,created_at,settled_at,settled_credits,
			fx_source,fx_publication_date,fx_reference_e4,fx_applied_e4) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			user, "generate", job, hold, at, settledAt, map[bool]any{true: 0, false: nil}[settled], source, published, reference, applied)
	}
	debit := func(job, lotID string, credits int) {
		t.Helper()
		exec(`INSERT INTO credit_hold_lots(job_id,lot_id,credits) VALUES(?,?,?)`, job, lotID, credits)
		exec(`INSERT INTO usage_admission_eligible_lots(job_id,lot_id) VALUES(?,?)`, job, lotID)
	}
	admission("pre-fx-open", "alice", 7, false, false)
	debit("pre-fx-open", "bonus", 5)
	debit("pre-fx-open", "purchased", 2)
	admission("pre-fx-capped", "alice", 3, false, false)
	debit("pre-fx-capped", "purchased", 3)
	admission("pre-fx-master", "root", 4, false, false)
	admission("fx-open", "alice", 4, false, true)
	debit("fx-open", "bonus", 4)
	admission("pre-fx-settled", "alice", 9, true, false)
	admission("all-free", "alice", 0, false, false)

	if _, err := p.UpTo(t.Context(), 131); err != nil {
		t.Fatal(err)
	}

	remaining := func(id string) int {
		t.Helper()
		var n int
		if err := h.Reader.QueryRow(`SELECT remaining FROM credit_lots WHERE id=?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := remaining("bonus"); got != 100-4 {
		t.Fatalf("bonus = %d, want only the FX hold taken", got)
	}
	if got := remaining("purchased"); got != 10 {
		t.Fatalf("purchased = %d, want the pre-FX debits back, stopped at its grant", got)
	}
	jobs := func(table string) map[string]bool {
		t.Helper()
		rows, err := h.Reader.Query(`SELECT DISTINCT job_id FROM ` + table)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var job string
			if err := rows.Scan(&job); err != nil {
				t.Fatal(err)
			}
			out[job] = true
		}
		return out
	}
	admitted := jobs("usage_admissions")
	if len(admitted) != 3 || !admitted["fx-open"] || !admitted["pre-fx-settled"] || !admitted["all-free"] {
		t.Fatalf("admissions left = %v", admitted)
	}
	for _, table := range []string{"credit_hold_lots", "usage_admission_eligible_lots"} {
		if left := jobs(table); len(left) != 1 || !left["fx-open"] {
			t.Fatalf("%s left = %v", table, left)
		}
	}

	expiry := func(id string) string {
		t.Helper()
		var at string
		if err := h.Reader.QueryRow(`SELECT expires_at FROM credit_lots WHERE id=?`, id).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	ended, err := time.Parse(time.RFC3339Nano, expiry("legacy-open"))
	if err != nil || ended.After(time.Now()) || time.Since(ended) > time.Minute || len(expiry("legacy-open")) != len(future) {
		t.Fatalf("the open legacy lot ends at %q (%v), want now in the store's layout", expiry("legacy-open"), err)
	}
	if expiry("legacy-lapsed") != past || expiry("covered") != future {
		t.Fatalf("other monthly lots moved: lapsed %q covered %q", expiry("legacy-lapsed"), expiry("covered"))
	}
}
