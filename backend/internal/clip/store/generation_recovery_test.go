package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

type interruptedActivation struct {
	generationJobs
	after func()
}

func (j interruptedActivation) Activate(ctx context.Context, user, id string) error {
	if j.after != nil {
		if err := j.generationJobs.Activate(ctx, user, id); err != nil {
			return err
		}
		j.after()
	}
	return context.Canceled
}

func TestGenerationActivationCompensationNeverDeletesRunningInputs(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "running"}[running], func(t *testing.T) {
			h := generationSetup(t)
			jobs := interruptedActivation{generationJobs: generationJobs{h.queue}}
			if running {
				jobs.after = func() {
					if _, err := h.jobs.PickNextQueued(context.Background(), time.Now()); err != nil {
						t.Fatal(err)
					}
				}
			}
			s := clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, jobs, h.cfg).WithCredits(&quotePricing{}, nil)
			id, err := startApproved(context.Background(), s, "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
			if running {
				if err != nil || id == "" {
					t.Fatal(id, err)
				}
				b, err := h.store.GetSourceBatch(context.Background(), "alice", h.batch.ID)
				if err != nil || b.State != "consuming" || len(h.objects.deleted) != 0 {
					t.Fatal("running inputs deleted", b, err)
				}
			} else {
				if err == nil {
					t.Fatal("activation failure hidden")
				}
				h.assertClean(t)
				latest, err := h.queue.LatestForClip(context.Background(), "alice", h.project.ID)
				if err != nil || latest == nil || latest.Status != "failed" {
					t.Fatal(latest, err)
				}
			}
			if len(h.admitter.calls) != 0 {
				t.Fatal("compensation held credits")
			}
		})
	}
}

func TestGenerationLinkFailureLeavesReadyBatchRetryable(t *testing.T) {
	h := generationSetup(t)
	if _, err := h.db.Writer.Exec("CREATE TRIGGER refuse_clip_link BEFORE UPDATE OF job_id ON clip_source_batches BEGIN SELECT RAISE(ABORT,'link failed'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := startApproved(context.Background(), h.service, "alice", h.project.ID, h.batch.ID, "p/o", "p/w"); err == nil {
		t.Fatal("link failure hidden")
	}
	b, err := h.store.GetSourceBatch(context.Background(), "alice", h.batch.ID)
	if err != nil || b.State != "ready" || b.JobID != "" {
		t.Fatal(b, err)
	}
	j, err := h.queue.LatestForClip(context.Background(), "alice", h.project.ID)
	if err != nil || j.Status != "failed" || len(h.admitter.calls) != 0 {
		t.Fatal(j, err)
	}
}

func TestGenerationCanceledOrMalformedRunStillCleansLease(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		h := generationSetup(t)
		h.start(t)
		j, err := h.jobs.PickNextQueued(context.Background(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		payload := []byte("not JSON")
		if cancel {
			var stop context.CancelFunc
			ctx, stop = context.WithCancel(ctx)
			stop()
			payload = j.Payload
		}
		if err = h.service.Run(ctx, "alice", j.ID, j.ClipProjectID, payload, func(string, int, int) {}); err == nil {
			t.Fatal("failure hidden")
		}
		h.assertClean(t)
		if len(h.admitter.calls) != 0 || h.planner.observe != 0 {
			t.Fatal("invalid run reached AI")
		}
	}
}

func TestGenerationProxyDeletionRetainsRecoveryAuthority(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	key := clip.SourcePrefix + "alice/" + h.batch.ID + "/proxy/interrupted.mp4"
	if err := h.store.AddProxy(context.Background(), "alice", h.batch.ID, key); err != nil {
		t.Fatal(err)
	}
	h.objects.info[key] = clip.SourceObjectInfo{Bytes: 5, ContentType: "video/mp4"}
	h.objects.failDelete[key] = true
	if _, err := h.queue.FailQueued(context.Background(), id, "alice", job.Failure{Reason: "JOB_INTERRUPTED"}); err != nil {
		t.Fatal(err)
	}
	if err := h.service.Sweep(context.Background()); err == nil {
		t.Fatal("failed delete hidden")
	}
	b, err := h.store.GetSourceBatch(context.Background(), "alice", h.batch.ID)
	if err != nil || b.State != "ready" || len(b.ProxyKeys) != 1 {
		t.Fatal(b, err)
	}
	h.objects.failDelete[key] = false
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.assertClean(t)
}

func TestGenerationOrphanSweepFailsClosedAndProtectsActiveAndRecent(t *testing.T) {
	h := generationSetup(t)
	old := clip.ResultPrefix + "alice/" + h.project.ID + "/orphan.mp4"
	recent := clip.ResultPrefix + "alice/" + h.project.ID + "/recent.mp4"
	h.objects.results = []clip.StoredObject{{Key: old, Modified: time.Now().Add(-2 * time.Hour)}, {Key: recent, Modified: time.Now()}}
	for _, key := range []string{old, recent} {
		h.objects.info[key] = clip.SourceObjectInfo{Bytes: 5}
	}
	h.objects.listErr = errors.New("short list")
	if err := h.service.Sweep(context.Background()); err == nil {
		t.Fatal("short list hidden")
	}
	if len(h.objects.deleted) != 0 {
		t.Fatal("partial list deleted objects")
	}
	h.objects.listErr = nil
	id := h.start(t)
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.info[old]; !ok {
		t.Fatal("active output reaped")
	}
	if _, err := h.queue.FailQueued(context.Background(), id, "alice", job.Failure{Reason: "JOB_INTERRUPTED"}); err != nil {
		t.Fatal(err)
	}
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.info[old]; ok {
		t.Fatal("orphan retained")
	}
	if _, ok := h.objects.info[recent]; !ok {
		t.Fatal("recent output reaped")
	}
}

func TestGenerationReplacementQueuesOnlyPreviousResult(t *testing.T) {
	h := generationSetup(t)
	old := clip.Result{Key: clip.ResultPrefix + "alice/" + h.project.ID + "/old.mp4", ContentType: "video/mp4", Bytes: 5, DurationMS: 30000, CreatedAt: time.Now()}
	if err := h.store.SaveGeneration(context.Background(), "alice", h.project.ID, "old analysis", "old plan", old); err != nil {
		t.Fatal(err)
	}
	h.objects.info[old.Key] = clip.SourceObjectInfo{Bytes: 5}
	seedCompletedGeneration(t, h)
	p, err := h.store.GetProject(context.Background(), "alice", h.project.ID)
	if err != nil || p.Result.Key == old.Key || p.EditPlanRevision != 2 || p.RenderedPlanRevision != 2 {
		t.Fatal(p, err)
	}
	keys, err := h.store.DeletionKeys(context.Background())
	if err != nil || len(keys) != 1 || keys[0] != old.Key {
		t.Fatal(keys, err)
	}
	h.objects.failDelete[old.Key] = true
	if err := h.service.Sweep(context.Background()); err == nil {
		t.Fatal("deletion failure hidden")
	}
	keys, _ = h.store.DeletionKeys(context.Background())
	if len(keys) != 1 {
		t.Fatal("lost deletion intent")
	}
	h.objects.failDelete[old.Key] = false
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.info[p.Result.Key]; !ok {
		t.Fatal("current result deleted")
	}
}
