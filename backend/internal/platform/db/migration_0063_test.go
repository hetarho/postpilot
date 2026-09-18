package db

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// The cancellation CHECKs named the generation and the render alone, so the
// table refused a revision's cancel (CLIP-131, CLIP-132). Widening them rebuilds
// generation_jobs, and two tables cascade from it, so what the rebuild must not
// do is take a saved checkpoint or recovery state with it.
func TestMigration0063CancelsRevisionsWithoutLosingAttemptEvidence(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "revision.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	prior := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Compare(e.Name(), "0063_") >= 0 {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		prior[e.Name()] = &fstest.MapFile{Data: raw}
	}
	ctx := t.Context()
	if err := migrate(ctx, d.Writer, prior); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-18T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','owner','clip','square',15000,'2026-09-18T00:00:00Z','2026-09-18T00:00:00Z')`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,cancellation_policy_version,created_at,updated_at) VALUES('revision','owner','clip','revise_clip','running',1,'2026-09-18T00:00:00Z','2026-09-18T00:00:00Z')`,
		`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','owner','revision','{"Version":1}')`,
		`INSERT INTO clip_recovery_states(project_id,user_id,job_id,state_json) VALUES('clip','owner','revision','{"Version":1}')`,
	} {
		if _, err := d.Writer.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	// The owner's cancel is what the old table refused.
	if _, err := d.Writer.Exec(`UPDATE generation_jobs SET cancel_requested_at='2026-09-18T00:01:00Z' WHERE id='revision'`); err == nil {
		t.Fatal("the pre-0063 table already accepted a cancelled revision")
	}
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal("migration was not idempotent", err)
	}
	for _, table := range []string{"clip_attempt_checkpoints", "clip_recovery_states"} {
		var kept int
		if err := d.Reader.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE job_id='revision'`).Scan(&kept); err != nil || kept != 1 {
			t.Fatal("the table rebuild cascaded through "+table, kept, err)
		}
	}
	if _, err := d.Writer.Exec(`UPDATE generation_jobs SET cancel_requested_at='2026-09-18T00:01:00Z' WHERE id='revision'`); err != nil {
		t.Fatal("a revision still cannot be cancelled", err)
	}
	if _, err := d.Writer.Exec(`UPDATE generation_jobs SET status='cancelled',finished_at='2026-09-18T00:01:01Z' WHERE id='revision'`); err != nil {
		t.Fatal("a cancelled revision cannot reach its terminal status", err)
	}
	// A revision approved under the legacy policy is still refused, exactly as a
	// generation under it is: the ceiling it never approved cannot be settled.
	if _, err := d.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,cancellation_policy_version,cancel_requested_at,created_at,updated_at) VALUES('legacy','owner','clip','revise_clip','running',0,'2026-09-18T00:02:00Z','2026-09-18T00:02:00Z','2026-09-18T00:02:00Z')`); err == nil {
		t.Fatal("a revision without the cancellation policy was accepted as cancellable")
	}
}
