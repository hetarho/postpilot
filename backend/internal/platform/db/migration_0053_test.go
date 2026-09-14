package db

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0053BackfillsKoreanAndChecksLanguage(t *testing.T) {
	d := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(t.Context(), 52); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-09-15T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('legacy','alice','기록','vertical',15000,'2026-09-15T00:00:00Z','2026-09-15T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = provider.UpTo(t.Context(), 53); err != nil {
			t.Fatal(err)
		}
		var language string
		if err = d.Reader.QueryRow(`SELECT language FROM clip_projects WHERE id='legacy'`).Scan(&language); err != nil || language != "ko" {
			t.Fatalf("legacy language=%q: %v", language, err)
		}
		for _, invalid := range []any{"", "ja", nil} {
			if _, err = d.Writer.Exec(`UPDATE clip_projects SET language=? WHERE id='legacy'`, invalid); err == nil {
				t.Fatalf("accepted %v", invalid)
			}
		}
		if _, err = d.Writer.Exec(`UPDATE clip_projects SET language='en' WHERE id='legacy'`); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err = provider.DownTo(t.Context(), 52); err != nil {
				t.Fatal(err)
			}
		}
	}
}
