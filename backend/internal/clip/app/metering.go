package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

// AdmitExecution is the clip half of the metering gate every provider call
// passes: an execution policy is honoured only inside a charged clip job, a
// render job may call no model, and a charged clip job's call must match the
// policy the reservation admitted — consumed here so it cannot be reused. The
// returned context carries the priced call for the ledger.
func AdmitExecution(ctx context.Context, ref llm.ModelRef, req llm.Request) (context.Context, error) {
	work, hasWork := usage.WorkFromContext(ctx)
	if req.Execution != nil && (!hasWork || !clip.ChargedJobKind(work.Kind)) {
		return ctx, clip.ErrCreditAllowance
	}
	if !hasWork {
		return ctx, nil
	}
	if work.Kind == clip.JobKindRender {
		return ctx, clip.ErrCreditAllowance
	}
	if !clip.ChargedJobKind(work.Kind) {
		return ctx, nil
	}
	policy, err := ConsumePolicy(ctx, work.UserID, work.JobID, ref.String(), req.MaxTokens, req.Stage)
	if err != nil {
		return ctx, err
	}
	if req.Execution == nil || req.Execution.Call != policy || !req.Execution.Matches(ref, req) {
		return ctx, clip.ErrCreditAllowance
	}
	return usage.WithCallPrice(ctx, work.UserID, work.JobID, policy), nil
}
