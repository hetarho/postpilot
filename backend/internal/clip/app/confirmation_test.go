package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// confirmableProject is an owner's unfinalized clip whose saved plan 확정하기 accepts.
func confirmableProject(t *testing.T) clip.Project {
	t.Helper()
	analysis, err := json.Marshal([]clip.SourceAnalysis{{
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fingerprint",
			Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}, Filename: "travel.mp4"},
		Segments: []clip.Segment{{StartMS: 0, EndMS: 15000, Event: "food", Focal: clip.Point{X: .5, Y: .5}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000,
		Cuts: []clip.EditCut{{ID: "cut", SourceID: "source", Fingerprint: "fingerprint", EndMS: 15000,
			Focal: clip.Point{X: .5, Y: .5}, PlaybackRatePermille: 1000,
			Copies: []clip.Caption{{Text: "hello", Anchor: "upper_mid", Align: "center", Style: "bold", StartMS: 200, EndMS: 3000}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return clip.Project{ID: "clip", UserID: "alice", Ratio: "vertical", Channel: clip.ChannelNaver, Analysis: string(analysis), EditPlan: plan, EditPlanRevision: 3,
		RenderedPlanRevision: 3, Result: &clip.Result{ID: "r3", Key: "clip-results/r3.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 15000, CreatedAt: time.Unix(5, 0)}}
}

func confirmationFinalizer(t *testing.T, p clip.Project, jobs *fakeJobs) (Finalizer, *fakeClips) {
	t.Helper()
	clips := newFakeClips(p)
	if jobs == nil {
		jobs = &fakeJobs{}
	}
	return NewFinalizer(memoryWriter(t), bindFakes(jobs, clips, nil), clips, clip.DefaultRenderConfig(clip.Environment{}), func() time.Time { return time.Unix(9, 0) }), clips
}

func TestConfirmRecordsThePlanRevisionWithoutRenderingOrDeleting(t *testing.T) {
	p := confirmableProject(t)
	p.Result, p.RenderedPlanRevision = nil, 0 // confirmation needs no render (CLIP-75)
	if refusal := clip.ConfirmationRefusal(p, false, clip.DefaultRenderConfig(clip.Environment{})); refusal != "" {
		t.Fatalf("a valid plan was refused: %q", refusal)
	}
	f, clips := confirmationFinalizer(t, p, nil)
	got, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "alice", ProjectID: "clip", ExpectedPlanRevision: 3})
	if err != nil || got.Confirmed == nil || got.Confirmed.PlanRevision != 3 || !clip.Confirmed(got) {
		t.Fatal(got.Confirmed, err)
	}
	if got.Finalized != nil || len(clips.finalized) != 0 || len(clips.deleted) != 0 {
		t.Fatal("confirmation finalized or deleted something")
	}
	// The same revision again is idempotent and writes nothing.
	if _, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "alice", ProjectID: "clip", ExpectedPlanRevision: 3}); err != nil || clips.confirmations != 1 {
		t.Fatal("a repeated confirmation wrote again", clips.confirmations, err)
	}
	// A later plan save reads unconfirmed with no write on the confirmation (CLIP-229).
	clips.project.EditPlanRevision = 4
	if clip.Confirmed(clips.project) {
		t.Fatal("a changed plan still reads as confirmed")
	}
}

func TestConfirmRefusals(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	stale := confirmableProject(t)
	f, clips := confirmationFinalizer(t, stale, nil)
	if _, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "alice", ProjectID: "clip", ExpectedPlanRevision: 2}); !errors.Is(err, clip.ErrPlanConflict) || clips.confirmations != 0 {
		t.Fatal("a stale expected revision was confirmed", err)
	}
	if _, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "bob", ProjectID: "clip", ExpectedPlanRevision: 3}); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("a foreign project was confirmed", err)
	}

	busy := confirmableProject(t)
	f, clips = confirmationFinalizer(t, busy, &fakeJobs{active: &job.Job{ID: "running", Status: "running"}})
	if _, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "alice", ProjectID: "clip", ExpectedPlanRevision: 3}); !errors.Is(err, clip.ErrBusy) || clips.confirmations != 0 {
		t.Fatal("confirmed while an attempt runs", err)
	}
	if refusal := clip.ConfirmationRefusal(busy, true, cfg); refusal != "busy" {
		t.Fatal(refusal)
	}

	finalized := confirmableProject(t)
	finalized.Finalized = &clip.Finalization{PlanRevision: 3, ResultID: "r3"}
	f, _ = confirmationFinalizer(t, finalized, nil)
	if _, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "alice", ProjectID: "clip", ExpectedPlanRevision: 3}); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal("a finalized project was confirmed", err)
	}
	if refusal := clip.ConfirmationRefusal(finalized, false, cfg); refusal != "finalized" {
		t.Fatal(refusal)
	}

	invalid := confirmableProject(t)
	invalid.EditPlan = "{"
	f, _ = confirmationFinalizer(t, invalid, nil)
	if _, err := f.Confirm(context.Background(), clip.ConfirmationRequest{UserID: "alice", ProjectID: "clip", ExpectedPlanRevision: 3}); !errors.Is(err, clip.ErrConfirmationInvalid) {
		t.Fatal("an unreadable plan was confirmed", err)
	}
	if refusal := clip.ConfirmationRefusal(invalid, false, cfg); refusal != "invalid_plan" {
		t.Fatal(refusal)
	}

	narrated := confirmableProject(t)
	plan, err := clip.DecodeEditPlan(narrated.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Narration = &clip.NarrationPlan{Enabled: true, VolumePermille: 1000}
	narrated.EditPlan, _ = clip.EncodeEditPlan(plan)
	if refusal := clip.ConfirmationRefusal(narrated, false, cfg); refusal != "narration" {
		t.Fatalf("unready narration was not named: %q", refusal)
	}
}

