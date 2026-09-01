package post

import (
	"context"
	"errors"
	"testing"
)

func TestFinalizedLifecycleKeepsIdenticalSavesAndDemotesChangedContent(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Final")
	content := PostContent{Title: "완성", Blocks: []Block{{Type: BlockText, Content: "생성 문장"}}}
	if err := svc.SetGeneratedContent(context.Background(), alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	target := 900
	withOption, err := svc.SaveGenerationOptions(context.Background(), alice, created.Slug, &target)
	if err != nil || withOption.ContentRevision != 1 || withOption.Status != StatusReview {
		t.Fatalf("option changed lifecycle: %+v err=%v", withOption, err)
	}
	finalized, err := svc.Finalize(context.Background(), alice, created.Slug, 1)
	if err != nil || finalized.Status != StatusFinalized || finalized.FinalizedRevision != 1 {
		t.Fatalf("finalize = %+v err=%v", finalized, err)
	}
	unchanged, err := svc.SaveContent(context.Background(), alice, created.Slug, content, 1)
	if err != nil || unchanged.Status != StatusFinalized || unchanged.ContentRevision != 1 {
		t.Fatalf("identical save = %+v err=%v", unchanged, err)
	}
	changed := PostContent{Title: "직접 수정", Blocks: []Block{{Type: BlockText, Content: "내 문장"}}}
	review, err := svc.SaveContent(context.Background(), alice, created.Slug, changed, 1)
	if err != nil || review.Status != StatusReview || review.ContentRevision != 2 || review.FinalizedRevision != 0 {
		t.Fatalf("changed save = %+v err=%v", review, err)
	}
	if _, err := svc.LearningSnapshot(context.Background(), alice, created.Slug); !errors.Is(err, ErrPostNotFinalized) {
		t.Fatalf("learning snapshot before re-finalize = %v", err)
	}
}

func TestFinalizeIsRevisionCheckedOwnedAndIdempotent(t *testing.T) {
	svc, _, _ := newTestService(t)
	created := mustCreatePost(t, svc, alice, "Final")
	if _, err := svc.Finalize(context.Background(), alice, created.Slug, 0); !errors.Is(err, ErrNoMachineBaseline) {
		t.Fatalf("finalize without baseline = %v", err)
	}
	content := PostContent{Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(context.Background(), alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(context.Background(), bob, created.Slug, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign finalize = %v", err)
	}
	if _, err := svc.Finalize(context.Background(), alice, created.Slug, 0); !errors.Is(err, ErrStaleContentRevision) {
		t.Fatalf("stale finalize = %v", err)
	}
	first, err := svc.Finalize(context.Background(), alice, created.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Finalize(context.Background(), alice, created.Slug, 1)
	if err != nil || second.FinalizedAt == nil || !second.FinalizedAt.Equal(*first.FinalizedAt) {
		t.Fatalf("idempotent finalize = %+v err=%v", second, err)
	}
}

func TestFinalizeAllowsCrossLanguageContentAndPreservesProvenance(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	voiceID := aliceVoice // Korean source voice.
	target := LanguageEnglish
	created, err := svc.SaveDraft(ctx, alice, "", "English target", "", &voiceID, nil, &target)
	if err != nil {
		t.Fatal(err)
	}
	content := PostContent{Title: "An English post", Blocks: []Block{{Type: BlockText, Content: "English content remains publishable under a Korean source voice."}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageEnglish); err != nil {
		t.Fatal(err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil || finalized.Status != StatusFinalized || finalized.TargetLanguage != LanguageEnglish || finalized.ContentLanguage == nil || *finalized.ContentLanguage != LanguageEnglish || finalized.Voice.SourceLanguage != LanguageKorean {
		t.Fatalf("cross-language finalize = %#v, err=%v", finalized, err)
	}
	learning, err := svc.LearningSnapshot(ctx, alice, created.Slug)
	if err != nil || learning.ContentLanguage != LanguageEnglish || learning.VoiceSourceLanguage != LanguageKorean {
		t.Fatalf("learning language projection = %#v, err=%v", learning, err)
	}
	snapshot, err := svc.PublishingSnapshot(ctx, alice, created.Slug)
	if err != nil || snapshot.TargetLanguage != LanguageEnglish || snapshot.ContentLanguage != LanguageEnglish || snapshot.VoiceSourceLanguage != LanguageKorean {
		t.Fatalf("cross-language publishing provenance = %#v, err=%v", snapshot, err)
	}
}

// A8/A9: 확정 copies the confirmed content title into posts.title, and nothing else moves with it.
func TestFinalizeCopiesContentTitleIntoThePost(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "가제")
	content := PostContent{Title: "  모델이 지은 제목  ", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	// The list read falls back to content.title only while the working title is empty, so a
	// draft is still listed under its 가제 before the confirmation.
	before, err := svc.List(ctx, alice)
	if err != nil || len(before) != 1 || before[0].Title != "가제" {
		t.Fatalf("list before finalize = %+v err=%v", before, err)
	}

	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Title != "모델이 지은 제목" {
		t.Fatalf("finalized title = %q", finalized.Title)
	}
	if finalized.Slug != created.Slug {
		t.Fatalf("slug moved: %q -> %q", created.Slug, finalized.Slug)
	}
	if finalized.ContentRevision != 1 || finalized.MachineBaselineRevision != 1 {
		t.Fatalf("revision moved: %+v", finalized)
	}
	after, err := svc.List(ctx, alice)
	if err != nil || len(after) != 1 || after[0].Title != "모델이 지은 제목" {
		t.Fatalf("list after finalize = %+v err=%v", after, err)
	}
	// The copy is not a content save: it starts no job and calls no provider, which is what
	// keeping it inside the one guarded UPDATE buys.
	stored, err := store.GetPost(ctx, created.Slug)
	if err != nil || stored.Title != "모델이 지은 제목" || stored.Content == nil || stored.Content.Title != "  모델이 지은 제목  " {
		t.Fatalf("stored post = %+v err=%v", stored, err)
	}
}

func TestFinalizeLeavesTheWorkingTitleWhenTheContentHasNone(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "가제")
	content := PostContent{Title: "   ", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil || finalized.Title != "가제" {
		t.Fatalf("untitled finalize = %+v err=%v", finalized, err)
	}
}

// The already-finalized early return happens BEFORE the store call, so a second confirmation of
// the same revision cannot re-copy over a title the user has edited since.
func TestSecondFinalizeOfTheSameRevisionDoesNotRewriteTheTitle(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "가제")
	content := PostContent{Title: "모델 제목", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(ctx, alice, created.Slug, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, "사람이 고친 제목", "", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Finalize(ctx, alice, created.Slug, 1)
	if err != nil || again.Title != "사람이 고친 제목" {
		t.Fatalf("re-finalize = %+v err=%v", again, err)
	}
	stored, err := store.GetPost(ctx, created.Slug)
	if err != nil || stored.Title != "사람이 고친 제목" {
		t.Fatalf("stored title = %+v err=%v", stored, err)
	}
}
