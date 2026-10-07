package store_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type stableLedgerGate struct {
	service  *usage.Service
	holds    atomic.Int64
	releases atomic.Int64
}

func (a *stableLedgerGate) Hold(ctx context.Context, s job.Start) error {
	a.holds.Add(1)
	calls := make([]usage.PlannedCall, len(s.Calls))
	for i, c := range s.Calls {
		p, m, _ := strings.Cut(c.Ref, "/")
		calls[i] = usage.PlannedCall{Ref: llm.ModelRef{ProviderID: p, ModelID: m}, Stage: c.Stage, Count: c.Count, PromptTokens: int64(c.PromptTokens), CompletionTokens: int64(c.CompletionTokens)}
	}
	return a.service.Hold(ctx, usage.Start{UserID: s.UserID, Plan: plan.Free, Kind: s.Kind, JobID: s.JobID, Calls: calls})
}
func (a *stableLedgerGate) Release(ctx context.Context, id string) {
	a.releases.Add(1)
	_ = a.service.Release(ctx, id)
}
func (a *stableLedgerGate) Settle(ctx context.Context, id, status string) {
	_ = a.service.Settle(ctx, id, usage.OutcomeSucceeded)
}
func (a *stableLedgerGate) OpenHolds(ctx context.Context) ([]string, error) {
	return a.service.OpenHolds(ctx)
}

func exactStableJob(owner string, budget int) job.NewJob {
	return job.NewJob{Kind: "bounded_test_work", UserID: owner, Payload: []byte("immutable"), PricingCalls: []job.PlannedCall{{Ref: pricedRef.String(), Stage: "write", Count: 1, PromptTokens: 30000, CompletionTokens: budget}}}
}

func TestSQLiteStableIdentityBeforeHoldRejectsEqualCreditDifferentPlansAcrossQueues(t *testing.T) {
	for _, differentOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget", true: "owner"}[differentOwner], func(t *testing.T) {
			svc, h := newServiceWithDB(t)
			gate := &stableLedgerGate{service: svc}
			store := jobstore.New(h.Writer, h.Reader, jobstore.Kinds{Deferred: []string{"bounded_test_work"}})
			queues := []*job.Queue{job.New(store, time.Second, nil), job.New(store, time.Second, nil)}
			for _, q := range queues {
				q.Admit(gate)
			}
			inputs := []job.NewJob{exactStableJob("alice", 1000), exactStableJob("alice", 1001)}
			if differentOwner {
				inputs[1].UserID = "bob"
			}
			var wg sync.WaitGroup
			results := make([]error, 2)
			start := make(chan struct{})
			for i := range queues {
				wg.Go(func() { <-start; _, results[i] = queues[i].EnqueueWithID(t.Context(), "same-stable-id", inputs[i]) })
			}
			close(start)
			wg.Wait()
			wins := 0
			for _, err := range results {
				if err == nil {
					wins++
				} else if !errors.Is(err, job.ErrEnqueueIdentityConflict) {
					t.Fatal(err)
				}
			}
			if wins != 1 || gate.holds.Load() != 1 {
				t.Fatal("changed plan reached ledger", wins, gate.holds.Load(), results)
			}
			var admissions int
			if err := h.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions WHERE job_id='same-stable-id'").Scan(&admissions); err != nil || admissions != 1 {
				t.Fatal(admissions, err)
			}
		})
	}
}

type orphanedIdentityStore struct{ *jobstore.Store }

func (s orphanedIdentityStore) InsertWithIdentity(context.Context, job.Job, job.EnqueueIdentity) error {
	return errors.New("row did not commit")
}
func TestSQLiteOrphanSweepAbandonsStableClaimBeforeReleaseAndPreventsUnreservedReplay(t *testing.T) {
	svc, h := newServiceWithDB(t)
	store := jobstore.New(h.Writer, h.Reader, jobstore.Kinds{Deferred: []string{"bounded_test_work"}})
	q := job.New(orphanedIdentityStore{store}, time.Second, nil)
	gate := &stableLedgerGate{service: svc}
	q.Admit(gate)
	input := exactStableJob("alice", 1000)
	if _, err := q.EnqueueWithID(t.Context(), "orphan", input); err == nil {
		t.Fatal("insert failure hidden")
	}
	if n, err := q.SweepOpenHolds(t.Context()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err := q.EnqueueWithID(t.Context(), "orphan", input); !errors.Is(err, job.ErrEnqueueIdentityConflict) {
		t.Fatal("released claim resumed", err)
	}
	if gate.holds.Load() != 1 || gate.releases.Load() != 1 {
		t.Fatal(gate.holds.Load(), gate.releases.Load())
	}
	var jobs int
	if err := h.Reader.QueryRow("SELECT COUNT(*) FROM generation_jobs WHERE id='orphan'").Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal(jobs, err)
	}
}

type rowCommitsDuringOrphanSweep struct {
	*jobstore.Store
	once      sync.Once
	owner, id string
}

