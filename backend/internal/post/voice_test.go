package post

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePendingExperiments map[string]string

func (f fakePendingExperiments) PendingForPost(_ context.Context, _, slug string) (string, error) {
	return f[slug], nil
}

// Plan 10 A4: a create names exactly one owned active voice, and the server never picks one.
func TestCreateRequiresAnOwnedActiveVoice(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	empty := ""
	unknown := "voice-nobody"
	foreign := bobVoice
	deleted := aliceDeleted
	language := LanguageKorean
	for name, tc := range map[string]struct {
		voice *string
		want  error
	}{
		"absent":  {nil, ErrVoiceRequired},
		"empty":   {&empty, ErrVoiceRequired},
		"unknown": {&unknown, ErrVoiceNotFound},
		"foreign": {&foreign, ErrVoiceNotFound},
		"deleted": {&deleted, ErrVoiceDeleted},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.SaveDraft(ctx, alice, "", "Jeju", "", tc.voice, nil, &language); !errors.Is(err, tc.want) {
				t.Fatalf("SaveDraft = %v, want %v", err, tc.want)
			}
		})
	}
	if len(store.posts) != 0 {
		t.Fatalf("a rejected create minted a post: %+v", store.posts)
	}
	// Without a wired directory the create fails closed rather than trusting the id.
	bare := NewService(newFakeStore(), newFakeBlobs(), time.Minute, time.Minute, testMaxBytes, 30)
	voiceID := aliceVoice
	if _, err := bare.SaveDraft(ctx, alice, "", "Jeju", "", &voiceID, nil, &language); err == nil {
		t.Fatal("create succeeded without a voice directory")
	}
}

// Plan 10 A5/A7: read models carry the voice's name and tombstone state, so a post whose
// voice was deleted still renders and exports.
func TestGetAndListProjectTheVoiceIncludingTombstones(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Jeju")
	gone := store.posts[created.Slug]
	gone.VoiceID = aliceDeleted
	store.posts[created.Slug] = gone

	found, err := svc.Get(ctx, alice, created.Slug)
	if err != nil || found.Voice.ID != aliceDeleted || found.Voice.Name != "옛 말투" || !found.Voice.Deleted {
		t.Fatalf("tombstone projection = %+v err=%v", found.Voice, err)
	}
	listed, err := svc.List(ctx, alice)
	if err != nil || len(listed) != 1 || listed[0].Voice.Name != "옛 말투" || !listed[0].Voice.Deleted {
		t.Fatalf("list projection = %+v err=%v", listed, err)
	}
	snapshot, err := svc.AttachedImages(ctx, alice, created.Slug)
	if err != nil || !snapshot.Voice.Deleted {
		t.Fatalf("generation snapshot projection = %+v err=%v", snapshot.Voice, err)
	}
}

// Plan 10 A8: reassignment keeps the canonical post/finalization and withdraws the old
// voice's machine baseline, which is what removes learn eligibility.
func TestReassignmentPreservesContentAndClearsTheBaselineVoice(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Jeju")
	content := PostContent{Title: "generated", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(ctx, alice, created.Slug, 1); err != nil {
		t.Fatal(err)
	}
	before := store.posts[created.Slug]
	if before.MachineBaselineVoiceID != aliceVoice {
		t.Fatalf("machine result did not record its voice: %+v", before)
	}
	snapshot, err := svc.LearningSnapshot(ctx, alice, created.Slug)
	if err != nil || snapshot.VoiceID != aliceVoice || snapshot.MachineBaselineVoiceID != aliceVoice {
		t.Fatalf("snapshot before reassignment = %+v err=%v", snapshot, err)
	}

	review := aliceReview
	moved, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "memo", &review, nil, nil)
	if err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if moved.VoiceID != aliceReview || moved.Voice.Name != "리뷰" || moved.MachineBaselineVoiceID != "" {
		t.Fatalf("reassigned post = %+v", moved)
	}
	if moved.Slug != created.Slug || moved.Content == nil || moved.Content.Title != "generated" || moved.ContentRevision != before.ContentRevision ||
		moved.MachineBaselineRevision != 0 || moved.Status != StatusFinalized || moved.FinalizedRevision != before.FinalizedRevision {
		t.Fatalf("reassignment changed the post: before=%+v after=%+v", before, moved)
	}
	// The learning hand-off is unavailable until a new machine result establishes a baseline.
	if _, err = svc.LearningSnapshot(ctx, alice, created.Slug); !errors.Is(err, ErrNoMachineBaseline) {
		t.Fatalf("snapshot after reassignment = %v, want ErrNoMachineBaseline", err)
	}
	// A fresh machine result re-establishes a baseline in the new voice.
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, PostContent{Title: "again", Blocks: []Block{{Type: BlockText, Content: "new"}}}, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	if after := store.posts[created.Slug]; after.MachineBaselineVoiceID != aliceReview {
		t.Fatalf("new baseline voice = %q, want %q", after.MachineBaselineVoiceID, aliceReview)
	}
	// The same present value is not a reassignment: an ordinary autosave patch.
	same := aliceReview
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "memo 2", &same, nil, nil); err != nil {
		t.Fatalf("unchanged assignment: %v", err)
	}
	if store.posts[created.Slug].MachineBaselineVoiceID != aliceReview {
		t.Fatal("an unchanged assignment cleared the baseline")
	}
}

