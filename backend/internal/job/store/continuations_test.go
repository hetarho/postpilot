package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

func TestContinuationTransactionalParkAndWake(t *testing.T) {
	s, d := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, s, job.Job{ID: "park", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko", Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	if err := s.Park(ctx, "park", "key", job.FailOnInterrupt, now); !errors.Is(err, job.ErrDispatchRefused) {
		t.Fatalf("queued park: %v", err)
	}
	if _, err := s.PickNextQueued(ctx, now); err != nil {
		t.Fatal(err)
	}
	tx, err := d.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	bound := jobstore.NewTx(tx, testKinds())
	if err := bound.Park(ctx, "park", "key", job.FailOnInterrupt, now); err != nil {
		t.Fatal(err)
	}
	if ready, err := bound.Wake(ctx, "park", "key", now); err != nil || !ready {
		t.Fatalf("transaction wake: %v %v", ready, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if waiting, err := s.HasPendingWait(ctx, "park"); err != nil || waiting {
		t.Fatal("rolled back wait survived", err)
	}
	if err := s.Park(ctx, "park", "key", "", now); err != nil {
		t.Fatal(err)
	}
	if ready, err := s.Wake(ctx, "park", "wrong", now); err != nil || ready {
		t.Fatal("wrong key woke work", err)
	}
	if ready, err := s.Wake(ctx, "park", "key", now); err != nil || !ready {
		t.Fatal(err)
	}
	if err := s.Park(ctx, "park", "key", job.FailOnInterrupt, now.Add(time.Hour)); err != nil {
		t.Fatal("duplicate park lost wake", err)
	}
	if err := s.Park(ctx, "park", "different", job.FailOnInterrupt, now); !errors.Is(err, job.ErrInvalidWait) {
		t.Fatal("pending wait overwritten", err)
	}
	var readyAt string
	if err := d.Reader.QueryRow(`SELECT ready_at FROM job_continuations WHERE job_id='park'`).Scan(&readyAt); err != nil {
		t.Fatal(err)
	}
	if readyAt != now.Format("2006-01-02T15:04:05.000000000Z07:00") {
		t.Fatal("duplicate park reset readiness")
	}
	resumed, err := s.PickNextQueued(ctx, now)
	if err != nil || resumed.Resume == nil || resumed.Resume.WaitKey != "key" {
		t.Fatalf("resume: %+v %v", resumed, err)
	}
	if changed, err := s.Wake(ctx, "park", "key", now); err != nil || changed {
		t.Fatal("claimed continuation woke twice", err)
	}
	if err := s.Park(ctx, "park", "next", job.ReplaySafe, now); err != nil {
		t.Fatal("next stage cannot park", err)
	}
	if changed, err := s.Wake(ctx, "park", "key", now); err != nil || changed {
		t.Fatal("old stage woke new wait", err)
	}
}

func TestContinuationCompetingWakesAndClaims(t *testing.T) {
	s, _ := subjectHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	insert(t, s, job.Job{ID: "race", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko", Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})
	first, err := s.PickNextQueued(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateProgress(ctx, "race", "external", 2, 3, now); err != nil {
		t.Fatal(err)
	}
	if err := s.Park(ctx, "race", "key", job.FailOnInterrupt, now); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wakes := make(chan bool, 8)
	for range 8 {
		wg.Go(func() {
			changed, err := s.Wake(ctx, "race", "key", now)
			if err != nil {
				t.Error(err)
			}
			wakes <- changed
		})
	}
	wg.Wait()
	close(wakes)
	count := 0
	for changed := range wakes {
		if changed {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("wake winners: %d", count)
	}
	claims := make(chan job.Job, 8)
	for range 8 {
		wg.Go(func() {
			j, err := s.PickNextQueued(ctx, now)
			if err == nil {
				claims <- j
			} else if !errors.Is(err, job.ErrNotFound) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	close(claims)
	if len(claims) != 1 {
		t.Fatalf("claim winners: %d", len(claims))
	}
	resumed := <-claims
	if resumed.Stage != "external" || resumed.ProgressDone != 2 || !resumed.StartedAt.Equal(*first.StartedAt) || resumed.Resume == nil {
		t.Fatalf("resume lost execution identity: %+v", resumed)
	}
	if err := s.Finish(ctx, "race", job.StatusDone, nil, now); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.Wake(ctx, "race", "key", now); err != nil || changed {
		t.Fatal("terminal job reopened", err)
	}
}

func TestContinuationBootRecoveryPoliciesAndCancellation(t *testing.T) {
	// Migrate once: each subcase owns and removes its job (including the
	// cascading continuation), so the state matrix stays isolated without
	// replaying all historical migrations twelve times under the race detector.
	_, d := subjectHarness(t)
	for _, policy := range []job.ResumePolicy{job.FailOnInterrupt, job.ReplaySafe} {
		for _, state := range []string{"waiting", "ready", "claimed"} {
			for _, cancelled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/cancel=%v", policy, state, cancelled), func(t *testing.T) {
					s := jobstore.New(d.Writer, d.Reader, testKinds())
					t.Cleanup(func() {
						if _, err := d.Writer.Exec(`DELETE FROM generation_jobs WHERE id='boot'`); err != nil {
							t.Error(err)
						}
					})
					ctx := context.Background()
					now := time.Now().UTC()
					insert(t, s, job.Job{ID: "boot", Kind: "render_clip", UserID: "alice", Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})
					if _, err := s.Activate(ctx, "alice", "boot"); err != nil {
						t.Fatal(err)
					}
					if _, err := s.PickNextQueued(ctx, now); err != nil {
						t.Fatal(err)
					}
					if err := s.Park(ctx, "boot", "key", policy, now); err != nil {
						t.Fatal(err)
					}
					if state != "waiting" {
						if _, err := s.Wake(ctx, "boot", "key", now); err != nil {
							t.Fatal(err)
						}
					}
					if state == "claimed" {
						if _, err := s.PickNextQueued(ctx, now); err != nil {
							t.Fatal(err)
						}
					}
					if cancelled {
						if err := s.RequestCancellation(ctx, "alice", "clip-alice", "boot", now); err != nil {
							t.Fatal(err)
						}
					}
					// A new store has no in-memory waiter or callback from the old process.
					s = jobstore.New(d.Writer, d.Reader, testKinds())
					if _, err := s.RecoverCancellations(ctx, now); err != nil {
						t.Fatal(err)
					}
					if _, err := s.SweepRunning(ctx, job.Failure{Reason: "JOB_INTERRUPTED"}, now); err != nil {
						t.Fatal(err)
					}
					j, err := s.GetByID(ctx, "boot")
					if err != nil {
						t.Fatal(err)
					}
					unsafe := state == "claimed" && policy == job.FailOnInterrupt
					if unsafe {
						want := job.StatusFailed
						if cancelled {
							want = job.StatusCancelled
						}
						if j.Status != want {
							t.Fatalf("interrupted status %s want %s", j.Status, want)
						}
						if !cancelled && (j.Failure == nil || j.Failure.Reason != "JOB_INTERRUPTED") {
							t.Fatal("unsafe provider continuation replayed")
						}
						return
					}
					if j.Status != job.StatusRunning || (j.CancelRequestedAt != nil) != cancelled {
						t.Fatalf("durable wait not preserved: %+v", j)
					}
					if cancelled {
						if _, err := s.PickNextQueued(ctx, now); !errors.Is(err, job.ErrNotFound) {
							t.Fatal("cancelled work dispatched", err)
						}
						if changed, err := s.AcknowledgeWaitCancellation(ctx, "boot", "wrong", now); err != nil || changed {
							t.Fatal("wrong acknowledgement", err)
						}
						if changed, err := s.AcknowledgeWaitCancellation(ctx, "boot", "key", now); err != nil || !changed {
							t.Fatal("cancellation not acknowledged", err)
						}
						if changed, err := s.AcknowledgeWaitCancellation(ctx, "boot", "key", now); err != nil || changed {
							t.Fatal("acknowledged twice", err)
						}
					} else if state != "waiting" {
						resumed, err := s.PickNextQueued(ctx, now)
						if err != nil || resumed.Resume == nil {
							t.Fatal("safe/ready continuation lost", err)
						}
					}
				})
			}
		}
	}
}
