package app

import (
	"context"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
)

type Queue interface {
	Enqueue(context.Context, job.NewJob) (string, error)
	CancelOwned(context.Context, string, string) (*job.JobSummary, error)
}
type QueueJobs struct{ queue Queue }

func NewJobs(q Queue) QueueJobs {
	if q == nil {
		panic("spoken queue required")
	}
	return QueueJobs{q}
}
func (s QueueJobs) Enqueue(ctx context.Context, o spoken.Operation, budgets []usage.UnitBudget, free bool) (string, error) {
	calls := make([]job.PlannedCall, 0, len(budgets))
	for _, b := range budgets {
		calls = append(calls, job.PlannedCall{Ref: b.Ref.String(), Stage: b.Operation, Count: b.Count})
	}
	ref := o.Profile.Design.String()
	if o.Kind == spoken.JobKindProbe {
		ref = o.Profile.Synthesis.String()
	}
	return s.queue.Enqueue(ctx, job.NewJob{Kind: o.Kind, UserID: o.OwnerID, Payload: []byte(o.ID), WriteModel: ref, PricingCalls: calls, NonMetered: free, CancellationPolicyVersion: 1})
}
func (s QueueJobs) SignalCancellation(ctx context.Context, owner, id string) error {
	_, err := s.queue.CancelOwned(ctx, owner, id)
	return err
}
