package db

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// F37: the refund guard triggers read the funding value a guard row carries instead of
// deciding by the payment's kind. A guard already held when 0130 runs is given its kind's
// value, so it blocks exactly the grants it blocked before; Down restores the kind-reading
// triggers.
func TestMigration0130KeepsHeldRefundGuardsBlockingTheSameGrants(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(t.Context(), 129); err != nil {
		t.Fatal(err)
	}
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := h.Writer.Exec(stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	const (
		start = "2026-03-10T00:00:00.000000000Z"
		end   = "2026-04-10T00:00:00.000000000Z"
	)
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, start)
	exec(`INSERT INTO credit_refund_funding_guards(request_id,user_id,order_id,kind,coverage_id,starts_at,ends_at)
		VALUES ('r-upg','alice','pp-upg','upgrade','cov-upg',?,?), ('r-sub','alice','pp-sub','subscribe','cov-sub',?,?)`,
		start, end, start, end)
	exec(`INSERT INTO server_export_refund_guards(request_id,user_id,order_id,kind,coverage_id,starts_at,ends_at)
		VALUES ('r-upg','alice','pp-upg','upgrade','cov-upg',?,?)`, start, end)

	issued := func(id string) bool {
		t.Helper()
		var n int
		if err := h.Reader.QueryRow(`SELECT count(*) FROM credit_lots WHERE id=?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}
	lot := func(id, kind, coverage, cause, correlation, window string) {
		t.Helper()
		exec(`INSERT INTO credit_lots(id,user_id,kind,granted,remaining,created_at,coverage_id,window_start,issuance_cause,correlation_id)
			VALUES (?,'alice',?,5,5,?,NULLIF(?,''),?,NULLIF(?,''),NULLIF(?,''))`, id, kind, start, coverage, window, cause, correlation)
	}
	// Each grant: whether the guards block it, under the kind-reading and the value-reading triggers alike.
	grants := []struct {
		id, kind, coverage, cause, correlation string
		blocked                                bool
	}{
		{"upg-lazy-daily", "daily", "cov-upg", "lazy", "", true},
		{"upg-coverage-daily", "daily", "cov-upg", "coverage", "", false},
		{"upg-bonus", "monthly", "cov-upg", "upgrade", "pp-upg", true},
		{"sub-coverage-daily", "daily", "cov-sub", "coverage", "", true},
		{"sub-lazy-monthly", "monthly", "cov-sub", "lazy", "", true},
		{"sub-upgrade-bonus", "monthly", "cov-sub", "upgrade", "pp-other", false},
	}
	// Each stage issues into its own window inside the guards' range, so no two stages share a
	// grant window.
	check := func(version int64, window string) {
		t.Helper()
		for _, g := range grants {
			id := window + "-" + g.id
			lot(id, g.kind, g.coverage, g.cause, g.correlation, window)
			if issued(id) == g.blocked {
				t.Errorf("at %d %s: blocked=%t, want %t", version, g.id, !issued(id), g.blocked)
			}
		}
		exec(`INSERT INTO server_export_windows(user_id,coverage_id,window_start,window_end,allowance) VALUES ('alice','cov-upg',?,?,3)`,
			window, end)
		var windows int
		if err := h.Reader.QueryRow(`SELECT count(*) FROM server_export_windows WHERE window_start=?`, window).Scan(&windows); err != nil {
			t.Fatal(err)
		}
		if windows != 0 {
			t.Errorf("at %d an upgrade's guarded window opened", version)
		}
	}
	check(129, "2026-03-12T00:00:00.000000000Z")

	if _, err = p.UpTo(t.Context(), 130); err != nil {
		t.Fatal(err)
	}
	var correlation, cause string
	if err := h.Reader.QueryRow(`SELECT correlation_id,window_cause FROM credit_refund_funding_guards WHERE request_id='r-upg'`).
		Scan(&correlation, &cause); err != nil || correlation != "pp-upg" || cause != "lazy" {
		t.Fatalf("upgrade guard correlation=%q cause=%q err=%v", correlation, cause, err)
	}
	if err := h.Reader.QueryRow(`SELECT correlation_id,window_cause FROM credit_refund_funding_guards WHERE request_id='r-sub'`).
		Scan(&correlation, &cause); err != nil || correlation != "" || cause != "" {
		t.Fatalf("subscription guard correlation=%q cause=%q err=%v", correlation, cause, err)
	}
	check(130, "2026-03-13T00:00:00.000000000Z")

	if _, err = p.DownTo(t.Context(), 129); err != nil {
		t.Fatalf("down: %v", err)
	}
	check(129, "2026-03-14T00:00:00.000000000Z")
}
