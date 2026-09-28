package guideline

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeVideoDirectory struct{ templates []TemplateRef }

func (f fakeVideoDirectory) VideoTemplates(context.Context, string) ([]TemplateRef, error) {
	return f.templates, nil
}

func clipService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	svc, store := newTestService(t, &fakeDirectory{templates: []TemplateRef{{ID: "post-tpl", Name: "리뷰"}}})
	svc.SetVideoTemplateDirectory(fakeVideoDirectory{templates: []TemplateRef{{ID: "vt", Name: "가게 소개"}}})
	return svc, store
}

// GUIDE-5: a clip guideline is global or scoped to video templates — validated against the video
// templates, never the post ones — and has no 분야 scope.
func TestAClipGuidelineIsScopedToVideoTemplatesAndNeverToFields(t *testing.T) {
	ctx := context.Background()
	svc, store := clipService(t)
	created, err := svc.Create(ctx, "alice", KindClip, "자막은 짧게", ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"vt"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if created.Kind != KindClip || !reflect.DeepEqual(created.Templates, []TemplateRef{{ID: "vt", Name: "가게 소개"}}) {
		t.Fatalf("created = %+v", created)
	}
	if store.inserted[0].Kind != KindClip {
		t.Fatalf("stored kind = %q", store.inserted[0].Kind)
	}
	if _, err := svc.Create(ctx, "alice", KindClip, "배경음 없이", ScopePatch{Scope: ScopeTemplates, TemplateIDs: []string{"post-tpl"}}, ""); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("a post template on a clip guideline: %v", err)
	}
	if _, err := svc.Create(ctx, "alice", KindClip, "가격 크게", ScopePatch{Scope: ScopeFields, Fields: []string{"cafe"}}, ""); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("a 분야 scope on a clip guideline: %v", err)
	}
	// A rescope follows the guideline's own kind.
	fields := ScopePatch{Scope: ScopeFields, Fields: []string{"cafe"}}
	if _, err := svc.Update(ctx, "alice", created.ID, Patch{Scope: &fields}); !errors.Is(err, ErrScopeShape) {
		t.Fatalf("rescoping a clip guideline to 분야: %v", err)
	}
	// And the list is per kind.
	if clips, _ := svc.List(ctx, "alice", KindClip); len(clips) != 1 {
		t.Fatalf("clip list = %+v", clips)
	}
	if posts, _ := svc.List(ctx, "alice", KindPost); len(posts) != 0 {
		t.Fatalf("post list = %+v", posts)
	}
}

// GUIDE-7: a candidate records its kind and names its source by that kind.
func TestACandidateNamesItsSourceByKind(t *testing.T) {
	ctx := context.Background()
	svc, store := clipService(t)
	if err := svc.RecordCandidate(ctx, "alice", KindClip, "project-1", " 자막을 짧게 "); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordCandidate(ctx, "alice", KindPost, "post-1", "짧게"); err != nil {
		t.Fatal(err)
	}
	if got := store.candidates[0]; got.Kind != KindClip || got.ClipID != "project-1" || got.PostSlug != "" || got.Text != "자막을 짧게" {
		t.Fatalf("clip candidate = %+v", got)
	}
	if got := store.candidates[1]; got.Kind != KindPost || got.PostSlug != "post-1" || got.ClipID != "" {
		t.Fatalf("post candidate = %+v", got)
	}
	if err := svc.DetachCandidateClip(ctx, "alice", "project-1"); err != nil || !reflect.DeepEqual(store.detachedClips, []string{"project-1"}) {
		t.Fatalf("detach = %v, %v", store.detachedClips, err)
	}
	store.pending = []Candidate{{UserID: "alice", Kind: KindClip, Text: "자막을 짧게"}, {UserID: "alice", Text: "짧게"}}
	clips, _, err := svc.ListCandidates(ctx, "alice", KindClip)
	if err != nil || len(clips) != 1 || clips[0].Text != "자막을 짧게" {
		t.Fatalf("clip candidates = %+v, %v", clips, err)
	}
}

// GUIDE-14: a clip is given its enabled clip 기본 지침, then its owner texts — global, then those
// linked to its video template.
func TestAClipResolvesItsDefaultsThenItsOwnerTexts(t *testing.T) {
	ctx := context.Background()
	svc, store := clipService(t)
	store.clipTexts = []string{"자막은 짧게", "가격은 크게"}
	vt := "vt"
	got, err := svc.ForPrompt(ctx, "alice", KindClip, &vt, nil, LanguageKorean, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Owner, store.clipTexts) || store.askedVideoTemplate != "vt" {
		t.Fatalf("owner texts = %v (asked %q)", got.Owner, store.askedVideoTemplate)
	}
	if len(got.Defaults) != len(Defaults(KindClip)) {
		t.Fatalf("clip defaults = %d, want all %d", len(got.Defaults), len(Defaults(KindClip)))
	}
	if store.askedTemplate != "" {
		t.Fatal("a clip resolution asked for post guidelines")
	}
}
