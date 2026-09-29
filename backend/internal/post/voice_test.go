package post

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakePendingExperiments map[string]string

func (f fakePendingExperiments) PendingForPost(_ context.Context, _, slug string) (string, error) {
	return f[slug], nil
}

// POST-23: a create names one owned, active, made voice or none — 말투 없음 is a real answer
// and the server never picks the 기본.
func TestCreateNamesAMadeVoiceOrNone(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	unknown := "voice-nobody"
	foreign := bobVoice
	deleted := aliceDeleted
	unmade := aliceUnmade
	language := LanguageKorean
	for name, tc := range map[string]struct {
		voice *string
		want  error
	}{
		"unknown":  {&unknown, ErrVoiceNotFound},
		"foreign":  {&foreign, ErrVoiceNotFound},
		"deleted":  {&deleted, ErrVoiceDeleted},
		"not made": {&unmade, ErrVoiceNotMade},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.SaveDraft(ctx, alice, DraftSave{Title: "Jeju", VoiceID: tc.voice, TargetLanguage: &language}); !errors.Is(err, tc.want) {
				t.Fatalf("SaveDraft = %v, want %v", err, tc.want)
			}
		})
	}
	if len(store.posts) != 0 {
		t.Fatalf("a rejected create minted a post: %+v", store.posts)
	}
	empty := ""
	for name, voice := range map[string]*string{"absent": nil, "empty": &empty} {
		created, err := svc.SaveDraft(ctx, alice, DraftSave{Title: "Jeju " + name, VoiceID: voice, TargetLanguage: &language})
		if err != nil || created.VoiceID != "" || created.Voice != (VoiceRef{}) {
			t.Fatalf("%s voice create = %+v err=%v, want 말투 없음", name, created, err)
		}
		if stored := store.posts[created.Slug]; stored.VoiceID != "" {
			t.Fatalf("%s voice create stored voice %q", name, stored.VoiceID)
		}
	}
	// A directory is constructor state (ARCH-40): a service that could be built without
	// one would fail every create closed at runtime instead of at boot.
	func() {
		defer func() {
			if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "voices") {
				t.Fatalf("panic = %v, want a loud voice-directory refusal", r)
			}
		}()
		deps := testDeps()
		deps.Voices = nil
		NewService(newFakeStore(), newFakeBlobs(), Limits{PutTTL: time.Minute, GetTTL: time.Minute, MaxImageBytes: testMaxBytes, MaxPhotos: 30}, deps)
	}()
}

// POST-25: read models carry the voice's name and tombstone state, so a post whose
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
	listed, err := listSummaries(svc.List(ctx, alice, ListQuery{}))
	if err != nil || len(listed) != 1 || listed[0].Voice.Name != "옛 말투" || !listed[0].Voice.Deleted {
		t.Fatalf("list projection = %+v err=%v", listed, err)
	}
	snapshot, err := svc.AttachedImages(ctx, alice, created.Slug)
	if err != nil || !snapshot.Voice.Deleted {
		t.Fatalf("generation snapshot projection = %+v err=%v", snapshot.Voice, err)
	}
}

