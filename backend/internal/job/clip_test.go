package job_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func clipInput(t *testing.T, h *harness, id, user string) job.NewJob {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := h.handle.Writer.Exec("INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES (?,?,'test','square',15000,?,?)", id, user, now, now)
	if err != nil {
		t.Fatal(err)
	}
	return job.NewJob{Kind: job.KindGenerateClip, UserID: user, ClipProjectID: id, ObserveModel: "p/observe", WriteModel: "p/write"}
}
func TestClipDeferredAdmissionAndConcurrentCallAllowance(t *testing.T) {
	h := newHarness(t)
	a := &recordingAdmitter{}
	h.queue.Admit(a)
	ctx := context.Background()
	input := clipInput(t, h, "clip", "alice")
	id, err := h.queue.Enqueue(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.starts) != 0 {
		t.Fatal("held before probing")
	}
	if _, err = h.store.PickNextQueued(ctx, time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("unlinked job dispatched", err)
	}
	calls := []job.PlannedCall{{Ref: "p/observe", Count: 3, CompletionTokens: 8192}, {Ref: "p/write", Count: 1, CompletionTokens: 32768}}
	if _, err = h.queue.ReserveClip(ctx, "alice", id, calls, approvedClipCalls(3)); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("queued job reserved", err)
	}
	if err = h.queue.ActivateClip(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	j, err := h.store.PickNextQueued(ctx, time.Now())
	if err != nil || j.Stage != "prepare" || j.ClipProjectID != "clip" {
		t.Fatal(j, err)
	}
	if _, err = h.queue.ReserveClip(ctx, "bob", id, calls, approvedClipCalls(3)); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("foreign allowance", err)
	}
	admitted, err := h.queue.ReserveClip(ctx, "alice", id, calls, approvedClipCalls(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.starts) != 1 || a.starts[0].Calls[0].Count != 3 || a.starts[0].Calls[1].CompletionTokens != 32768 {
		t.Fatal(a.starts)
	}
	if _, err = h.queue.ReserveClip(ctx, "alice", id, calls); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("legacy reservation accepted", err)
	}
	if _, err = job.ConsumeClipPolicy(admitted, "alice", id, "p/observe", 8192, "write"); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("wrong stage consumed allowance", err)
	}
	if a.starts[0].Clip == nil || a.starts[0].Clip.ApprovedMaxCredits != 100 || a.starts[0].Clip.Calls[0].Policy.InputUSDPerMillion != "0.1" {
		t.Fatal("admission lost approved pricing")
	}
	for _, bad := range []struct {
		user, id, ref string
		budget        int
	}{{"bob", id, "p/observe", 8192}, {"alice", "other", "p/observe", 8192}, {"alice", id, "p/other", 8192}, {"alice", id, "p/observe", 8193}} {
		if err = job.ConsumeClipCall(admitted, bad.user, bad.id, bad.ref, bad.budget); !errors.Is(err, job.ErrCreditAllowance) {
			t.Fatal(bad, err)
		}
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if job.ConsumeClipCall(admitted, "alice", id, "p/observe", 8192) == nil {
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 3 {
		t.Fatal("call allowance overspent", successes.Load())
	}
	if err = job.ConsumeClipCall(admitted, "alice", id, "p/write", 32768); err != nil {
		t.Fatal(err)
	}
	if err = job.ConsumeClipCall(admitted, "alice", id, "p/write", 32768); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("unplanned repair", err)
	}
	if err = job.ConsumeClipCall(ctx, "alice", id, "p/write", 32768); !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal("unreserved call", err)
	}
}
func TestClipOwnerGuardPerProjectAndUnactivatedRecovery(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	one := clipInput(t, h, "one", "alice")
	two := clipInput(t, h, "two", "alice")
	foreign := one
	foreign.UserID = "bob"
	if _, err := h.queue.Enqueue(ctx, foreign); !errors.Is(err, job.ErrInvalidTarget) {
		t.Fatal(err)
	}
	id, err := h.queue.Enqueue(ctx, one)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.queue.Enqueue(ctx, one); !errors.Is(err, job.ErrActiveConflict) {
		t.Fatal(err)
	}
	if _, err = h.queue.Enqueue(ctx, two); err != nil {
		t.Fatal("separate projects must not share account-kind guard", err)
	}
	if n, err := h.queue.SweepUnactivatedClips(ctx); n != 2 || err != nil {
		t.Fatal(n, err)
	}
	j, err := h.queue.Get(ctx, id, "alice")
	if err != nil || j.Status != "failed" || j.Failure.Reason != job.FailureReasonInterrupted {
		t.Fatal(j, err)
	}
}
func TestClipRefusedReservationReturnsNoAllowance(t *testing.T) {
	h := newHarness(t)
	a := &recordingAdmitter{refuse: errors.New("not enough credits")}
	h.queue.Admit(a)
	ctx := context.Background()
	id, err := h.queue.Enqueue(ctx, clipInput(t, h, "clip", "alice"))
	if err != nil {
		t.Fatal(err)
	}
	if err = h.queue.ActivateClip(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	if _, err = h.store.PickNextQueued(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	admitted, err := h.queue.ReserveClip(ctx, "alice", id, []job.PlannedCall{{Ref: "p/observe", Count: 1, CompletionTokens: 8192}, {Ref: "p/write", Count: 1, CompletionTokens: 32768}}, approvedClipCalls(1))
	if admitted != nil || !errors.Is(err, a.refuse) {
		t.Fatal(admitted, err)
	}
}

func approvedClipCalls(count int) job.ClipReservation {
	r := job.ClipReservation{ApprovedMaxCredits: 100, Calls: []job.ClipCall{
		{Policy: llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "p", ModelID: "observe"}, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, Count: count},
		{Policy: llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "p", ModelID: "write"}, Stage: "write", CompletionTokens: 32768, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, Count: 1},
	}}
	for i := range r.Calls {
		delivery := llm.ExecutionInlineStatic
		if i == 1 {
			delivery = llm.ExecutionTextOnly
		}
		r.Calls[i].Policy.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: delivery, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: "0.1", CompletionUSDPerMillion: "0.7", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}
	}
	return r
}
