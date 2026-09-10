package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestSignedVideoGateRefusesInlineOnlyModelEvenWhenAllObservationsAreReused(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		models := videoModels()
		info := models.infos[videoObserveRef]
		info.VideoDelivery = llm.VideoDelivery{InlineStaticVideo: true}
		models.infos[videoObserveRef] = info
		posts := &fakePosts{input: PostInput{Slug: "p", UserID: "alice", Voice: VoiceRef{ID: "voice"}, TargetLanguage: LanguageKorean, Images: []Image{clip("a.mp4")}, Observations: []Observation{{File: "a.mp4", Speech: "known"}}}}
		jobs := &fakeJobs{id: "job"}
		linker := &fakeLinker{}
		svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget)
		svc.SetVideoLinker(linker, 1)
		var selected *[]string
		if reuse {
			none := []string{}
			selected = &none
		}
		_, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "p", ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String(), ObserveFiles: selected})
		if !errors.Is(err, ErrVideoUnsupported) {
			t.Fatalf("start reuse=%v err=%v", reuse, err)
		}
		_, err = svc.SnapshotWriteInput(context.Background(), "alice", "p", videoObserveRef, nil, selected)
		if !errors.Is(err, ErrVideoUnsupported) {
			t.Fatalf("comparison reuse=%v err=%v", reuse, err)
		}
		if jobs.enqueues != 0 || len(models.calls) != 0 || len(linker.keys) != 0 {
			t.Fatal("refused post caused queue/model/storage work")
		}
		if !models.infos[videoObserveRef].VideoInput || !models.infos[videoObserveRef].VideoDelivery.InlineStaticVideo {
			t.Fatal("post refusal cleared clip readiness")
		}
	}
}
