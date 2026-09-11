package main

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/clip"
	clipai "github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

type clipQuotePricing struct {
	registry *llm.Registry
	cfg      clipai.Config
}

func (p clipQuotePricing) Freeze(ctx context.Context, observe, write llm.ModelRef, count int) (clip.GenerationPricing, error) {
	a, err := p.registry.FreezeExecution(ctx, observe, "observe", p.cfg.ObserveCompletionTokens, p.cfg.ObserveReasoning, llm.ExecutionInlineStatic)
	if err != nil {
		// An admission answer keeps its shape so the clip context can name the
		// model and the check; anything else is the generic pricing refusal.
		var admission *llm.AdmissionError
		if errors.As(err, &admission) {
			return clip.GenerationPricing{}, err
		}
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	b, err := p.registry.FreezeExecution(ctx, write, "write", p.cfg.PlanCompletionTokens, p.cfg.PlanReasoning, llm.ExecutionTextOnly)
	if err != nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	credits, err := usage.ClipCredits([]usage.PricedCall{{Policy: a, Count: count}, {Policy: b, Count: 1}})
	if err != nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	return clip.GenerationPricing{Version: clip.PricingPolicyVersion, Observe: a, Plan: b, ObservationCalls: count, MaxCredits: credits}, nil
}

type clipAccounting struct{ ledger *usage.Service }

func (a clipAccounting) ForJob(ctx context.Context, user, id string) (*clip.Accounting, error) {
	r, err := a.ledger.ClipAccounting(ctx, user, id)
	if err != nil || r == nil {
		return nil, err
	}
	return &clip.Accounting{JobID: id, ApprovedMax: r.Approved, Reserved: r.Reserved, FinalCharge: r.FinalCharge, Refund: r.Refund, ShadowCharge: r.ShadowCharge, Exempt: r.Exempt, Settled: r.Settled}, nil
}

func (a clipJobs) Latest(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.LatestClipSnapshot(ctx, user, id)
	if err != nil || j == nil {
		return nil, err
	}
	return &clip.ClipJob{ID: j.ID, Kind: j.Kind, Status: j.Status, Stage: j.Stage, Payload: j.Payload, DispatchReady: j.DispatchReady}, nil
}

// Reserved only after T084 has verified every prepared proxy; no public caller can
// reach this through a request-supplied price or a client-side duration estimate.
func (a clipJobs) ReserveApproved(ctx context.Context, user, id string, approval clip.GenerationApproval, chunks int) (context.Context, error) {
	p := approval.Pricing
	if chunks < 1 || chunks > p.ObservationCalls || !p.Valid() || approval.MaxCredits != p.MaxCredits || approval.QuoteID == "" {
		return nil, job.ErrCreditAllowance
	}
	calls := clipPricingCalls(p.Observe.Ref.String(), p.Plan.Ref.String(), chunks, clip.CompletionBudgets{Observe: p.Observe.CompletionTokens, Plan: p.Plan.CompletionTokens})
	return a.queue.ReserveClip(ctx, user, id, calls, job.ClipReservation{ApprovedMaxCredits: approval.MaxCredits, Calls: []job.ClipCall{{Policy: p.Observe, Count: chunks}, {Policy: p.Plan, Count: 1}}})
}

// clipAdmission is the AnalysisAdmission port over the registry: the observe
// models in registry order with their raw modality, and the same frozen
// qualification the quote and the job run — nothing here calls a model or
// writes a row.
type clipAdmission struct {
	registry *llm.Registry
	cfg      clipai.Config
}

func (a clipAdmission) ObserveModels() []clip.AnalysisCandidate {
	var out []clip.AnalysisCandidate
	for _, m := range a.registry.Models() {
		if m.ServesStage(llm.StageNameObserve) {
			out = append(out, clip.AnalysisCandidate{Ref: m.Ref, VideoInput: m.VideoInput})
		}
	}
	return out
}

func (a clipAdmission) QualifyObserve(ctx context.Context, ref llm.ModelRef) error {
	_, err := a.registry.FreezeExecution(ctx, ref, llm.StageNameObserve, a.cfg.ObserveCompletionTokens, a.cfg.ObserveReasoning, llm.ExecutionInlineStatic)
	return err
}