// POST-24: a reassignment changes voice_id alone. Content, its revision, finalization and the
// machine baseline stay, so an untouched draft still reads as untouched (POST-16, POST-98).
func TestReassignmentKeepsContentAndTheMachineBaseline(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Jeju")
	content := PostContent{Title: "generated", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	if err := svc.SetGeneratedContent(ctx, alice, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(ctx, alice, created.Slug, 1); err != nil {
		t.Fatal(err)
	}
	before := store.posts[created.Slug]
	if before.MachineBaselineRevision != before.ContentRevision {
		t.Fatalf("a machine result did not set its baseline: %+v", before)
	}

	review := aliceReview
	moved, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", Memo: "memo", VoiceID: &review})
	if err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if moved.VoiceID != aliceReview || moved.Voice.Name != "리뷰" {
		t.Fatalf("reassigned post = %+v", moved)
	}
	if moved.Slug != created.Slug || moved.Content == nil || moved.Content.Title != "generated" || moved.ContentRevision != before.ContentRevision ||
		moved.MachineBaselineRevision != before.MachineBaselineRevision || moved.Status != StatusFinalized || moved.FinalizedRevision != before.FinalizedRevision {
		t.Fatalf("reassignment changed the post: before=%+v after=%+v", before, moved)
	}
	// Moving back is a reassignment too, and it keeps the baseline just the same.
	back := aliceVoice
	returned, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", Memo: "memo", VoiceID: &back})
	if err != nil || returned.VoiceID != aliceVoice || returned.MachineBaselineRevision != returned.ContentRevision {
		t.Fatalf("second reassignment = %+v err=%v", returned, err)
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
		"unknown":  {"voice-nobody", ErrVoiceNotFound},
		"foreign":  {bobVoice, ErrVoiceNotFound},
		"deleted":  {aliceDeleted, ErrVoiceDeleted},
		"not made": {aliceUnmade, ErrVoiceNotMade},
	} {
		t.Run(name, func(t *testing.T) {
			voiceID := tc.voice
			if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &voiceID}); !errors.Is(err, tc.want) {
				t.Fatalf("reassign to %q = %v, want %v", tc.voice, err, tc.want)
			}
			if store.posts[created.Slug].VoiceID != aliceVoice {
				t.Fatal("a refused reassignment moved the post")
			}
		})
	}
	review := aliceReview
	svc.jobs = fakeActiveJobs{created.Slug: {ID: "job-1", Status: "running"}}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &review}); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("reassign during a job = %v", err)
	}
	svc.jobs = fakeActiveJobs{}
	svc.experiments = fakePendingExperiments{created.Slug: "experiment-1"}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &review}); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("reassign during an undecided experiment = %v", err)
	}
	if store.posts[created.Slug].VoiceID != aliceVoice {
		t.Fatal("a refused reassignment moved the post")
	}
	svc.experiments = fakePendingExperiments{}
	if moved, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &review}); err != nil || moved.VoiceID != aliceReview {
		t.Fatalf("idle reassign = %+v err=%v", moved, err)
	}
	// A title-only autosave arriving afterwards preserves the newer assignment.
	if kept, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju 2"}); err != nil || kept.VoiceID != aliceReview {
		t.Fatalf("absent voice_id changed the assignment: %+v err=%v", kept, err)
	}
}

// POST-24: an empty voice_id clears the post to 말투 없음, a present one sets a voice on a post
// that had none, and each is a reassignment under the same busy refusals, touching voice_id
// alone.
func TestAPostClearsToNoVoiceAndBack(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	created := mustCreatePost(t, svc, alice, "Jeju")
	before := store.posts[created.Slug]
	none := ""
	svc.jobs = fakeActiveJobs{created.Slug: {ID: "job-1", Status: "running"}}
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &none}); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("clearing during a job = %v", err)
	}
	svc.jobs = fakeActiveJobs{}
	cleared, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &none})
	if err != nil || cleared.VoiceID != "" || cleared.Voice != (VoiceRef{}) {
		t.Fatalf("clear = %+v err=%v", cleared, err)
	}
	after := store.posts[created.Slug]
	if after.ContentRevision != before.ContentRevision || after.MachineBaselineRevision != before.MachineBaselineRevision || after.Status != before.Status {
		t.Fatalf("clearing moved more than the voice: before=%+v after=%+v", before, after)
	}
	// Clearing twice is no change at all.
	if again, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &none}); err != nil || again.VoiceID != "" {
		t.Fatalf("second clear = %+v err=%v", again, err)
	}
	unmade := aliceUnmade
	if _, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &unmade}); !errors.Is(err, ErrVoiceNotMade) {
		t.Fatalf("setting an unmade voice = %v", err)
	}
	review := aliceReview
	if set, err := svc.SaveDraft(ctx, alice, DraftSave{Slug: created.Slug, Title: "Jeju", VoiceID: &review}); err != nil || set.VoiceID != aliceReview || set.Voice.Name != "리뷰" {
		t.Fatalf("setting a voice on 말투 없음 = %+v err=%v", set, err)
	}
}