func (s *rowCommitsDuringOrphanSweep) AbandonEnqueueIdentity(ctx context.Context, id string) (bool, error) {
	var insertErr error
	s.once.Do(func() {
		identity, err := s.Store.GetEnqueueIdentity(ctx, id)
		if err != nil {
			insertErr = err
			return
		}
		now := time.Now()
		insertErr = s.Store.InsertWithIdentity(ctx, job.Job{ID: id, UserID: s.owner, Kind: "bounded_test_work", Payload: []byte("immutable"), CreatedAt: now, UpdatedAt: now}, identity)
	})
	if insertErr != nil {
		return false, insertErr
	}
	return s.Store.AbandonEnqueueIdentity(ctx, id)
}
func TestSQLiteOrphanSweepCannotReleaseStableAdmissionThatCommitsDuringTheReadRace(t *testing.T) {
	svc, h := newServiceWithDB(t)
	store := jobstore.New(h.Writer, h.Reader, jobstore.Kinds{Deferred: []string{"bounded_test_work"}})
	gate := &stableLedgerGate{service: svc}
	initial := job.New(orphanedIdentityStore{store}, time.Second, nil)
	initial.Admit(gate)
	input := exactStableJob("alice", 1000)
	if _, err := initial.EnqueueWithID(t.Context(), "raced", input); err == nil {
		t.Fatal("expected admission window")
	}
	q := job.New(&rowCommitsDuringOrphanSweep{Store: store, owner: "alice", id: "raced"}, time.Second, nil)
	q.Admit(gate)
	if n, err := q.SweepOpenHolds(t.Context()); n != 0 || err != nil {
		t.Fatal("live admission was released", n, err)
	}
	if gate.releases.Load() != 0 {
		t.Fatal("live hold released")
	}
	if id, err := q.EnqueueWithID(t.Context(), "raced", input); id != "raced" || err != nil || gate.holds.Load() != 1 {
		t.Fatal("race changed admission", id, err, gate.holds.Load())
	}
}

type pausedStableGate struct {
	*stableLedgerGate
	ready, proceed chan struct{}
}

func (g *pausedStableGate) Hold(ctx context.Context, start job.Start) error {
	close(g.ready)
	<-g.proceed
	return g.stableLedgerGate.Hold(ctx, start)
}
func TestSQLiteAbandonedIdentityReleasesAHoldThatArrivesAfterTheBootSweep(t *testing.T) {
	svc, h := newServiceWithDB(t)
	store := jobstore.New(h.Writer, h.Reader, jobstore.Kinds{Deferred: []string{"bounded_test_work"}})
	gate := &stableLedgerGate{service: svc}
	initial := job.New(orphanedIdentityStore{store}, time.Second, nil)
	initial.Admit(gate)
	input := exactStableJob("alice", 1000)
	before, err := svc.BalanceFor(t.Context(), "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := initial.EnqueueWithID(t.Context(), "late-hold", input); err == nil {
		t.Fatal("expected interrupted row insert")
	}
	paused := &pausedStableGate{stableLedgerGate: gate, ready: make(chan struct{}), proceed: make(chan struct{})}
	retrying := job.New(store, time.Second, nil)
	retrying.Admit(paused)
	done := make(chan error, 1)
	go func() { _, err := retrying.EnqueueWithID(t.Context(), "late-hold", input); done <- err }()
	select {
	case <-paused.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("retry did not reach original admission")
	}
	if n, err := initial.SweepOpenHolds(t.Context()); n != 1 || err != nil {
		t.Fatal(n, err)
	}
	close(paused.proceed)
	select {
	case err := <-done:
		if !errors.Is(err, job.ErrEnqueueIdentityConflict) {
			t.Fatal("abandoned row committed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("abandoned admission did not return")
	}
	if holds, err := svc.OpenHolds(t.Context()); err != nil || len(holds) != 0 {
		t.Fatal("late reservation stranded", holds, err)
	}
	after, err := svc.BalanceFor(t.Context(), "alice", plan.Free)
	if err != nil || after.Credits != before.Credits {
		t.Fatal("never-dispatched work charged", before, after, err)
	}
}

func TestSQLiteSettledChargeUsesOwnerActualReceiptAndNeverQuote(t *testing.T) {
	svc, h := newServiceWithDB(t)
	charges := usage.NewSettledCharges(usagestore.New(h.Writer, h.Reader))
	if _, err := charges.SettledJobCharge(t.Context(), "alice", "unknown"); !errors.Is(err, usage.ErrChargeNotSettled) {
		t.Fatal(err)
	}
	if err := svc.Hold(t.Context(), holdFor("settled")); err != nil {
		t.Fatal(err)
	}
	if _, err := charges.SettledJobCharge(t.Context(), "alice", "settled"); !errors.Is(err, usage.ErrChargeNotSettled) {
		t.Fatal("pending cost became value", err)
	}
	ctx := usage.WithWork(t.Context(), usage.Work{UserID: "alice", Kind: "generate", JobID: "settled"})
	if err := svc.RecordCall(ctx, pricedRef, "write", llm.Usage{PromptTokens: 3000, CompletionTokens: 1000}, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(t.Context(), "settled", usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	charge, err := charges.SettledJobCharge(t.Context(), "alice", "settled")
	if err != nil || charge != 1 {
		t.Fatal("actual receipt", charge, err)
	}
	if _, err := charges.SettledJobCharge(t.Context(), "bob", "settled"); !errors.Is(err, usage.ErrChargeNotSettled) {
		t.Fatal("foreign receipt exposed", err)
	}
	if again, err := charges.SettledJobCharge(t.Context(), "alice", "settled"); err != nil || again != charge {
		t.Fatal(again, err)
	}
}
