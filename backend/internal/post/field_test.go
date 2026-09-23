package post

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func fieldPtr(id string) *string { return &id }

// The 분야 is presence-aware like the template: absent keeps it, a present "" clears it to 없음,
// and a present id assigns it.
func TestSaveDraftAssignsClearsOrKeepsTheField(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "제주")
	if created.Field != "" {
		t.Fatalf("a post created without a 분야 has %q", created.Field)
	}
	for _, step := range []struct {
		name  string
		field *string
		want  string
	}{
		{"assigned", fieldPtr("restaurant"), "restaurant"},
		{"kept when absent", nil, "restaurant"},
		{"replaced", fieldPtr("cafe"), "cafe"},
		{"cleared to none", fieldPtr(""), ""},
		{"kept as none", nil, ""},
	} {
		saved, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "제주 " + step.name, Field: step.field})
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if saved.Field != step.want || store.posts[created.Slug].Field != step.want {
			t.Fatalf("%s: field = %q (stored %q), want %q", step.name, saved.Field, store.posts[created.Slug].Field, step.want)
		}
	}
}

// A create from /posts/new mints the post with its 분야 in the same insert: nothing writes the
// field afterwards.
func TestACreateCarriesItsField(t *testing.T) {
	svc, store, _ := newTestService(t)
	voiceID, language := aliceVoice, LanguageKorean
	created, err := svc.SaveDraft(context.Background(), alice, DraftSave{
		Title: "제주", VoiceID: &voiceID, TargetLanguage: &language, Field: fieldPtr("cafe"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Field != "cafe" || store.posts[created.Slug].Field != "cafe" {
		t.Fatalf("created field = %q, stored %q", created.Field, store.posts[created.Slug].Field)
	}
	if store.fieldAssignments != 0 {
		t.Fatalf("the create wrote the field %d more times after its insert", store.fieldAssignments)
	}
}

// An id the product does not list mints nothing on a create, and on an update it changes
// nothing of the request: not the title, memo, voice, template, answers or 분야.
func TestAnUnknownFieldIsRefusedAndNothingLands(t *testing.T) {
	ctx := context.Background()
	t.Run("create", func(t *testing.T) {
		svc, store, _ := newTestService(t)
		voiceID, language := aliceVoice, LanguageKorean
		if _, err := svc.SaveDraft(ctx, alice, DraftSave{
			Title: "제주", VoiceID: &voiceID, TargetLanguage: &language, Field: fieldPtr("space_travel"),
		}); !errors.Is(err, ErrFieldNotFound) {
			t.Fatalf("err = %v, want ErrFieldNotFound", err)
		}
		if len(store.posts) != 0 {
			t.Fatalf("a refused create minted %d posts", len(store.posts))
		}
	})
	t.Run("update", func(t *testing.T) {
		svc, store, _ := newTestService(t)
		svc.SetTemplateDirectory(testTemplates())
		svc.answerLabelMax, svc.answerValueMax = 40, 500
		created := mustCreatePost(t, svc, alice, "제주")
		if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "제주", Field: fieldPtr("restaurant")}); err != nil {
			t.Fatal(err)
		}
		before, answers := store.posts[created.Slug], len(store.answers[created.Slug])
		review, template := aliceReview, "template-review"
		_, err := svc.SaveDraft(ctx, alice, DraftSave{
			Slug: created.Slug, Title: "새 제목", Memo: "새 메모", VoiceID: &review, TemplateID: &template,
			Field: fieldPtr("space_travel"), Answers: []TemplateAnswer{{Label: "총평", Text: "맑았다", Enabled: true}},
		})
		if !errors.Is(err, ErrFieldNotFound) {
			t.Fatalf("err = %v, want ErrFieldNotFound", err)
		}
		if after := store.posts[created.Slug]; !reflect.DeepEqual(after, before) || len(store.answers[created.Slug]) != answers {
			t.Fatalf("a refused update changed the post:\nbefore %+v\nafter  %+v", before, after)
		}
	})
}

// POST-82: the 분야 is an input of the next run, not an edit of the post. Saving it on a
// finalized post leaves the status, the revisions, the baseline and the finalization alone, and
// the post stays learnable; an equal 분야 names no write at all.
func TestAFieldSaveIsAnOptionSave(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	before := store.posts[finalized.Slug]
	saved, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo, Field: fieldPtr("restaurant")})
	if err != nil {
		t.Fatal(err)
	}
	after := store.posts[finalized.Slug]
	if saved.Field != "restaurant" || after.Status != StatusFinalized || after.ContentRevision != before.ContentRevision ||
		after.MachineBaselineRevision != before.MachineBaselineRevision || after.FinalizedRevision != before.FinalizedRevision ||
		!after.FinalizedAt.Equal(*before.FinalizedAt) {
		t.Fatalf("a 분야 save moved the lifecycle: before %+v\nafter %+v", before, after)
	}
	if _, err := svc.LearningSnapshot(ctx, alice, finalized.Slug); err != nil {
		t.Fatalf("the post stopped being learnable: %v", err)
	}
	calls := store.fieldAssignments
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Memo: before.Memo, Field: fieldPtr("restaurant")}); err != nil {
		t.Fatal(err)
	}
	if store.fieldAssignments != calls {
		t.Fatal("an equal 분야 was written again")
	}
}

func TestAFieldSaveOnAPublishedPostIsLocked(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	before := store.posts[finalized.Slug]
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: finalized.Slug, Title: before.Title, Field: fieldPtr("cafe")}); !errors.Is(err, ErrPostPublished) {
		t.Fatalf("err = %v, want ErrPostPublished", err)
	}
	if after := store.posts[finalized.Slug]; !reflect.DeepEqual(after, before) {
		t.Fatalf("the lock let the 분야 through: %+v", after)
	}
}

func TestNewServiceRequiresAFieldDirectory(t *testing.T) {
	deps := testDeps()
	deps.Fields = nil
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "fields") {
			t.Fatalf("panic = %v, want a loud 분야-directory refusal", r)
		}
	}()
	NewService(newFakeStore(), newFakeBlobs(), Limits{}, deps)
}
