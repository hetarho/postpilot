package post

import (
	"context"
	"errors"
	"testing"
)

// fakeTemplates is the directory port. Unlike voices there are no tombstones: a deleted
// template is simply not listed.
type fakeTemplates map[string][]TemplateRef

func (f fakeTemplates) Templates(_ context.Context, userID string) ([]TemplateRef, error) {
	return f[userID], nil
}

func testTemplates() fakeTemplates {
	return fakeTemplates{
		alice: {{ID: "template-review", Name: "정보성 리뷰"}, {ID: "template-diary", Name: "일기"}},
		bob:   {{ID: "template-bob", Name: "밥의 템플릿"}},
	}
}

func newTemplateAwareService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	svc, store, _ := newTestService(t)
	svc.SetTemplateDirectory(testTemplates())
	return svc, store
}

// Plan 11 A3: the field has three meanings, and only presence tells them apart.
func TestSaveDraftAssignsClearsOrPreservesTheTemplate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTemplateAwareService(t)
	voiceID := defaultVoiceFor(alice)
	review := "template-review"
	blank := ""
	language := LanguageKorean

	created, err := svc.SaveDraft(ctx, alice, "", "Jeju", "memo", &voiceID, &review, &language)
	if err != nil || created.TemplateID != review {
		t.Fatalf("create with a template: %+v err=%v", created, err)
	}
	if created.Template.Name != "정보성 리뷰" {
		t.Fatalf("create did not project the template name: %+v", created.Template)
	}

	// Absent is what ordinary autosave sends. It must not disturb the assignment.
	kept, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "memo 2", nil, nil, nil)
	if err != nil || kept.TemplateID != review {
		t.Fatalf("absent template changed the assignment: %+v err=%v", kept, err)
	}

	// A present empty string is the explicit 없음.
	cleared, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "memo 3", nil, &blank, nil)
	if err != nil || cleared.TemplateID != "" || cleared.Template != (TemplateRef{}) {
		t.Fatalf("clear failed: %+v err=%v", cleared, err)
	}

	// And a present id assigns again, from 없음, with no voice in the request.
	reassigned, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "memo 4", nil, &review, nil)
	if err != nil || reassigned.TemplateID != review {
		t.Fatalf("reassign failed: %+v err=%v", reassigned, err)
	}
}

// Plan 11 A3: an unknown or foreign id is refused and NOTHING else in the request lands —
// not the title, not the memo, not a create.
func TestSaveDraftRejectsAnUnknownOrForeignTemplateAndAppliesNothingElse(t *testing.T) {
	ctx := context.Background()
	svc, store := newTemplateAwareService(t)
	voiceID := defaultVoiceFor(alice)
	created := mustCreatePost(t, svc, alice, "Jeju")

	for name, id := range map[string]string{"unknown": "template-nobody", "foreign": "template-bob"} {
		t.Run(name, func(t *testing.T) {
			bad := id
			if _, err := svc.SaveDraft(ctx, alice, created.Slug, "새 제목", "새 메모", nil, &bad, nil); !errors.Is(err, ErrTemplateNotFound) {
				t.Fatalf("SaveDraft = %v, want ErrTemplateNotFound", err)
			}
			current := store.posts[created.Slug]
			if current.Title != "Jeju" || current.Memo != "" || current.TemplateID != "" {
				t.Fatalf("a refused template let the rest of the request through: %+v", current)
			}
			language := LanguageKorean
			if _, err := svc.SaveDraft(ctx, alice, "", "새 글", "", &voiceID, &bad, &language); !errors.Is(err, ErrTemplateNotFound) {
				t.Fatalf("create = %v, want ErrTemplateNotFound", err)
			}
			if len(store.posts) != 1 {
				t.Fatalf("a refused create minted a post: %+v", store.posts)
			}
		})
	}
}

// Plan 11 A3: a template is never learned from, so assignment costs the post nothing — unlike
// a voice reassignment, which withdraws the machine baseline.
func TestAssigningATemplateTouchesNoContentOrFinalizationState(t *testing.T) {
	ctx := context.Background()
	svc, store := newTemplateAwareService(t)
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
	after, err := svc.SaveDraft(ctx, alice, created.Slug, before.Title, before.Memo, nil, &review, nil)
	if err != nil {
		t.Fatalf("assignment refused on a finalized post: %v", err)
	}
	if after.TemplateID != review {
		t.Fatalf("assignment did not land: %+v", after)
	}
	stored := store.posts[created.Slug]
	if stored.Status != before.Status ||
		stored.ContentRevision != before.ContentRevision ||
		stored.MachineBaselineRevision != before.MachineBaselineRevision ||
		stored.MachineBaselineVoiceID != before.MachineBaselineVoiceID ||
		stored.FinalizedRevision != before.FinalizedRevision {
		t.Fatalf("assignment disturbed lifecycle state:\nbefore=%+v\nafter=%+v", before, stored)
	}
}

// Plan 11 A12: the list badge needs the name, and a template deleted since is projected as
// absent rather than as a dangling id with no label.
func TestGetAndListProjectTheTemplateName(t *testing.T) {
	ctx := context.Background()
	svc, store := newTemplateAwareService(t)
	review := "template-review"
	voiceID := defaultVoiceFor(alice)
	language := LanguageKorean
	created, err := svc.SaveDraft(ctx, alice, "", "Jeju", "", &voiceID, &review, &language)
	if err != nil {
		t.Fatal(err)
	}

	listed, err := svc.List(ctx, alice)
	if err != nil || len(listed) != 1 || listed[0].Template.Name != "정보성 리뷰" {
		t.Fatalf("list projection = %+v err=%v", listed, err)
	}

	// A row whose template is gone still reads: the id is projected with no name, exactly as
	// an unlisted voice is. The database detaches on delete, so this is a transient state.
	stale := store.posts[created.Slug]
	stale.TemplateID = "template-gone"
	store.posts[created.Slug] = stale
	found, err := svc.Get(ctx, alice, created.Slug)
	if err != nil || found.Template.ID != "template-gone" || found.Template.Name != "" {
		t.Fatalf("stale projection = %+v err=%v", found.Template, err)
	}
}

// A service with no template directory still serves posts: the template is optional, so an
// unwired process degrades to "no template" rather than failing every read.
func TestWithoutATemplateDirectoryPostsStillWorkAndAssignmentIsRefused(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Jeju")

	if found, err := svc.Get(ctx, alice, created.Slug); err != nil || found.Template != (TemplateRef{}) {
		t.Fatalf("get without a directory: %+v err=%v", found.Template, err)
	}
	review := "template-review"
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "", nil, &review, nil); err == nil {
		t.Fatal("assignment succeeded without a template directory")
	}
}
