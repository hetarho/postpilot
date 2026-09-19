package app

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// Jobs is the clip.GenerationJobs port over the job queue. It translates the
// queue's vocabulary into the clip context's and owns the approval rule a
// reservation must satisfy.
type Jobs struct{ queue Queue }

func NewJobs(queue Queue) Jobs {
	if queue == nil {
		panic("clip app: jobs need a queue")
	}
	return Jobs{queue: queue}
}

func (a Jobs) Enqueue(ctx context.Context, s clip.GenerationStart) (string, error) {
	kind := job.KindGenerateClip
	if s.RenderOnly {
		kind = job.KindRenderClip
	}
	if s.Revise {
		kind = job.KindReviseClip
	}
	policy := 0
	if s.Quote != nil {
		policy = s.Quote.Pricing.CancellationPolicyVersion
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{CancellationPolicyVersion: policy, Kind: kind, NonMetered: s.RenderOnly, UserID: s.UserID, ClipProjectID: s.ProjectID, ObserveModel: s.Observe, WriteModel: s.Write, Payload: s.Payload})
	if errors.Is(err, job.ErrActiveConflict) {
		return "", clip.ErrBusy
	}
	if errors.Is(err, job.ErrInvalidTarget) {
		return "", clip.ErrNotFound
	}
	return id, err
}

func (a Jobs) Activate(ctx context.Context, user, id string) error {
	return a.queue.ActivateClip(ctx, user, id)
}

func (a Jobs) FailQueued(ctx context.Context, user, id string) (bool, error) {
	return a.queue.FailQueued(ctx, id, user, job.Failure{Reason: "CLIP_PROCESSING_FAILED"})
}

func (a Jobs) Active(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.ActiveForClip(ctx, user, id)
	return summary(j), err
}

func (a Jobs) Get(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.Get(ctx, id, user)
	if errors.Is(err, job.ErrNotFound) {
		return nil, nil
	}
	return summary(j), err
}

func (a Jobs) Snapshot(ctx context.Context, user, project, id string) (*clip.ClipJob, error) {
	j, err := a.queue.ClipJobSnapshot(ctx, user, project, id)
	if errors.Is(err, job.ErrNotFound) {
		return nil, clip.ErrNotFound
	}
	return snapshot(j), err
}

func (a Jobs) Latest(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.LatestClipSnapshot(ctx, user, id)
	return snapshot(j), err
}

// ReserveApproved holds the credits an approved quote allows for the calls the
// job will still make. It runs only after every prepared proxy has been
// verified; no public caller reaches it through a request-supplied price or a
// client-side duration estimate.
func (a Jobs) ReserveApproved(ctx context.Context, user, id string, approval clip.GenerationApproval, chunks int) (context.Context, error) {
	calls, reservation, err := PlanReservation(approval, chunks)
	if err != nil {
		return nil, err
	}
	if len(calls) == 0 {
		return ctx, nil
	}
	return a.queue.ReserveClip(ctx, user, id, calls, reservation)
}

// PlanReservation is the approval rule: the approval must carry a valid quote
// whose credit ceiling it repeats, and the chunk count must fit the quote. It
// returns the planned calls and the reservation the ledger prices them under.
func PlanReservation(approval clip.GenerationApproval, chunks int) ([]job.PlannedCall, job.ClipReservation, error) {
	p := approval.Pricing
	if chunks < 0 || chunks > p.ObservationCalls || !p.Valid() || approval.MaxCredits != p.MaxCredits || approval.QuoteID == "" {
		return nil, job.ClipReservation{}, job.ErrCreditAllowance
	}
	calls := []job.PlannedCall{}
	if chunks > 0 {
		calls = append(calls, job.PlannedCall{Ref: p.Observe.Ref.String(), Count: p.ObserveCalls(chunks), CompletionTokens: p.Observe.CompletionTokens})
	}
	if p.PlanCalls() > 0 {
		calls = append(calls, job.PlannedCall{Ref: p.Plan.Ref.String(), Count: p.PlanCalls(), CompletionTokens: p.Plan.CompletionTokens})
	}
	reservation := job.ClipReservation{CancellationPolicyVersion: p.CancellationPolicyVersion, ApprovedMaxCredits: approval.MaxCredits, Calls: []job.ClipCall{{Policy: p.Observe, Count: p.ObserveCalls(chunks)}, {Policy: p.Plan, Count: p.PlanCalls()}}}
	return calls, reservation, nil
}

func summary(j *job.JobSummary) *clip.ClipJob {
	if j == nil {
		return nil
	}
	return &clip.ClipJob{ID: j.ID, Status: j.Status, Stage: j.Stage, FinishedAt: j.FinishedAt}
}

func snapshot(j *job.Job) *clip.ClipJob {
	if j == nil {
		return nil
	}
	return &clip.ClipJob{ID: j.ID, Kind: j.Kind, Status: j.Status, Stage: j.Stage, Payload: j.Payload, DispatchReady: j.DispatchReady, FinishedAt: j.FinishedAt}
}
