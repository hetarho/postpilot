package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
)

// seedPost writes the owning row every answer needs: the table's foreign key is what makes
// an answer belong to a post rather than float beside it.
func seedAnswerPost(t *testing.T, s interface {
	CreatePost(context.Context, post.Post) error
}, slug, userID string) {
	t.Helper()
	if err := s.CreatePost(context.Background(), post.Post{
		Slug: slug, UserID: userID, VoiceID: "voice-" + userID, Title: "제주", Memo: "갔다",
		Status: post.StatusDraft, TargetLanguage: post.LanguageKorean,
		CreatedAt: testNow, UpdatedAt: testNow,
	}); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
}

func TestTemplateAnswersUpsertAndList(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedAnswerPost(t, s, "20260301-jeju", "alice")

	if err := s.UpsertTemplateAnswers(ctx, "20260301-jeju", []post.TemplateAnswer{
		{Label: "총평 별점", Text: "4.5점", Enabled: true},
		{Label: "방문일", Text: "2026-03-01", Enabled: false},
	}, testNow); err != nil {
		t.Fatalf("UpsertTemplateAnswers: %v", err)
	}

	got, err := s.ListTemplateAnswers(ctx, "20260301-jeju")
	if err != nil {
		t.Fatalf("ListTemplateAnswers: %v", err)
	}
	// Ordered by label, and `enabled` survives as the boolean it went in as.
	want := []post.TemplateAnswer{
		{Label: "방문일", Text: "2026-03-01", Enabled: false},
		{Label: "총평 별점", Text: "4.5점", Enabled: true},
	}
	if len(got) != len(want) {
		t.Fatalf("answers = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("answer %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	// A second write of one label replaces that row and leaves the other alone: this is the
	// per-field autosave POST-62 asks for, not a replacement of the set.
	if err := s.UpsertTemplateAnswers(ctx, "20260301-jeju", []post.TemplateAnswer{
		{Label: "총평 별점", Text: "5점", Enabled: true},
	}, testNow.Add(time.Minute)); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err = s.ListTemplateAnswers(ctx, "20260301-jeju")
	if err != nil {
		t.Fatalf("ListTemplateAnswers: %v", err)
	}
	if len(got) != 2 || got[0].Text != "2026-03-01" || got[1].Text != "5점" {
		t.Fatalf("after the second upsert answers = %+v", got)
	}

	// An empty request writes nothing rather than clearing: absent means "no answer in this
	// save", which is what ordinary autosave sends.
	if err := s.UpsertTemplateAnswers(ctx, "20260301-jeju", nil, testNow); err != nil {
		t.Fatalf("empty upsert: %v", err)
	}
	if got, _ = s.ListTemplateAnswers(ctx, "20260301-jeju"); len(got) != 2 {
		t.Fatalf("an empty upsert changed the set: %+v", got)
	}
}

func TestTemplateAnswersRequireTheirPostAndCascade(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// No post, no answer: the row has nothing to belong to.
	if err := s.UpsertTemplateAnswers(ctx, "missing", []post.TemplateAnswer{
		{Label: "총평", Text: "x", Enabled: true},
	}, testNow); err == nil {
		t.Error("an answer for a post that does not exist was accepted")
	}

	seedAnswerPost(t, s, "20260301-jeju", "alice")
	if err := s.UpsertTemplateAnswers(ctx, "20260301-jeju", []post.TemplateAnswer{
		{Label: "총평", Text: "x", Enabled: true},
	}, testNow); err != nil {
		t.Fatalf("UpsertTemplateAnswers: %v", err)
	}
	if deleted, err := s.DeletePost(ctx, "20260301-jeju", "alice"); err != nil || !deleted {
		t.Fatalf("DeletePost: %v %v", deleted, err)
	}
	// Deleting the POST takes its answers with it — they are part of that aggregate.
	got, err := s.ListTemplateAnswers(ctx, "20260301-jeju")
	if err != nil {
		t.Fatalf("ListTemplateAnswers after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("answers survived their post: %+v", got)
	}
}
