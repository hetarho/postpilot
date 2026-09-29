package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// POST r25: 108 rebuilds posts with a nullable voice_id and without machine_baseline_voice_id.
// Every existing post keeps its voice and every child row survives the rebuild; a post with
// no voice inserts and can be reassigned to and from 말투 없음; a deleted voice is still
// refused on insert and on reassign.
func TestMigration0108LetsAPostHaveNoVoice(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 107); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-29T00:00:00Z"
	exec := func(statement string, args ...any) error {
		_, err := handle.Writer.Exec(statement, args...)
		return err
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,deleted_at,created_at,updated_at) VALUES('voice-gone','alice','옛 말투',0,'` + at + `','` + at + `','` + at + `')`,
		`INSERT INTO templates(id,user_id,name,body,created_at,updated_at) VALUES('template','alice','리뷰','본문','` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,content,content_revision,machine_baseline,machine_baseline_revision,machine_baseline_voice_id,template_id,created_at,updated_at) VALUES('post','alice','voice-a','제목','{}',2,'{}',2,'voice-a','template','` + at + `','` + at + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('image','post','IMG_1.jpg','k',1,1,1,'` + at + `')`,
		`INSERT INTO uploads(id,post_slug,filename,r2_key,expires_at,created_at) VALUES('upload','post','IMG_2.jpg','k2','` + at + `','` + at + `')`,
		`INSERT INTO videos(id,post_slug,filename,r2_key,content_type,bytes,duration_ms,width,height,created_at) VALUES('video','post','CLIP.mp4','k3','video/mp4',1,1000,1,1,'` + at + `')`,
		`INSERT INTO generation_jobs(id,post_slug,user_id,voice_id,kind,status,created_at,updated_at) VALUES('job','post','alice','voice-a','generate','done','` + at + `','` + at + `')`,
		`INSERT INTO post_template_answers(post_slug,label,answer,updated_at) VALUES('post','평점','4.5','` + at + `')`,
		`INSERT INTO post_measurements(post_slug,user_id,content_revision,measure_version,char_count,photo_count,distinct_block_types,computed_at) VALUES('post','alice',2,1,10,1,1,'` + at + `')`,
		`INSERT INTO model_experiments(id,user_id,post_slug,voice_id,stage,status,input_hash,prompt_version,created_at) VALUES('experiment','alice','post','voice-a','write','decided','hash','v1','` + at + `')`,
		`INSERT INTO memories(id,user_id,kind,text,created_at,updated_at,last_seen_at) VALUES('memory','alice','preference','매운 음식을 못 먹는다','` + at + `','` + at + `','` + at + `')`,
		`INSERT INTO memory_sources(memory_id,post_slug,user_id,created_at) VALUES('memory','post','alice','` + at + `')`,
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if err := exec(`INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('early','alice','` + at + `','` + at + `')`); err == nil {
		t.Fatal("the pre-108 table already admitted a post with no voice")
	}

	if _, err := provider.UpTo(ctx, 108); err != nil {
		t.Fatal(err)
	}
	count := func(query string) int {
		t.Helper()
		var n int
		if err := handle.Reader.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for table, query := range map[string]string{
		"images":                `SELECT count(*) FROM images WHERE post_slug='post'`,
		"uploads":               `SELECT count(*) FROM uploads WHERE post_slug='post'`,
		"videos":                `SELECT count(*) FROM videos WHERE post_slug='post'`,
		"generation_jobs":       `SELECT count(*) FROM generation_jobs WHERE post_slug='post'`,
		"post_template_answers": `SELECT count(*) FROM post_template_answers WHERE post_slug='post'`,
		"post_measurements":     `SELECT count(*) FROM post_measurements WHERE post_slug='post'`,
		"model_experiments":     `SELECT count(*) FROM model_experiments WHERE post_slug='post'`,
		"memory_sources":        `SELECT count(*) FROM memory_sources WHERE post_slug='post'`,
	} {
		if n := count(query); n != 1 {
			t.Fatalf("the rebuild lost the post's %s: %d", table, n)
		}
	}
	var voice, template string
	var revision, baseline int
	if err := handle.Reader.QueryRow(`SELECT voice_id, template_id, content_revision, machine_baseline_revision FROM posts WHERE slug='post'`).Scan(&voice, &template, &revision, &baseline); err != nil ||
		voice != "voice-a" || template != "template" || revision != 2 || baseline != 2 {
		t.Fatalf("the post after 108 = %s %s %d %d %v", voice, template, revision, baseline, err)
	}
	if n := count(`SELECT count(*) FROM pragma_table_info('posts') WHERE name='machine_baseline_voice_id'`); n != 0 {
		t.Fatal("machine_baseline_voice_id survived 108")
	}

	if err := exec(`INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('none','alice','` + at + `','` + at + `')`); err != nil {
		t.Fatalf("a post with no voice was refused: %v", err)
	}
	if err := exec(`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('gone','alice','voice-gone','` + at + `','` + at + `')`); err == nil || !strings.Contains(err.Error(), "post voice must be active") {
		t.Fatalf("a post was created in a deleted voice: %v", err)
	}
	if err := exec(`UPDATE posts SET voice_id='voice-a' WHERE slug='none'`); err != nil {
		t.Fatalf("setting a voice on a 말투 없음 post was refused: %v", err)
	}
	if err := exec(`UPDATE posts SET voice_id=NULL WHERE slug='none'`); err != nil {
		t.Fatalf("clearing a post's voice was refused: %v", err)
	}
	if err := exec(`UPDATE posts SET voice_id='voice-gone' WHERE slug='none'`); err == nil || !strings.Contains(err.Error(), "post voice must be active") {
		t.Fatalf("a post was moved to a deleted voice: %v", err)
	}
	// The template detach still runs.
	if err := exec(`DELETE FROM templates WHERE id='template'`); err != nil {
		t.Fatal(err)
	}
	if n := count(`SELECT count(*) FROM posts WHERE template_id IS NOT NULL`); n != 0 {
		t.Fatal("deleting a template no longer detaches its posts")
	}
	var violations int
	rows, err := handle.Reader.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		violations++
	}
	rows.Close()
	if violations != 0 {
		t.Fatalf("108 left %d foreign key violations", violations)
	}
}
