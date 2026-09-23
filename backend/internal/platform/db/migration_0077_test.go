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

// The six columns 0077 adds to posts, in the order it adds them.
var postColumns0077 = []string{"published_url", "published_at", "field", "content_nouns", "replacement_candidates", "quality_rules"}

// templateBody0077 is a legacy body the migration must leave byte for byte: Korean, the
// grammar's markup and a trailing newline, the three things a careless rewrite would change.
const templateBody0077 = "<write>오늘 다녀온 곳을 소개해 주세요.</write>\n<ask label=\"총평 별점\"></ask>\n"

const at0077 = "2026-09-24T00:00:00Z"

// legacyPost0077 is what a post held before the delta, which 0077 must not touch.
type legacyPost0077 struct {
	status, title, content                 string
	contentRevision, finalizedRevision     sql.NullInt64
	finalizedAt, machineBaseline, template sql.NullString
}

// provider0077 takes a fresh database to 76, seeds the rows the delta has to leave alone and
// hands back the provider, so each test moves to 77 itself.
func provider0077(t *testing.T) (*DB, *goose.Provider) {
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
	if _, err := provider.UpTo(context.Background(), 76); err != nil {
		t.Fatal(err)
	}
	content := `{"title":"제주 3일","tags":["제주"],"blocks":[{"type":"TEXT","content":"바다가 보였다."}]}`
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?),('bob','hash',?)`, []any{at0077, at0077}},
		{`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,?,?),('voice-b','bob','기본',1,?,?)`, []any{at0077, at0077, at0077, at0077}},
		{`INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at) VALUES('template','alice','맛집 리뷰','',?,?,?)`, []any{templateBody0077, at0077, at0077}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,created_at,updated_at) VALUES('draft-post','alice','voice-a','초안',?,?)`, []any{at0077, at0077}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,status,content,content_revision,machine_baseline,machine_baseline_revision,template_id,created_at,updated_at)
		  VALUES('review-post','alice','voice-a','검토','review',?,2,?,2,'template',?,?)`, []any{content, content, at0077, at0077}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,status,content,content_revision,machine_baseline,machine_baseline_revision,finalized_revision,finalized_at,created_at,updated_at)
		  VALUES('finalized-post','alice','voice-a','확정','finalized',?,3,?,3,3,?,?,?)`, []any{content, content, at0077, at0077, at0077}},
		{`INSERT INTO posts(slug,user_id,voice_id,title,created_at,updated_at) VALUES('bob-post','bob','voice-b','남의 글',?,?)`, []any{at0077, at0077}},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

func legacyPosts0077(t *testing.T, handle *DB) map[string]legacyPost0077 {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT slug, status, title, COALESCE(content, ''), content_revision, finalized_revision,
		finalized_at, machine_baseline, template_id FROM posts`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]legacyPost0077{}
	for rows.Next() {
		var slug string
		var post legacyPost0077
		if err := rows.Scan(&slug, &post.status, &post.title, &post.content, &post.contentRevision, &post.finalizedRevision,
			&post.finalizedAt, &post.machineBaseline, &post.template); err != nil {
			t.Fatal(err)
		}
		out[slug] = post
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// column0077 is one PRAGMA table_info row, less its position, which the slice order carries.
type column0077 struct {
	name, kind string
	notNull    bool
	defaultSQL sql.NullString
	primaryKey bool
}

func columns0077(t *testing.T, handle *DB, table string) []column0077 {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []column0077
	for rows.Next() {
		var column column0077
		var primaryKey int
		if err := rows.Scan(&column.name, &column.kind, &column.notNull, &column.defaultSQL, &primaryKey); err != nil {
			t.Fatal(err)
		}
		column.primaryKey = primaryKey > 0
		out = append(out, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func hasColumn0077(t *testing.T, handle *DB, table, column string) bool {
	t.Helper()
	var count int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func schemaObject0077(t *testing.T, handle *DB, kind, name string) (string, bool) {
	t.Helper()
	var definition sql.NullString
	err := handle.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE type = ? AND name = ?`, kind, name).Scan(&definition)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return definition.String, true
}

