package app

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

// Budgets are the per-call completion budgets and reasoning efforts the quote
// freezes. They come from the clip AI configuration at the composition root.
type Budgets struct {
	ObserveCompletionTokens, FlowCompletionTokens, NarrationCompletionTokens int
	ObserveReasoning, PlanReasoning                                          llm.ReasoningEffort
}

// DefaultQuoteRetries is how many response retries a quote allows each call.
const DefaultQuoteRetries = 3

// QuoteCredits is the quote formula: the observation call priced for every
// retry it may need, and both writing calls priced as one line on the same
// model at the same budget (usage.ReservationCredits takes at most two).
func QuoteCredits(pricing clip.GenerationPricing, count, retries int) (int, error) {
	return usage.ReservationCredits([]usage.PricedCall{{Policy: pricing.Observe, Count: count * (1 + retries)}, {Policy: pricing.Plan, Count: pricing.PlanCalls()}})
}

// Pricing is the clip.QuotePricing port over the registry.
type Pricing struct {
	freezer Freezer
	budgets Budgets
}

func NewPricing(freezer Freezer, budgets Budgets) Pricing {
	if freezer == nil {
		panic("clip app: pricing needs a freezer")
	}
	return Pricing{freezer: freezer, budgets: budgets}
}

func (p Pricing) Freeze(ctx context.Context, observe, write llm.ModelRef, count int) (clip.GenerationPricing, error) {
	return p.FreezeWork(ctx, observe, write, count, false, false, DefaultQuoteRetries)
}

// FreezeWork prices the calls this generation still has to make: the remaining
// observations and each writing call the recovery cannot answer. Both writing
// calls are the same model at its own budget, so each is frozen on its own
// allowance and priced by its own count (CLIP-135).
func (p Pricing) FreezeWork(ctx context.Context, observe, write llm.ModelRef, count int, skipFlow, skipNarration bool, retries int) (clip.GenerationPricing, error) {
	a, err := p.freezer.FreezeExecution(ctx, observe, "observe", p.budgets.ObserveCompletionTokens, p.budgets.ObserveReasoning, llm.ExecutionInlineStatic)
	if err != nil {
		// An admission answer keeps its shape so the clip context can name the
		// model and the check; anything else is the generic pricing refusal.
		var admission *llm.AdmissionError
		if errors.As(err, &admission) {
			return clip.GenerationPricing{}, err
		}
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	b, err := p.freezer.FreezeExecution(ctx, write, "write", p.budgets.FlowCompletionTokens, p.budgets.PlanReasoning, llm.ExecutionTextOnly)
	if err != nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	c, err := p.freezer.FreezeExecution(ctx, write, "write", p.budgets.NarrationCompletionTokens, p.budgets.PlanReasoning, llm.ExecutionTextOnly)
	if err != nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	a.ResponseRetries, b.ResponseRetries, c.ResponseRetries = retries, retries, retries
	pricing := clip.GenerationPricing{Version: clip.PricingPolicyVersion, SkipFlow: skipFlow, SkipNarration: skipNarration, Observe: a, Plan: b, Narration: c, ObservationCalls: count}
	credits, err := QuoteCredits(pricing, count, retries)
	if err != nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	pricing.MaxCredits = credits
	return pricing, nil
}

// Accounting is the clip.AccountingReader port over the ledger.
type Accounting struct{ ledger AccountingLedger }

func NewAccounting(ledger AccountingLedger) Accounting {
	if ledger == nil {
		panic("clip app: accounting needs a ledger")
	}
	return Accounting{ledger: ledger}
}

func (a Accounting) ForJob(ctx context.Context, user, id string) (*clip.Accounting, error) {
	r, err := a.ledger.ReservationAccounting(ctx, user, id)
	if err != nil || r == nil {
		return nil, err
	}
	return &clip.Accounting{CancellationPolicyVersion: r.CancellationPolicyVersion, SettlementReason: r.SettlementReason, NominalReservation: r.NominalReservation, ConfirmedCharge: r.ConfirmedCharge, CancellationFee: r.CancellationFee, ShadowConfirmedCharge: r.ShadowConfirmedCharge, ShadowCancellationFee: r.ShadowCancellationFee, JobID: id, ApprovedMax: r.Approved, Reserved: r.Reserved, FinalCharge: r.FinalCharge, Refund: r.Refund, ShadowCharge: r.ShadowCharge, Exempt: r.Exempt, Settled: r.Settled}, nil
}

// ModelAdmission is the clip.AnalysisAdmission port over the registry: the
// observe models in registry order with their raw modality, and the same
// frozen qualification the quote and the job run — nothing here calls a model
// or writes a row.
type ModelAdmission struct {
	freezer Freezer
	budgets Budgets
}

func NewModelAdmission(freezer Freezer, budgets Budgets) ModelAdmission {
	if freezer == nil {
		panic("clip app: model admission needs a freezer")
	}
	return ModelAdmission{freezer: freezer, budgets: budgets}
}

func (a ModelAdmission) ObserveModels() []clip.AnalysisCandidate {
	var out []clip.AnalysisCandidate
	for _, m := range a.freezer.Models() {
		if m.ServesStage(llm.StageNameObserve) {
			out = append(out, clip.AnalysisCandidate{Ref: m.Ref, VideoInput: m.VideoInput})
		}
	}
	return out
}

// QualifyObserve is the read-only eligibility answer, so an unexpired document
// the adapter already read may serve it; a quote or a job never takes this path.
func (a ModelAdmission) QualifyObserve(ctx context.Context, ref llm.ModelRef) error {
	_, err := a.freezer.FreezeExecution(llm.AllowCachedEndpoints(ctx), ref, llm.StageNameObserve, a.budgets.ObserveCompletionTokens, a.budgets.ObserveReasoning, llm.ExecutionInlineStatic)
	return err
}
