package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
)

// The queue's own row lifecycle, over a migrated temp database: what a worker writes as it
// picks a job up, reports on it and finishes it, and what the boot sweeps find.
func TestQueuedJobRunsThroughProgressToADurableFailure(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	insert(t, store, job.Job{ID: "run", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	now := time.Now().UTC()

	picked, err := store.PickNextQueued(ctx, now)
	if err != nil || picked.ID != "run" || picked.Status != job.StatusRunning {
		t.Fatalf("pick = %+v, %v; want the queued job as running", picked, err)
	}
	if _, err := store.PickNextQueued(ctx, now); err == nil {
		t.Fatal("a second pick found work; the queue held only one job")
	}
	if err := store.UpdateProgress(ctx, "run", "observe", 3, 7, now); err != nil {
		t.Fatalf("progress: %v", err)
	}
	mid, err := store.GetByID(ctx, "run")
	if err != nil || mid.Stage != "observe" || mid.ProgressDone != 3 || mid.ProgressTotal != 7 {
		t.Fatalf("progress row = %+v, %v", mid, err)
	}

	// A failure is stored in its three columns and comes back as the same structured value.
	failure := job.Failure{Reason: "PROVIDER_UNAVAILABLE", Params: map[string]string{"provider": "openai"}}
	if err := store.Finish(ctx, "run", job.StatusFailed, &failure, now); err != nil {
		t.Fatalf("finish: %v", err)
	}
	done, err := store.GetByID(ctx, "run")
	if err != nil || done.Status != job.StatusFailed || done.Failure == nil {
		t.Fatalf("finished row = %+v, %v", done, err)
	}
	if done.Failure.Reason != failure.Reason || done.Failure.Params["provider"] != "openai" {
		t.Fatalf("failure round trip = %+v", done.Failure)
	}
	if done.FinishedAt == nil {
		t.Fatal("a finished job carries no finished_at")
	}
}

func TestFinishRefusesAContradictionAndAnAlreadyFinishedImmediateJob(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, store, job.Job{ID: "run", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	if _, err := store.PickNextQueued(ctx, now); err != nil {
		t.Fatalf("pick: %v", err)
	}
	// A failed status without a failure, and a done status carrying one, are refused before
	// they reach the database: the row would say two different things.
	if err := store.Finish(ctx, "run", job.StatusFailed, nil, now); err == nil {
		t.Fatal("failed status without a failure was accepted")
	}
	if err := store.Finish(ctx, "run", job.StatusDone, &job.Failure{Reason: "UNKNOWN_FAILURE"}, now); err == nil {
		t.Fatal("done status carrying a failure was accepted")
	}
	if err := store.Finish(ctx, "run", job.StatusDone, nil, now); err != nil {
		t.Fatalf("finish: %v", err)
	}
	// An immediate kind may only be finished once; a deferred kind may be finished twice with
	// the same terminal status, because its cancellation can race the worker.
	if err := store.Finish(ctx, "run", job.StatusDone, nil, now); err == nil {
		t.Fatal("an immediate job was finished twice")
	}
}

func TestFailQueuedIsOwnerScopedAndOnlyTouchesQueuedWork(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, store, job.Job{ID: "waiting", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	failure := job.Failure{Reason: "QUOTA_EXCEEDED"}

	if failed, err := store.FailQueued(ctx, "waiting", "bob", failure, now); err != nil || failed {
		t.Fatalf("another account failed the job: %v, %v", failed, err)
	}
	failed, err := store.FailQueued(ctx, "waiting", "alice", failure, now)
	if err != nil || !failed {
		t.Fatalf("owner fail = %v, %v", failed, err)
	}
	after, err := store.GetByID(ctx, "waiting")
	if err != nil || after.Status != job.StatusFailed || after.Failure.Reason != "QUOTA_EXCEEDED" {
		t.Fatalf("failed row = %+v, %v", after, err)
	}
	if failed, err := store.FailQueued(ctx, "waiting", "alice", failure, now); err != nil || failed {
		t.Fatalf("a finished job was failed again: %v, %v", failed, err)
	}
}

func TestSweepRunningFailsWhatARestartAbandoned(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, store, job.Job{ID: "running", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	insert(t, store, job.Job{ID: "queued", Kind: job.KindGenerate, UserID: "bob", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-bob"}}})
	if _, err := store.PickNextQueued(ctx, now); err != nil {
		t.Fatalf("pick: %v", err)
	}

	swept, err := store.SweepRunning(ctx, job.Failure{Reason: "INTERNAL"}, now)
	if err != nil || swept != 1 {
		t.Fatalf("sweep = %d, %v; want only the running job", swept, err)
	}
	waiting, err := store.GetByID(ctx, "queued")
	if err != nil || waiting.Status != job.StatusQueued {
		t.Fatalf("the sweep touched queued work: %+v, %v", waiting, err)
	}
}

// The serialization the review named: one model call against one cancellation request, decided
// by the writer connection rather than by who read the row first.
func TestDispatchAuthorizationSerializesWithCancellation(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, store, job.Job{ID: "clip-job", Kind: "generate_clip", UserID: "alice",
		ObserveModel: "p/o", WriteModel: "p/w", CancellationPolicyVersion: 1,
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})
	if _, err := store.Activate(ctx, "alice", "clip-job"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := store.PickNextQueued(ctx, now); err != nil {
		t.Fatalf("pick: %v", err)
	}

	// Before any cancellation the call is authorized, and authorizing twice is not a race
	// with itself — the same worker may dispatch several calls for one job.
	if err := store.AuthorizeDispatch(ctx, "alice", "clip-job"); err != nil {
		t.Fatalf("first authorization: %v", err)
	}
	if err := store.AuthorizeDispatch(ctx, "alice", "clip-job"); err != nil {
		t.Fatalf("second authorization: %v", err)
	}
	// Another account cannot authorize this job's calls at all.
	if err := store.AuthorizeDispatch(ctx, "bob", "clip-job"); !errors.Is(err, job.ErrDispatchRefused) {
		t.Fatalf("foreign authorization = %v; want refused", err)
	}

	if err := store.RequestCancellation(ctx, "alice", "clip-alice", "clip-job", now); err != nil {
		t.Fatalf("cancellation: %v", err)
	}
	// The owner got there first: every further call is refused rather than spent.
	if err := store.AuthorizeDispatch(ctx, "alice", "clip-job"); !errors.Is(err, job.ErrDispatchRefused) {
		t.Fatalf("authorization after cancellation = %v; want refused", err)
	}
}

func TestRecoverCancellationsReleasesRequestsNobodyFinished(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, store, job.Job{ID: "clip-job", Kind: "generate_clip", UserID: "alice",
		ObserveModel: "p/o", WriteModel: "p/w", CancellationPolicyVersion: 1,
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})
	if _, err := store.Activate(ctx, "alice", "clip-job"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := store.PickNextQueued(ctx, now); err != nil {
		t.Fatalf("pick: %v", err)
	}
	if err := store.RequestCancellation(ctx, "alice", "clip-alice", "clip-job", now); err != nil {
		t.Fatalf("cancellation: %v", err)
	}

	recovered, err := store.RecoverCancellations(ctx, now)
	if err != nil || recovered != 1 {
		t.Fatalf("recover = %d, %v; want the abandoned cancellation", recovered, err)
	}
	after, err := store.GetByID(ctx, "clip-job")
	if err != nil || after.Status != job.StatusCancelled {
		t.Fatalf("recovered row = %+v, %v", after, err)
	}
}

func TestLatestForReadsTheNewestJobOfASubjectWhateverItsStatus(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, store, job.Job{ID: "first", Kind: "render_clip", UserID: "alice",
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})
	if _, err := store.Activate(ctx, "alice", "first"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := store.PickNextQueued(ctx, now); err != nil {
		t.Fatalf("pick: %v", err)
	}
	if err := store.Finish(ctx, "first", job.StatusDone, nil, now); err != nil {
		t.Fatalf("finish: %v", err)
	}
	insert(t, store, job.Job{ID: "second", Kind: "render_clip", UserID: "alice",
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})

	// `LatestFor` answers for the clip project, the one dimension whose screen shows the last
	// attempt whatever became of it.
	subject := job.Subject{Dimension: clipProjectSubject, ID: "clip-alice"}
	latest, err := store.LatestFor(ctx, subject, job.Filter{UserID: "alice"})
	if err != nil || latest == nil || latest.ID != "second" {
		t.Fatalf("latest = %+v, %v", latest, err)
	}
	active, err := store.ActiveFor(ctx, subject, job.Filter{UserID: "alice"})
	if err != nil || active == nil || active.ID != "second" {
		t.Fatalf("active = %+v, %v; the finished job is not active", active, err)
	}
	foreign, err := store.LatestFor(ctx, subject, job.Filter{UserID: "bob"})
	if err != nil || foreign != nil {
		t.Fatalf("another account read the job: %+v, %v", foreign, err)
	}
}
