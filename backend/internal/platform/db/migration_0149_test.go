package db

import (
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

const legacyWaitExpiry144 = `-- +goose Up
ALTER TABLE generation_jobs ADD COLUMN wait_expires_at TEXT;

-- +goose Down
ALTER TABLE generation_jobs DROP COLUMN wait_expires_at;
`

func legacy144Files(t *testing.T) fstest.MapFS {
	t.Helper()
	files := migrationsBefore(t, "0144_")
	files["0144_job_wait_expiry.sql"] = &fstest.MapFile{Data: []byte(legacyWaitExpiry144)}
	return files
}

func seedWaitExpiryUpgradeJob(t *testing.T, h *DB) string {
	t.Helper()
	if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,created_at,plan) VALUES('owner','hash','created','free')"); err != nil {
		t.Fatal(err)
	}
	const payload = `{"owner":"owner","immutable":"original"}`
	if _, err := h.Writer.Exec("INSERT INTO generation_jobs(id,user_id,kind,status,stage,payload,created_at,updated_at) VALUES('kept','owner','clip_render','queued','render_wait',?,'created','updated')", payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func assertWaitExpiryUpgrade(t *testing.T, h *DB, payload string, expiry sql.NullString, legacy144 bool) {
	t.Helper()
	var gotPayload, owner, kind, status, stage string
	var gotExpiry sql.NullString
	if err := h.Reader.QueryRow("SELECT user_id,kind,status,stage,payload,wait_expires_at FROM generation_jobs WHERE id='kept'").Scan(&owner, &kind, &status, &stage, &gotPayload, &gotExpiry); err != nil {
		t.Fatal(err)
	}
	if owner != "owner" || kind != "clip_render" || status != "queued" || stage != "render_wait" || gotPayload != payload || gotExpiry != expiry {
		t.Fatal("existing job or expiry changed", owner, kind, status, stage, gotPayload, gotExpiry)
	}
	for _, version := range []int{145, 146, 147, 148, 149} {
		var count int
		if err := h.Reader.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=? AND is_applied=1", version).Scan(&count); err != nil || count != 1 {
			t.Fatal("forward migration not applied exactly once", version, count, err)
		}
	}
	var legacyCount int
	if err := h.Reader.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=144 AND is_applied=1").Scan(&legacyCount); err != nil || (legacyCount == 1) != legacy144 {
		t.Fatal("legacy144 history changed or invented", legacyCount, err)
	}
	rows, err := h.Reader.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation after upgrade")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestMigration0149UpgradesDeployed147Without144(t *testing.T) {
	h := openTemp(t)
	if err := migrate(t.Context(), h.Writer, migrationsBefore(t, "0144_")); err != nil {
		t.Fatal(err)
	}
	payload := seedWaitExpiryUpgradeJob(t, h)
	main147 := migrationsBefore(t, "0148_")
	if err := migrate(t.Context(), h.Writer, main147); err != nil {
		t.Fatal(err)
	}
	// The old reviewed144 placement is an actual Goose out-of-order failure
	// on the deployed main147 lineage; it must not be silently enabled.
	withOld144 := migrationsBefore(t, "0149_")
	withOld144["0144_job_wait_expiry.sql"] = &fstest.MapFile{Data: []byte(legacyWaitExpiry144)}
	if err := migrate(t.Context(), h.Writer, withOld144); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal("old144 placement did not reproduce the deployment refusal", err)
	}
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	assertWaitExpiryUpgrade(t, h, payload, sql.NullString{}, false)
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal("second boot", err)
	}
	assertWaitExpiryUpgrade(t, h, payload, sql.NullString{}, false)
}

func TestMigration0149PreservesAppliedLegacy144ValuesAndHistory(t *testing.T) {
	h := openTemp(t)
	if err := migrate(t.Context(), h.Writer, legacy144Files(t)); err != nil {
		t.Fatal(err)
	}
	payload := seedWaitExpiryUpgradeJob(t, h)
	const expiry = "2026-10-07T04:30:00.000000000Z"
	if _, err := h.Writer.Exec("UPDATE generation_jobs SET wait_expires_at=? WHERE id='kept'", expiry); err != nil {
		t.Fatal(err)
	}
	if err := migrate(t.Context(), h.Writer, migrationsBefore(t, "0148_")); err != nil {
		t.Fatal("legacy144 to main147", err)
	}
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	assertWaitExpiryUpgrade(t, h, payload, sql.NullString{String: expiry, Valid: true}, true)
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal("second legacy boot", err)
	}
	assertWaitExpiryUpgrade(t, h, payload, sql.NullString{String: expiry, Valid: true}, true)
}

func TestMigration0149FreshDatabase(t *testing.T) {
	h := openTemp(t)
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	payload := seedWaitExpiryUpgradeJob(t, h)
	assertWaitExpiryUpgrade(t, h, payload, sql.NullString{}, false)
}

func TestMigration0149RejectsIncompatibleExistingColumn(t *testing.T) {
	for _, definition := range []string{"INTEGER", "TEXT NOT NULL DEFAULT ''", "TEXT DEFAULT 'invented'"} {
		t.Run(definition, func(t *testing.T) {
			h := openTemp(t)
			if err := migrate(t.Context(), h.Writer, migrationsBefore(t, "0148_")); err != nil {
				t.Fatal(err)
			}
			if _, err := h.Writer.Exec("ALTER TABLE generation_jobs ADD COLUMN wait_expires_at " + definition); err != nil {
				t.Fatal(err)
			}
			if err := Migrate(t.Context(), h.Writer); err == nil || !strings.Contains(err.Error(), "must be nullable TEXT") {
				t.Fatal("incompatible wait column was accepted", err)
			}
			var count int
			if err := h.Reader.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=149").Scan(&count); err != nil || count != 0 {
				t.Fatal("failed migration altered149 history", count, err)
			}
		})
	}
}

func TestMigration0149RefusesBranch148MissingDeployed145To147(t *testing.T) {
	h := openTemp(t)
	branch := legacy144Files(t)
	body, err := fs.ReadFile(migrationsFS, "migrations/0148_browser_analysis_preparations.sql")
	if err != nil {
		t.Fatal(err)
	}
	branch["0148_browser_analysis_preparations.sql"] = &fstest.MapFile{Data: body}
	if err := migrate(t.Context(), h.Writer, branch); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), h.Writer); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal("unsupported branch148 lineage silently skipped missing main migrations", err)
	}
	var count int
	if err := h.Reader.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=149").Scan(&count); err != nil || count != 0 {
		t.Fatal("refused lineage altered149 history", count, err)
	}
}

func TestMigration0149DoesNotEnterUnrelatedMigrationFS(t *testing.T) {
	h := openTemp(t)
	files := fstest.MapFS{"0001_fixture.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE unrelated(value TEXT);\n-- +goose Down\nDROP TABLE unrelated;\n")}}
	if err := migrate(t.Context(), h.Writer, files); err != nil {
		t.Fatal("149 guard entered a filesystem without149", err)
	}
	var version int
	if err := h.Reader.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil || version != 1 {
		t.Fatal("unrelated migration history changed", version, err)
	}
}
