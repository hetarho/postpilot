package job_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

type continuationAdmitter struct{ holds, settles atomic.Int32 }

func (a *continuationAdmitter) Hold(context.Context, job.Start) error     { a.holds.Add(1); return nil }
func (*continuationAdmitter) Release(context.Context, string)             {}
func (a *continuationAdmitter) Settle(context.Context, string, string)    { a.settles.Add(1) }
func (*continuationAdmitter) OpenHolds(context.Context) ([]string, error) { return nil, nil }

func runContinuationQueue(t *testing.T, q *job.Queue) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); q.Run(ctx) }()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(stateDeadline):
			t.Error("queue did not stop")
		}
	}
	t.Cleanup(stop)
	return stop
}
func awaitContinuation(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(stateDeadline):
		t.Fatal("external wait blocked unrelated work")
	}
}

func TestContinuationYieldFreesWorkerAndResumesSameParent(t *testing.T) {
	for _, wakeBeforeYield := range []bool{false, true} {
		t.Run(fmt.Sprint(wakeBeforeYield), func(t *testing.T) {
			h := newHarness(t)
			ctx := context.Background()
			admission := &continuationAdmitter{}
			h.queue.Admit(admission)
			var calls, hooks atomic.Int32
			parked, otherRan := make(chan struct{}), make(chan struct{})
			h.queue.OnTerminal(job.KindGenerate, func(context.Context, job.Job, time.Time) error { hooks.Add(1); return nil })
			h.queue.Register(job.KindGenerate, func(ctx context.Context, j job.Job, _ job.Progress) error {
				calls.Add(1)
				if j.Resume != nil {
					if j.Resume.WaitKey != "external-key" || j.Resume.Policy != job.FailOnInterrupt {
						return fmt.Errorf("wrong continuation: %+v", j.Resume)
					}
					return nil
				}
				if err := h.queue.Park(ctx, j.ID, "external-key", job.FailOnInterrupt); err != nil {
					return err
				}
				if wakeBeforeYield {
					if changed, err := h.queue.Wake(ctx, j.ID, "external-key"); err != nil || !changed {
						return fmt.Errorf("early wake: %v %v", changed, err)
					}
				}
				close(parked)
				return job.ErrYield
			})
			h.queue.Register(job.KindRevise, func(context.Context, job.Job, job.Progress) error { close(otherRan); return nil })
			id, err := h.queue.Enqueue(ctx, attach(job.NewJob{Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko"}, "post-a", ""))
			if err != nil {
				t.Fatal(err)
			}
			other, err := h.queue.Enqueue(ctx, attach(job.NewJob{Kind: job.KindRevise, UserID: "alice", TargetLanguage: "ko"}, "post-b", ""))
			if err != nil {
				t.Fatal(err)
			}
			stop := runContinuationQueue(t, h.queue)
			awaitContinuation(t, parked)
			awaitContinuation(t, otherRan)
			waitFor(t, h.queue, other, "alice", func(j *job.JobSummary) bool { return j.Status == job.StatusDone })
			if !wakeBeforeYield {
				waiting, err := h.store.GetByID(ctx, id)
				if err != nil || waiting.Status != job.StatusRunning || waiting.FinishedAt != nil || hooks.Load() != 0 {
					t.Fatalf("yield finished/released the parent: %+v %v", waiting, err)
				}
				if changed, err := h.queue.Wake(ctx, id, "wrong"); err != nil || changed {
					t.Fatal("wrong wake", err)
				}
				if changed, err := h.queue.Wake(ctx, id, "external-key"); err != nil || !changed {
					t.Fatal("wake failed", err)
				}
			}
			waitFor(t, h.queue, id, "alice", func(j *job.JobSummary) bool { return j.Status == job.StatusDone })
			stop()
			if calls.Load() != 2 || hooks.Load() != 1 || admission.holds.Load() != 2 || admission.settles.Load() != 2 {
				t.Fatalf("calls=%d hooks=%d holds=%d settles=%d", calls.Load(), hooks.Load(), admission.holds.Load(), admission.settles.Load())
			}
			if changed, err := h.queue.Wake(ctx, id, "external-key"); err != nil || changed {
				t.Fatal("terminal wake", err)
			}
		})
	}
}

func TestContinuationYieldRequiresDurableWait(t *testing.T) {
	h := newHarness(t)
	h.queue.Register(job.KindGenerate, func(context.Context, job.Job, job.Progress) error { return job.ErrYield })
	id, err := h.queue.Enqueue(context.Background(), attach(job.NewJob{Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko"}, "post-a", ""))
	if err != nil {
		t.Fatal(err)
	}
	runContinuationQueue(t, h.queue)
	waitFor(t, h.queue, id, "alice", func(j *job.JobSummary) bool { return j.Status == job.StatusFailed })
}

type continuationCancellation struct{}

func (continuationCancellation) Kind(kind string) bool           { return kind == "render_clip" }
func (continuationCancellation) Allowed(kind string, _ int) bool { return kind == "render_clip" }

func TestContinuationCancellationWaitsForOwnerAcknowledgement(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := h.handle.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('external-project','alice','clip','square',15000,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	h.store = jobstore.New(h.handle.Writer, h.handle.Reader, jobstore.Kinds{Cancellable: []string{"render_clip"}})
	h.queue = job.New(h.store, 10*time.Millisecond, reportingForTest{})
	h.queue.AllowCancellation(continuationCancellation{})
	admission := &continuationAdmitter{}
	h.queue.Admit(admission)
	var hooks atomic.Int32
	parked := make(chan struct{})
	returnYield := make(chan struct{})
	defer close(returnYield)
	h.queue.OnTerminal("render_clip", func(context.Context, job.Job, time.Time) error { hooks.Add(1); return nil })
	h.queue.Register("render_clip", func(ctx context.Context, j job.Job, _ job.Progress) error {
		if err := h.queue.Park(ctx, j.ID, "external", job.ReplaySafe); err != nil {
			return err
		}
		close(parked)
		<-returnYield
		return job.ErrYield
	})
	subject := job.Subject{Dimension: "clip_project", ID: "external-project"}
	id, err := h.queue.Enqueue(ctx, job.NewJob{Kind: "render_clip", UserID: "alice", Subjects: []job.Subject{subject}})
	if err != nil {
		t.Fatal(err)
	}
	stop := runContinuationQueue(t, h.queue)
	awaitContinuation(t, parked)
	cancelled, err := h.queue.Cancel(ctx, "alice", subject, id)
	if err != nil || cancelled.Status != job.StatusRunning {
		t.Fatalf("premature cancellation: %+v %v", cancelled, err)
	}
	if _, err := h.queue.SweepRunning(ctx); err != nil {
		t.Fatal(err)
	}
	if hooks.Load() != 0 || admission.settles.Load() != 0 {
		t.Fatal("external resources released before acknowledgement")
	}
	if changed, err := h.queue.Wake(ctx, id, "external"); err != nil || changed {
		t.Fatal("cancelled provider continuation became runnable", err)
	}
	if changed, err := h.queue.AcknowledgeWaitCancellation(ctx, id, "wrong"); err != nil || changed {
		t.Fatal("wrong wait acknowledged", err)
	}
	// The external owner establishes stopped/expired work before this call.
	if changed, err := h.queue.AcknowledgeWaitCancellation(ctx, id, "external"); err != nil || !changed {
		t.Fatal("acknowledgement failed", err)
	}
	if changed, err := h.queue.AcknowledgeWaitCancellation(ctx, id, "external"); err != nil || changed {
		t.Fatal("duplicate acknowledgement", err)
	}
	waitFor(t, h.queue, id, "alice", func(j *job.JobSummary) bool { return j.Status == job.StatusCancelled })
	// Release the yielding handler only after the external acknowledgement;
	// its return must not settle or notify the already-terminal job again.
	returnYield <- struct{}{}
	stop()
	if hooks.Load() != 1 || admission.settles.Load() != 1 {
		t.Fatalf("duplicate terminal work: hooks=%d settles=%d", hooks.Load(), admission.settles.Load())
	}
}
