package db

import (
	"context"
	"testing"
)

// VOICE r22 (T874): 169 adds the learning chat's experiences and keys a thread answer by its
// experience, beside the situation answer's one-per-voice rule.
func TestMigration0169AddsVoiceExperiences(t *testing.T) {
	handle := openTemp(t)
	if err := Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	exec := func(statement string, args ...any) error {
		_, err := handle.Writer.Exec(statement, args...)
		return err
	}
	const at = "2026-11-16T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','h','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('v','alice','n',0,'` + at + `','` + at + `')`,
	} {
		if err := exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	experience := func(id, kind string) error {
		return exec(`INSERT INTO voice_experiences(id,user_id,voice_id,kind,created_at) VALUES(?,?,?,?,?)`, id, "alice", "v", kind, at)
	}
	answer := func(id, key string, experienceID any) error {
		return exec(`INSERT INTO voice_samples(id,user_id,voice_id,kind,prompt_key,label,body,created_at,experience_id)
			VALUES(?,?,?,?,?,?,?,?,?)`, id, "alice", "v", "answer", key, "", "body", at, experienceID)
	}

	for _, kind := range []string{"food", "cafe", "beauty", "stay", "leisure", "product"} {
		if err := experience("e-"+kind, kind); err != nil {
			t.Fatalf("kind %s refused: %v", kind, err)
		}
	}
	if err := experience("bad", "snack"); err == nil {
		t.Fatal("an unknown kind was accepted")
	}
	var name string
	if err := handle.Reader.QueryRow(`SELECT name FROM voice_experiences WHERE id='e-food'`).Scan(&name); err != nil || name != "" {
		t.Fatalf("an unnamed experience reads %q: %v", name, err)
	}

	// Two experiences hold separate answers to one key; a second answer in one is refused.
	if err := answer("a1", "k", "e-food"); err != nil {
		t.Fatal(err)
	}
	if err := answer("a2", "k", "e-cafe"); err != nil {
		t.Fatalf("a second experience's answer to the same key: %v", err)
	}
	if err := answer("a3", "k", "e-food"); err == nil {
		t.Fatal("an experience held two answers to one key")
	}
	// The situation answer keeps one per voice, beside the thread answers.
	if err := answer("s1", "k", nil); err != nil {
		t.Fatalf("a situation answer beside thread answers: %v", err)
	}
	if err := answer("s2", "k", nil); err == nil {
		t.Fatal("a voice held two situation answers to one key")
	}
	var indexes int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name IN ('voice_samples_voice_prompt','voice_samples_experience_prompt')`).Scan(&indexes); err != nil || indexes != 2 {
		t.Fatalf("indexes = %d: %v", indexes, err)
	}

	// Deleting the voice removes its experiences and their answers.
	if err := exec(`DELETE FROM voice_samples WHERE experience_id IS NULL`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`DELETE FROM voices WHERE id='v'`); err != nil {
		t.Fatalf("delete voice: %v", err)
	}
	for _, table := range []string{"voice_experiences", "voice_samples"} {
		var n int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s left %d rows: %v", table, n, err)
		}
	}
}