func TestFinalizeRequiresConfirmationAndRecordsThePostedURL(t *testing.T) {
	ctx := context.Background()
	p := confirmableProject(t)
	f, clips := confirmationFinalizer(t, p, nil)
	req := clip.FinalizationRequest{UserID: "alice", ProjectID: "clip", ExpectedRevision: 3, ExpectedResultID: "r3"}
	if _, err := f.Finalize(ctx, req); !errors.Is(err, clip.ErrFinalizationUnconfirmed) || !errors.Is(err, clip.ErrFinalizationInvalid) || len(clips.finalized) != 0 {
		t.Fatal("an unconfirmed plan was finalized", err)
	}
	// A confirmation of an earlier revision does not confirm this one.
	p.Confirmed = &clip.Confirmation{At: time.Unix(1, 0), PlanRevision: 2}
	f, clips = confirmationFinalizer(t, p, nil)
	if _, err := f.Finalize(ctx, req); !errors.Is(err, clip.ErrFinalizationUnconfirmed) {
		t.Fatal("a stale confirmation finalized", err)
	}

	p.Confirmed = &clip.Confirmation{At: time.Unix(1, 0), PlanRevision: 3}
	f, clips = confirmationFinalizer(t, p, nil)
	withURL := req
	withURL.PublishedURL = "  https://m.naver.com/clip/123  "
	got, err := f.Finalize(ctx, withURL)
	if err != nil || got.Published == nil || got.Published.URL != "https://m.naver.com/clip/123" {
		t.Fatal("게시됨 did not record its trimmed URL", got.Published, err)
	}

	f, clips = confirmationFinalizer(t, p, nil)
	got, err = f.Finalize(ctx, req)
	if err != nil || got.Finalized == nil || got.Published != nil {
		t.Fatal("완료 did not finalize without a URL", got.Published, err)
	}

	f, clips = confirmationFinalizer(t, p, nil)
	foreign := req
	foreign.PublishedURL = "https://www.instagram.com/reel/abc"
	if _, err := f.Finalize(ctx, foreign); !errors.Is(err, clip.ErrPublishedURLInvalid) || len(clips.finalized) != 0 {
		t.Fatal("a URL of the other platform finalized a Naver clip", err)
	}
}

func TestFinalizationRefusalNamesAnUnconfirmedPlan(t *testing.T) {
	p := confirmableProject(t)
	g := &GenerationService{cfg: clip.GenerationConfig{Render: clip.DefaultRenderConfig(clip.Environment{})}}
	if refusal := g.FinalizationRefusal(p, false); refusal != "unconfirmed" {
		t.Fatalf("an unconfirmed plan read %q", refusal)
	}
	p.Confirmed = &clip.Confirmation{At: time.Unix(1, 0), PlanRevision: 3}
	if refusal := g.FinalizationRefusal(p, false); refusal != "" {
		t.Fatalf("a confirmed rendered plan read %q", refusal)
	}
}

func TestSavePublishedURLOnlyOnAFinalizedIdleProject(t *testing.T) {
	ctx := context.Background()
	p := confirmableProject(t)
	f, clips := confirmationFinalizer(t, p, nil)
	if _, err := f.SavePublishedURL(ctx, "alice", "clip", "https://naver.me/abc"); !errors.Is(err, clip.ErrFinalizationInvalid) || clips.project.Published != nil {
		t.Fatal("an unfinalized project saved a URL", err)
	}

	p.Finalized = &clip.Finalization{At: time.Unix(2, 0), PlanRevision: 3, ResultID: "r3"}
	f, clips = confirmationFinalizer(t, p, nil)
	got, err := f.SavePublishedURL(ctx, "alice", "clip", "https://naver.me/abc")
	if err != nil || got.Published == nil || got.Published.URL != "https://naver.me/abc" {
		t.Fatal("완료 did not become 게시됨", got.Published, err)
	}
	got, err = f.SavePublishedURL(ctx, "alice", "clip", "https://clip.naver.com/v/2")
	if err != nil || got.Published.URL != "https://clip.naver.com/v/2" || got.Finalized == nil || got.EditPlanRevision != 3 {
		t.Fatal("a replaced URL changed something else", got, err)
	}
	if _, err := f.SavePublishedURL(ctx, "alice", "clip", "http://naver.me/abc"); !errors.Is(err, clip.ErrPublishedURLInvalid) {
		t.Fatal("an http URL was saved", err)
	}

	f, clips = confirmationFinalizer(t, p, &fakeJobs{active: &job.Job{ID: "upload-text", Status: "running"}})
	if _, err := f.SavePublishedURL(ctx, "alice", "clip", "https://naver.me/abc"); !errors.Is(err, clip.ErrBusy) || clips.project.Published != nil {
		t.Fatal("a URL was saved while a job runs", err)
	}
}
