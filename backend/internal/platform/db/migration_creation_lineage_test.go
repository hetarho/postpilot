package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func legacyCreationFiles(t *testing.T, writing bool) fstest.MapFS {
	t.Helper()
	files := migrationsBefore(t, "0148_")
	for old, current := range map[string]string{"0148_creation_working_sources.sql": creationMigrationFile, "0149_writing_tests.sql": writingMigrationFile} {
		if strings.HasPrefix(old, "0149_") && !writing {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + current)
		if err != nil {
			t.Fatal(err)
		}
		files[old] = &fstest.MapFile{Data: body}
	}
	return files
}

func seedCreationPreservation(t *testing.T, h *DB, writing bool) {
	t.Helper()
	exec := func(q string) {
		t.Helper()
		if _, err := h.Writer.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','created')`)
	exec(`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','owner','Kept voice',0,'created','updated')`)
	exec(`INSERT INTO posts(slug,user_id,created_at,updated_at,input_revision) VALUES('post','owner','created','updated',12)`)
	exec(`INSERT INTO voice_samples(id,voice_id,user_id,kind,label,body,created_at,content_revision) VALUES('sample','voice','owner','post','Kept label','Kept current prose','created',7)`)
	exec(`INSERT INTO voice_analyses(voice_id,user_id,slot,snapshot,material_ids,analyze_model,created_at,source_versions_known,accepted_sources,accepted_material_snapshot) VALUES('voice','owner','current','{"version":1,"origin":"personal","counted":{},"ai":{},"material_count":1}','["sample"]','registry/model','created',1,'[{"SampleID":"sample","ContentRevision":5}]','[{"Source":{"SampleID":"sample","ContentRevision":5},"Body":"Kept accepted prose"}]')`)
	exec(`INSERT INTO voice_sample_mutations(user_id,voice_id,sample_id,operation_key,expected_content_revision,resulting_content_revision,fingerprint,response,created_at) VALUES('owner','voice','sample','sample-save',5,7,'fingerprint','{"receipt":"kept"}','created')`)
	exec(`INSERT INTO configuration_authoring_sessions(id,user_id,kind,request_id,revision,phase,snapshot,created_at,updated_at,saved_baseline,working_source,draft_state,has_unpublished_changes,saved_available,publication_pending,target_conflict,display_name,candidate_count) VALUES('session','owner','post_template','create',8,'editing','{"version":1}','created','updated','{"Body":"Kept baseline"}','{"Body":"Kept invalid work"}','invalid',1,1,0,0,'Kept work',16)`)
	exec(`INSERT INTO configuration_authoring_mutations(user_id,session_id,operation_key,action,expected_revision,fingerprint,response,created_at) VALUES('owner','session','draft-save','patch',8,'fingerprint','{"receipt":"kept"}','created')`)
	exec(`INSERT INTO generation_jobs(id,user_id,kind,status,stage,payload,created_at,updated_at) VALUES('job','owner','clip_render','queued','render_wait','{"immutable":"kept"}','created','updated')`)
	if !writing {
		return
	}
	exec(`INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,source_post_slug,context,common_snapshot,common_hash,prompt_version,created_at,updated_at) VALUES('test','owner','test-start','fingerprint','ab','model','write',2,'review','post','{"input":"kept"}',X'010203','common','v1','created','updated')`)
	exec(`INSERT INTO writing_test_candidates(id,user_id,test_id,seed_position,source_kind,source_id,source_revision,semantic_key,output,status) VALUES('left','owner','test',0,'model','registry/left','v1','left',X'040506','succeeded'),('right','owner','test',1,'model','registry/right','v1','right',X'070809','succeeded')`)
	exec(`INSERT INTO writing_test_matches(id,user_id,test_id,round,match_index,left_candidate_id,right_candidate_id,winner_candidate_id,decision_key,decided_at) VALUES('match','owner','test',1,0,'left','right','left','decision','decided')`)
	exec(`INSERT INTO writing_test_publications(id,user_id,test_id,winner_candidate_id,action,request_key,fingerprint,target_id,status,created_at,confirmed_at) VALUES('publication','owner','test','left','adopt_model','winner-save','fingerprint','write','confirmed','created','confirmed')`)
	exec(`INSERT INTO post_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,post_slug,resulting_content_revision,receipt,created_at) VALUES('owner','test','left','apply_output','post-save','fingerprint','post',4,'{"receipt":"kept"}','created')`)
	exec(`INSERT INTO provider_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,stage,model_ref,receipt,created_at) VALUES('owner','test','left','adopt_model','model-save','fingerprint','write','registry/left','{"receipt":"kept"}','created')`)
	for _, table := range []string{"template_test_publications", "guideline_test_publications", "voice_test_publications"} {
		exec(fmt.Sprintf(`INSERT INTO %s(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,target_id,receipt,created_at) VALUES('owner','test','left','save_setting','%s','fingerprint','target','{"receipt":"kept"}','created')`, table, table))
	}
}

func creationRows(t *testing.T, h *DB, table string) [][]string {
	t.Helper()
	projection := "*"
	if table == "posts" {
		// Compare every historical field even when an additive migration appends
		// result sidecars. Their required NULL upgrade state is asserted separately.
		columns, err := h.Reader.Query("SELECT name FROM pragma_table_info('posts') WHERE name NOT IN ('content_origins','storyline_origins') ORDER BY cid")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for columns.Next() {
			var name string
			if err := columns.Scan(&name); err != nil {
				t.Fatal(err)
			}
			names = append(names, `"`+strings.ReplaceAll(name, `"`, `""`)+`"`)
		}
		if err := columns.Err(); err != nil {
			t.Fatal(err)
		}
		columns.Close()
		projection = strings.Join(names, ",")
	}
	rows, err := h.Reader.Query("SELECT " + projection + " FROM " + table + " ORDER BY rowid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var result [][]string
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		record := make([]string, len(values))
		for i, v := range values {
			if raw, ok := v.([]byte); ok {
				record[i] = fmt.Sprintf("bytes:%x", raw)
			} else {
				record[i] = fmt.Sprintf("%T:%v", v, v)
			}
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertMergedCreationSchemas(t *testing.T, h *DB) {
	t.Helper()
	tx, err := h.Writer.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{creationMigrationFile, writingMigrationFile, browserAnalysisMigrationFile} {
		up, err := creationUp(sub, name)
		if err != nil {
			t.Fatal(err)
		}
		present, err := creationSchemaPresent(t.Context(), tx, up)
		if err != nil || !present {
			t.Fatal("merged schema missing", name, err)
		}
	}
	if err := addWaitExpiry(t.Context(), tx); err != nil {
		t.Fatal(err)
	}
}

func TestCreationLineageUpgradesBothPublishedAndDivergentDatabases(t *testing.T) {
	for _, lineage := range []string{"fresh", "remote149", "local148", "local149"} {
		t.Run(lineage, func(t *testing.T) {
			h := openTemp(t)
			writing := lineage == "local149"
			switch lineage {
			case "fresh":
			case "remote149":
				if err := migrate(t.Context(), h.Writer, migrationsBefore(t, "0150_")); err != nil {
					t.Fatal(err)
				}
			case "local148", "local149":
				if err := migrate(t.Context(), h.Writer, legacyCreationFiles(t, writing)); err != nil {
					t.Fatal(err)
				}
			}
			var kept map[string][][]string
			var oldLedger []struct {
				id, version int64
				stamp       string
			}
			if lineage == "local148" || lineage == "local149" {
				seedCreationPreservation(t, h, writing)
				kept = map[string][][]string{}
				tables := []string{"voice_samples", "voice_analyses", "voice_sample_mutations", "configuration_authoring_sessions", "configuration_authoring_mutations", "posts"}
				if writing {
					tables = append(tables, "writing_tests", "writing_test_candidates", "writing_test_matches", "writing_test_publications", "post_test_publications", "provider_test_publications", "template_test_publications", "guideline_test_publications", "voice_test_publications")
				}
				for _, table := range tables {
					kept[table] = creationRows(t, h, table)
				}
				rows, err := h.Reader.Query("SELECT id,version_id,tstamp FROM goose_db_version WHERE version_id IN(148,149) ORDER BY id")
				if err != nil {
					t.Fatal(err)
				}
				for rows.Next() {
					var r struct {
						id, version int64
						stamp       string
					}
					if err := rows.Scan(&r.id, &r.version, &r.stamp); err != nil {
						t.Fatal(err)
					}
					oldLedger = append(oldLedger, r)
				}
				rows.Close()
			}
			if err := Migrate(t.Context(), h.Writer); err != nil {
				t.Fatal("first merged startup", err)
			}
			assertMergedCreationSchemas(t, h)
			for table, rows := range kept {
				if got := creationRows(t, h, table); !reflect.DeepEqual(rows, got) {
					t.Fatal("migration changed private data or receipt", table)
				}
			}
			var inferredOrigins int
			if err := h.Reader.QueryRow("SELECT count(*) FROM posts WHERE content_origins IS NOT NULL OR storyline_origins IS NOT NULL").Scan(&inferredOrigins); err != nil || inferredOrigins != 0 {
				t.Fatal("migration inferred private origin evidence for historical posts", inferredOrigins, err)
			}
			for _, old := range oldLedger {
				var version int64
				var stamp string
				if err := h.Reader.QueryRow("SELECT version_id,tstamp FROM goose_db_version WHERE id=?", old.id).Scan(&version, &stamp); err != nil || version != old.version+2 || stamp != old.stamp {
					t.Fatal("local ledger identity/timestamp was not retained", old.id, version, err)
				}
			}
			for _, version := range []int{148, 149, 150, 151} {
				var n int
				if err := h.Reader.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=? AND is_applied=1", version).Scan(&n); err != nil || n != 1 {
					t.Fatal("migration not applied once", version, n, err)
				}
			}
			if err := Migrate(t.Context(), h.Writer); err != nil {
				t.Fatal("second merged startup", err)
			}
			for table, rows := range kept {
				if got := creationRows(t, h, table); !reflect.DeepEqual(rows, got) {
					t.Fatal("second boot changed data", table)
				}
			}
		})
	}
}

func TestCreationLineageRejectsPartialSchemaAndRollsBackRepairs(t *testing.T) {
	for _, damage := range []string{"missing-index", "bad-literal", "missing-prerequisite", "incompatible-wait", "partial-browser"} {
		t.Run(damage, func(t *testing.T) {
			h := openTemp(t)
			if err := migrate(t.Context(), h.Writer, legacyCreationFiles(t, true)); err != nil {
				t.Fatal(err)
			}
			seedCreationPreservation(t, h, true)
			var query string
			switch damage {
			case "missing-index":
				query = "DROP INDEX voice_samples_owned_revision"
			case "bad-literal":
				query = `DROP TABLE voice_sample_mutations; CREATE TABLE voice_sample_mutations(user_id TEXT)`
			case "missing-prerequisite":
				query = "DELETE FROM goose_db_version WHERE version_id=147"
			case "incompatible-wait":
				query = "ALTER TABLE generation_jobs ADD COLUMN wait_expires_at INTEGER"
			case "partial-browser":
				query = "CREATE TABLE clip_analysis_preparations(id TEXT PRIMARY KEY)"
			}
			if _, err := h.Writer.Exec(query); err != nil {
				t.Fatal(err)
			}
			ledger := creationRows(t, h, "goose_db_version")
			samples := creationRows(t, h, "voice_samples")
			if err := Migrate(t.Context(), h.Writer); err == nil {
				t.Fatal("partial/incompatible state silently admitted")
			}
			if !reflect.DeepEqual(ledger, creationRows(t, h, "goose_db_version")) || !reflect.DeepEqual(samples, creationRows(t, h, "voice_samples")) {
				t.Fatal("failed compatibility changed ledger or prose")
			}
			var n int
			if err := h.Reader.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='clip_analysis_copies'").Scan(&n); err != nil || n != 0 {
				t.Fatal("failed compatibility left browser DDL", n, err)
			}
		})
	}
}

func TestCreationLineageReappliesForwardOnlyRollbackWithoutRepeatingDDL(t *testing.T) {
	h := openTemp(t)
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	seedCreationPreservation(t, h, true)
	before := creationRows(t, h, "writing_test_publications")
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal("forward-only151 reapply", err)
	}
	if !reflect.DeepEqual(before, creationRows(t, h, "writing_test_publications")) {
		t.Fatal("reapply changed paid publication")
	}
}

func TestCreationLineageGuardRejectsWrongColumnType(t *testing.T) {
	h := openTemp(t)
	if err := migrate(t.Context(), h.Writer, migrationsBefore(t, "0150_")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("ALTER TABLE voice_samples ADD COLUMN content_revision TEXT NOT NULL DEFAULT '1'"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), h.Writer); err == nil || !strings.Contains(err.Error(), "incompatible creation column") {
		t.Fatal("wrong type was admitted", err)
	}
	var version sql.NullInt64
	if err := h.Reader.QueryRow("SELECT max(version_id) FROM goose_db_version").Scan(&version); err != nil || version.Int64 != 149 {
		t.Fatal("failed150 was recorded", version, err)
	}
}
