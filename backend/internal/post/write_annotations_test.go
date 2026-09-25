package post

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

var (
	annotatedContent = PostContent{
		Title: "성수 카페 투어",
		Tags:  []string{"성수 카페", "라떼"},
		Blocks: []Block{
			{Type: BlockText, Content: "분위기 좋은 성수 카페를 찾았다."},
			{Type: BlockText, Content: "라떼가 맛있었다."},
		},
	}
	annotations = WriteAnnotations{
		Nouns: []string{"성수", "카페", "라떼"},
		Candidates: []ReplacementCandidate{
			{Surface: ReplacementSurfaceTitle, Index: 0, Source: "성수 카페", Phrases: []string{"성수동 카페"}},
			{Surface: ReplacementSurfaceTag, Index: 1, Source: "라떼", Phrases: []string{"카페라떼", "라떼 맛집"}},
			{Surface: ReplacementSurfaceBody, Index: 0, Source: "분위기 좋은", Phrases: []string{"감성 가득한"}},
		},
	}
)

// generatedWith writes the annotated content as a generation would and returns the post.
func generatedWith(t *testing.T, svc *Service, userID string, value *WriteAnnotations) Post {
	t.Helper()
	ctx := context.Background()
	created := mustCreatePost(t, svc, userID, "성수")
	if err := svc.SetGeneratedContent(ctx, userID, created.Slug, annotatedContent, LanguageKorean, value); err != nil {
		t.Fatal(err)
	}
	found, err := svc.Get(ctx, userID, created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// GEN-53, GEN-55: a generation's nouns and candidates are stored beside its content, and a
// later generation's replace them — none included, so stale ones never outlive their content.
func TestAGeneratedWriteStoresNounsAndCandidates(t *testing.T) {
	svc, _, _ := newTestService(t)
	found := generatedWith(t, svc, "alice", &annotations)
	if !reflect.DeepEqual(found.ContentNouns, annotations.Nouns) || !reflect.DeepEqual(found.ReplacementCandidates, annotations.Candidates) {
		t.Fatalf("stored nouns %v candidates %+v", found.ContentNouns, found.ReplacementCandidates)
	}

	// A later generation with no nouns and no candidates clears both.
	next := PostContent{Title: "다른 글", Blocks: []Block{{Type: BlockText, Content: "새로 쓴 글."}}}
	if err := svc.SetGeneratedContent(context.Background(), "alice", found.Slug, next, LanguageKorean, &WriteAnnotations{}); err != nil {
		t.Fatal(err)
	}
	cleared, err := svc.Get(context.Background(), "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.ContentNouns != nil || cleared.ReplacementCandidates != nil {
		t.Fatalf("a write with none kept nouns %v candidates %+v", cleared.ContentNouns, cleared.ReplacementCandidates)
	}
}

// GEN-55, R16: a revision has no nouns answer and no phrase list, so what the generation said
// stands, byte for byte, while the content moves on.
func TestARevisionKeepsNounsAndCandidates(t *testing.T) {
	svc, _, _ := newTestService(t)
	found := generatedWith(t, svc, "alice", &annotations)

	revised := annotatedContent
	revised.Blocks = []Block{{Type: BlockText, Content: "고쳐 쓴 문장."}}
	if err := svc.SetGeneratedContent(context.Background(), "alice", found.Slug, revised, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Get(context.Background(), "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentRevision != found.ContentRevision+1 {
		t.Fatalf("revision %d, want %d", after.ContentRevision, found.ContentRevision+1)
	}
	if !reflect.DeepEqual(after.ContentNouns, annotations.Nouns) || !reflect.DeepEqual(after.ReplacementCandidates, annotations.Candidates) {
		t.Fatalf("a revision changed nouns %v candidates %+v", after.ContentNouns, after.ReplacementCandidates)
	}
}

// POST-80: a manual edit that took no candidate never touches them; stale spans are the
// browser's to drop at render (GEN-53).
func TestAManualSaveKeepsNounsAndCandidates(t *testing.T) {
	svc, _, _ := newTestService(t)
	found := generatedWith(t, svc, "alice", &annotations)

	edited := annotatedContent
	edited.Title = "성수동 카페 투어"
	saved, err := svc.SaveContent(context.Background(), "alice", found.Slug, edited, found.ContentRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContentRevision != found.ContentRevision+1 {
		t.Fatalf("revision %d", saved.ContentRevision)
	}
	if !reflect.DeepEqual(saved.ContentNouns, annotations.Nouns) || !reflect.DeepEqual(saved.ReplacementCandidates, annotations.Candidates) {
		t.Fatalf("a manual save changed nouns %v candidates %+v", saved.ContentNouns, saved.ReplacementCandidates)
	}
}

// POST-79: a take spends exactly the candidates it names, in the same write as the content, and
// every other candidate keeps its mark.
func TestATakeSpendsExactlyThoseCandidates(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	found := generatedWith(t, svc, "alice", &annotations)

	edited := annotatedContent
	edited.Title = "성수동 카페 투어"
	saved, err := svc.SaveContent(ctx, "alice", found.Slug, edited, found.ContentRevision, []int{0, 2})
	if err != nil {
		t.Fatal(err)
	}
	only := []ReplacementCandidate{annotations.Candidates[1]}
	reread, err := svc.Get(ctx, "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]Post{"answer": saved, "re-read": reread} {
		if !reflect.DeepEqual(got.ReplacementCandidates, only) || got.ContentRevision != found.ContentRevision+1 || got.Status != StatusReview {
			t.Fatalf("%s: candidates %+v at revision %d, %s", name, got.ReplacementCandidates, got.ContentRevision, got.Status)
		}
	}

	// A later save that took nothing keeps what is left.
	edited.Title = "성수동 카페 산책"
	kept, err := svc.SaveContent(ctx, "alice", found.Slug, edited, reread.ContentRevision, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(kept.ReplacementCandidates, only) {
		t.Fatalf("a save with no takes changed the list: %+v", kept.ReplacementCandidates)
	}

	// A malformed take refuses the whole save.
	edited.Title = "성수동 카페 거리"
	for _, taken := range [][]int{{-1}, {len(only)}, {0, 0}} {
		if _, err := svc.SaveContent(ctx, "alice", found.Slug, edited, kept.ContentRevision, taken); !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("taken %v: err = %v, want ErrInvalidContent", taken, err)
		}
		after, err := svc.Get(ctx, "alice", found.Slug)
		if err != nil {
			t.Fatal(err)
		}
		if after.ContentRevision != kept.ContentRevision || !reflect.DeepEqual(after.ReplacementCandidates, only) {
			t.Fatalf("taken %v wrote something: revision %d, %+v", taken, after.ContentRevision, after.ReplacementCandidates)
		}
	}

	// The revision answers first.
	if _, err := svc.SaveContent(ctx, "alice", found.Slug, edited, found.ContentRevision, []int{0}); !errors.Is(err, ErrStaleContentRevision) {
		t.Fatalf("stale take: err = %v, want ErrStaleContentRevision", err)
	}

	// An identical save writes nothing, its takes included (POST-15).
	same, err := svc.SaveContent(ctx, "alice", found.Slug, *kept.Content, kept.ContentRevision, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if same.ContentRevision != kept.ContentRevision || !reflect.DeepEqual(same.ReplacementCandidates, only) {
		t.Fatalf("an identical save changed something: revision %d, %+v", same.ContentRevision, same.ReplacementCandidates)
	}
}

// A retried identical write is a no-op, and "identical" includes what the write said beside the
// content: the same content with different nouns or candidates is a new machine write.
func TestAnIdenticalRetriedWriteKeepsThemStored(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	found := generatedWith(t, svc, "alice", &annotations)

	retry := annotations
	if err := svc.SetGeneratedContent(ctx, "alice", found.Slug, annotatedContent, LanguageKorean, &retry); err != nil {
		t.Fatal(err)
	}
	same, err := svc.Get(ctx, "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if same.ContentRevision != found.ContentRevision || !reflect.DeepEqual(same.ReplacementCandidates, annotations.Candidates) {
		t.Fatalf("an identical retry moved the post: revision %d candidates %+v", same.ContentRevision, same.ReplacementCandidates)
	}

	// Each member on its own makes it a new write: the nouns alone, then the candidates, then one
	// candidate's phrases.
	rephrased := annotations.Candidates[0]
	rephrased.Phrases = []string{"성수 핫플"}
	for i, changed := range []WriteAnnotations{
		{Nouns: []string{"성수"}, Candidates: annotations.Candidates},
		{Nouns: []string{"성수"}, Candidates: annotations.Candidates[:1]},
		{Nouns: []string{"성수"}, Candidates: []ReplacementCandidate{rephrased}},
	} {
		if err := svc.SetGeneratedContent(ctx, "alice", found.Slug, annotatedContent, LanguageKorean, &changed); err != nil {
			t.Fatal(err)
		}
		rewritten, err := svc.Get(ctx, "alice", found.Slug)
		if err != nil {
			t.Fatal(err)
		}
		if rewritten.ContentRevision != found.ContentRevision+int64(i)+1 || !reflect.DeepEqual(rewritten.ContentNouns, changed.Nouns) || !reflect.DeepEqual(rewritten.ReplacementCandidates, changed.Candidates) {
			t.Fatalf("change %d was not a new write: revision %d nouns %v candidates %+v", i, rewritten.ContentRevision, rewritten.ContentNouns, rewritten.ReplacementCandidates)
		}
	}
}

// Post owns that a span names a surface it has and a position that can exist; the bounds are
// generation's (GEN-54). A refused write changes nothing.
func TestAnInvalidCandidateIsRefused(t *testing.T) {
	for name, candidate := range map[string]ReplacementCandidate{
		"an unknown surface": {Surface: "summary", Index: 0, Source: "성수", Phrases: []string{"성수동"}},
		"a negative index":   {Surface: ReplacementSurfaceTag, Index: -1, Source: "라떼", Phrases: []string{"카페라떼"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, _ := newTestService(t)
			ctx := context.Background()
			created := mustCreatePost(t, svc, "alice", "성수")
			err := svc.SetGeneratedContent(ctx, "alice", created.Slug, annotatedContent, LanguageKorean, &WriteAnnotations{Candidates: []ReplacementCandidate{candidate}})
			var invalid *InvalidContentError
			if !errors.Is(err, ErrInvalidContent) || !errors.As(err, &invalid) || invalid.Reason != "replacement candidate" {
				t.Fatalf("err = %v, want the replacement candidate refusal", err)
			}
			after, err := svc.Get(ctx, "alice", created.Slug)
			if err != nil {
				t.Fatal(err)
			}
			if after.Content != nil || after.ContentRevision != 0 {
				t.Fatalf("a refused write wrote revision %d", after.ContentRevision)
			}
		})
	}
}
