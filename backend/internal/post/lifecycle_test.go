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
	if err := svc.SetGeneratedContent(context.Background(), alice, created.Slug, content); err != nil {
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
	if err := svc.SetGeneratedContent(context.Background(), alice, created.Slug, content); err != nil {
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
