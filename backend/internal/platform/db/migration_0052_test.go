package db

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

// A legacy source keeps the audio meaning its saved plan already had, so an
// existing project rerenders to the same clip (CLIP-101). Everything else —
// a plan that never mentions the source, a malformed one, no plan at all —
// starts off, which is CLIP-18's default.
func TestMigration0052BackfillsOwnerSoundFromEverySavedPlanShape(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "audio.db"))
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
		if entry.Name() >= "0052_" {
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
	cut := func(source, fingerprint, volume string) string {
		return `{"ID":"c-` + source + `","SourceID":"` + source + `","Fingerprint":"` + fingerprint + `","StartMS":0,"EndMS":10000,"TransitionMS":0,` + volume + `}`
	}
	cases := []struct {
		project, plan string
		retain        bool
	}{
		// Version 0: a nil Volume rendered at full original sound.
		{"v0on", `{"Ratio":"vertical","DurationMS":10000,"Cuts":[` + cut("s-v0on", "f-v0on", `"Volume":null`) + `]}`, true},
		{"v0off", `{"Ratio":"vertical","DurationMS":10000,"Cuts":[` + cut("s-v0off", "f-v0off", `"Volume":0`) + `]}`, false},
		// Versions 1-5 state it in permille.
		{"v4on", `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":10000,"Hook":"","Cuts":[` + cut("s-v4on", "f-v4on", `"VolumePermille":800`) + `]}}`, true},
		{"v4off", `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":10000,"Hook":"","Cuts":[` + cut("s-v4off", "f-v4off", `"VolumePermille":0`) + `]}}`, false},
		// Mixed legacy cuts of ONE source: any audible cut enables the source.
		{"mixed", `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":20000,"Hook":"","Cuts":[` +
			`{"ID":"a","SourceID":"s-mixed","Fingerprint":"f-mixed","StartMS":0,"EndMS":10000,"TransitionMS":0,"VolumePermille":0},` +
			`{"ID":"b","SourceID":"s-mixed","Fingerprint":"f-mixed","StartMS":10000,"EndMS":20000,"TransitionMS":0,"VolumePermille":1000}]}}`, true},
		// Version 6 already states the owner's answer; per-cut volume does not.
		{"v6", `{"Version":6,"Ratio":"vertical","Plan":{"DurationMS":10000,"Hook":"","Cuts":[` + cut("s-v6", "f-v6", `"VolumePermille":1000`) + `]},` +
			`"SourceAudio":[{"SourceID":"s-v6","Fingerprint":"f-v6","RetainOriginal":false}]}`, false},
		{"v6on", `{"Version":6,"Ratio":"vertical","Plan":{"DurationMS":10000,"Hook":"","Cuts":[` + cut("s-v6on", "f-v6on", `"VolumePermille":0`) + `]},` +
			`"SourceAudio":[{"SourceID":"s-v6on","Fingerprint":"f-v6on","RetainOriginal":true}]}`, true},
		// A source the plan never names, a plan that is not JSON at all, and a
		// project with no plan: all safely off.
		{"other", `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":10000,"Hook":"","Cuts":[` + cut("someone-else", "f-other", `"VolumePermille":1000`) + `]}}`, false},
		{"malformed", `{"Version":4,"Plan":{`, false},
		{"noplan", "", false},
		// A fingerprint that does not match is a different file: the decision
		// made about the old one never transfers.
		{"replaced", `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":10000,"Hook":"","Cuts":[` + cut("s-replaced", "old-fingerprint", `"VolumePermille":1000`) + `]}}`, false},
	}
	for _, c := range cases {
		plan := any(nil)
		if c.plan != "" {
			plan = c.plan
		}
		if _, err := d.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,created_at,updated_at) VALUES(?,'owner','fixture','vertical',15000,?,1,'2026-09-14T00:00:00Z','2026-09-14T00:00:00Z')`, c.project, plan); err != nil {
			t.Fatal(c.project, err)
		}
		if _, err := d.Writer.Exec(`INSERT INTO clip_source_batches(id,user_id,project_id,state,created_at,expires_at,put_expires_at) VALUES(?,'owner',?,'ready','2026-09-14T00:00:00Z','2026-09-15T00:00:00Z','2026-09-15T00:00:00Z')`, "b-"+c.project, c.project); err != nil {
			t.Fatal(c.project, err)
		}
		if _, err := d.Writer.Exec(`INSERT INTO clip_source_leases(id,canonical_id,batch_id,user_id,object_key,filename,content_type,fingerprint,declared_bytes,duration_ms,width,height,state,ordinal) VALUES(?,?,?,'owner',?,'a.mp4','video/mp4',?,1,10000,1920,1080,'ready',0)`,
			"l-"+c.project, "s-"+c.project, "b-"+c.project, "key-"+c.project, "f-"+c.project); err != nil {
			t.Fatal(c.project, err)
		}
	}
	if err := migrateBeforePublishingRemoval(t.Context(), d.Writer); err != nil {
		t.Fatal("a saved plan prevented startup", err)
	}
	if err := migrateBeforePublishingRemoval(t.Context(), d.Writer); err != nil {
		t.Fatal("migration was not idempotent", err)
	}
	for _, c := range cases {
		var retain int
		if err := d.Reader.QueryRow(`SELECT retain_original_audio FROM clip_source_leases WHERE id=?`, "l-"+c.project).Scan(&retain); err != nil {
			t.Fatal(c.project, err)
		}
		if (retain == 1) != c.retain {
			t.Fatalf("%s: retained=%d, wanted %v", c.project, retain, c.retain)
		}
	}
	// The stored plans themselves are never rewritten by the migration.
	var plan string
	if err := d.Reader.QueryRow(`SELECT edit_plan_json FROM clip_projects WHERE id='mixed'`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan != cases[4].plan {
		t.Fatal("the migration rewrote a stored plan", plan)
	}
	// Rolling back removes the column and leaves every project intact.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(t.Context(), 51); err != nil {
		t.Fatal("source-sound rollback failed", err)
	}
	var projects, leases int
	if err := d.Reader.QueryRow(`SELECT count(*) FROM clip_projects`).Scan(&projects); err != nil || projects != len(cases) {
		t.Fatal("rollback lost projects", projects, err)
	}
	if err := d.Reader.QueryRow(`SELECT count(*) FROM clip_source_leases`).Scan(&leases); err != nil || leases != len(cases) {
		t.Fatal("rollback lost source leases", leases, err)
	}
	if err := d.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('clip_source_leases') WHERE name='retain_original_audio'`).Scan(&leases); err != nil || leases != 0 {
		t.Fatal("rollback kept the column", leases, err)
	}
	if err := migrateBeforePublishingRemoval(t.Context(), d.Writer); err != nil {
		t.Fatal("re-applying after rollback failed", err)
	}
}
