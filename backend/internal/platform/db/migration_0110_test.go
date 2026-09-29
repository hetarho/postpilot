package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// VOICE r5 (T469): 110 turns every existing 학습 글 into a pasted post, lets an answer name its
// prompt and photo, holds one answer per prompt and voice, and adds the pending photo uploads.
func TestMigration0110HoldsPostsAndAnswers(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 109); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-29T00:00:00Z"
	exec := func(statement string, args ...any) error {
		_, err := handle.Writer.Exec(statement, args...)
		return err
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','리뷰',0,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-b','alice','일상',0,'` + at + `','` + at + `')`,
		`INSERT INTO voice_samples(id,user_id,voice_id,label,body,created_at) VALUES('old','alice','voice-a','붙여 넣은 글','본문','` + at + `')`,
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if _, err := provider.UpTo(ctx, 110); err != nil {
		t.Fatal(err)
	}
	var kind, label string
	if err := handle.Reader.QueryRow(`SELECT kind, label FROM voice_samples WHERE id='old'`).Scan(&kind, &label); err != nil || kind != "post" || label != "붙여 넣은 글" {
		t.Fatalf("the existing row = %s %s %v", kind, label, err)
	}
	insert := func(id, voice, kind, prompt, photo string) error {
		var promptValue, photoValue, size any
		if prompt != "" {
			promptValue = prompt
		}
		if photo != "" {
			photoValue, size = photo, 100
		}
		return exec(`INSERT INTO voice_samples(id,user_id,voice_id,kind,prompt_key,label,body,photo_key,photo_width,photo_height,created_at)
			VALUES(?,'alice',?,?,?,'','답',?,?,?,?)`, id, voice, kind, promptValue, photoValue, size, size, at)
	}
	for name, err := range map[string]error{
		"a post with a prompt":     insert("p1", "voice-a", "post", "opening_greeting", ""),
		"a post with a photo":      insert("p2", "voice-a", "post", "", "voices/voice-a/x.jpg"),
		"an answer with no prompt": insert("a1", "voice-a", "answer", "", ""),
		"an unknown kind":          insert("k1", "voice-a", "note", "", ""),
	} {
		if err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if err := insert("a2", "voice-a", "answer", "photo_food", "voices/voice-a/food.jpg"); err != nil {
		t.Fatalf("a photo answer: %v", err)
	}
	if err := insert("a3", "voice-a", "answer", "photo_food", ""); err == nil {
		t.Fatal("a second answer to one prompt was accepted")
	}
	if err := insert("a4", "voice-b", "answer", "photo_food", ""); err != nil {
		t.Fatalf("another voice's answer to the same prompt: %v", err)
	}
	if err := exec(`INSERT INTO voice_photo_uploads(id,user_id,voice_id,prompt_key,object_key,expires_at,created_at)
		VALUES('u1','alice','voice-a','photo_space','voices/voice-a/space.jpg',?,?)`, at, at); err != nil {
		t.Fatalf("a pending photo upload: %v", err)
	}
	if err := exec(`INSERT INTO voice_photo_uploads(id,user_id,voice_id,prompt_key,object_key,expires_at,created_at)
		VALUES('u2','alice','voice-foreign','photo_space','voices/x.jpg',?,?)`, at, at); err == nil {
		t.Fatal("a pending upload for a voice the account does not own was accepted")
	}
}
