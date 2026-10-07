package job_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

func stableInput() job.NewJob {
	return job.NewJob{UserID: "alice", Kind: deferredKind, TargetLanguage: "ko", Payload: []byte(`{"private":"source"}`),
		PricingCalls: []job.PlannedCall{{Ref: "p/writer", Stage: "write", Count: 16, PromptTokens: 12000, CompletionTokens: 9000}}}
}

func TestStableEnqueueConcurrentLostResponsesReserveOnceAndWaitForActivation(t *testing.T) {
	h := newHarness(t)
	a := &recordingAdmitter{}
	h.queue.Admit(a)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			id, err := h.queue.EnqueueWithID(t.Context(), "stable", stableInput())
			if err != nil || id != "stable" {
				t.Errorf("start %q: %v", id, err)
			}
		})
	}
	wg.Wait()
	if len(a.starts) != 1 || len(a.starts[0].Calls) != 1 || a.starts[0].Calls[0].Count != 16 {
		t.Fatal("not exactly one full admission", a.starts)
	}
	if _, err := h.store.PickNextQueued(t.Context(), time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("unbound job dispatched", err)
	}
	if err := h.queue.Activate(t.Context(), "alice", "stable"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.PickNextQueued(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.EnqueueWithID(t.Context(), "stable", stableInput()); err != nil || len(a.starts) != 1 {
		t.Fatal("response retry took another hold", err, a.starts)
	}
}

type responseLostIdentityStore struct{ *jobstore.Store }

func (s responseLostIdentityStore) InsertWithIdentity(ctx context.Context, j job.Job, i job.EnqueueIdentity) error {
	if err := s.Store.InsertWithIdentity(ctx, j, i); err != nil {
		return err
	}
	return errors.New("response lost after commit")
}
func TestStableEnqueueReadsBackCommitAndRetainsReceiptAfterTerminalPayloadPurge(t *testing.T) {
	h := newHarness(t)
	q := job.New(responseLostIdentityStore{h.store}, time.Second, nil)
	a := &recordingAdmitter{}
	q.Admit(a)
	if id, err := q.EnqueueWithID(t.Context(), "lost", stableInput()); id != "lost" || err != nil {
		t.Fatal(id, err)
	}
	if _, err := h.handle.Writer.ExecContext(t.Context(), "UPDATE generation_jobs SET status='done',payload='' WHERE id='lost'"); err != nil {
		t.Fatal(err)
	}
	if id, err := q.EnqueueWithID(t.Context(), "lost", stableInput()); id != "lost" || err != nil || len(a.starts) != 1 || len(a.released) != 0 {
		t.Fatal(id, err, a)
	}
	var receipts, jobs int
	if err := h.handle.Reader.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM job_enqueue_receipts WHERE job_id='lost'").Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM generation_jobs WHERE id='lost'").Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if receipts != 1 || jobs != 1 {
		t.Fatal(receipts, jobs)
	}
}

func TestStableEnqueueRejectsChangedOwnerPayloadPlanFlagsAndMissingJob(t *testing.T) {
	h := newHarness(t)
	a := &recordingAdmitter{}
	h.queue.Admit(a)
	if _, err := h.queue.EnqueueWithID(t.Context(), "stable", stableInput()); err != nil {
		t.Fatal(err)
	}
	changes := []func(*job.NewJob){
		func(i *job.NewJob) { i.UserID = "bob" }, func(i *job.NewJob) { i.Payload = []byte("changed") },
		func(i *job.NewJob) { i.PricingCalls[0].CompletionTokens++ }, func(i *job.NewJob) { i.PricingCalls[0].Count = 15 },
		func(i *job.NewJob) { i.TargetLanguage = "en" }, func(i *job.NewJob) { i.CancellationPolicyVersion = 1 },
		func(i *job.NewJob) { i.NonMetered = true }, func(i *job.NewJob) { i.DeferHold = true },
	}
	for _, change := range changes {
		input := stableInput()
		change(&input)
		if _, err := h.queue.EnqueueWithID(t.Context(), "stable", input); !errors.Is(err, job.ErrEnqueueIdentityConflict) {
			t.Fatal("changed admission accepted", err)
		}
	}
	if len(a.starts) != 1 {
		t.Fatal("changed inputs reserved", a.starts)
	}
	if _, err := h.handle.Writer.ExecContext(t.Context(), "DELETE FROM generation_jobs WHERE id='stable'"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.EnqueueWithID(t.Context(), "stable", stableInput()); !errors.Is(err, job.ErrEnqueueIdentityConflict) {
		t.Fatal("missing job was recreated", err)
	}
	if len(a.starts) != 1 {
		t.Fatal("missing row reserved again", a.starts)
	}
}

func TestStableEnqueueForeignJobWithoutReceiptCannotBeOverwritten(t *testing.T) {
	h := newHarness(t)
	found := job.Job{ID: "unreceipted", Kind: deferredKind, UserID: "alice", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := h.store.Insert(t.Context(), found); err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.EnqueueWithID(t.Context(), found.ID, stableInput()); !errors.Is(err, job.ErrEnqueueIdentityConflict) {
		t.Fatal(err)
	}
}
