package job_test

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"testing"
	"time"
)

func TestRenderClipAdmissionIsExplicitAndRejectsEveryPossibleCall(t *testing.T) {
	h := newHarness(t)
	base := clipInput(t, h, "render", "alice")
	base.Kind = job.KindRenderClip
	base.NonMetered = true
	base.ObserveModel = ""
	base.WriteModel = ""
	a := &recordingAdmitter{refuse: errors.New("no credits")}
	h.queue.Admit(a)
	ctx := context.Background()
	for _, mutate := range []func(*job.NewJob){func(n *job.NewJob) { n.NonMetered = false }, func(n *job.NewJob) { n.Kind = job.KindGenerate }, func(n *job.NewJob) { n.Kind = job.KindGenerateClip }, func(n *job.NewJob) { n.ClipProjectID = "" }, func(n *job.NewJob) { n.ObserveModel = "p/o" }, func(n *job.NewJob) { n.WriteModel = "p/w" }, func(n *job.NewJob) { n.ExtraModels = []string{"p/x"} }, func(n *job.NewJob) { n.CallCounts = map[string]int{"p/x": 0} }, func(n *job.NewJob) { n.PricingCalls = []job.PlannedCall{{Ref: "p/x", Count: 0}} }, func(n *job.NewJob) { s := "post"; n.PostSlug = &s }, func(n *job.NewJob) { n.VoiceID = "voice" }} {
		bad := base
		mutate(&bad)
		if _, err := h.queue.Enqueue(ctx, bad); !errors.Is(err, job.ErrInvalidTarget) {
			t.Fatal(bad, err)
		}
	}
	id, err := h.queue.Enqueue(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.store.PickNextQueued(ctx, time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("unlinked render dispatched", err)
	}
	if err = h.queue.ActivateClip(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	h.queue.Register(job.KindRenderClip, func(ctx context.Context, j job.Job, progress job.Progress) error {
		if _, err := h.queue.ReserveClip(ctx, j.UserID, j.ID, []job.PlannedCall{{Ref: "p/o", Count: 1, CompletionTokens: 100}}); !errors.Is(err, job.ErrCreditAllowance) {
			return errors.New("render admitted AI")
		}
		return nil
	})
	worker, cancel := context.WithCancel(ctx)
	exited := make(chan struct{})
	go func() { defer close(exited); h.queue.Run(worker) }()
	waitFor(t, h.queue, id, "alice", func(j *job.JobSummary) bool { return j.Status == job.StatusDone })
	cancel()
	<-exited
	a.open = []string{id}
	if n, err := h.queue.SweepOpenHolds(ctx); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if len(a.starts) != 0 || len(a.settled) != 0 || len(a.released) != 0 {
		t.Fatal("render touched ledger", a)
	}
}
func TestRenderClipUnlinkedRecovery(t *testing.T) {
	h := newHarness(t)
	base := clipInput(t, h, "render", "alice")
	base.Kind = job.KindRenderClip
	base.NonMetered = true
	base.ObserveModel = ""
	base.WriteModel = ""
	id, err := h.queue.Enqueue(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	n, err := h.queue.SweepUnactivatedClips(context.Background())
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	j, err := h.queue.Get(context.Background(), id, "alice")
	if err != nil || j.Status != job.StatusFailed {
		t.Fatal(j, err)
	}
}
