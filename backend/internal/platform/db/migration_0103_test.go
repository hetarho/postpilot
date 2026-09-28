package db

import (
	"github.com/pressly/goose/v3"
	"io/fs"
	"testing"
)

func TestMigration0103KeepsExistingRegionPresenceUnspecified(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(t.Context(), 102); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-09-28T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,intro_preset,outro_preset,created_at,updated_at) VALUES('p','alice','Legacy','vertical',15000,'serif','credits','2026-09-28T00:00:00Z','2026-09-28T00:00:00Z')`,
	} {
		if _, err = h.Writer.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = p.UpTo(t.Context(), 103); err != nil {
		t.Fatal(err)
	}
	var missing bool
	var intro, outro string
	if err = h.Reader.QueryRow(`SELECT regions_json IS NULL,intro_preset,outro_preset FROM clip_projects WHERE id='p'`).Scan(&missing, &intro, &outro); err != nil || !missing || intro != "serif" || outro != "credits" {
		t.Fatal(missing, intro, outro, err)
	}
	if _, err = h.Writer.Exec(`UPDATE clip_projects SET regions_json='bad' WHERE id='p'`); err == nil {
		t.Fatal("invalid envelope admitted")
	}
}
