package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

func runningJob(kind string) job.Job {
	return job.Job{ID: "job", Kind: kind, UserID: "alice", Subjects: []job.Subject{{Dimension: clip.JobSubject, ID: "clip"}}, Status: job.StatusRunning, Stage: "render"}
}
func oldProject() clip.Project {
	return clip.Project{ID: "clip", UserID: "alice", EditPlan: "old-plan", EditPlanRevision: 1, Result: &clip.Result{Key: "clip-results/alice/clip/old.mp4"}}
}
func fileCandidate() clip.AttemptResult {
	return clip.AttemptResult{JobID: "job", UserID: "alice", ProjectID: "clip", ExpectedRevision: 1, Analysis: "[]", EditPlan: "new-plan", Result: clip.Result{Key: "clip-results/alice/clip/new.mp4", ContentType: "video/mp4", Bytes: 20, DurationMS: 15000, CreatedAt: time.Now()}}
}

func newFinisherUnderTest(t *testing.T, jobs *fakeJobs, clips *fakeClips) Finisher {
	return NewFinisher(memoryWriter(t), bindFakes(jobs, clips, nil), jobs, clips, func() time.Time { return time.Unix(1, 0) })
}

func TestFinisherCommitsAStagedFileWithTheJobAndClearsTheStage(t *testing.T) {
	jobs := &fakeJobs{jobs: map[string]job.Job{"job": runningJob(job.KindGenerateClip)}}
	clips := newFakeClips(oldProject())
	f := newFinisherUnderTest(t, jobs, clips)
	if err := f.Complete(context.Background(), fileCandidate()); err != nil {
		t.Fatal(err)
	}
	if len(clips.applied) != 1 || jobs.jobs["job"].Status != job.StatusDone || len(clips.deleted) != 1 || len(clips.staged) != 0 {
		t.Fatal("commit must apply, finish and clear the stage together", clips.applied, jobs.jobs["job"], clips.deleted)
	}
	// Idempotent: the durable rows already say the result landed.
	if err := f.Complete(context.Background(), fileCandidate()); err != nil || len(clips.applied) != 1 {
		t.Fatal("a repeated completion must be a no-op", clips.applied, err)
	}
}

func TestFinisherAppliesAPlanOnlyCompletionWithoutAStagingRow(t *testing.T) {
	jobs := &fakeJobs{jobs: map[string]job.Job{"job": runningJob(job.KindGenerateClip)}}
	clips := newFakeClips(oldProject())
	c := fileCandidate()
	c.Result = clip.Result{}
	if err := newFinisherUnderTest(t, jobs, clips).Complete(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if len(clips.staged) != 0 || len(clips.deleted) != 0 || clips.project.EditPlan != "new-plan" || clips.project.EditPlanRevision != 2 || jobs.jobs["job"].Status != job.StatusDone {
		t.Fatal("a plan commit stages nothing and sweeps nothing (CLIP-151)", clips.staged, clips.deleted, clips.project)
	}
}

func TestFinisherRefusesForeignConflictingAndCancelledJobs(t *testing.T) {
	t.Run("foreign job", func(t *testing.T) {
		j := runningJob(job.KindGenerateClip)
		j.UserID = "bob"
		jobs := &fakeJobs{jobs: map[string]job.Job{"job": j}}
		clips := newFakeClips(oldProject())
		if err := newFinisherUnderTest(t, jobs, clips).Complete(context.Background(), fileCandidate()); !errors.Is(err, clip.ErrNotFound) || len(clips.staged) != 0 {
			t.Fatal(err, clips.staged)
		}
	})
	t.Run("cancellation requested inside the transaction", func(t *testing.T) {
		j := runningJob(job.KindGenerateClip)
		at := time.Now()
		j.CancelRequestedAt = &at
		jobs := &fakeJobs{jobs: map[string]job.Job{"job": j}}
		clips := newFakeClips(oldProject())
		err := newFinisherUnderTest(t, jobs, clips).Complete(context.Background(), fileCandidate())
		if !errors.Is(err, context.Canceled) || len(clips.applied) != 0 || len(clips.discarded) != 1 || len(clips.staged) != 0 {
			t.Fatal("cancellation wins and the staged file is discarded", err, clips.applied, clips.discarded)
		}
	})
	t.Run("job no longer running", func(t *testing.T) {
		j := runningJob(job.KindGenerateClip)
		j.Status = job.StatusFailed
		jobs := &fakeJobs{jobs: map[string]job.Job{"job": j}}
		clips := newFakeClips(oldProject())
		err := newFinisherUnderTest(t, jobs, clips).Complete(context.Background(), fileCandidate())
		if !errors.Is(err, clip.ErrBusy) || len(clips.discarded) != 1 {
			t.Fatal("a terminal job discards the stage and reports busy", err, clips.discarded)
		}
	})
	t.Run("staged candidate disagrees with the completion", func(t *testing.T) {
		jobs := &fakeJobs{jobs: map[string]job.Job{"job": runningJob(job.KindGenerateClip)}}
		clips := newFakeClips(oldProject())
		other := fileCandidate()
		other.Result.Key = "clip-results/alice/clip/other.mp4"
		clips.staged["job"] = other
		clips.stageErr = errors.New("unique constraint")
		err := newFinisherUnderTest(t, jobs, clips).Complete(context.Background(), fileCandidate())
		if err == nil || len(clips.applied) != 0 {
			t.Fatal("a conflicting stage row must not be committed", err, clips.applied)
		}
	})
	t.Run("render job carrying a plan", func(t *testing.T) {
		jobs := &fakeJobs{jobs: map[string]job.Job{"job": runningJob(job.KindRenderClip)}}
		clips := newFakeClips(oldProject())
		err := newFinisherUnderTest(t, jobs, clips).Complete(context.Background(), fileCandidate())
		if !errors.Is(err, clip.ErrInvalid) || len(clips.applied) != 0 {
			t.Fatal("a render completion may not rewrite the plan", err)
		}
	})
}

func TestFinisherRecoverDiscardsOnlyOrphanedStages(t *testing.T) {
	running := runningJob(job.KindGenerateClip)
	done := runningJob(job.KindGenerateClip)
	done.ID, done.Status = "done", job.StatusDone
	jobs := &fakeJobs{jobs: map[string]job.Job{"job": running, "done": done}}
	clips := newFakeClips(oldProject())
	for _, id := range []string{"job", "done", "missing"} {
		c := fileCandidate()
		c.JobID = id
		clips.staged[id] = c
	}
	if err := newFinisherUnderTest(t, jobs, clips).Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, kept := clips.staged["job"]; !kept || len(clips.staged) != 1 {
		t.Fatal("only the running job's stage survives", clips.staged)
	}
}