func TestMigration0077KeepsLegacyPostsAndTemplates(t *testing.T) {
	handle, provider := provider0077(t)
	before := legacyPosts0077(t, handle)
	if _, err := provider.UpTo(context.Background(), 77); err != nil {
		t.Fatal(err)
	}

	if after := legacyPosts0077(t, handle); !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy posts changed:\n got %+v\nwant %+v", after, before)
	}
	for _, name := range postColumns0077 {
		var nulls, all int
		if err := handle.Reader.QueryRow(`SELECT count(*) FILTER (WHERE `+name+` IS NULL), count(*) FROM posts`).Scan(&nulls, &all); err != nil {
			t.Fatal(err)
		}
		if nulls != all || all != 4 {
			t.Errorf("posts.%s is NULL on %d of %d legacy posts", name, nulls, all)
		}
	}
	// Nullable TEXT with no default: the shape sqlc turns into sql.NullString.
	columns := columns0077(t, handle, "posts")
	added := columns[len(columns)-len(postColumns0077):]
	for index, name := range postColumns0077 {
		if want := (column0077{name: name, kind: "TEXT"}); added[index] != want {
			t.Errorf("posts column %d = %+v, want %+v", len(columns)-len(postColumns0077)+index, added[index], want)
		}
	}

	var body, titleArea string
	if err := handle.Reader.QueryRow(`SELECT body, title_area FROM templates WHERE id = 'template'`).Scan(&body, &titleArea); err != nil {
		t.Fatal(err)
	}
	if body != templateBody0077 || titleArea != "" {
		t.Fatalf("template body = %q, title_area = %q; want the legacy body and ''", body, titleArea)
	}

	definition, ok := schemaObject0077(t, handle, "index", "posts_user_published_idx")
	if !ok || !strings.Contains(definition, "ON posts(user_id, published_at) WHERE status = 'published'") {
		t.Fatalf("posts_user_published_idx = %q, %v", definition, ok)
	}

	// Nothing is backfilled: the Up block only adds, so no statement in it writes a row.
	source, err := fs.ReadFile(migrationsFS, "migrations/0077_published_quality.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(source), "-- +goose Down")
	for _, line := range strings.Split(up, "\n") {
		if statement := strings.ToUpper(strings.TrimSpace(line)); strings.HasPrefix(statement, "UPDATE ") ||
			strings.HasPrefix(statement, "INSERT ") || strings.HasPrefix(statement, "DELETE ") {
			t.Errorf("0077's Up writes rows: %s", line)
		}
	}
}

