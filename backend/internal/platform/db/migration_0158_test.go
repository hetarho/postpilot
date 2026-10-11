package db

import (
	"io/fs"
	"reflect"
	"testing"

	"github.com/pressly/goose/v3"
)

// POST-116: each post's existing photos take positions 0..n-1 in the order they were listed
// in before the column existed, by created_at then id, and another post's photos count apart.
func TestMigration0158BackfillsEachPostsPhotoOrder(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(t.Context(), 157); err != nil {
		t.Fatal(err)
	}
	const early, late = "2026-10-11T00:00:00.000000000Z", "2026-10-11T00:00:01.000000000Z"
	for _, stmt := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + early + `')`,
		`INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('a','alice','` + early + `','` + early + `')`,
		`INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('b','alice','` + early + `','` + early + `')`,
		// Inserted out of order: a later photo first, then two photos of one instant.
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('a3','a','3.jpg','k3',1,1,1,'` + late + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('a2','a','2.jpg','k2',1,1,1,'` + early + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('a1','a','1.jpg','k1',1,1,1,'` + early + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('b1','b','1.jpg','k4',1,1,1,'` + late + `')`,
	} {
		if _, err = h.Writer.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = p.UpTo(t.Context(), 158); err != nil {
		t.Fatal(err)
	}
	rows, err := h.Reader.Query(`SELECT id, position FROM images ORDER BY post_slug, position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	var order []string
	for rows.Next() {
		var id string
		var position int
		if err := rows.Scan(&id, &position); err != nil {
			t.Fatal(err)
		}
		got[id] = position
		order = append(order, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"a1": 0, "a2": 1, "a3": 2, "b1": 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	if want := []string{"a1", "a2", "a3", "b1"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if _, err = p.DownTo(t.Context(), 157); err != nil {
		t.Fatalf("down: %v", err)
	}
}