func TestReassignedReviewCanFinalizeWithoutPublishingLearningEvidence(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Finalize after move")
	content := PostContent{Title: "kept", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean); err != nil {
		t.Fatal(err)
	}
	target := aliceReview
	moved, err := svc.SaveDraft(ctx, alice, created.Slug, created.Title, created.Memo, &target, nil, nil)
	if err != nil || moved.Content == nil || moved.MachineBaselineRevision != 0 {
		t.Fatalf("reassign = %+v err=%v", moved, err)
	}
	finalized, err := svc.Finalize(ctx, alice, created.Slug, moved.ContentRevision)
	if err != nil || finalized.Status != StatusFinalized {
		t.Fatalf("finalize preserved content = %+v err=%v", finalized, err)
	}
	if _, err := svc.LearningSnapshot(ctx, alice, created.Slug); !errors.Is(err, ErrNoMachineBaseline) {
		t.Fatalf("learning snapshot without new baseline = %v", err)
	}
}

func TestReassignmentTargetsAndBusyPostsAreRefused(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Jeju")
	for name, tc := range map[string]struct {
		voice string
		want  error
	}{
		"empty":   {"", ErrVoiceRequired},
		"unknown": {"voice-nobody", ErrVoiceNotFound},
		"foreign": {bobVoice, ErrVoiceNotFound},
		"deleted": {aliceDeleted, ErrVoiceDeleted},
	} {
		t.Run(name, func(t *testing.T) {
			voiceID := tc.voice
			if _, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "", &voiceID, nil, nil); !errors.Is(err, tc.want) {
				t.Fatalf("reassign to %q = %v, want %v", tc.voice, err, tc.want)
			}
			if store.posts[created.Slug].VoiceID != aliceVoice {
				t.Fatal("a refused reassignment moved the post")
			}
		})
	}
	review := aliceReview
	svc.jobs = fakeActiveJobs{created.Slug: {ID: "job-1", Status: "running"}}
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "", &review, nil, nil); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("reassign during a job = %v", err)
	}
	svc.jobs = fakeActiveJobs{}
	svc.SetPendingExperimentFinder(fakePendingExperiments{created.Slug: "experiment-1"})
	if _, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "", &review, nil, nil); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("reassign during an undecided experiment = %v", err)
	}
	if store.posts[created.Slug].VoiceID != aliceVoice {
		t.Fatal("a refused reassignment moved the post")
	}
	svc.SetPendingExperimentFinder(fakePendingExperiments{})
	if moved, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju", "", &review, nil, nil); err != nil || moved.VoiceID != aliceReview {
		t.Fatalf("idle reassign = %+v err=%v", moved, err)
	}
	// A title-only autosave arriving afterwards preserves the newer assignment.
	if kept, err := svc.SaveDraft(ctx, alice, created.Slug, "Jeju 2", "", nil, nil, nil); err != nil || kept.VoiceID != aliceReview {
		t.Fatalf("absent voice_id changed the assignment: %+v err=%v", kept, err)
	}
}
