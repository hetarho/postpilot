package db

import (
	"context"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0078 = "2026-09-24T00:00:00Z"

// guideline0078 is one guidelines row, every column the rebuild has to carry across.
type guideline0078 struct {
	id, userID, text, scope, createdAt, updatedAt string
}

// provider0078 takes a fresh database to 77 and seeds two accounts, alice's template, a global
// guideline and a templates guideline linked to that template, then hands back the provider so
// each test moves to 78 itself.
func provider0078(t *testing.T) (*DB, *goose.Provider) {
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
	if _, err := provider.UpTo(context.Background(), 77); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?),('bob','hash',?)`, []any{at0078, at0078}},
		{`INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at) VALUES('template','alice','맛집 리뷰','','<write>본문</write>',?,?)`, []any{at0078, at0078}},
		{`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES
		  ('global','alice','과장 금지','global','2026-09-01T00:00:00Z','2026-09-02T00:00:00Z'),
		  ('scoped','alice','가격 먼저','templates','2026-09-03T00:00:00Z','2026-09-04T00:00:00Z')`, nil},
		{`INSERT INTO guideline_templates(guideline_id,template_id,user_id) VALUES('scoped','template','alice')`, nil},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

func guidelines0078(t *testing.T, handle *DB) []guideline0078 {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT id, user_id, text, scope, created_at, updated_at FROM guidelines ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []guideline0078
	for rows.Next() {
		var g guideline0078
		if err := rows.Scan(&g.id, &g.userID, &g.text, &g.scope, &g.createdAt, &g.updatedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func templateLinks0078(t *testing.T, handle *DB) []string {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT guideline_id || '>' || template_id || '@' || user_id FROM guideline_templates ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var link string
		if err := rows.Scan(&link); err != nil {
			t.Fatal(err)
		}
		out = append(out, link)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// indexes0078 maps each index on table to its columns, joined by commas, split by uniqueness.
func indexes0078(t *testing.T, handle *DB, table string) (unique, plain map[string]string) {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT il.name, il."unique", ii.name FROM pragma_index_list(?) il, pragma_index_info(il.name) ii ORDER BY il.name, ii.seqno`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	unique, plain = map[string]string{}, map[string]string{}
	for rows.Next() {
		var index, column string
		var isUnique bool
		if err := rows.Scan(&index, &isUnique, &column); err != nil {
			t.Fatal(err)
		}
		target := plain
		if isUnique {
			target = unique
		}
		if target[index] != "" {
			column = target[index] + "," + column
		}
		target[index] = column
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return unique, plain
}

func hasColumns0078(indexes map[string]string, columns string) bool {
	for _, got := range indexes {
		if got == columns {
			return true
		}
	}
	return false
}

