package voice_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/voice"
)

func TestStartExperienceValidatesAndScopes(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")

	e, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceFood, "  성수 칼국수집  ")
	if err != nil || e.Name != "성수 칼국수집" || e.Kind != voice.ExperienceFood || e.VoiceID != alice || e.UserID != "alice" {
		t.Fatalf("start = %+v err=%v", e, err)
	}
	if _, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceCafe, ""); err != nil {
		t.Fatalf("an empty name is valid: %v", err)
	}
	if _, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceFood, strings.Repeat("가", voice.VoiceExperienceNameMaxChars)); err != nil {
		t.Fatalf("a name at the bound = %v", err)
	}
	var tooLong *voice.ExperienceNameError
	if _, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceFood, strings.Repeat("가", voice.VoiceExperienceNameMaxChars+1)); !errors.As(err, &tooLong) || tooLong.Chars != 41 {
		t.Fatalf("over-long name = %v", err)
	}
	if _, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceKind("spa"), "x"); !errors.Is(err, voice.ErrExperienceKindUnknown) {
		t.Fatalf("unknown kind = %v", err)
	}
	if _, err := h.svc.StartExperience(ctx, "bob", alice, voice.ExperienceFood, "x"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign voice = %v", err)
	}
	if _, err := h.svc.StartExperience(ctx, "alice", "no-such-voice", voice.ExperienceFood, "x"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("unknown voice = %v", err)
	}

	list, err := h.svc.ListExperiences(ctx, "alice", alice)
	if err != nil || len(list) != 3 || list[0].Name != strings.Repeat("가", 40) || list[2].ID != e.ID {
		t.Fatalf("list should be newest first: %+v err=%v", list, err)
	}
	if _, err := h.svc.ListExperiences(ctx, "bob", alice); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign list = %v", err)
	}

	if _, err := h.svc.DeleteVoice(ctx, "alice", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceFood, "x"); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("deleted voice = %v", err)
	}
	if after, err := h.svc.ListExperiences(ctx, "alice", alice); err != nil || len(after) != 3 {
		t.Fatalf("a deleted voice stays readable: %+v err=%v", after, err)
	}
}

func TestAnswerPromptByExperience(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	food, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceFood, "")
	if err != nil {
		t.Fatal(err)
	}
	cafe, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceCafe, "")
	if err != nil {
		t.Fatal(err)
	}
	bobFood, err := h.svc.StartExperience(ctx, "bob", h.voice("bob"), voice.ExperienceFood, "")
	if err != nil {
		t.Fatal(err)
	}
	const key = "thread_food_opening_1"
	body := "친구가 추천해 줘서 퇴근길에 들렀어요."

	for name, tc := range map[string]struct {
		key, experience string
		want            error
	}{
		"thread without experience":  {key, "", voice.ErrExperienceMismatch},
		"experience of another kind": {key, cafe.ID, voice.ErrExperienceMismatch},
		"situation with experience":  {"opening_greeting", food.ID, voice.ErrExperienceMismatch},
		"unknown experience":         {key, "no-such-experience", voice.ErrExperienceNotFound},
		"foreign experience":         {key, bobFood.ID, voice.ErrExperienceNotFound},
	} {
		if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: tc.key, ExperienceID: tc.experience, Body: body}); !errors.Is(err, tc.want) {
			t.Fatalf("%s = %v, want %v", name, err, tc.want)
		}
	}
	if samples, err := h.store.ListSampleBodies(ctx, "alice", alice); err != nil || len(samples) != 0 {
		t.Fatalf("refusals wrote %+v err=%v", samples, err)
	}

	first, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: key, ExperienceID: food.ID, Body: "  " + body + "  "})
	if err != nil || first.ExperienceID != food.ID || first.Body != body || first.ContentRevision != 1 {
		t.Fatalf("first answer = %+v err=%v", first, err)
	}
	same, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: key, ExperienceID: food.ID, Body: body})
	if err != nil || same.ID != first.ID || same.ContentRevision != 1 {
		t.Fatalf("the same text changes nothing: %+v err=%v", same, err)
	}
	replaced, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: key, ExperienceID: food.ID, Body: "퇴근길에 우연히 들어갔는데 분위기가 좋았어요."})
	if err != nil || replaced.ID != first.ID || replaced.ContentRevision != 2 {
		t.Fatalf("replace = %+v err=%v", replaced, err)
	}
	// A situation answer and a second food experience are separate answers to what looks like
	// the same question.
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "안녕하세요! 오늘도 반가워요."}); err != nil {
		t.Fatal(err)
	}
	food2, err := h.svc.StartExperience(ctx, "alice", alice, voice.ExperienceFood, "두 번째")
	if err != nil {
		t.Fatal(err)
	}
	if other, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: key, ExperienceID: food2.ID, Body: body}); err != nil || other.ID == first.ID {
		t.Fatalf("second experience's answer = %+v err=%v", other, err)
	}
	samples, err := h.store.ListSampleBodies(ctx, "alice", alice)
	if err != nil || len(samples) != 3 {
		t.Fatalf("samples = %+v err=%v", samples, err)
	}
	byExperience := map[string]int{}
	for _, sample := range samples {
		byExperience[sample.ExperienceID]++
	}
	if byExperience[food.ID] != 1 || byExperience[food2.ID] != 1 || byExperience[""] != 1 {
		t.Fatalf("lists must carry each answer's experience: %v", byExperience)
	}
}
