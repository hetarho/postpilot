package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// GUIDE-2, GUIDE-5, GUIDE-10: 97 gives guidelines and their candidates a kind for good. Every row
// and link from before becomes a post's and survives the rebuild; a text is unique within a kind
// and free across kinds; a clip guideline links video templates, and deleting either side
// cascades the link.
func TestMigration0097GivesGuidelinesAKind(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 96); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := handle.Writer.Exec(statement, args...); err != nil {
			t.Fatalf("%q: %v", statement, err)
		}
	}
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at)
	exec(`INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at) VALUES('tpl','alice','리뷰','','<write>본문</write>',?,?)`, at, at)
	exec(`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('g1','alice','CCTV를 언급하지 않기','templates',?,?)`, at, at)
	exec(`INSERT INTO guideline_templates(guideline_id,template_id,user_id) VALUES('g1','tpl','alice')`)
	exec(`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('g2','alice','가격은 쓰지 않기','fields',?,?)`, at, at)
	exec(`INSERT INTO guideline_fields(guideline_id,field,user_id) VALUES('g2','cafe','alice')`)
	exec(`INSERT INTO guideline_candidates(id,user_id,text,post_slug,status,occurrences,first_seen_at,last_seen_at) VALUES('c1','alice','짧게','post-1','pending',2,?,?)`, at, at)

	if _, err := provider.UpTo(ctx, 97); err != nil {
		t.Fatal(err)
	}

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := handle.Reader.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("%q: %v", query, err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM guidelines WHERE kind='post'`); n != 2 {
		t.Fatalf("post guidelines after 97 = %d, want both", n)
	}
	if n := count(`SELECT count(*) FROM guideline_templates`) + count(`SELECT count(*) FROM guideline_fields`); n != 2 {
		t.Fatalf("links after the rebuild = %d, want both", n)
	}
	var kind, slug string
	var occurrences int
	if err := handle.Reader.QueryRow(`SELECT kind, post_slug, occurrences FROM guideline_candidates WHERE id='c1'`).Scan(&kind, &slug, &occurrences); err != nil ||
		kind != "post" || slug != "post-1" || occurrences != 2 {
		t.Fatalf("candidate after 97 = %s %s %d (%v)", kind, slug, occurrences, err)
	}

	// Unique within a kind, free across kinds.
	insert := `INSERT INTO guidelines(id,user_id,kind,text,scope,created_at,updated_at) VALUES(?, 'alice', ?, ?, 'global', ?, ?)`
	if _, err := handle.Writer.Exec(insert, "g3", "post", "CCTV를 언급하지 않기", at, at); err == nil {
		t.Fatal("the same post text was saved twice")
	}
	exec(insert, "g4", "clip", "CCTV를 언급하지 않기", at, at)
	if _, err := handle.Writer.Exec(insert, "g5", "video", "다른 것", at, at); err == nil {
		t.Fatal("a kind outside post/clip was saved")
	}
	candidate := `INSERT INTO guideline_candidates(id,user_id,kind,text,clip_id,status,first_seen_at,last_seen_at) VALUES(?, 'alice', ?, '짧게', ?, 'pending', ?, ?)`
	exec(candidate, "c2", "clip", "project-1", at, at)
	if _, err := handle.Writer.Exec(candidate, "c3", "clip", "project-2", at, at); err == nil {
		t.Fatal("the same clip candidate text was saved twice")
	}

	// A clip guideline's video template links cascade from either side.
	exec(`INSERT INTO video_templates(id,user_id,name,created_at,updated_at) VALUES('vt','alice','가게 소개',?,?)`, at, at)
	exec(`INSERT INTO guidelines(id,user_id,kind,text,scope,created_at,updated_at) VALUES('g6','alice','clip','자막은 짧게','templates',?,?)`, at, at)
	exec(`INSERT INTO guideline_video_templates(guideline_id,video_template_id,user_id) VALUES('g6','vt','alice')`)
	exec(`INSERT INTO guideline_video_templates(guideline_id,video_template_id,user_id) VALUES('g4','vt','alice')`)
	if _, err := handle.Writer.Exec(`INSERT INTO guideline_video_templates(guideline_id,video_template_id,user_id) VALUES('g6','missing','alice')`); err == nil {
		t.Fatal("a link to a video template that does not exist was saved")
	}
	exec(`DELETE FROM guidelines WHERE id='g4'`)
	if n := count(`SELECT count(*) FROM guideline_video_templates`); n != 1 {
		t.Fatalf("links after a guideline delete = %d, want 1", n)
	}
	exec(`DELETE FROM video_templates WHERE id='vt'`)
	if n := count(`SELECT count(*) FROM guideline_video_templates`); n != 0 {
		t.Fatalf("links after a video template delete = %d, want none", n)
	}
	if n := count(`SELECT count(*) FROM guidelines WHERE id='g6'`); n != 1 {
		t.Fatal("deleting the video template took the guideline with it")
	}

	if _, err := provider.DownTo(ctx, 96); err != nil {
		t.Fatalf("down: %v", err)
	}
	if n := count(`SELECT count(*) FROM guidelines`); n != 2 {
		t.Fatalf("guidelines after the rollback = %d, want the two post ones", n)
	}
}
