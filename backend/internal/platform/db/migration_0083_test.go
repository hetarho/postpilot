package db

import (
	"context"
	"database/sql"
	"io/fs"
	"reflect"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0083 = "2026-09-25T00:00:00Z"

// measurementRow0083 is every post_measurements value but top_noun, which the drop must leave.
type measurementRow0083 struct {
	slug, user, computedAt                 string
	revision, version, chars, photos, kind int64
	sentence, share, relevance             sql.NullFloat64
}

// provider0083 takes a fresh database to 82 and seeds a measurement row with every column set,
// top_noun included, so each test moves across 83 itself.
func provider0083(t *testing.T) (*DB, *goose.Provider) {
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
	if _, err := provider.UpTo(context.Background(), 82); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, []any{at0083}},
		{`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,?,?)`, []any{at0083, at0083}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,created_at,updated_at) VALUES('measured','alice','voice-a','을지로',?,?)`, []any{at0083, at0083}},
		{`INSERT INTO post_measurements(post_slug,user_id,content_revision,measure_version,char_count,photo_count,
		    distinct_block_types,avg_sentence_length,repetition_share,top_noun,title_relevance,computed_at)
		  VALUES('measured','alice',3,2,820,2,3,12.5,0.25,'감자탕',0.6,?)`, []any{at0083}},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

func measurement0083(t *testing.T, handle *DB) measurementRow0083 {
	t.Helper()
	var row measurementRow0083
	if err := handle.Reader.QueryRow(`SELECT post_slug, user_id, content_revision, measure_version, char_count, photo_count,
		distinct_block_types, avg_sentence_length, repetition_share, title_relevance, computed_at FROM post_measurements`).
		Scan(&row.slug, &row.user, &row.revision, &row.version, &row.chars, &row.photos, &row.kind,
			&row.sentence, &row.share, &row.relevance, &row.computedAt); err != nil {
		t.Fatal(err)
	}
	return row
}

func foreignKeys0083(t *testing.T, handle *DB) []string {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT "table", "from", "to", on_delete FROM pragma_foreign_key_list('post_measurements') ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var table, from, to, onDelete string
		if err := rows.Scan(&table, &from, &to, &onDelete); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, table+"."+from+"->"+to+" "+onDelete)
	}
	return keys
}

// QUAL-43: M3 names no noun, and nothing has read top_noun since T335, so 0083 drops it and
// leaves every other value of the row, its keys and its integrity as they were.
func TestMigration0083DropsTheUnusedTopNoun(t *testing.T) {
	ctx := context.Background()
	handle, provider := provider0083(t)
	before := measurement0083(t, handle)
	keys := foreignKeys0083(t, handle)

	if _, err := provider.UpTo(ctx, 83); err != nil {
		t.Fatal(err)
	}
	want := []column0077{
		{name: "post_slug", kind: "TEXT", primaryKey: true},
		{name: "user_id", kind: "TEXT", notNull: true},
		{name: "content_revision", kind: "INTEGER", notNull: true},
		{name: "measure_version", kind: "INTEGER", notNull: true},
		{name: "char_count", kind: "INTEGER", notNull: true},
		{name: "photo_count", kind: "INTEGER", notNull: true},
		{name: "distinct_block_types", kind: "INTEGER", notNull: true},
		{name: "avg_sentence_length", kind: "REAL"},
		{name: "repetition_share", kind: "REAL"},
		{name: "title_relevance", kind: "REAL"},
		{name: "computed_at", kind: "TEXT", notNull: true},
	}
	if got := columns0077(t, handle, "post_measurements"); !reflect.DeepEqual(got, want) {
		t.Fatalf("post_measurements = %+v, want %+v", got, want)
	}
	if after := measurement0083(t, handle); after != before {
		t.Fatalf("the drop changed the row: %+v, want %+v", after, before)
	}
	if got := foreignKeys0083(t, handle); !reflect.DeepEqual(got, keys) {
		t.Fatalf("foreign keys = %v, want %v", got, keys)
	}
	var violations int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations = %d, %v", violations, err)
	}

	// Down brings the column back empty, for a rolled-back binary whose upsert names it.
	if _, err := provider.DownTo(ctx, 82); err != nil {
		t.Fatal(err)
	}
	if !hasColumn0077(t, handle, "post_measurements", "top_noun") {
		t.Fatal("down did not restore top_noun")
	}
	for _, column := range columns0077(t, handle, "post_measurements") {
		if column.name == "top_noun" && (column.kind != "TEXT" || column.notNull) {
			t.Fatalf("restored top_noun = %+v, want a nullable TEXT", column)
		}
	}
	var noun sql.NullString
	if err := handle.Reader.QueryRow(`SELECT top_noun FROM post_measurements`).Scan(&noun); err != nil || noun.Valid {
		t.Fatalf("restored top_noun = %+v, %v; want NULL", noun, err)
	}
	if after := measurement0083(t, handle); after != before {
		t.Fatalf("the rollback changed the row: %+v, want %+v", after, before)
	}
}