func foreignKeyViolations0078(t *testing.T, handle *DB) int {
	t.Helper()
	var n int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func count0078(t *testing.T, handle *DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := handle.Reader.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func exec0078(t *testing.T, handle *DB, query string, args ...any) error {
	t.Helper()
	_, err := handle.Writer.Exec(query, args...)
	return err
}

func TestMigration0078RebuildsGuidelinesKeepingRowsAndLinks(t *testing.T) {
	handle, provider := provider0078(t)
	rowsBefore, linksBefore := guidelines0078(t, handle), templateLinks0078(t, handle)
	if len(rowsBefore) != 2 || len(linksBefore) != 1 {
		t.Fatalf("seed = %v, %v", rowsBefore, linksBefore)
	}
	if _, err := provider.UpTo(context.Background(), 78); err != nil {
		t.Fatal(err)
	}
	if got := guidelines0078(t, handle); !reflect.DeepEqual(got, rowsBefore) {
		t.Fatalf("rows = %v, want %v", got, rowsBefore)
	}
	if got := templateLinks0078(t, handle); !reflect.DeepEqual(got, linksBefore) {
		t.Fatalf("template links = %v, want %v", got, linksBefore)
	}

	unique, plain := indexes0078(t, handle, "guidelines")
	for _, columns := range []string{"id,user_id", "user_id,text"} {
		if !hasColumns0078(unique, columns) {
			t.Errorf("no UNIQUE (%s) after the rebuild: %v", columns, unique)
		}
	}
	if plain["idx_guidelines_user_created"] != "user_id,created_at,id" {
		t.Errorf("idx_guidelines_user_created = %q", plain["idx_guidelines_user_created"])
	}
	if n := foreignKeyViolations0078(t, handle); n != 0 {
		t.Fatalf("%d foreign-key violations after the rebuild", n)
	}
	if err := exec0078(t, handle, `INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('again','alice','과장 금지','global',?,?)`, at0078, at0078); err == nil {
		t.Fatal("the rebuilt table accepted a duplicate text within the account")
	}

	// The surviving link still resolves to the renamed table and still cascades with it.
	if err := exec0078(t, handle, `DELETE FROM guidelines WHERE id = 'scoped'`); err != nil {
		t.Fatal(err)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_templates`); n != 0 {
		t.Fatalf("deleting the guideline left %d template links", n)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM templates WHERE id = 'template'`); n != 1 {
		t.Fatal("the cascade reached the template")
	}
}

func TestMigration0078AdmitsTheFieldsScopeOnly(t *testing.T) {
	handle, provider := provider0078(t)
	if _, err := provider.UpTo(context.Background(), 78); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES(?,'alice',?,?,?,?)`
	for _, scope := range []string{"global", "templates", "fields"} {
		if err := exec0078(t, handle, insert, "ok-"+scope, "허용 "+scope, scope, at0078, at0078); err != nil {
			t.Errorf("scope %q refused: %v", scope, err)
		}
	}
	for _, scope := range []string{"purposes", "FIELDS", "field", ""} {
		if err := exec0078(t, handle, insert, "bad-"+scope, "거부 "+scope, scope, at0078, at0078); err == nil {
			t.Errorf("scope %q accepted", scope)
		}
	}

	link := `INSERT INTO guideline_fields(guideline_id,field,user_id) VALUES(?,?,?)`
	if err := exec0078(t, handle, link, "ok-fields", "restaurant", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := exec0078(t, handle, link, "ok-fields", "restaurant", "alice"); err == nil {
		t.Fatal("a second link of the same field to the same guideline was accepted")
	}
	if err := exec0078(t, handle, link, "ok-fields", "domestic_travel", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := exec0078(t, handle, link, "ok-global", "restaurant", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := exec0078(t, handle, `DELETE FROM guidelines WHERE id = 'ok-fields'`); err != nil {
		t.Fatal(err)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_fields WHERE guideline_id = 'ok-fields'`); n != 0 {
		t.Fatalf("deleting the guideline left %d field links", n)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_fields`); n != 1 {
		t.Fatalf("the cascade reached another guideline's links: %d left", n)
	}
	_, plain := indexes0078(t, handle, "guideline_fields")
	if plain["idx_guideline_fields_field"] != "user_id,field" {
		t.Fatalf("idx_guideline_fields_field = %q", plain["idx_guideline_fields_field"])
	}
}

func TestMigration0078RefusesACrossAccountFieldLink(t *testing.T) {
	handle, provider := provider0078(t)
	if _, err := provider.UpTo(context.Background(), 78); err != nil {
		t.Fatal(err)
	}
	if err := exec0078(t, handle, `INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('fields','alice','분야 지침','fields',?,?)`, at0078, at0078); err != nil {
		t.Fatal(err)
	}
	link := `INSERT INTO guideline_fields(guideline_id,field,user_id) VALUES(?,?,?)`
	if err := exec0078(t, handle, link, "fields", "restaurant", "bob"); err == nil {
		t.Fatal("a field link carrying bob's account was accepted for alice's guideline")
	}
	if err := exec0078(t, handle, link, "missing", "restaurant", "alice"); err == nil {
		t.Fatal("a field link to no guideline was accepted")
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_fields`); n != 0 {
		t.Fatalf("%d refused links were written", n)
	}
}

func TestMigration0078StartsWithNoPreset(t *testing.T) {
	handle, provider := provider0078(t)
	if _, err := provider.UpTo(context.Background(), 78); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"guideline_presets", "guideline_preset_fields"} {
		if n := count0078(t, handle, `SELECT count(*) FROM `+table); n != 0 {
			t.Fatalf("%s holds %d rows after the migration", table, n)
		}
	}
	field := `INSERT INTO guideline_preset_fields(user_id,field) VALUES(?,?)`
	if err := exec0078(t, handle, field, "alice", "restaurant"); err == nil {
		t.Fatal("a preset field was accepted without its account's preset row")
	}
	preset := `INSERT INTO guideline_presets(user_id,enabled,updated_at) VALUES(?,?,?)`
	if err := exec0078(t, handle, preset, "alice", 2, at0078); err == nil {
		t.Fatal("the preset accepted an enabled value other than 0 or 1")
	}
	for _, user := range []string{"alice", "bob"} {
		if err := exec0078(t, handle, preset, user, 1, at0078); err != nil {
			t.Fatal(err)
		}
		if err := exec0078(t, handle, field, user, "restaurant"); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec0078(t, handle, field, "alice", "restaurant"); err == nil {
		t.Fatal("the preset accepted the same field twice")
	}

	// The fields go with their preset row, and the row goes with its account.
	if err := exec0078(t, handle, `DELETE FROM guideline_presets WHERE user_id = 'alice'`); err != nil {
		t.Fatal(err)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_preset_fields WHERE user_id = 'alice'`); n != 0 {
		t.Fatalf("deleting the preset row left %d fields", n)
	}
	if err := exec0078(t, handle, `DELETE FROM users WHERE id = 'bob'`); err != nil {
		t.Fatal(err)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_presets`) + count0078(t, handle, `SELECT count(*) FROM guideline_preset_fields`); n != 0 {
		t.Fatalf("deleting the account left %d preset rows", n)
	}
}

func TestMigration0078RollsBackFieldsToOrphanedTemplates(t *testing.T) {
	handle, provider := provider0078(t)
	ctx := context.Background()
	if _, err := provider.UpTo(ctx, 78); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('fields','alice','분야 지침','fields','2026-09-05T00:00:00Z','2026-09-06T00:00:00Z')`,
		`INSERT INTO guideline_fields(guideline_id,field,user_id) VALUES('fields','restaurant','alice'),('fields','domestic_travel','alice')`,
		`INSERT INTO guideline_presets(user_id,enabled,updated_at) VALUES('alice',1,'2026-09-06T00:00:00Z')`,
		`INSERT INTO guideline_preset_fields(user_id,field) VALUES('alice','restaurant')`,
	} {
		if err := exec0078(t, handle, statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	linksBefore := templateLinks0078(t, handle)

	if _, err := provider.DownTo(ctx, 77); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"guideline_fields", "guideline_presets", "guideline_preset_fields"} {
		if n := count0078(t, handle, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table); n != 0 {
			t.Errorf("%s survived the rollback", table)
		}
	}
	if n := count0078(t, handle, `SELECT count(*) FROM sqlite_master WHERE name = 'idx_guideline_fields_field'`); n != 0 {
		t.Error("idx_guideline_fields_field survived the rollback")
	}
	want := []guideline0078{
		{"fields", "alice", "분야 지침", "templates", "2026-09-05T00:00:00Z", "2026-09-06T00:00:00Z"},
		{"global", "alice", "과장 금지", "global", "2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z"},
		{"scoped", "alice", "가격 먼저", "templates", "2026-09-03T00:00:00Z", "2026-09-04T00:00:00Z"},
	}
	if got := guidelines0078(t, handle); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows after the rollback = %v, want %v", got, want)
	}
	// Never widened to global: the former fields guideline holds no link and reaches no prompt.
	if got := templateLinks0078(t, handle); !reflect.DeepEqual(got, linksBefore) || strings.Contains(strings.Join(got, " "), "fields>") {
		t.Fatalf("template links after the rollback = %v, want %v", got, linksBefore)
	}
	if err := exec0078(t, handle, `INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('again','alice','다시','fields',?,?)`, at0078, at0078); err == nil {
		t.Fatal("the rolled-back CHECK still admits the fields scope")
	}
	if n := foreignKeyViolations0078(t, handle); n != 0 {
		t.Fatalf("%d foreign-key violations after the rollback", n)
	}

	if _, err := provider.UpTo(ctx, 78); err != nil {
		t.Fatalf("up again after the rollback: %v", err)
	}
	if got := guidelines0078(t, handle); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows after re-applying = %v", got)
	}
}

func TestMigration0078IsIdempotent(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	versions := count0078(t, handle, `SELECT count(*) FROM goose_db_version`)
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if again := count0078(t, handle, `SELECT count(*) FROM goose_db_version`); again != versions {
		t.Fatalf("goose_db_version rows = %d then %d", versions, again)
	}
	if applied := count0078(t, handle, `SELECT count(*) FROM goose_db_version WHERE version_id = 78 AND is_applied = 1`); applied != 1 {
		t.Fatalf("78 applied %d times", applied)
	}
	for _, table := range []string{"guideline_fields", "guideline_presets", "guideline_preset_fields"} {
		if n := count0078(t, handle, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table); n != 1 {
			t.Errorf("a fresh install has no %s", table)
		}
	}
}
