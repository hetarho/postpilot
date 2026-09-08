package post

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ptrOf is local to this file: the package's other suites name a local per pointer, which
// reads worse when one call needs four of them.
func ptrOf[T any](value T) *T { return &value }

// answerService is the template-aware service with the answer ceilings wired, which is what
// cmd/api does from config. Without the setter every non-empty answer is refused, and that
// safe direction is asserted at the bottom of this file.
func newAnswerService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	svc, store := newTemplateAwareService(t)
	svc.SetTemplateAnswerLimits(40, 500)
	return svc, store
}

func TestSaveDraftStoresTemplateAnswersPerLabel(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAnswerService(t)

	created, err := svc.SaveDraft(ctx, alice, "", "제주", "갔다", ptrOf(defaultVoiceFor(alice)), ptrOf("template-review"),
		ptrOf(LanguageKorean), []TemplateAnswer{{Label: "총평 별점", Text: "4.5점", Enabled: true}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.TemplateAnswers) != 1 || created.TemplateAnswers[0].Text != "4.5점" {
		t.Fatalf("the create did not carry its answers back: %+v", created.TemplateAnswers)
	}

	// One field saved alone leaves the other where it was: absent is preserved, which is what
	// keeps two tabs answering two fields from overwriting each other (POST-62).
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, "제주", "갔다", nil, nil, nil,
		[]TemplateAnswer{{Label: "방문일", Text: "2026-03-01", Enabled: true}}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	saved, err := svc.SaveDraft(ctx, alice, created.Slug, "제주", "갔다", nil, nil, nil,
		[]TemplateAnswer{{Label: "총평 별점", Text: "5점", Enabled: false}})
	if err != nil {
		t.Fatalf("third save: %v", err)
	}
	if len(saved.TemplateAnswers) != 2 {
		t.Fatalf("answers = %+v", saved.TemplateAnswers)
	}
	// Ordered by label: 방문일 before 총평 별점.
	if saved.TemplateAnswers[0].Label != "방문일" || saved.TemplateAnswers[0].Text != "2026-03-01" {
		t.Errorf("first answer = %+v", saved.TemplateAnswers[0])
	}
	// The switch went off and the text was KEPT: losing what was typed would make trying a
	// field twice expensive.
	if saved.TemplateAnswers[1].Enabled || saved.TemplateAnswers[1].Text != "5점" {
		t.Errorf("second answer = %+v", saved.TemplateAnswers[1])
	}

	// An ordinary autosave carries no answers and disturbs none.
	plain, err := svc.SaveDraft(ctx, alice, created.Slug, "제주도", "갔다 왔다", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("plain autosave: %v", err)
	}
	if len(plain.TemplateAnswers) != 2 || plain.Title != "제주도" {
		t.Errorf("a title-only save changed the answers: %+v", plain.TemplateAnswers)
	}
}

func TestTemplateAnswersOutliveTheirTemplate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAnswerService(t)

	created, err := svc.SaveDraft(ctx, alice, "", "제주", "갔다", ptrOf(defaultVoiceFor(alice)), ptrOf("template-review"),
		ptrOf(LanguageKorean), []TemplateAnswer{{Label: "총평 별점", Text: "4.5점", Enabled: true}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Swapping the template, then clearing it, touches no answer: the label is the key, and a
	// swap back has to find what was typed still there (POST-62).
	swapped, err := svc.SaveDraft(ctx, alice, created.Slug, "제주", "갔다", nil, ptrOf("template-diary"), nil, nil)
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	if len(swapped.TemplateAnswers) != 1 || swapped.Template.ID != "template-diary" {
		t.Fatalf("after the swap: template=%s answers=%+v", swapped.Template.ID, swapped.TemplateAnswers)
	}
	cleared, err := svc.SaveDraft(ctx, alice, created.Slug, "제주", "갔다", nil, ptrOf(""), nil, nil)
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.TemplateID != "" || len(cleared.TemplateAnswers) != 1 {
		t.Fatalf("after clearing: template=%q answers=%+v", cleared.TemplateID, cleared.TemplateAnswers)
	}
}

func TestSaveDraftRefusesAnswersBeforeAnythingElse(t *testing.T) {
	ctx := context.Background()
	svc, store := newAnswerService(t)

	created, err := svc.SaveDraft(ctx, alice, "", "제주", "갔다", ptrOf(defaultVoiceFor(alice)), nil, ptrOf(LanguageKorean), nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cases := []struct {
		name    string
		answers []TemplateAnswer
		want    error
	}{
		{"a blank label", []TemplateAnswer{{Label: "  ", Text: "x", Enabled: true}}, ErrTemplateAnswerInvalid},
		{"one label twice", []TemplateAnswer{
			{Label: "총평", Text: "a", Enabled: true},
			{Label: "총평", Text: "b", Enabled: true},
		}, ErrTemplateAnswerInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.SaveDraft(ctx, alice, created.Slug, "다른 제목", "다른 메모", nil, nil, nil, tc.answers); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			// Refused ahead of every write: the title it carried must not have landed.
			stored, err := svc.Get(ctx, alice, created.Slug)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if stored.Title != "제주" || len(stored.TemplateAnswers) != 0 {
				t.Errorf("a refused save changed the post: title=%q answers=%+v", stored.Title, stored.TemplateAnswers)
			}
		})
	}

	// Too long, on either half, names the field and both counts.
	var tooLong *TemplateAnswerTooLongError
	_, err = svc.SaveDraft(ctx, alice, created.Slug, "제주", "갔다", nil, nil, nil,
		[]TemplateAnswer{{Label: strings.Repeat("가", 41), Text: "x", Enabled: true}})
	if !errors.As(err, &tooLong) || tooLong.Field != "label" || tooLong.Max != 40 || tooLong.Chars != 41 {
		t.Fatalf("label bound: err = %v (%+v)", err, tooLong)
	}
	_, err = svc.SaveDraft(ctx, alice, created.Slug, "제주", "갔다", nil, nil, nil,
		[]TemplateAnswer{{Label: "총평", Text: strings.Repeat("가", 501), Enabled: true}})
	if !errors.As(err, &tooLong) || tooLong.Field != "text" || tooLong.Max != 500 {
		t.Fatalf("text bound: err = %v (%+v)", err, tooLong)
	}

	// A mint is a write like any other: a bad answer on a create leaves no post behind.
	if _, err := svc.SaveDraft(ctx, alice, "", "새 글", "메모", ptrOf(defaultVoiceFor(alice)), nil, ptrOf(LanguageKorean),
		[]TemplateAnswer{{Label: "", Text: "x", Enabled: true}}); !errors.Is(err, ErrTemplateAnswerInvalid) {
		t.Fatalf("create with a bad answer: %v", err)
	}
	if len(store.posts) != 1 {
		t.Errorf("a refused create minted a post: %d posts", len(store.posts))
	}
}

// A service whose config never supplied the ceilings refuses every non-empty answer, the
// same safe direction the video ceilings take.
func TestTemplateAnswersRefusedWithoutConfiguredLimits(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTemplateAwareService(t)

	var tooLong *TemplateAnswerTooLongError
	if _, err := svc.SaveDraft(ctx, alice, "", "제주", "갔다", ptrOf(defaultVoiceFor(alice)), nil, ptrOf(LanguageKorean),
		[]TemplateAnswer{{Label: "총평", Text: "4.5점", Enabled: true}}); !errors.As(err, &tooLong) {
		t.Fatalf("err = %v, want a too-long refusal", err)
	}
}

// An answer save is an OPTION save (POST-13): it moves nothing about the lifecycle, and a
// finalized post still takes one — the answers are the input the NEXT run works from, not
// content.
func TestSavingAnAnswerIsAnOptionSave(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAnswerService(t)

	created, err := svc.SaveDraft(ctx, alice, "", "제주", "갔다", ptrOf(defaultVoiceFor(alice)), ptrOf("template-review"),
		ptrOf(LanguageKorean), nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	content := PostContent{Title: "완성", Blocks: []Block{{Type: BlockText, Content: "생성 문장"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatalf("SetGeneratedContent: %v", err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}

	answered, err := svc.SaveDraft(ctx, alice, created.Slug, finalized.Title, finalized.Memo, nil, nil, nil,
		[]TemplateAnswer{{Label: "총평 별점", Text: "4.5점", Enabled: true}})
	if err != nil {
		t.Fatalf("answering a finalized post: %v", err)
	}
	if answered.Status != StatusFinalized {
		t.Errorf("status = %q, want %q", answered.Status, StatusFinalized)
	}
	if answered.ContentRevision != finalized.ContentRevision || answered.FinalizedRevision != finalized.FinalizedRevision {
		t.Errorf("revisions moved: %d/%d, want %d/%d",
			answered.ContentRevision, answered.FinalizedRevision,
			finalized.ContentRevision, finalized.FinalizedRevision)
	}
	if answered.MachineBaselineRevision != finalized.MachineBaselineRevision ||
		answered.MachineBaselineVoiceID != finalized.MachineBaselineVoiceID {
		t.Errorf("the machine baseline moved: %+v", answered)
	}
	// Learn eligibility is what a voice reassignment withdraws; an answer must not.
	if _, err := svc.LearningSnapshot(ctx, alice, created.Slug); err != nil {
		t.Errorf("an answer cost the post its learn eligibility: %v", err)
	}
	if len(answered.TemplateAnswers) != 1 || answered.TemplateAnswers[0].Text != "4.5점" {
		t.Errorf("answers = %+v", answered.TemplateAnswers)
	}
}
