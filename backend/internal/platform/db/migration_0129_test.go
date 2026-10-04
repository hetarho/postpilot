package db

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func provider0129(t *testing.T) (*DB, *goose.Provider) {
	t.Helper()
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(t.Context(), 128); err != nil {
		t.Fatal(err)
	}
	return h, p
}

// jobObjects0129 is every index and trigger that names generation_jobs, with the SQL that
// created it: what a rebuild must write back unchanged.
func jobObjects0129(t *testing.T, h *DB) map[string]string {
	t.Helper()
	rows, err := h.Reader.Query(`SELECT type || ' ' || name, COALESCE(sql, '') FROM sqlite_master
		WHERE type IN ('index', 'trigger') AND (tbl_name = 'generation_jobs' OR sql LIKE '%generation_jobs%')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	objects := map[string]string{}
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatal(err)
		}
		objects[name] = sql
	}
	return objects
}

// F13: which kinds may be stopped is Go's answer alone, so 129 rebuilds generation_jobs without
// the CHECKs that named them. The rebuild keeps every job and child row and writes back every
// index and trigger exactly as 128 left them; the one rule left names no kind. The way back
// drops only the stopped jobs of a kind the old CHECKs do not name.
func TestMigration0129TakesTheKindListsOutOfGenerationJobs(t *testing.T) {
	h, p := provider0129(t)
	const at = "2026-10-04T00:00:00Z"
	exec := func(stmt string) error {
		_, err := h.Writer.Exec(stmt)
		return err
	}
	for _, stmt := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','alice','clip','vertical',30000,'` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('render','alice','clip','render_clip','running','` + at + `','` + at + `')`,
		`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','render','{"Version":1}')`,
		`INSERT INTO job_continuations(job_id,wait_key,state,created_at,updated_at) VALUES('render','media','waiting','` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,user_id,kind,status,payload,cancellation_policy_version,created_at,updated_at) VALUES('kept','alice','template_request','done','{}',1,'` + at + `','` + at + `')`,
	} {
		if err := exec(stmt); err != nil {
			t.Fatalf("seed %q: %v", stmt, err)
		}
	}
	stopNewKind := []string{
		`INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('future','alice','future_kind','running','` + at + `','` + at + `')`,
		`UPDATE generation_jobs SET cancel_requested_at='` + at + `' WHERE id='future'`,
		`UPDATE generation_jobs SET status='cancelled', finished_at='` + at + `' WHERE id='future'`,
	}
	if err := exec(`INSERT INTO generation_jobs(id,user_id,kind,status,cancel_requested_at,created_at,updated_at) VALUES('early','alice','future_kind','running','` + at + `','` + at + `','` + at + `')`); err == nil {
		t.Fatal("the pre-129 table already accepted a stop of a kind it does not name")
	}
	before := jobObjects0129(t, h)

	if _, err := p.UpTo(t.Context(), 129); err != nil {
		t.Fatal(err)
	}
	after := jobObjects0129(t, h)
	if len(before) == 0 || len(after) != len(before) {
		t.Fatalf("the rebuild changed the set of indexes and triggers: %d before, %d after", len(before), len(after))
	}
	for name, sql := range before {
		if after[name] != sql {
			t.Errorf("%s was not written back unchanged:\nbefore %s\nafter  %s", name, sql, after[name])
		}
	}
	var create string
	if err := h.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='generation_jobs'`).Scan(&create); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(create, "kind IN (") {
		t.Fatalf("generation_jobs still names kinds in its CREATE statement:\n%s", create)
	}
	for table, want := range map[string]int{"generation_jobs": 2, "clip_attempt_checkpoints": 1, "job_continuations": 1} {
		var kept int
		if err := h.Reader.QueryRow(`SELECT count(*) FROM ` + table).Scan(&kept); err != nil || kept != want {
			t.Fatalf("the rebuild kept %d rows of %s, want %d: %v", kept, table, want, err)
		}
	}
	for _, stmt := range stopNewKind {
		if err := exec(stmt); err != nil {
			t.Fatalf("after 129 %q: %v", stmt, err)
		}
	}
	if err := exec(`INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('unasked','alice','future_kind','cancelled','` + at + `','` + at + `')`); err == nil {
		t.Fatal("a job was cancelled without being asked to stop")
	}
	// Every trigger came back: one active clip job per project still holds.
	if err := exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('second','alice','clip','render_clip','queued','` + at + `','` + at + `')`); err == nil {
		t.Fatal("a second active clip job was accepted after the rebuild")
	}

	if _, err := p.DownTo(t.Context(), 128); err != nil {
		t.Fatalf("down: %v", err)
	}
	var future, kept int
	if err := h.Reader.QueryRow(`SELECT (SELECT count(*) FROM generation_jobs WHERE id='future'), (SELECT count(*) FROM generation_jobs WHERE id IN ('render','kept'))`).Scan(&future, &kept); err != nil || future != 0 || kept != 2 {
		t.Fatalf("the way back kept %d stopped jobs of an unnamed kind and %d other jobs: %v", future, kept, err)
	}
	if err := exec(`INSERT INTO generation_jobs(id,user_id,kind,status,cancel_requested_at,created_at,updated_at) VALUES('late','alice','future_kind','running','` + at + `','` + at + `','` + at + `')`); err == nil {
		t.Fatal("the way back did not restore the kind-naming CHECKs")
	}
	if back := jobObjects0129(t, h); len(back) != len(before) {
		t.Fatalf("the way back left %d indexes and triggers, want %d", len(back), len(before))
	}
}

// The rebuild proves only its own graph: an orphan in generation_jobs or a child of it aborts
// the migration, while a violation elsewhere in the database is left for its own owner.
func TestMigration0129ChecksForeignKeysOfTheJobGraphOnly(t *testing.T) {
	const at = "2026-10-04T00:00:00Z"
	for _, tc := range []struct {
		name     string
		orphan   string
		upPasses bool
	}{
		{"an unrelated orphan", `INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('lost','ghost','` + at + `','` + at + `')`, true},
		{"an orphaned checkpoint", `INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','gone','{"Version":1}')`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, p := provider0129(t)
			for _, stmt := range []string{
				`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
				`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','alice','clip','vertical',30000,'` + at + `','` + at + `')`,
				`PRAGMA foreign_keys=OFF`,
				tc.orphan,
				`PRAGMA foreign_keys=ON`,
			} {
				if _, err := h.Writer.Exec(stmt); err != nil {
					t.Fatalf("seed %q: %v", stmt, err)
				}
			}
			var violations int
			if err := h.Writer.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations == 0 {
				t.Fatalf("the seeded orphan is not a violation: %d %v", violations, err)
			}
			_, err := p.UpTo(t.Context(), 129)
			if tc.upPasses && err != nil {
				t.Fatalf("an orphan outside the job graph stopped the rebuild: %v", err)
			}
			if !tc.upPasses && (err == nil || !strings.Contains(err.Error(), "CHECK constraint failed")) {
				t.Fatalf("an orphan in the job graph passed the rebuild's guard: %v", err)
			}
		})
	}
}
