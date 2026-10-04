package usage

import (
	"context"
	"fmt"
	"math/big"
	"reflect"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func (s *Service) WithUnitAccounting(checker UnitBudgetChecker) *Service {
	if checker == nil {
		panic("unit pricing checker is required")
	}
	s.unitCheck = checker
	return s
}

func (s *Service) checkUnits(ctx context.Context, owner string, tier plan.Plan, calls []UnitBudget) error {
	if s.unitCheck == nil || owner == "" || !tier.Valid() {
		return ErrUnitPricing
	}
	if _, err := unitBudgetTotal(calls); err != nil {
		return err
	}
	for _, b := range calls {
		if b.CheckedAt.After(s.now()) {
			return ErrUnitPricing
		}
		if err := s.unitCheck.ValidateUnitBudget(ctx, owner, tier, b); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) QuoteUnits(ctx context.Context, owner string, tier plan.Plan, kind string, calls []UnitBudget) (UnitQuote, error) {
	if kind == "" {
		return UnitQuote{}, ErrUnitApproval
	}
	if err := s.checkUnits(ctx, owner, tier, calls); err != nil {
		return UnitQuote{}, err
	}
	usd, err := unitBudgetTotal(calls)
	if err != nil {
		return UnitQuote{}, err
	}
	var rate plan.RateSnapshot
	if usd.Sign() > 0 {
		if s.rates == nil {
			return UnitQuote{}, ErrRateUnavailable
		}
		rate, err = s.SelectRate(ctx)
		if err != nil {
			return UnitQuote{}, err
		}
	}
	credits, err := exactCredits(usd, rate)
	if err != nil {
		return UnitQuote{}, err
	}
	now := s.now()
	q := UnitQuote{ID: s.newID(), UserID: owner, Kind: kind, Digest: unitCallsDigest(calls), Calls: calls, Rate: rate, MaxCredits: credits, ExpiresAt: now.Add(UnitQuoteTTL)}
	err = s.tx.InWriteTx(ctx, func(tx Storage) error {
		u, err := unitStore(tx)
		if err != nil {
			return err
		}
		return u.SaveUnitQuote(ctx, q, now)
	})
	return q, err
}

// ReservationForUnits reads an approval, never a user-supplied price or budget.
// The owning context recalculates current exact inputs and passes them here.
func (s *Service) ReservationForUnits(ctx context.Context, owner string, tier plan.Plan, kind string, approval UnitApproval, calls []UnitBudget) (*Reservation, error) {
	u, err := unitStoreFromService(s)
	if err != nil {
		return nil, err
	}
	q, err := u.GetUnitQuote(ctx, owner, approval.QuoteID)
	if err != nil || approval.QuoteID == "" || approval.ApprovedMaxCredits == nil || approval.CancellationPolicyVersion != UnitCancellationPolicyVersion ||
		q.Kind != kind || q.MaxCredits != *approval.ApprovedMaxCredits || q.Digest != unitCallsDigest(calls) || q.ConsumedJobID != "" || !s.now().Before(q.ExpiresAt) {
		return nil, ErrUnitApproval
	}
	if err := s.checkUnits(ctx, owner, tier, calls); err != nil {
		return nil, err
	}
	return &Reservation{CancellationPolicyVersion: approval.CancellationPolicyVersion, ApprovedMaxCredits: q.MaxCredits, Rate: q.Rate, UnitQuoteID: q.ID, Units: q.Calls}, nil
}

func unitStoreFromService(s *Service) (UnitLedger, error) {
	u, ok := s.tx.(UnitLedger)
	if !ok {
		return nil, fmt.Errorf("unit ledger not wired")
	}
	return u, nil
}

func plannedUnits(start Start) ([]UnitBudget, error) {
	if start.Approval == nil || len(start.Approval.Units) == 0 || start.Approval.UnitQuoteID == "" || len(start.Approval.Calls) != 0 {
		return nil, ErrUnitApproval
	}
	calls := make([]UnitBudget, 0, len(start.Calls))
	for _, c := range start.Calls {
		if c.Units == nil || c.Ref != c.Units.Ref || c.Stage != c.Units.Operation || c.Count != c.Units.Count || c.CompletionTokens != 0 {
			return nil, ErrUnitApproval
		}
		calls = append(calls, *c.Units)
	}
	if unitCallsDigest(calls) != unitCallsDigest(start.Approval.Units) {
		return nil, ErrUnitApproval
	}
	return calls, nil
}

func (s *Service) holdUnits(ctx context.Context, start Start) error {
	calls, err := plannedUnits(start)
	if err != nil {
		return err
	}
	u, err := unitStoreFromService(s)
	if err != nil {
		return err
	}
	if prior, _, found, err := s.holds.HoldForJob(ctx, start.JobID); err != nil {
		return err
	} else if found {
		quoteID, frozen, err := u.UnitAdmission(ctx, start.JobID)
		if err != nil || prior.UserID != start.UserID || prior.Kind != start.Kind || prior.ApprovedMaxCredits == nil ||
			*prior.ApprovedMaxCredits != start.Approval.ApprovedMaxCredits || prior.CancellationPolicyVersion != start.Approval.CancellationPolicyVersion ||
			quoteID != start.Approval.UnitQuoteID || unitCallsDigest(frozen) != unitCallsDigest(calls) || !reflect.DeepEqual(prior.Rate, start.Approval.Rate) {
			return ErrUnitApproval
		}
		return nil
	}
	q, err := u.GetUnitQuote(ctx, start.UserID, start.Approval.UnitQuoteID)
	if err != nil || q.Kind != start.Kind || q.ConsumedJobID != "" || !s.now().Before(q.ExpiresAt) || q.Digest != unitCallsDigest(calls) ||
		q.MaxCredits != start.Approval.ApprovedMaxCredits || start.Approval.CancellationPolicyVersion != UnitCancellationPolicyVersion || !reflect.DeepEqual(q.Rate, start.Approval.Rate) {
		return ErrUnitApproval
	}
	// This live safety check never runs in the writer; persisted execution counts
	// are rechecked atomically below. Speech never uses completion fallback prices.
	if err := s.checkUnits(ctx, start.UserID, start.Plan, calls); err != nil {
		return err
	}
	return s.commitHold(ctx, start, q.MaxCredits, q.Rate, &q.MaxCredits, UnitCancellationPolicyVersion, s.now())
}

func (s *Service) AdmitUnitCall(ctx context.Context, input llm.SpeechInput) (UnitClaim, error) {
	w, ok := WorkFromContext(ctx)
	if !ok || s.unitCheck == nil {
		return UnitClaim{}, ErrUnitCall
	}
	u, err := unitStoreFromService(s)
	if err != nil {
		return UnitClaim{}, err
	}
	a, _, found, err := s.holds.HoldForJob(ctx, w.JobID)
	if err != nil || !found || a.UserID != w.UserID || a.Kind != w.Kind {
		return UnitClaim{}, ErrUnitCall
	}
	_, calls, err := u.UnitAdmission(ctx, w.JobID)
	if err != nil {
		return UnitClaim{}, err
	}
	var selected *UnitBudget
	for i := range calls {
		b := &calls[i]
		if b.ScopeDigest == w.UnitScopeDigest && b.Ref == input.Ref && b.Operation == input.Operation && b.InputDigest == input.Digest && b.InputCharacters == input.InputCharacters && b.AuxiliaryCharacters == input.AuxiliaryCharacters && b.ParametersDigest == input.ParametersDigest {
			if selected != nil {
				return UnitClaim{}, ErrUnitCall
			}
			selected = b
		}
	}
	if selected == nil {
		return UnitClaim{}, ErrUnitCall
	}
	if err := s.checkUnits(ctx, w.UserID, a.AdmittedPlan, []UnitBudget{*selected}); err != nil {
		return UnitClaim{}, err
	}
	claim := UnitClaim{ID: s.newID(), Budget: *selected, Work: w}
	err = s.tx.InWriteTx(ctx, func(tx Storage) error {
		current, _, open, err := tx.HoldForJob(ctx, w.JobID)
		if err != nil {
			return err
		}
		if !open || current.UserID != w.UserID || current.Kind != w.Kind || ctx.Err() != nil {
			return ErrUnitCall
		}
		u, err := unitStore(tx)
		if err != nil {
			return err
		}
		accepted, err := u.ClaimUnitCall(ctx, claim.ID, w.JobID, selected.Fingerprint(), selected.Count, s.now())
		if err != nil {
			return err
		}
		if !accepted {
			return ErrUnitCall
		}
		return nil
	})
	return claim, err
}

func (s *Service) RecordUnitCall(ctx context.Context, c UnitClaim, evidence llm.SpeechEvidence) error {
	if c.ID == "" || c.Work.UserID == "" || c.Work.JobID == "" {
		return ErrUnitCall
	}
	usd, source := UnitCost(c.Budget, evidence)
	projection := int64(0)
	if cost, ok := new(big.Rat).SetString(usd); ok {
		micro := new(big.Rat).Mul(cost, new(big.Rat).SetInt64(1_000_000))
		n := new(big.Int).Quo(micro.Num(), micro.Denom())
		if n.IsInt64() {
			projection = n.Int64()
		}
	}
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		u, err := unitStore(tx)
		if err != nil {
			return err
		}
		return u.InsertUnitEvent(ctx, Event{UserID: c.Work.UserID, Kind: c.Work.Kind, JobID: c.Work.JobID, Stage: c.Budget.Operation, Model: c.Budget.Ref.String(), CostMicrousd: projection, CostSource: source, CreatedAt: s.now()},
			UnitEvent{ClaimID: c.ID, SupplierRequestID: evidence.RequestID, BudgetFingerprint: c.Budget.Fingerprint(), Evidence: evidence.Units, ReportedUSD: evidence.ReportedUSD, ExactUSD: usd, CostSource: source})
	})
}

