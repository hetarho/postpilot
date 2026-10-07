package db

import (
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func authoringBefore0152(t *testing.T) (*DB, *goose.Provider) {
	t.Helper()
	h := openTemp(t)
	if err := migrate(t.Context(), h.Writer, migrationsBefore(t, "0152_")); err != nil {
		t.Fatal(err)
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	return h, p
}

func objectsWithoutAuthoringParent0152(t *testing.T, h *DB) map[string]string {
	t.Helper()
	rows, err := h.Reader.Query(`SELECT name,sql FROM sqlite_master WHERE sql IS NOT NULL AND name<>'configuration_authoring_sessions' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatal(err)
		}
		result[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMigration0152PreservesAuthoringAndPaidEvidence(t *testing.T) {
	h, p := authoringBefore0152(t)
	seedCreationPreservation(t, h, true)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := h.Writer.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	// The aggregate snapshot owns recommendations, chat, active work and save
	// receipts. The rebuild must copy its exact bytes, including unused fields.
	exec(`UPDATE configuration_authoring_sessions SET target_id='target',phase='generating',snapshot=?,publication_pending=1,target_conflict=1 WHERE id='session'`, `{"Candidates":[{"ID":"kept-candidate","Revision":7}],"Turns":[{"Request":"kept message","Reply":"kept reply"}],"ActiveJobID":"job","Saved":{"ID":"kept-saved"},"unknown":"preserved"}`)
	exec(`INSERT INTO configuration_authoring_operations(id,user_id,session_id,request_id,fingerprint,base_revision,mode,payload,job_id,status,created_at) VALUES('operation','owner','session','prepare','kept-fingerprint',7,'recommend',X'0080FF','job','admitted','created')`)
	for _, table := range []string{"template_authoring_publications", "video_template_authoring_publications", "voice_authoring_publications"} {
		exec(fmt.Sprintf(`INSERT INTO %s(user_id,session_id,revision,publication_key,target_id,created_at) VALUES('owner','session',7,'%s','target','created')`, table, table))
	}
	exec(`INSERT INTO guideline_authoring_publications(user_id,session_id,revision,publication_key,kind,target_id,created_at) VALUES('owner','session',7,'guide-publication','post','target','created')`)
	tables := []string{
		"configuration_authoring_sessions", "configuration_authoring_operations", "configuration_authoring_mutations", "generation_jobs",
		"template_authoring_publications", "video_template_authoring_publications", "guideline_authoring_publications", "voice_authoring_publications",
		"writing_tests", "writing_test_candidates", "writing_test_matches", "writing_test_publications",
		"post_test_publications", "provider_test_publications", "template_test_publications", "guideline_test_publications", "voice_test_publications",
	}
	before := map[string][][]string{}
	for _, table := range tables {
		before[table] = creationRows(t, h, table)
	}
	objects := objectsWithoutAuthoringParent0152(t, h)
	if _, err := p.UpTo(t.Context(), 152); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if !reflect.DeepEqual(before[table], creationRows(t, h, table)) {
			t.Fatal("migration changed retained evidence", table)
		}
	}
	if !reflect.DeepEqual(objects, objectsWithoutAuthoringParent0152(t, h)) {
		t.Fatal("migration changed an index, trigger or unrelated table")
	}
	var enforced, violations int
	if err := h.Writer.QueryRow("PRAGMA foreign_keys").Scan(&enforced); err != nil || enforced != 1 {
		t.Fatal("writer foreign keys remain disabled", enforced, err)
	}
	if err := h.Reader.QueryRow("SELECT count(*) FROM pragma_foreign_key_check").Scan(&violations); err != nil || violations != 0 {
		t.Fatal("foreign-key graph changed", violations, err)
	}
	// Replay after binary rollback remains forward-only and recognizes0152's
	// canonical successor while retaining all old creation-lineage guarantees.
	if _, err := p.DownTo(t.Context(), 149); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal("forward-only creation replay failed", err)
	}
	var uncaptured int
	if err := h.Reader.QueryRow(`SELECT count(*) FROM configuration_authoring_operations WHERE request_capture IS NULL AND capture_revision IS NULL AND capture_purged=0`).Scan(&uncaptured); err != nil || uncaptured != len(before["configuration_authoring_operations"]) {
		t.Fatal("upgrade fabricated or purged historical authoring request evidence", uncaptured, err)
	}
	for _, table := range tables {
		if !reflect.DeepEqual(before[table], creationRows(t, h, table)) {
			t.Fatal("rollback/replay changed retained evidence", table)
		}
	}
	assertMergedCreationSchemas(t, h)
	// Both composite ownership and ordinary FK enforcement survive rebuilding.
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('other','hash','created')`)
	for _, query := range []string{
		`INSERT INTO configuration_authoring_sessions(id,user_id,kind,request_id,revision,phase,snapshot,created_at,updated_at) VALUES('duplicate','owner','post_template','create',1,'editing','{}','created','updated')`,
		`INSERT INTO configuration_authoring_operations(id,user_id,session_id,request_id,fingerprint,base_revision,mode,payload,status,created_at) VALUES('foreign','other','session','foreign','fingerprint',1,'recommend',X'00','done','created')`,
		`INSERT INTO configuration_authoring_mutations(user_id,session_id,operation_key,action,expected_revision,fingerprint,response,created_at) VALUES('other','session','foreign','patch',1,'fingerprint','{}','created')`,
	} {
		if _, err := h.Writer.Exec(query); err == nil {
			t.Fatal("owner or idempotency constraint lost", query)
		}
	}
	exec(`DELETE FROM configuration_authoring_sessions WHERE id='session'`)
	for _, table := range []string{"configuration_authoring_sessions", "configuration_authoring_operations", "configuration_authoring_mutations"} {
		if rows := creationRows(t, h, table); len(rows) != 0 {
			t.Fatal("session deletion did not cascade", table, rows)
		}
	}
	for _, table := range []string{"generation_jobs", "template_authoring_publications", "video_template_authoring_publications", "guideline_authoring_publications", "voice_authoring_publications"} {
		if !reflect.DeepEqual(before[table], creationRows(t, h, table)) {
			t.Fatal("session deletion changed independent paid receipts", table)
		}
	}
}

func TestMigration0152EnforcesExactCountsAndKeepsTheDefault(t *testing.T) {
	h, p := authoringBefore0152(t)
	if _, err := h.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','created')`); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO configuration_authoring_sessions(id,user_id,kind,request_id,revision,phase,snapshot,created_at,updated_at,candidate_count) VALUES(?,'owner','post_template',?,0,'choosing','{}','created','updated',?)`
	if _, err := h.Writer.Exec(insert, "not-yet", "not-yet", 1); err == nil {
		t.Fatal("the old schema already accepted a single candidate")
	}
	if _, err := p.UpTo(t.Context(), 152); err != nil {
		t.Fatal(err)
	}
	for count := 1; count <= 16; count++ {
		id := fmt.Sprintf("count%d", count)
		if _, err := h.Writer.Exec(insert, id, id, count); err != nil {
			t.Fatal("exact count rejected", count, err)
		}
		var actual int
		if err := h.Reader.QueryRow(`SELECT candidate_count FROM configuration_authoring_sessions WHERE id=?`, id).Scan(&actual); err != nil || actual != count {
			t.Fatal("exact count changed", count, actual, err)
		}
	}
	for _, count := range []any{0, -1, 17, 1.5, "unknown", nil} {
		if _, err := h.Writer.Exec(insert, "invalid", "invalid", count); err == nil {
			t.Fatal("invalid stored count admitted", count)
		}
	}
	if _, err := h.Writer.Exec(`INSERT INTO configuration_authoring_sessions(id,user_id,kind,request_id,revision,phase,snapshot,created_at,updated_at) VALUES('default','owner','post_template','default',0,'choosing','{}','created','updated')`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := h.Reader.QueryRow(`SELECT candidate_count FROM configuration_authoring_sessions WHERE id='default'`).Scan(&count); err != nil || count != 8 {
		t.Fatal("ordinary default changed", count, err)
	}
	if _, err := h.Writer.Exec(`DELETE FROM users WHERE id='owner'`); err != nil {
		t.Fatal(err)
	}
	if rows := creationRows(t, h, "configuration_authoring_sessions"); len(rows) != 0 {
		t.Fatal("account deletion no longer removes private sessions")
	}
}

func TestMigration0152RejectsOnlyBrokenAuthoringForeignKeys(t *testing.T) {
	for _, tc := range []struct {
		name, orphan string
		passes       bool
	}{
		{"unrelated", `INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('lost','ghost','created','updated')`, true},
		{"session", `INSERT INTO configuration_authoring_sessions(id,user_id,kind,request_id,revision,phase,snapshot,created_at,updated_at) VALUES('lost','ghost','post_template','create',0,'choosing','{}','created','updated')`, false},
		{"operation", `INSERT INTO configuration_authoring_operations(id,user_id,session_id,request_id,fingerprint,base_revision,mode,payload,status,created_at) VALUES('lost','owner','gone','create','fingerprint',0,'recommend',X'00','done','created')`, false},
		{"mutation", `INSERT INTO configuration_authoring_mutations(user_id,session_id,operation_key,action,expected_revision,fingerprint,response,created_at) VALUES('owner','gone','lost','patch',0,'fingerprint','{}','created')`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, p := authoringBefore0152(t)
			for _, query := range []string{
				`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','created')`,
				`PRAGMA foreign_keys=OFF`, tc.orphan, `PRAGMA foreign_keys=ON`,
			} {
				if _, err := h.Writer.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			_, err := p.UpTo(t.Context(), 152)
			if tc.passes && err != nil {
				t.Fatal("unrelated orphan stopped the authoring migration", err)
			}
			if !tc.passes && (err == nil || !strings.Contains(err.Error(), "CHECK constraint failed")) {
				t.Fatal("broken authoring graph passed the integrity guard", err)
			}
		})
	}
}

func TestCreationLineageAcceptsOnlyTheCanonicalCountSuccessor(t *testing.T) {
	const previous = "INTEGER NOT NULL DEFAULT 8 CHECK(candidate_count IN (2,4,8,16))"
	const successor = "candidate_count INTEGER NOT NULL DEFAULT 8 CHECK(candidate_count IN (1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16))"
	if !creationColumnCompatible(successor, "configuration_authoring_sessions", "candidate_count", previous) {
		t.Fatal("canonical successor refused")
	}
	for _, definition := range []string{
		strings.Replace(successor, "INTEGER", "TEXT", 1),
		strings.Replace(successor, "DEFAULT 8", "DEFAULT 1", 1),
		strings.Replace(successor, "NOT NULL", "", 1),
		strings.Replace(successor, "15,16", "15,16,17", 1),
		"candidate_count INTEGER NOT NULL DEFAULT 8 CHECK(candidate_count BETWEEN 1 AND 16)",
	} {
		if creationColumnCompatible(definition, "configuration_authoring_sessions", "candidate_count", previous) {
			t.Fatal("changed successor constraint accepted", definition)
		}
	}
	if creationColumnCompatible(successor, "unrelated_table", "candidate_count", previous) ||
		creationColumnCompatible(successor, "configuration_authoring_sessions", "other_column", previous) {
		t.Fatal("successor compatibility escaped its owned column")
	}
}
