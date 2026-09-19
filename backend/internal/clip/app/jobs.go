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
type Jobs struct {
	queue Queue
	guard Reserver
}

// Reserver is the credit hold and the per-call dispatch authorization, both of which
// commit against the job row in one short transaction. Without one no charged clip work
// can reserve, which is the mode the queue's own tests run in.
type Reserver interface {
	Reserve(ctx context.Context, hold Hold) error
	Authorize(ctx context.Context, user, id string) error
}

func NewJobs(queue Queue, guard Reserver) Jobs {
	if queue == nil {
		panic("clip app: jobs need a queue")
	}
	return Jobs{queue: queue, guard: guard}
}

func (a Jobs) Enqueue(ctx context.Context, s clip.GenerationStart) (string, error) {
	kind := clip.JobKindGenerate
	if s.RenderOnly {
		kind = clip.JobKindRender
	}
	if s.Revise {
		kind = clip.JobKindRevise
	}
	policy := 0
	if s.Quote != nil {
		policy = s.Quote.Pricing.CancellationPolicyVersion
	}
	if err := validClipStart(kind, s, policy); err != nil {
		return "", err
	}
	subject := job.Subject{Dimension: clip.JobSubject, ID: s.ProjectID}
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		CancellationPolicyVersion: policy, Kind: kind, NonMetered: s.RenderOnly, DeferHold: clip.ChargedJobKind(kind), UserID: s.UserID,
		Subjects: []job.Subject{subject},
		// One project runs one job at a time, whoever asked: the project is the lock.
		Guards:       []job.Guard{{Subject: subject, Filter: job.Filter{UserID: s.UserID}}},
		ObserveModel: s.Observe, WriteModel: s.Write, Payload: s.Payload,
	})
	if errors.Is(err, job.ErrActiveConflict) {
		return "", clip.ErrBusy
	}
	if errors.Is(err, job.ErrInvalidTarget) {
		return "", clip.ErrNotFound
	}
	return id, err
}

func (a Jobs) Activate(ctx context.Context, user, id string) error {
	return a.queue.Activate(ctx, user, id)
}

func (a Jobs) FailQueued(ctx context.Context, user, id string) (bool, error) {
	return a.queue.FailQueued(ctx, id, user, job.Failure{Reason: "CLIP_PROCESSING_FAILED"})
}

func (a Jobs) Active(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: id}, job.Filter{UserID: user})
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
	j, err := a.queue.Snapshot(ctx, user, job.Subject{Dimension: clip.JobSubject, ID: project}, id)
	if errors.Is(err, job.ErrNotFound) {
		return nil, clip.ErrNotFound
	}
	return snapshot(j), err
}

func (a Jobs) Latest(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.LatestSnapshot(ctx, user, job.Subject{Dimension: clip.JobSubject, ID: id})
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
	return a.Reserve(ctx, user, id, calls, reservation)
}

// PlanReservation is the approval rule: the approval must carry a valid quote
// whose credit ceiling it repeats, and the chunk count must fit the quote. It
// returns the planned calls and the reservation the ledger prices them under.
func PlanReservation(approval clip.GenerationApproval, chunks int) ([]job.PlannedCall, Reservation, error) {
	p := approval.Pricing
	if chunks < 0 || chunks > p.ObservationCalls || !p.Valid() || approval.MaxCredits != p.MaxCredits || approval.QuoteID == "" {
		return nil, Reservation{}, clip.ErrCreditAllowance
	}
	calls := []job.PlannedCall{}
	if chunks > 0 {
		calls = append(calls, job.PlannedCall{Ref: p.Observe.Ref.String(), Count: p.ObserveCalls(chunks), CompletionTokens: p.Observe.CompletionTokens})
	}
	if p.PlanCalls() > 0 {
		calls = append(calls, job.PlannedCall{Ref: p.Plan.Ref.String(), Count: p.PlanCalls(), CompletionTokens: p.Plan.CompletionTokens})
	}
	reservation := Reservation{CancellationPolicyVersion: p.CancellationPolicyVersion, ApprovedMaxCredits: approval.MaxCredits, Calls: []Call{{Policy: p.Observe, Count: p.ObserveCalls(chunks)}, {Policy: p.Plan, Count: p.PlanCalls()}}}
	return calls, reservation, nil
}

// validClipStart is the shape rule the queue used to hold: only charged clip work may
// carry a cancellation policy, a render spends nothing and names no model, and a
// generation must name both of its models.
func validClipStart(kind string, s clip.GenerationStart, policy int) error {
	switch {
	case s.ProjectID == "" || s.UserID == "":
		return clip.ErrNotFound
	case policy != 0 && !clip.ChargedJobKind(kind):
		return clip.ErrNotFound
	case kind == clip.JobKindRender && (s.Observe != "" || s.Write != ""):
		return clip.ErrNotFound
	case kind == clip.JobKindGenerate && (s.Observe == "" || s.Write == ""):
		return clip.ErrNotFound
	}
	return nil
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
