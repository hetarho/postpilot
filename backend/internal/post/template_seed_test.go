package post

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/platform/config"
)

func number(value int) *int { return &value }

// seedingTemplates is the directory with opinions: one template decides both numbers, one
// only the length, and one has no opinion at all.
func seedingTemplates() fakeTemplates {
	return fakeTemplates{
		alice: {
			{ID: "template-review", Name: "정보성 리뷰", TargetLength: number(1800), TagCount: number(7)},
			{ID: "template-long", Name: "긴 글", TargetLength: number(2500)},
			{ID: "template-quiet", Name: "의견 없음"},
		},
		bob: {{ID: "template-bob", Name: "밥의 템플릿", TargetLength: number(900)}},
	}
}

func newSeedingService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	svc, store, _ := newTestService(t)
	svc.SetTemplateDirectory(seedingTemplates())
	return svc, store
}

// TEMPLATE-48: assigning a template copies the numbers it HAS set onto the post's own
// options, and leaves the post's value alone for a number it has no opinion about.
func TestAssigningATemplateSeedsThePostsGenerationOptions(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeedingService(t)
	created := mustCreatePost(t, svc, alice, "Jeju")

	review := "template-review"
	seeded, err := svc.SaveDraft(ctx, alice, created.Slug, created.Title, created.Memo, nil, &review, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if seeded.TargetLength == nil || *seeded.TargetLength != 1800 || seeded.TagCount != 7 {
		t.Fatalf("the template's numbers were not seeded: length=%v tags=%d", seeded.TargetLength, seeded.TagCount)
	}

	// The author types over the seeded values, and the template is not consulted again.
	typed := 1200
	if _, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, &typed, number(3)); err != nil {
		t.Fatal(err)
	}
	kept, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju 2", created.Memo, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if kept.TargetLength == nil || *kept.TargetLength != typed || kept.TagCount != 3 {
		t.Fatalf("an ordinary save re-seeded the options: length=%v tags=%d", kept.TargetLength, kept.TagCount)
	}

	// A template with an opinion about one number only leaves the other as it stands.
	long := "template-long"
	partial, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju 2", created.Memo, nil, &long, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if partial.TargetLength == nil || *partial.TargetLength != 2500 || partial.TagCount != 3 {
		t.Fatalf("a partial opinion overwrote the other number: length=%v tags=%d", partial.TargetLength, partial.TagCount)
	}

	// A template with no opinion at all changes neither.
	quiet := "template-quiet"
	untouched, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju 2", created.Memo, nil, &quiet, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if untouched.TargetLength == nil || *untouched.TargetLength != 2500 || untouched.TagCount != 3 {
		t.Fatalf("a template with no opinion seeded something: length=%v tags=%d", untouched.TargetLength, untouched.TagCount)
	}
}

// Only an assignment seeds. Clearing to 없음 keeps what the post last received, an absent
// field is an ordinary autosave, and re-sending the id already stored is not an assignment.
func TestOnlyAnAssignmentSeeds(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeedingService(t)
	created := mustCreatePost(t, svc, alice, "Jeju")

	review := "template-review"
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, created.Title, created.Memo, nil, &review, nil, nil); err != nil {
		t.Fatal(err)
	}
	typed := 1200
	if _, err := svc.SaveGenerationOptions(ctx, alice, created.Slug, &typed, number(3)); err != nil {
		t.Fatal(err)
	}

	// The same id again: nothing to assign, so nothing to seed.
	same, err := svc.SaveDraft(ctx, alice, created.Slug, created.Title, created.Memo, nil, &review, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if same.TargetLength == nil || *same.TargetLength != typed || same.TagCount != 3 {
		t.Fatalf("re-sending the stored id re-seeded: length=%v tags=%d", same.TargetLength, same.TagCount)
	}

	// 없음 clears the assignment and keeps the numbers the post last received.
	blank := ""
	cleared, err := svc.SaveDraft(ctx, alice, created.Slug, created.Title, created.Memo, nil, &blank, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.TemplateID != "" {
		t.Fatalf("the assignment survived: %+v", cleared)
	}
	if cleared.TargetLength == nil || *cleared.TargetLength != typed || cleared.TagCount != 3 {
		t.Fatalf("clearing the template moved a number: length=%v tags=%d", cleared.TargetLength, cleared.TagCount)
	}
}

// A refused assignment writes nothing at all — the numbers included.
func TestARefusedAssignmentSeedsNothing(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeedingService(t)
	created := mustCreatePost(t, svc, alice, "Jeju")

	foreign := "template-bob"
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, created.Title, created.Memo, nil, &foreign, nil, nil); err == nil {
		t.Fatal("a foreign template was accepted")
	}
	after, err := svc.Get(ctx, alice, created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if after.TargetLength != nil || after.TagCount != config.PostTagCountDefault {
		t.Fatalf("a refused assignment seeded: length=%v tags=%d", after.TargetLength, after.TagCount)
	}
}

// Creating a post with a template seeds it the same way, in the insert itself.
func TestCreatingAPostWithATemplateSeedsIt(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSeedingService(t)
	voiceID := defaultVoiceFor(alice)
	language := LanguageKorean

	review := "template-review"
	created, err := svc.SaveDraft(ctx, alice, "", "Jeju", "memo", &voiceID, &review, &language, nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.TargetLength == nil || *created.TargetLength != 1800 || created.TagCount != 7 {
		t.Fatalf("the create did not seed: length=%v tags=%d", created.TargetLength, created.TagCount)
	}

	// A create naming a template with no opinion stores no number at all: natural length and
	// the configured default count, exactly as a create with no template does.
	quiet := "template-quiet"
	plain, err := svc.SaveDraft(ctx, alice, "", "Busan", "memo", &voiceID, &quiet, &language, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain.TargetLength != nil || plain.TagCount != config.PostTagCountDefault {
		t.Fatalf("a create invented a number: length=%v tags=%d", plain.TargetLength, plain.TagCount)
	}
}

// Seeding is an option change: it advances no revision, moves no baseline and demotes
// nothing (POST-69, POST-20, POST-63).
func TestSeedingAdvancesNoRevisionAndMovesNoBaseline(t *testing.T) {
	ctx := context.Background()
	svc, store := newSeedingService(t)
	created := mustCreatePost(t, svc, alice, "Jeju")

	generated := PostContent{Title: "생성됨", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, generated, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	baseline := store.posts[created.Slug]
	if _, err := svc.Finalize(ctx, alice, created.Slug, baseline.ContentRevision); err != nil {
		t.Fatal(err)
	}
	before := store.posts[created.Slug]

	review := "template-review"
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, before.Title, before.Memo, nil, &review, nil, nil); err != nil {
		t.Fatal(err)
	}
	after := store.posts[created.Slug]
	if after.TargetLength == nil || *after.TargetLength != 1800 || after.TagCount != 7 {
		t.Fatalf("the seed did not land: length=%v tags=%d", after.TargetLength, after.TagCount)
	}
	if after.Status != before.Status ||
		after.ContentRevision != before.ContentRevision ||
		after.MachineBaselineRevision != before.MachineBaselineRevision ||
		after.MachineBaselineVoiceID != before.MachineBaselineVoiceID ||
		after.FinalizedRevision != before.FinalizedRevision {
		t.Fatalf("seeding disturbed lifecycle state:\nbefore=%+v\nafter=%+v", before, after)
	}
}
