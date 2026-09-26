package db

import (
	"context"
	"io/fs"
	"reflect"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0089 = "2026-09-26T00:00:00Z"

// provider0089 takes a fresh database to 88 and seeds two phrase lists beside a post and its
// measurement, so each test moves across 89 itself.
func provider0089(t *testing.T) (*DB, *goose.Provider) {
	t.Helper()
	handle := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), 88); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, []any{at0089}},
		{`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,?,?)`, []any{at0089, at0089}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,field,created_at,updated_at) VALUES('measured','alice','voice-a','성수','cafe',?,?)`, []any{at0089, at0089}},
		{`INSERT INTO post_measurements(post_slug,user_id,content_revision,measure_version,char_count,photo_count,
		    distinct_block_types,avg_sentence_length,repetition_share,title_relevance,computed_at)
		  VALUES('measured','alice',3,2,820,2,3,12.5,0.25,0.6,?)`, []any{at0089}},
		{`INSERT INTO field_phrase_lists(field,phrases,corpus_size,refreshed_at,next_refresh_at)
		  VALUES('cafe','["분위기 좋은 카페"]',300,?,?),('pets','[]',0,NULL,?)`, []any{at0089, at0089, at0089}},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

// qualityState0089 is the post and measurement the drop must leave, one string each.
func qualityState0089(t *testing.T, handle *DB) []string {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT 'p:' || slug || '|' || user_id || '|' || title || '|' || field FROM posts
		UNION ALL SELECT 'm:' || post_slug || '|' || user_id || '|' || content_revision || '|' || measure_version || '|' ||
		  char_count || '|' || photo_count || '|' || distinct_block_types || '|' || avg_sentence_length || '|' ||
		  repetition_share || '|' || title_relevance || '|' || computed_at FROM post_measurements ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// QUAL-47: no batch collects 분야 phrases and nothing reads a list, so 0089 drops
// field_phrase_lists, rows and all, and leaves posts, their measurements and the graph's
// integrity as they were.
func TestMigration0089DropsTheFieldPhraseLists(t *testing.T) {
	ctx := context.Background()
	handle, provider := provider0089(t)
	before := qualityState0089(t, handle)
	if len(before) != 2 {
		t.Fatalf("seeded %q, want the post and its measurement", before)
	}
	definition, ok := schemaObject0077(t, handle, "table", "field_phrase_lists")
	if !ok {
		t.Fatal("field_phrase_lists is missing at 88")
	}

	if _, err := provider.UpTo(ctx, 89); err != nil {
		t.Fatal(err)
	}
	if _, ok := schemaObject0077(t, handle, "table", "field_phrase_lists"); ok {
		t.Fatal("field_phrase_lists survived 0089")
	}
	if n := count0078(t, handle, `SELECT count(*) FROM sqlite_master WHERE tbl_name = 'field_phrase_lists'`); n != 0 {
		t.Fatalf("%d schema objects still hang off field_phrase_lists", n)
	}
	if after := qualityState0089(t, handle); !reflect.DeepEqual(after, before) {
		t.Fatalf("the drop changed the posts or measurements: %q, want %q", after, before)
	}
	if n := foreignKeyViolations0078(t, handle); n != 0 {
		t.Fatalf("%d foreign-key violations after the drop", n)
	}
	var integrity string
	if err := handle.Reader.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, %v", integrity, err)
	}

	// Down brings the table back empty, with 0077's DDL, for a rolled-back binary whose batch
	// reads and replaces its rows.
	if _, err := provider.DownTo(ctx, 88); err != nil {
		t.Fatal(err)
	}
	restored, ok := schemaObject0077(t, handle, "table", "field_phrase_lists")
	if !ok || restored != definition {
		t.Fatalf("restored field_phrase_lists = %q, want 0077's %q", restored, definition)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM field_phrase_lists`); n != 0 {
		t.Fatalf("restored field_phrase_lists holds %d rows, want none", n)
	}
	insert := `INSERT INTO field_phrase_lists(field,phrases,corpus_size,next_refresh_at) VALUES(?,?,0,?)`
	if err := exec0078(t, handle, insert, "cafe", "not json", at0089); err == nil {
		t.Error("the restored table accepted phrases that are not JSON")
	}
	if err := exec0078(t, handle, insert, "cafe", `["분위기 좋은 카페"]`, at0089); err != nil {
		t.Fatalf("the restored table refused a JSON list: %v", err)
	}
	if after := qualityState0089(t, handle); !reflect.DeepEqual(after, before) {
		t.Fatalf("the rollback changed the posts or measurements: %q, want %q", after, before)
	}

	// And forward again, over a restored table that holds a row.
	if _, err := provider.UpTo(ctx, 89); err != nil {
		t.Fatalf("migrating up again: %v", err)
	}
	if _, ok := schemaObject0077(t, handle, "table", "field_phrase_lists"); ok {
		t.Fatal("field_phrase_lists survived 0089 a second time")
	}
}
