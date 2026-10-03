package db

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// POST-107: every photo stored before rotations existed reads as unturned and never turned by
// its owner, and the columns admit only the four quarter turns and a 0/1 flag.
func TestMigration0127KeepsExistingPhotosUnturned(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(t.Context(), 126); err != nil {
		t.Fatal(err)
	}
	const at = "2026-10-04T00:00:00Z"
	for _, stmt := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('post','alice','` + at + `','` + at + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('img','post','a.jpg','k',1024,768,10,'` + at + `')`,
	} {
		if _, err = h.Writer.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = p.UpTo(t.Context(), 127); err != nil {
		t.Fatal(err)
	}
	var rotation, byOwner int
	if err = h.Reader.QueryRow(`SELECT rotation, rotation_by_owner FROM images WHERE id='img'`).Scan(&rotation, &byOwner); err != nil || rotation != 0 || byOwner != 0 {
		t.Fatal(rotation, byOwner, err)
	}
	if _, err = h.Writer.Exec(`UPDATE images SET rotation=90, rotation_by_owner=1 WHERE id='img'`); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`UPDATE images SET rotation=45 WHERE id='img'`, `UPDATE images SET rotation_by_owner=2 WHERE id='img'`} {
		if _, err = h.Writer.Exec(bad); err == nil {
			t.Fatalf("%s was admitted", bad)
		}
	}
}
