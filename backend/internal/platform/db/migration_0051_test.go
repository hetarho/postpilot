package db

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMigration0051PreservesInspectionAtRecoverySizeBoundary(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "0051_" {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: raw}
	}
	if err := migrate(t.Context(), d.Writer, before); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-14T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"small", "boundary"} {
		if _, err := d.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES(?,'owner','fixture','square',15000,'2026-09-14T00:00:00Z','2026-09-14T00:00:00Z')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES(?,'owner',?,'generate_clip','failed','2026-09-14T00:00:00Z','2026-09-14T00:00:00Z')`, id, id); err != nil {
			t.Fatal(err)
		}
		raw := `{"Version":1,"JobID":"` + id + `","Observations":[{"Segments":[{"Event":""}]}]}`
		if id == "boundary" {
			raw = strings.Replace(raw, `"Event":""`, `"Event":"`+strings.Repeat("x", 2097152-len(raw))+`"`, 1)
			if len(raw) != 2097152 || !json.Valid([]byte(raw)) {
				t.Fatal("invalid boundary fixture")
			}
		}
		if _, err := d.Writer.Exec(`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES(?,'owner',?,?)`, id, id, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateBeforePublishingRemoval(t.Context(), d.Writer); err != nil {
		t.Fatal("valid legacy evidence prevented startup", err)
	}
	if err := migrateBeforePublishingRemoval(t.Context(), d.Writer); err != nil {
		t.Fatal("migration was not idempotent", err)
	}
	var copied, retained, bytes int
	if err := d.Reader.QueryRow(`SELECT count(*) FROM clip_recovery_states WHERE project_id='small' AND json_extract(state_json,'$.Legacy.JobID')='small'`).Scan(&copied); err != nil || copied != 1 {
		t.Fatal("compatible legacy evidence not copied", copied, err)
	}
	if err := d.Reader.QueryRow(`SELECT count(*) FROM clip_recovery_states WHERE project_id='boundary'`).Scan(&copied); err != nil || copied != 0 {
		t.Fatal("oversized recovery was accepted", copied, err)
	}
	if err := d.Reader.QueryRow(`SELECT count(*),length(CAST(checkpoint_json AS BLOB)) FROM clip_attempt_checkpoints WHERE project_id='boundary'`).Scan(&retained, &bytes); err != nil || retained != 1 || bytes != 2097152 {
		t.Fatal("original inspection changed", retained, bytes, err)
	}
}
