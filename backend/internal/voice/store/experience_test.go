package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db/dbtest"
	"github.com/postpilot/backend/internal/voice"
	"github.com/postpilot/backend/internal/voice/store"
)

var experienceNow = time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)

// seedVoices makes two owners with one voice each plus a second voice for alice.
func seedVoices(t *testing.T) *store.Store {
	t.Helper()
	handle := dbtest.Open(t)
	stamp := experienceNow.Format(time.RFC3339Nano)
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','h','` + stamp + `'),('bob','h','` + stamp + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES
			('va','alice','a',0,'` + stamp + `','` + stamp + `'),
			('va2','alice','a2',0,'` + stamp + `','` + stamp + `'),
			('vb','bob','b',0,'` + stamp + `','` + stamp + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return store.New(handle.Writer, handle.Reader)
}

func experience(id, user, voiceID string, kind voice.ExperienceKind, name string, minutes int) voice.Experience {
	return voice.Experience{ID: id, UserID: user, VoiceID: voiceID, Kind: kind, Name: name, CreatedAt: experienceNow.Add(time.Duration(minutes) * time.Minute)}
}

func answer(id, experienceID, key, body string) voice.Sample {
	return voice.Sample{ID: id, UserID: "alice", VoiceID: "va", Kind: voice.SampleKindAnswer, PromptKey: key, Body: body, ExperienceID: experienceID, CreatedAt: experienceNow}
}

func TestExperiencesListNewestFirstAndScoped(t *testing.T) {
	s := seedVoices(t)
	ctx := context.Background()
	for _, e := range []voice.Experience{
		experience("e1", "alice", "va", voice.ExperienceFood, "보리밥집", 0),
		experience("e2", "alice", "va", voice.ExperienceCafe, "", 5),
		experience("e3", "alice", "va2", voice.ExperienceStay, "", 9),
		experience("e4", "bob", "vb", voice.ExperienceProduct, "", 9),
	} {
		if err := s.InsertExperience(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListExperiences(ctx, "alice", "va")
	if err != nil || len(list) != 2 || list[0].ID != "e2" || list[1].ID != "e1" || list[1].Name != "보리밥집" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if got, err := s.GetExperience(ctx, "alice", "va", "e1"); err != nil || got == nil || got.Kind != voice.ExperienceFood {
		t.Fatalf("get = %+v, %v", got, err)
	}
	// An unknown, another voice's and another owner's id are all just absent (VOICE-3).
	for _, c := range [][3]string{{"alice", "va", "nope"}, {"alice", "va", "e3"}, {"alice", "va", "e4"}, {"bob", "vb", "e1"}, {"bob", "va", "e1"}} {
		if got, err := s.GetExperience(ctx, c[0], c[1], c[2]); err != nil || got != nil {
			t.Fatalf("get %v = %+v, %v", c, got, err)
		}
	}
	if list, err := s.ListExperiences(ctx, "bob", "va"); err != nil || len(list) != 0 {
		t.Fatalf("foreign list = %+v, %v", list, err)
	}
	if err := s.InsertExperience(ctx, experience("e5", "alice", "va", "snack", "", 0)); err == nil {
		t.Fatal("an unknown kind was stored")
	}
	if err := s.InsertExperience(ctx, experience("e6", "alice", "vb", voice.ExperienceFood, "", 0)); err == nil {
		t.Fatal("an experience on another owner's voice was stored")
	}
}

func TestExperienceAnswerReplacesPerExperienceAndQuestion(t *testing.T) {
	s := seedVoices(t)
	ctx := context.Background()
	for _, e := range []voice.Experience{
		experience("e1", "alice", "va", voice.ExperienceFood, "", 0),
		experience("e2", "alice", "va", voice.ExperienceFood, "", 1),
		experience("e4", "bob", "vb", voice.ExperienceFood, "", 1),
	} {
		if err := s.InsertExperience(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	const key = "thread_food_opening_1"
	for _, a := range []voice.Sample{answer("s1", "e1", key, "first one"), answer("s2", "e2", key, "second one")} {
		if ok, err := s.AnswerExperiencePrompt(ctx, a); err != nil || !ok {
			t.Fatalf("answer %s: %v %v", a.ID, ok, err)
		}
	}
	// Two experiences hold separate answers to one key.
	for id, want := range map[string]string{"e1": "first one", "e2": "second one"} {
		got, err := s.GetExperienceAnswer(ctx, "alice", "va", id, key)
		if err != nil || got == nil || got.Body != want || got.ExperienceID != id {
			t.Fatalf("%s answer = %+v, %v", id, got, err)
		}
	}
	// Replying again replaces only that experience's answer, in place.
	if ok, err := s.AnswerExperiencePrompt(ctx, answer("s3", "e1", key, "changed")); err != nil || !ok {
		t.Fatalf("replace: %v %v", ok, err)
	}
	first, _ := s.GetExperienceAnswer(ctx, "alice", "va", "e1", key)
	second, _ := s.GetExperienceAnswer(ctx, "alice", "va", "e2", key)
	if first.Body != "changed" || first.ID != "s1" || first.ContentRevision != 2 || second.Body != "second one" || second.ContentRevision != 1 {
		t.Fatalf("after replace: %+v / %+v", first, second)
	}
	if n, err := s.CountSamples(ctx, "alice", "va"); err != nil || n != 2 {
		t.Fatalf("samples = %d, %v", n, err)
	}
	// A thread answer is not the voice's situation answer to the same key.
	if got, err := s.GetPromptAnswer(ctx, "alice", "va", key); err != nil || got != nil {
		t.Fatalf("situation answer = %+v, %v", got, err)
	}
	// A foreign experience writes nothing.
	for _, a := range []voice.Sample{answer("s9", "e4", key, "x"), answer("s9", "missing", key, "x")} {
		if ok, err := s.AnswerExperiencePrompt(ctx, a); err != nil || ok {
			t.Fatalf("foreign answer wrote: %v %v", ok, err)
		}
	}
	// The situation answer beside it keeps its own one-per-voice rule.
	situation := voice.Sample{ID: "s5", UserID: "alice", VoiceID: "va", Kind: voice.SampleKindAnswer, PromptKey: key, Body: "scene", CreatedAt: experienceNow}
	if err := s.InsertSample(ctx, situation); err != nil {
		t.Fatalf("situation answer beside the thread answers: %v", err)
	}
	situation.ID = "s6"
	if err := s.InsertSample(ctx, situation); err == nil {
		t.Fatal("a voice held two situation answers to one key")
	}
}
