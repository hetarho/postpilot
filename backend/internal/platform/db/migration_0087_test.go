package db

import (
	"context"
	"database/sql"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0087 = "2026-09-26T00:00:00Z"

// postRow0087 is every value of the seeded post the drop must leave.
type postRow0087 struct {
	slug, user, status, content, nouns, rules, field string
	revision                                         int64
}

// provider0087 takes a fresh database to 86 and seeds a generated post with every annotation
// column set, replacement_candidates included, so each test moves across 87 itself.
func provider0087(t *testing.T) (*DB, *goose.Provider) {
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
	if _, err := provider.UpTo(context.Background(), 86); err != nil {
		t.Fatal(err)
	}
	content := `{"title":"성수 카페","tags":["카페"],"blocks":[{"type":"TEXT","content":"성수 카페에 갔다."}]}`
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, []any{at0087}},
		{`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,?,?)`, []any{at0087, at0087}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,status,content,content_revision,machine_baseline,machine_baseline_revision,
		    field,content_nouns,replacement_candidates,quality_rules,created_at,updated_at)
		  VALUES('generated','alice','voice-a','성수','review',?,2,?,2,'cafe','["성수","카페"]',
		    '[{"surface":"title","index":0,"source":"성수 카페","phrases":["분위기 좋은 카페"]}]','["composition"]',?,?)`,
			[]any{content, content, at0087, at0087}},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

func post0087(t *testing.T, handle *DB) postRow0087 {
	t.Helper()
	var row postRow0087
	if err := handle.Reader.QueryRow(`SELECT slug, user_id, status, content, content_nouns, quality_rules, field, content_revision FROM posts`).
		Scan(&row.slug, &row.user, &row.status, &row.content, &row.nouns, &row.rules, &row.field, &row.revision); err != nil {
		t.Fatal(err)
	}
	return row
}

func withoutColumn0087(columns []column0077, name string) []column0077 {
	var out []column0077
	for _, column := range columns {
		if column.name != name {
			out = append(out, column)
		}
	}
	return out
}

// GEN-14, GEN-55: generation offers no replacement candidates and posts carry none, so 0087
// drops the column and leaves every other column, value and key of posts as it was.
func TestMigration0087DropsTheReplacementCandidates(t *testing.T) {
	ctx := context.Background()
	handle, provider := provider0087(t)
	before := post0087(t, handle)
	columns := columns0077(t, handle, "posts")
	keys := foreignKeys0087(t, handle)

	if _, err := provider.UpTo(ctx, 87); err != nil {
		t.Fatal(err)
	}
	if hasColumn0077(t, handle, "posts", "replacement_candidates") {
		t.Fatal("posts.replacement_candidates survived 0087")
	}
	if got, want := columns0077(t, handle, "posts"), withoutColumn0087(columns, "replacement_candidates"); !reflect.DeepEqual(got, want) {
		t.Fatalf("posts columns = %+v, want %+v", got, want)
	}
	if after := post0087(t, handle); after != before {
		t.Fatalf("the drop changed the post: %+v, want %+v", after, before)
	}
	if got := foreignKeys0087(t, handle); !reflect.DeepEqual(got, keys) {
		t.Fatalf("foreign keys = %v, want %v", got, keys)
	}
	var violations int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations = %d, %v", violations, err)
	}
	var integrity string
	if err := handle.Reader.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, %v", integrity, err)
	}

	// Down brings the column back empty, with 0077's CHECK, for a rolled-back binary that reads it.
	if _, err := provider.DownTo(ctx, 86); err != nil {
		t.Fatal(err)
	}
	// ADD COLUMN appends, so the restored column is last: a nullable TEXT with no default.
	restored := append(withoutColumn0087(columns, "replacement_candidates"), column0077{name: "replacement_candidates", kind: "TEXT"})
	if got := columns0077(t, handle, "posts"); !reflect.DeepEqual(got, restored) {
		t.Fatalf("restored posts columns = %+v, want %+v", got, restored)
	}
	definition, ok := schemaObject0077(t, handle, "table", "posts")
	if !ok || !strings.Contains(definition, "replacement_candidates TEXT CHECK (replacement_candidates IS NULL OR json_valid(replacement_candidates))") {
		t.Fatalf("posts definition lost 0077's CHECK: %q", definition)
	}
	var candidates sql.NullString
	if err := handle.Reader.QueryRow(`SELECT replacement_candidates FROM posts`).Scan(&candidates); err != nil || candidates.Valid {
		t.Fatalf("restored replacement_candidates = %+v, %v; want NULL", candidates, err)
	}
	if _, err := handle.Writer.Exec(`UPDATE posts SET replacement_candidates = 'not json'`); err == nil {
		t.Error("the restored column accepted a value that is not JSON")
	}
	if _, err := handle.Writer.Exec(`UPDATE posts SET replacement_candidates = '[]'`); err != nil {
		t.Errorf("the restored column refused JSON: %v", err)
	}
	if after := post0087(t, handle); after != before {
		t.Fatalf("the rollback changed the post: %+v, want %+v", after, before)
	}
}

func foreignKeys0087(t *testing.T, handle *DB) []string {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT "table", "from", "to", on_delete FROM pragma_foreign_key_list('posts') ORDER BY id, seq`)
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
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}