func TestMigration0077ConstrainsThePublishedPairAndTheJSONColumns(t *testing.T) {
	handle, provider := provider0077(t)
	if _, err := provider.UpTo(context.Background(), 77); err != nil {
		t.Fatal(err)
	}
	exec := func(statement string, args ...any) error {
		_, err := handle.Writer.Exec(statement, args...)
		return err
	}
	const address = "https://blog.naver.com/alice/223000000000"

	// The pair is set and cleared together, on UPDATE and on INSERT alike.
	for name, statement := range map[string]string{
		"the address alone": `UPDATE posts SET published_url = '` + address + `' WHERE slug = 'finalized-post'`,
		"the time alone":    `UPDATE posts SET published_at = '` + at0077 + `' WHERE slug = 'finalized-post'`,
	} {
		if err := exec(statement); err == nil {
			t.Errorf("an UPDATE setting %s was accepted", name)
		}
	}
	if err := exec(`UPDATE posts SET published_url = ?, published_at = ? WHERE slug = 'finalized-post'`, address, at0077); err != nil {
		t.Fatalf("setting both in one UPDATE: %v", err)
	}
	if err := exec(`UPDATE posts SET published_url = NULL WHERE slug = 'finalized-post'`); err == nil {
		t.Error("an UPDATE clearing only the address was accepted")
	}
	if err := exec(`UPDATE posts SET published_url = NULL, published_at = NULL WHERE slug = 'finalized-post'`); err != nil {
		t.Fatalf("clearing both in one UPDATE: %v", err)
	}
	insert := `INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at,published_url,published_at) VALUES(?,'alice','voice-a',?,?,?,?)`
	if err := exec(insert, "only-url", at0077, at0077, address, nil); err == nil {
		t.Error("an INSERT with only the address was accepted")
	}
	if err := exec(insert, "only-time", at0077, at0077, nil, at0077); err == nil {
		t.Error("an INSERT with only the time was accepted")
	}
	if err := exec(insert, "both", at0077, at0077, address, at0077); err != nil {
		t.Errorf("an INSERT with both: %v", err)
	}

	// The three JSON columns hold JSON or nothing.
	for _, column := range []string{"content_nouns", "replacement_candidates", "quality_rules"} {
		for _, value := range []string{"not json", "", "[\"커피\""} {
			if err := exec(`UPDATE posts SET `+column+` = ? WHERE slug = 'draft-post'`, value); err == nil {
				t.Errorf("%s accepted %q", column, value)
			}
		}
		for _, value := range []any{`["커피","디저트"]`, `[]`, `{"surface":1}`, nil} {
			if err := exec(`UPDATE posts SET `+column+` = ? WHERE slug = 'draft-post'`, value); err != nil {
				t.Errorf("%s refused %v: %v", column, value, err)
			}
		}
	}

	// The blog field's validity is the parser's, not the schema's.
	for _, value := range []string{"cafe", "restaurant", "not-a-field", "맛집", ""} {
		if err := exec(`UPDATE posts SET field = ? WHERE slug = 'draft-post'`, value); err != nil {
			t.Errorf("field refused %q: %v", value, err)
		}
	}

	// field_phrase_lists: product-owned, so no user_id, and its phrases are JSON.
	want := []column0077{
		{name: "field", kind: "TEXT", primaryKey: true},
		{name: "phrases", kind: "TEXT", notNull: true},
		{name: "corpus_size", kind: "INTEGER", notNull: true},
		{name: "refreshed_at", kind: "TEXT"},
		{name: "next_refresh_at", kind: "TEXT", notNull: true},
	}
	if got := columns0077(t, handle, "field_phrase_lists"); !reflect.DeepEqual(got, want) {
		t.Fatalf("field_phrase_lists = %+v, want %+v", got, want)
	}
	phrases := `INSERT INTO field_phrase_lists(field, phrases, corpus_size, next_refresh_at) VALUES(?, ?, 0, ?)`
	if err := exec(phrases, "cafe", "not json", at0077); err == nil {
		t.Error("field_phrase_lists accepted phrases that are not JSON")
	}
	if err := exec(phrases, "cafe", `["분위기 좋은 카페"]`, at0077); err != nil {
		t.Errorf("field_phrase_lists refused a JSON list: %v", err)
	}
}

