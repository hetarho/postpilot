package store_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"testing"
	"time"
)

type beforeRenderEnqueue struct {
	generationJobs
	before func()
}

func (j beforeRenderEnqueue) Enqueue(ctx context.Context, s clip.GenerationStart) (string, error) {
	j.before()
	return j.generationJobs.Enqueue(ctx, s)
}
func TestRenderRejectsRevisionRaceBeforeLinkWithoutConsumingSources(t *testing.T) {
	h, p, draft := completedClip(t)
	ctx := context.Background()
	b := rerenderBatch(t, h, true)
	jobs := beforeRenderEnqueue{generationJobs: generationJobs{h.queue}, before: func() {
		if _, err := h.service.SaveCorrection(ctx, "alice", p.ID, 1, draft); err != nil {
			t.Fatal(err)
		}
	}}
	s := clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, jobs, h.cfg)
	if _, err := s.StartRender(ctx, "alice", p.ID, b.ID, 1); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	b, err := h.store.GetSourceBatch(ctx, "alice", b.ID)
	if err != nil || b.State != "ready" || b.JobID != "" {
		t.Fatal(b, err)
	}
	j, err := h.queue.LatestForClip(ctx, "alice", p.ID)
	if err != nil || j.Status != job.StatusFailed {
		t.Fatal(j, err)
	}
	if len(h.admitter.calls) != 1 {
		t.Fatal("charged at failed enqueue")
	}
}
func TestRenderCancellationMalformedPanicAndBootAlwaysClean(t *testing.T) {
	for _, mode := range []string{"cancel", "malformed", "panic", "boot"} {
		t.Run(mode, func(t *testing.T) {
			h, p, _ := completedClip(t)
			ctx := context.Background()
			b := rerenderBatch(t, h, true)
			if _, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1); err != nil {
				t.Fatal(err)
			}
			j, err := h.jobs.PickNextQueued(ctx, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if mode == "boot" {
				if _, err = h.queue.SweepRunning(ctx); err != nil {
					t.Fatal(err)
				}
				if err = h.service.Sweep(ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				payload := j.Payload
				runCtx := ctx
				switch mode {
				case "cancel":
					var cancel context.CancelFunc
					runCtx, cancel = context.WithCancel(ctx)
					cancel()
				case "malformed":
					payload = []byte("{}")
				case "panic":
					h.renderer.panicRender = true
				}
				panicked := false
				func() {
					defer func() {
						if recover() != nil {
							panicked = true
						}
					}()
					err = h.service.RunRender(runCtx, "alice", j.ID, p.ID, payload, func(string, int, int) {})
				}()
				if mode == "panic" && !panicked {
					t.Fatal("panic fixture did not run")
				}
				if mode != "panic" && err == nil {
					t.Fatal("failure hidden")
				}
			}
			if _, err = h.store.GetSourceBatch(ctx, "alice", b.ID); !errors.Is(err, clip.ErrNotFound) {
				t.Fatal(err)
			}
			got, err := h.projects.GetProject(ctx, "alice", p.ID)
			if err != nil || got.Result.Key != p.Result.Key || got.EditPlan != p.EditPlan {
				t.Fatal(got, err)
			}
			if len(h.admitter.calls) != 1 || h.planner.plans != 1 {
				t.Fatal("recovery charged")
			}
		})
	}
}
func TestRenderExpiryCleanupRetryAndAtomicRevisionSwap(t *testing.T) {
	h, p, _ := completedClip(t)
	ctx := context.Background()
	b := rerenderBatch(t, h, true)
	if _, err := h.db.Writer.Exec("UPDATE clip_source_batches SET expires_at=? WHERE id=?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal(err)
	}
	b = rerenderBatch(t, h, true)
	h.objects.failDelete[b.Sources[0].Key] = true
	if _, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := runRender(t, h); err != nil {
		t.Fatal(err)
	}
	pending, err := h.store.GetSourceBatch(ctx, "alice", b.ID)
	if err != nil || pending.State != "cleanup_pending" {
		t.Fatal(pending, err)
	}
	h.objects.failDelete[b.Sources[0].Key] = false
	if err = h.service.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = h.store.GetSourceBatch(ctx, "alice", b.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	got, _ := h.projects.GetProject(ctx, "alice", p.ID)
	replacement := *got.Result
	replacement.Key = "clip-results/alice/other/new.mp4"
	if err = h.store.SaveRender(ctx, "alice", p.ID, 2, replacement); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	after, _ := h.projects.GetProject(ctx, "alice", p.ID)
	if after.Result.Key != got.Result.Key {
		t.Fatal("stale swap wrote")
	}
	keys, err := h.store.DeletionKeys(ctx)
	if err != nil || len(keys) != 0 {
		t.Fatal("stale swap queued deletion", keys, err)
	}
}
