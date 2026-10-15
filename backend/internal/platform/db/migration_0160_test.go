package db

import (
	"github.com/pressly/goose/v3"
	"io/fs"
	"testing"
)

func TestMigration0160DefaultsEarlierClipChannelsWithoutChangingRatios(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(t.Context(), 159); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-10-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		if _, err = h.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES(?,'alice','keep',?,15000,'2026-10-01T00:00:00Z','2026-10-01T00:00:00Z')`, ratio, ratio); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = provider.UpTo(t.Context(), 160); err != nil {
		t.Fatal(err)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		var channel, savedRatio, title string
		if err = h.Reader.QueryRow(`SELECT channel,ratio,title FROM clip_projects WHERE id=?`, ratio).Scan(&channel, &savedRatio, &title); err != nil {
			t.Fatal(err)
		}
		if channel != "naver_clip" || savedRatio != ratio || title != "keep" {
			t.Fatal(channel, savedRatio, title)
		}
	}
	if _, err = h.Writer.Exec(`UPDATE clip_projects SET channel='unknown'`); err == nil {
		t.Fatal("accepted unknown channel")
	}
	if _, err = provider.DownTo(t.Context(), 159); err != nil {
		t.Fatal(err)
	}
}
