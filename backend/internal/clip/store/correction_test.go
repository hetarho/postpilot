package store_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func completedClip(t *testing.T) (*generationHarness, clip.Project, clip.CorrectionPlan) {
	t.Helper()
	h := generationSetup(t)
	seedCompletedGeneration(t, h)
	p, err := h.projects.GetProject(context.Background(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := h.service.EditingState(p)
	if err != nil {
		t.Fatal(err)
	}
	return h, p, state.Plan
}
func rerenderBatch(t *testing.T, h *generationHarness, subset bool) clip.SourceBatch {
	t.Helper()
	ctx := context.Background()
	values := []clip.SourceMetadata{h.batch.Sources[0].SourceMetadata}
	if !subset {
		values = append(values, h.batch.Sources[1].SourceMetadata)
	}
	u, err := h.sources.Create(ctx, "alice", h.project.ID, values)
	if err != nil {
		t.Fatal(err)
	}
	h.objects.upload(u.Batch)
	var b clip.SourceBatch
	for _, s := range u.Batch.Sources {
		b, err = h.sources.Confirm(ctx, "alice", u.Batch.ID, s.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	return b
}
func runRender(t *testing.T, h *generationHarness) error {
	t.Helper()
	ctx := context.Background()
	j, err := h.jobs.PickNextQueued(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != job.KindRenderClip {
		t.Fatal(j.Kind)
	}
	err = h.service.RunRender(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, func(stage string, n, total int) {
		if e := h.jobs.UpdateProgress(ctx, j.ID, stage, n, total, time.Now()); e != nil {
			t.Fatal(e)
		}
	})
	status := job.StatusDone
	var failure *job.Failure
	if err != nil {
		status = job.StatusFailed
		failure = &job.Failure{Reason: "CLIP_PROCESSING_FAILED"}
	}
	if e := h.jobs.Finish(ctx, j.ID, status, failure, time.Now()); e != nil {
		t.Fatal(e)
	}
	return err
}
func TestCorrectionSaveConflictSubsetRenderAndZeroUsage(t *testing.T) {
	h, old, draft := completedClip(t)
	ctx := context.Background()
	draft.Cuts[0].Copies[0].Text = "手書き・정확한 수정"
	draft.Cuts[0].VolumePermille = 0
	if _, err := h.service.SaveCorrection(ctx, "bob", old.ID, 1, draft); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	saved, err := h.service.SaveCorrection(ctx, "alice", old.ID, 1, draft)
	if err != nil {
		t.Fatal(err)
	}
	if saved.EditPlanRevision != 2 || saved.RenderedPlanRevision != 1 || saved.Result.Key != old.Result.Key {
		t.Fatal(saved)
	}
	if _, err = h.service.SaveCorrection(ctx, "alice", old.ID, 1, draft); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	if _, err = h.service.StartRender(ctx, "alice", old.ID, "missing", 1); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	all := rerenderBatch(t, h, false)
	if _, err = h.service.StartRender(ctx, "alice", old.ID, all.ID, 2); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("unused source accepted", err)
	}
	b := rerenderBatch(t, h, true)
	if _, err = h.service.StartRender(ctx, "bob", old.ID, b.ID, 2); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	id, err := h.service.StartRender(ctx, "alice", old.ID, b.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.service.StartRender(ctx, "alice", old.ID, b.ID, 2); !errors.Is(err, clip.ErrBusy) {
		t.Fatal(err)
	}
	if _, err = h.service.SaveCorrection(ctx, "alice", old.ID, 2, draft); !errors.Is(err, clip.ErrBusy) {
		t.Fatal(err)
	}
	if err = h.projects.DeleteProject(ctx, "alice", old.ID); !errors.Is(err, clip.ErrBusy) {
		t.Fatal(err)
	}
	if err = runRender(t, h); err != nil {
		t.Fatal(err)
	}
	got, err := h.projects.GetProject(ctx, "alice", old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EditPlan != saved.EditPlan || got.Analysis != old.Analysis || got.EditPlanRevision != 2 || got.RenderedPlanRevision != 2 || got.Result.Key == old.Result.Key {
		t.Fatal(got)
	}
	if len(h.admitter.calls) != 0 || h.planner.observe != 0 || h.planner.plans != 0 {
		t.Fatal("render used AI or credits")
	}
	for _, table := range []string{"usage_admissions", "usage_events"} {
		var n int
		if err = h.db.Reader.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE job_id=?", id).Scan(&n); err != nil || n != 0 {
			t.Fatal(table, n, err)
		}
	}
	if _, err = h.store.GetSourceBatch(ctx, "alice", b.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	keys, err := h.store.DeletionKeys(ctx)
	if err != nil || len(keys) != 1 || keys[0] != old.Result.Key {
		t.Fatal(keys, err)
	}
	if err = h.projects.DeleteProject(ctx, "alice", old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = h.projects.GetProject(ctx, "alice", old.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestCorrectionFailuresPreserveSavedPlanAndOldVideo(t *testing.T) {
	for _, stage := range []string{"probe", "render", "upload", "workspace"} {
		t.Run(stage, func(t *testing.T) {
			h, old, draft := completedClip(t)
			ctx := context.Background()
			draft.Cuts[0].VolumePermille = 321
			saved, err := h.service.SaveCorrection(ctx, "alice", old.ID, 1, draft)
			if err != nil {
				t.Fatal(err)
			}
			b := rerenderBatch(t, h, true)
			if _, err = h.service.StartRender(ctx, "alice", old.ID, b.ID, 2); err != nil {
				t.Fatal(err)
			}
			switch stage {
			case "probe":
				h.media.durations = []int{10}
			case "render":
				h.renderer.fail = errors.New("failed")
			case "upload":
				h.objects.failUpload = true
			case "workspace":
				h.media.cleanupErr = errors.New("failed")
			}
			if err = runRender(t, h); err == nil {
				t.Fatal("expected failure")
			}
			got, err := h.projects.GetProject(ctx, "alice", old.ID)
			if err != nil || got.EditPlan != saved.EditPlan || got.Result.Key != old.Result.Key || got.RenderedPlanRevision != 1 {
				t.Fatal(got, err)
			}
			if _, err = h.store.GetSourceBatch(ctx, "alice", b.ID); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal(err)
			}
			if h.planner.plans != 0 || len(h.admitter.calls) != 0 {
				t.Fatal("render charged")
			}
		})
	}
}
func TestConcurrentCorrectionSaveOnlyOneWinsAndInvalidCaptionDoesNotWrite(t *testing.T) {
	h, old, draft := completedClip(t)
	ctx := context.Background()
	h.renderer.captionErr = clip.ErrCopyTooLong
	if _, err := h.service.SaveCorrection(ctx, "alice", old.ID, 1, draft); !errors.Is(err, clip.ErrCopyTooLong) {
		t.Fatal(err)
	}
	got, _ := h.projects.GetProject(ctx, "alice", old.ID)
	if got.EditPlanRevision != 1 || got.EditPlan != old.EditPlan {
		t.Fatal("invalid save wrote")
	}
	h.renderer.captionErr = nil
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, err := h.service.SaveCorrection(ctx, "alice", old.ID, 1, draft); results <- err })
	}
	wg.Wait()
	close(results)
	ok, conflicts := 0, 0
	for e := range results {
		if e == nil {
			ok++
		} else if errors.Is(e, clip.ErrPlanConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal(ok, conflicts)
	}
	got, _ = h.projects.GetProject(ctx, "alice", old.ID)
	state, _ := h.service.EditingState(got)
	if !reflect.DeepEqual(state.Plan, draft) || !strings.Contains(got.EditPlan, `"VolumePermille":1000`) {
		t.Fatal(got.EditPlan)
	}
	if err := h.projects.DeleteProject(ctx, "alice", old.ID); err != nil {
		t.Fatal("unrendered deletion", err)
	}
}
