package db

import (
	"path/filepath"
	"testing"
)

// 0069 adds the three memory tables (MEM r1). What this pins is the shape the context rests
// on: the closed kind, the per-account text uniqueness dedup is built from, the composite
// target the children point at, and the cascade that takes tags and links with a memory.
// The post link is deliberately NOT a foreign key — see the migration.
func TestMigration0069MemoriesCarryTheirKindUniquenessAndCascades(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := t.Context()
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal("migration was not idempotent", err)
	}
	for _, q := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-20T00:00:00Z')`,
		`INSERT INTO users(id,password_hash,created_at) VALUES('other','hash','2026-09-20T00:00:00Z')`,
		`INSERT INTO memories(id,user_id,text,kind,created_at,updated_at,last_seen_at) VALUES('m1','owner','매운 음식을 못 먹는다','preference','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z')`,
		`INSERT INTO memory_tags(memory_id,user_id,tag) VALUES('m1','owner','음식')`,
		`INSERT INTO memory_sources(memory_id,user_id,post_slug,created_at) VALUES('m1','owner','post-1','2026-09-20T00:00:00Z')`,
	} {
		if _, err := d.Writer.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}

	// The five and nothing else (MEM-5): a sixth kind is a schema error, not a row.
	if _, err := d.Writer.Exec(`INSERT INTO memories(id,user_id,text,kind,created_at,updated_at,last_seen_at) VALUES('m2','owner','기분','mood','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z')`); err == nil {
		t.Fatal("an unknown kind was accepted")
	}
	// Exact-text dedup as a constraint, so two concurrent approvals cannot both insert.
	if _, err := d.Writer.Exec(`INSERT INTO memories(id,user_id,text,kind,created_at,updated_at,last_seen_at) VALUES('m3','owner','매운 음식을 못 먹는다','persona','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z')`); err == nil {
		t.Fatal("the same account stored one text twice")
	}
	// The same text for another account is a different fact.
	if _, err := d.Writer.Exec(`INSERT INTO memories(id,user_id,text,kind,created_at,updated_at,last_seen_at) VALUES('m4','other','매운 음식을 못 먹는다','preference','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z','2026-09-20T00:00:00Z')`); err != nil {
		t.Fatal("a second account was refused the same text", err)
	}
	// A child row must name the OWNER of the memory it points at, which is what the
	// (id, user_id) composite target is for.
	if _, err := d.Writer.Exec(`INSERT INTO memory_tags(memory_id,user_id,tag) VALUES('m1','other','남의 태그')`); err == nil {
		t.Fatal("a tag was linked across accounts")
	}

	// Deleting the memory takes its tags and its links with it.
	if _, err := d.Writer.Exec(`DELETE FROM memories WHERE id='m1'`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"memory_tags", "memory_sources"} {
		var left int
		if err := d.Reader.QueryRow(`SELECT count(*) FROM ` + table).Scan(&left); err != nil || left != 0 {
			t.Fatalf("%s kept %d rows after the memory went (%v)", table, left, err)
		}
	}

	var integrity string
	if err := d.Reader.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q (%v)", integrity, err)
	}
	var problems int
	if err := d.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&problems); err != nil || problems != 0 {
		t.Fatalf("foreign key check found %d problems: %v", problems, err)
	}
}