func unitCharge(ctx context.Context, tx Storage, admission Admission, completionMicrousd int64) (int, bool, error) {
	u, ok := tx.(UnitLedger)
	if !ok {
		return 0, false, nil
	}
	quoteID, _, err := u.UnitAdmission(ctx, admission.JobID)
	if err != nil {
		return 0, false, err
	}
	if quoteID == "" {
		return 0, false, nil
	}
	usd, err := u.UnitCostForJob(ctx, admission.JobID)
	if err != nil {
		return 0, true, err
	}
	total, ok := new(big.Rat).SetString(usd)
	if !ok || total.Sign() < 0 {
		return 0, true, ErrUnitPricing
	}
	total.Add(total, new(big.Rat).SetFrac(big.NewInt(completionMicrousd), big.NewInt(1_000_000)))
	if total.Sign() == 0 {
		return 0, true, nil
	}
	// Clamp as rational USD before converting, so extreme supplier overage
	// cannot overflow credit integers and never charges another lot.
	cap := admission.HoldCredits
	if admission.ApprovedMaxCredits != nil {
		cap = min(cap, *admission.ApprovedMaxCredits)
	}
	if cap <= 0 {
		return 0, true, nil
	}
	if !admission.Rate.Valid() {
		return 0, true, ErrUnitPricing
	}
	ceiling := new(big.Rat).SetFrac(big.NewInt(int64(cap)*10_000), big.NewInt(admission.Rate.AppliedE4))
	if total.Cmp(ceiling) >= 0 {
		return cap, true, nil
	}
	credits, err := exactCredits(total, admission.Rate)
	return credits, true, err
}