func TestMigration0077MeasurementsFollowTheirPost(t *testing.T) {
	handle, provider := provider0077(t)
	if _, err := provider.UpTo(context.Background(), 77); err != nil {
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
		{name: "top_noun", kind: "TEXT"},
		{name: "title_relevance", kind: "REAL"},
		{name: "computed_at", kind: "TEXT", notNull: true},
	}
	if got := columns0077(t, handle, "post_measurements"); !reflect.DeepEqual(got, want) {
		t.Fatalf("post_measurements = %+v, want %+v", got, want)
	}
	var keys []string
	rows, err := handle.Reader.Query(`SELECT "table", "from", "to", on_delete FROM pragma_foreign_key_list('post_measurements') ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var table, from, to, onDelete string
		if err := rows.Scan(&table, &from, &to, &onDelete); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, table+"."+from+"->"+to+" "+onDelete)
	}
	rows.Close()
	if want := []string{"posts.post_slug->slug CASCADE", "posts.user_id->user_id CASCADE"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("post_measurements foreign key = %v, want %v", keys, want)
	}
	// Every read is by slug, so the primary key's own index is the only one.
	var indexes int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = 'post_measurements' AND sql IS NOT NULL`).Scan(&indexes); err != nil || indexes != 0 {
		t.Fatalf("post_measurements secondary indexes = %d, %v", indexes, err)
	}

	measure := `INSERT INTO post_measurements(post_slug, user_id, content_revision, measure_version, char_count, photo_count,
		distinct_block_types, computed_at) VALUES(?, ?, 3, 1, 120, 2, 3, ?)`
	if _, err := handle.Writer.Exec(measure, "no-such-post", "alice", at0077); err == nil {
		t.Error("a measurement of a post that does not exist was accepted")
	}
	if _, err := handle.Writer.Exec(measure, "bob-post", "alice", at0077); err == nil {
		t.Error("a measurement filed under another account's post was accepted")
	}
	if _, err := handle.Writer.Exec(measure, "finalized-post", "alice", at0077); err != nil {
		t.Fatalf("a measurement of the account's own post: %v", err)
	}

	if _, err := handle.Writer.Exec(`DELETE FROM posts WHERE slug = 'finalized-post'`); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM post_measurements`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("measurements left after their post was deleted = %d, %v", left, err)
	}
}

func TestMigration0077RollsBackPublishedPostsToFinalized(t *testing.T) {
	handle, provider := provider0077(t)
	ctx := context.Background()
	before := legacyPosts0077(t, handle)
	if _, err := provider.UpTo(ctx, 77); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE posts SET status = 'published', published_url = 'https://blog.naver.com/alice/223000000000', published_at = '` + at0077 + `',
		   field = 'cafe', content_nouns = '["바다"]', replacement_candidates = '[]', quality_rules = '[]' WHERE slug = 'finalized-post'`,
		`INSERT INTO post_measurements(post_slug, user_id, content_revision, measure_version, char_count, photo_count, distinct_block_types, computed_at)
		   VALUES('finalized-post', 'alice', 3, 1, 120, 2, 3, '` + at0077 + `')`,
		`INSERT INTO field_phrase_lists(field, phrases, corpus_size, next_refresh_at) VALUES('cafe', '[]', 0, '` + at0077 + `')`,
		`UPDATE templates SET title_area = '<write>제목</write>' WHERE id = 'template'`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("%q: %v", statement, err)
		}
	}

	if _, err := provider.DownTo(ctx, 76); err != nil {
		t.Fatalf("rollback to 76: %v", err)
	}
	// The published post is finalized again, exactly as it was, and nothing else moved.
	if after := legacyPosts0077(t, handle); !reflect.DeepEqual(after, before) {
		t.Fatalf("posts after the rollback:\n got %+v\nwant %+v", after, before)
	}
	for _, name := range postColumns0077 {
		if hasColumn0077(t, handle, "posts", name) {
			t.Errorf("posts.%s survived the rollback", name)
		}
	}
	if hasColumn0077(t, handle, "templates", "title_area") {
		t.Error("templates.title_area survived the rollback")
	}
	for _, object := range []struct{ kind, name string }{
		{"table", "post_measurements"}, {"table", "field_phrase_lists"}, {"index", "posts_user_published_idx"},
	} {
		if _, ok := schemaObject0077(t, handle, object.kind, object.name); ok {
			t.Errorf("%s %s survived the rollback", object.kind, object.name)
		}
	}
	var body string
	if err := handle.Reader.QueryRow(`SELECT body FROM templates WHERE id = 'template'`).Scan(&body); err != nil || body != templateBody0077 {
		t.Fatalf("template body after the rollback = %q, %v", body, err)
	}

	if _, err := provider.UpTo(ctx, 77); err != nil {
		t.Fatalf("migrating up again: %v", err)
	}
	var published int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM posts WHERE published_url IS NOT NULL OR status = 'published'`).Scan(&published); err != nil || published != 0 {
		t.Fatalf("published posts after migrating up again = %d, %v", published, err)
	}
}

func TestMigration0077IsIdempotent(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	var versions int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM goose_db_version`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var again, applied int
	if err := handle.Reader.QueryRow(`SELECT count(*), count(*) FILTER (WHERE version_id = 77 AND is_applied = 1) FROM goose_db_version`).Scan(&again, &applied); err != nil {
		t.Fatal(err)
	}
	if again != versions || applied != 1 {
		t.Fatalf("goose_db_version rows = %d then %d, 77 applied %d times", versions, again, applied)
	}
	for _, name := range postColumns0077 {
		if !hasColumn0077(t, handle, "posts", name) {
			t.Errorf("a fresh install has no posts.%s", name)
		}
	}
	if !hasColumn0077(t, handle, "templates", "title_area") {
		t.Error("a fresh install has no templates.title_area")
	}
	for _, table := range []string{"post_measurements", "field_phrase_lists"} {
		if _, ok := schemaObject0077(t, handle, "table", table); !ok {
			t.Errorf("a fresh install has no %s", table)
		}
	}
}
