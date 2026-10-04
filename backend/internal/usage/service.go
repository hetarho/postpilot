package usage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// holdInputTokens is the prompt size one call is priced at when its hold is computed.
//
// It is deliberately above what the product's largest real prompt carries (a write call
// with a full profile, few-shot excerpts and an observation set, or an observation call
// with a batch of photos): a hold that under-estimates lets work start on credits the
// account turns out not to have, and settlement returns whatever was not used within the
// minute. Erring high costs a user nothing; erring low costs us the difference.
const holdInputTokens = 30_000

var ErrLotTouched = errors.New("credit lot has already been touched")
var ErrLotNotFound = errors.New("credit lot was not found")
var ErrSettlementOutcome = errors.New("settlement requires a persisted terminal outcome")

// Service is the credit gate and the ledger writer.
type Service struct {
	// tx is also the handle every narrow port below is served from: the ledger has one
	// store, and a use-case names the behaviour it uses (ARCH-6).
	tx          WriteScope
	lots        LotLedger
	purchases   PurchasedLotLedger
	vouchers    VoucherLotLedger
	charges     SpendLedger
	holds       HoldLedger
	renewals    RenewalReads
	models      Models
	anchors     Anchors
	rates       *RateSelector
	modelGrades bool
	unitCheck   UnitBudgetChecker

	// approvedKinds is the work that may not start without an approved credit ceiling.
	// The composition root names it: the ledger enforces the rule and never learns which
	// product asked for it.
	approvedKinds map[string]bool

	// ownerCancellableKinds is work that holds no approved ceiling yet may still be stopped by
	// its owner (TMPL-63). The composition root names it, as it names approvedKinds.
	ownerCancellableKinds map[string]bool

	// maxCompletionTokens is the same cap the registry sends on a call that sets none. It is
	// only a fallback for a planned call whose caller did not declare a stage budget.
	maxCompletionTokens int64

	// now and newID are seams for tests in this package, not configuration: every window
	// is a calendar boundary, so the only way to exercise one is to move the clock.
	now   func() time.Time
	newID func() string
}

// WithOwnerCancellation lets an owner stop work of these kinds although it was admitted without
// an approved ceiling; its settlement then charges confirmed usage only and returns the rest of
// the hold, with no cancellation fee (QUOTA-49).
func (s *Service) WithOwnerCancellation(kinds ...string) *Service {
	s.ownerCancellableKinds = make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		s.ownerCancellableKinds[kind] = true
	}
	return s
}

// cancellable reports whether an admission may settle as cancelled: approved work under the
// cancellation policy it was approved with, or unapproved work of a kind its owner may stop.
func (s *Service) cancellable(admission Admission) bool {
	if admission.ApprovedMaxCredits != nil {
		return admission.CancellationPolicyVersion == 1
	}
	return s.ownerCancellableKinds[admission.Kind]
}

// WithModelGrades enforces the code-owned free/paid classification for every
// production admission. Existing ledger fixtures can opt in explicitly.
func (s *Service) WithModelGrades() *Service {
	s.modelGrades = true
	return s
}

// NewService wires the ledger. anchors is the account-specific monthly-window resolver
// and is required: every path that renews a balance reads it, and a ledger without one
// would silently revert to a calendar month (ARCH-40). approvedKinds is the work that
// must carry an approved ceiling; an empty list is a ledger where every start is priced
// from its planned calls alone, which is a real mode and must be stated.
func NewService(store Storage, models Models, maxCompletionTokens int64, anchors Anchors, approvedKinds ...string) *Service {
	if anchors == nil {
		panic("usage: monthly anchors are required")
	}
	approved := make(map[string]bool, len(approvedKinds))
	for _, kind := range approvedKinds {
		approved[kind] = true
	}
	return &Service{
		tx: store, lots: store, purchases: store, vouchers: store, charges: store, holds: store, renewals: store,
		models: models, anchors: anchors, approvedKinds: approved,
		maxCompletionTokens: maxCompletionTokens,
		now:                 time.Now, newID: newID,
	}
}

// WithStore preserves pricing, anchors and clocks inside a composition-owned
// transaction. The caller owns committing or rolling back that scoped store.
// requiresApproval reports whether this work must reserve an approved ceiling before it
// may start.
func (s *Service) requiresApproval(kind string) bool { return s.approvedKinds[kind] }

// approvedKindList is the same set in a stable order, for the statements that filter on it.
func (s *Service) approvedKindList() []string {
	kinds := make([]string, 0, len(s.approvedKinds))
	for kind := range s.approvedKinds {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func (s *Service) WithStore(store Storage) *Service {
	clone := *s
	clone.tx, clone.lots, clone.purchases, clone.charges, clone.holds, clone.renewals = store, store, store, store, store, store
	return &clone
}

// WithRateSelector attaches the official FX policy to a production ledger.
// Tests that construct the historical ledger directly keep their old fixture
// denomination until they opt into a frozen rate.
func (s *Service) WithRateSelector(selector *RateSelector) *Service {
	clone := *s
	clone.rates = selector
	return &clone
}

// WithClock keeps entitlement-window tests on the same instant as billing.
func (s *Service) WithClock(now func() time.Time) *Service {
	clone := *s
	if now != nil {
		clone.now = now
	}
	return &clone
}

func (s *Service) SelectRate(ctx context.Context) (plan.RateSnapshot, error) {
	if s.rates == nil {
		return plan.RateSnapshot{}, ErrRateUnavailable
	}
	return s.rates.Select(ctx, s.now())
}

func (s *Service) Select(ctx context.Context, at time.Time) (plan.RateSnapshot, error) {
	if s.rates == nil {
		return plan.RateSnapshot{}, ErrRateUnavailable
	}
	return s.rates.Select(ctx, at)
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("usage: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}

// OpenMonthlyLot installs one explicit subscription window. Its deterministic id makes a
// provider retry harmless while retaining every lapsed window for history.
func (s *Service) OpenMonthlyLot(ctx context.Context, userID string, tier plan.Plan, start, end time.Time) error {
	if userID == "" || !tier.Valid() || plan.Unlimited(tier) || !start.Before(end) {
		return errors.New("open monthly lot: invalid user, tier, or window")
	}
	credits := plan.MonthlyCredits(tier)
	_, err := s.lots.InsertLotIfAbsent(ctx, Lot{
		ID: monthlyLotID(userID, start), UserID: userID, Kind: LotMonthly,
		Granted: credits, Remaining: credits, ExpiresAt: &end, CreatedAt: s.now(),
	})
	return err
}

// StartMonthlyWindow opens the window a subscription's first charge paid for (QUOTA-42).
//
// It is not OpenMonthlyLot with a different name. Starting a subscription is not a tier
// change: the window the account was running closes at that instant with no carry-over,
// and a whole month's grant opens on the subscription day. The two writes are one
// transaction because a closed window with no replacement is an account with no credits.
//
// The overwrite is the point. A window's id is its start date, so an account that signed up
// and subscribes on the same Seoul date derives the id it already holds its free window
// under; an absent-only insert would keep the free 50 and drop the tier's grant. Deriving
// granted, remaining and the expiry from the tier and the window rather than from the row
// is also what makes a provider retry harmless.
func (s *Service) StartMonthlyWindow(ctx context.Context, userID string, tier plan.Plan, start, end time.Time) error {
	if userID == "" || !tier.Valid() || plan.Unlimited(tier) || !start.Before(end) {
		return errors.New("start monthly window: invalid user, tier, or window")
	}
	credits := plan.MonthlyCredits(tier)
	id := monthlyLotID(userID, start)
	now := s.now()
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		if err := tx.ExpireMonthlyLotsExcept(ctx, userID, id, start); err != nil {
			return err
		}
		return tx.UpsertLot(ctx, Lot{
			ID: id, UserID: userID, Kind: LotMonthly,
			Granted: credits, Remaining: credits, ExpiresAt: &end, CreatedAt: now,
		})
	})
}

// RaiseMonthlyLot is the credit half of an upgrade reached from the RPC path, the same
// operation TopUpMonthlyLot performs (QUOTA-35).
//
// An account with no current monthly lot is a no-op, not an error, for the reason stated
// there: its next request opens one at the new tier's size anyway. Refusing would be worse
// than doing nothing — the card has already been charged outside the transaction (ARCH-10),
// so an error here discards a captured payment along with the subscription row.
func (s *Service) RaiseMonthlyLot(ctx context.Context, userID string, credits int) error {
	if credits <= 0 {
		return errors.New("raise monthly lot: credits must be positive")
	}
	lot, found, err := s.lots.ActiveMonthlyLot(ctx, userID, s.now())
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return s.lots.RaiseLot(ctx, lot.ID, credits)
}

func (s *Service) OpenPurchasedLot(ctx context.Context, userID string, credits int) (string, error) {
	if userID == "" || credits <= 0 {
		return "", errors.New("open purchased lot: user and positive credits are required")
	}
	id := "purchased:" + s.newID()
	err := s.lots.InsertLot(ctx, Lot{ID: id, UserID: userID, Kind: LotPurchased, Granted: credits, Remaining: credits, CreatedAt: s.now()})
	if err != nil {
		return "", err
	}
	return id, nil
}

// OpenVoucherLot is the credit half of a voucher redemption (QUOTA-58): one lot of the
// voucher's credits that expires at the given instant.
func (s *Service) OpenVoucherLot(ctx context.Context, userID string, credits int, expiresAt time.Time) (string, error) {
	if userID == "" || credits <= 0 || expiresAt.IsZero() {
		return "", errors.New("open voucher lot: user, positive credits and an expiry are required")
	}
	id := "voucher:" + s.newID()
	expires := expiresAt
	err := s.lots.InsertLot(ctx, Lot{
		ID: id, UserID: userID, Kind: LotVoucher, Granted: credits, Remaining: credits,
		ExpiresAt: &expires, CreatedAt: s.now(),
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// ExpireVoucherLot is the credit half of revoking a redeemed voucher: its unspent remainder
// stops counting at `at`, and what was already spent stays spent (QUOTA-58). A lot that has
// already expired by then is left as it is; an id that is not a voucher lot is refused.
func (s *Service) ExpireVoucherLot(ctx context.Context, lotID string, at time.Time) error {
	moved, err := s.vouchers.ExpireVoucherLot(ctx, lotID, at)
	if err != nil || moved {
		return err
	}
	lots, err := s.vouchers.VoucherLots(ctx, []string{lotID})
	if err != nil {
		return err
	}
	if len(lots) == 0 {
		return ErrLotNotFound
	}
	return nil
}

// VoucherLotStanding is where one voucher lot stands at an instant: what it still holds,
// which is zero once it has expired, and when it expires or expired.
type VoucherLotStanding struct {
	Remaining int
	ExpiresAt time.Time
}

// VoucherLotStandings answers, for each of the given voucher lots, what it still holds at
// `at` and when it expires. It is the plural read behind the operator's voucher list, on the
// read pool; an id the answer omits is absent or not a voucher lot.
func (s *Service) VoucherLotStandings(ctx context.Context, lotIDs []string, at time.Time) (map[string]VoucherLotStanding, error) {
	if len(lotIDs) == 0 {
		return nil, nil
	}
	lots, err := s.vouchers.VoucherLots(ctx, lotIDs)
	if err != nil {
		return nil, err
	}
	standings := make(map[string]VoucherLotStanding, len(lots))
	for _, lot := range lots {
		standing := VoucherLotStanding{Remaining: lot.Remaining}
		if lot.ExpiresAt != nil {
			standing.ExpiresAt = *lot.ExpiresAt
		}
		if lot.Expired(at) {
			standing.Remaining = 0
		}
		standings[lot.ID] = standing
	}
	return standings, nil
}

func (s *Service) VoidUntouchedLot(ctx context.Context, lotID string) error {
	ok, err := s.purchases.VoidUntouchedLot(ctx, lotID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLotTouched
	}
	return nil
}

func (s *Service) LotUntouched(ctx context.Context, lotID string) (bool, error) {
	if lotID == "" {
		return false, nil
	}
	return s.purchases.LotUntouched(ctx, lotID)
}

// UntouchedLots answers, for each of the given purchased lots, whether it is still whole.
//
// It is the plural read behind a screen that renders a refund button per purchase: one
// statement on the read pool instead of one writer statement per row, which would queue a
// read-only screen behind every concurrent hold (ARCH-10 caps the writer at one connection).
// A lot the query does not return is absent, spent, or not a purchase, and answers false.
func (s *Service) UntouchedLots(ctx context.Context, lotIDs []string) (map[string]bool, error) {
	if len(lotIDs) == 0 {
		return nil, nil
	}
	ids, err := s.purchases.UntouchedPurchasedLots(ctx, lotIDs)
	if err != nil {
		return nil, err
	}
	untouched := make(map[string]bool, len(ids))
	for _, id := range ids {
		untouched[id] = true
	}
	return untouched, nil
}

func (s *Service) RestoreLot(ctx context.Context, lotID string, credits int) error {
	if credits <= 0 {
		return errors.New("restore lot: credits must be positive")
	}
	ok, err := s.purchases.RestoreLot(ctx, lotID, credits)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLotNotFound
	}
	return nil
}

func (s *Service) GrantBonusOnce(ctx context.Context, id, userID string, credits int) (bool, error) {
	if id == "" || userID == "" || credits <= 0 {
		return false, errors.New("grant bonus: id, user, and positive credits are required")
	}
	return s.lots.InsertLotIfAbsent(ctx, Lot{ID: id, UserID: userID, Kind: LotBonus, Granted: credits, Remaining: credits, CreatedAt: s.now()})
}

// matchExistingHold checks a retried start against the admission's original
// rate and ceiling, so retries never need a second official lookup.
func (s *Service) matchExistingHold(start Start, prior Admission) error {
	conflict := errors.New("hold conflicts with existing reservation")
	if prior.UserID != start.UserID || prior.Kind != start.Kind {
		return conflict
	}
	var required int
	if s.requiresApproval(start.Kind) {
		if start.Approval == nil {
			return ErrApprovalRequired
		}
		if prior.ApprovedMaxCredits == nil || *prior.ApprovedMaxCredits != start.Approval.ApprovedMaxCredits ||
			prior.CancellationPolicyVersion != start.Approval.CancellationPolicyVersion ||
			prior.Rate != start.Approval.Rate {
			return conflict
		}
		var err error
		if prior.Rate.Valid() || s.rates != nil && prior.HoldCredits == 0 {
			required, err = ReservationCreditsAt(start.Approval.Calls, prior.Rate)
		} else {
			required, err = ReservationCredits(start.Approval.Calls)
		}
		if err != nil {
			return err
		}
	} else {
		if prior.ApprovedMaxCredits != nil {
			return conflict
		}
		cost, err := s.worstCaseMicrousd(start.Calls)
		if err != nil {
			return err
		}
		if prior.Rate.Valid() {
			required, err = plan.ChargeAt(cost, prior.Rate)
			if err != nil {
				return err
			}
		} else if s.rates != nil && prior.HoldCredits == 0 {
			required = 0
		} else {
			required = plan.Charge(cost)
		}
	}
	if prior.HoldCredits != required {
		return conflict
	}
	return nil
}

// CheckModelAccess applies the same classification and live free-path gate to
// quotes and new holds. It has no balance or FX side effects.
func (s *Service) CheckModelAccess(ctx context.Context, acting plan.Plan, kind string, calls []PlannedCall) error {
	for _, c := range calls {
		if c.Units != nil {
			return ErrUnitApproval
		}
	}
	if !s.modelGrades {
		return nil
	}
	if len(calls) == 0 {
		return &ModelGradeError{}
	}
	for _, call := range calls {
		if call.Count <= 0 {
			continue
		}
		info, found := s.models.Lookup(call.Ref)
		if !found || call.Stage == "" || !info.ServesStage(call.Stage) {
			return &ModelGradeError{Ref: call.Ref.String(), Stage: call.Stage, Unavailable: true}
		}
		grade := info.Levels[call.Stage]
		required, allowed := plan.AllowsModelGrade(acting, grade)
		if !allowed {
			return &ModelGradeError{Ref: call.Ref.String(), Stage: call.Stage, Required: required, Grade: grade}
		}
		if grade == "free" {
			if !llm.ZeroUnitPrice(info.InputUSDPerMillion) || !llm.ZeroUnitPrice(info.OutputUSDPerMillion) {
				return &FreePathError{Ref: call.Ref.String(), Stage: call.Stage}
			}
			if qualifier, ok := s.models.(interface {
				QualifyFree(context.Context, string, llm.FreePath) (bool, error)
			}); ok {
				path := llm.FreeText
				if strings.Contains(kind, "clip") && info.VideoInput {
					path = llm.FreeVideoInput
				} else if call.Stage == llm.StageNameObserve {
					path = llm.FreeImageInput
				}
				qualified, err := qualifier.QualifyFree(ctx, call.Ref.ModelID, path)
				if err != nil || !qualified {
					return &FreePathError{Ref: call.Ref.String(), Stage: call.Stage}
				}
			}
		}
	}
	return nil
}

// Hold reserves the credits one piece of LLM work could cost, and records the start.
// It prices planned calls before the provider runs and spends from current lots in
// one transaction with the admission row. A retry keeps the first snapshot.
func (s *Service) Hold(ctx context.Context, start Start) error {
	if start.UserID == "" || start.Kind == "" || start.JobID == "" {
		return fmt.Errorf("hold: user, kind and job id are required")
	}
	if !start.Plan.Valid() {
		return fmt.Errorf("hold: acting plan is unknown")
	}
	for _, call := range start.Calls {
		if call.Units != nil {
			return s.holdUnits(ctx, start)
		}
	}
	if start.Approval != nil && len(start.Approval.Units) > 0 {
		return ErrUnitApproval
	}
	// A committed admission can be retried while the official source is down.
	// Its own snapshot is sufficient; no external FX fetch belongs on that path.
	if prior, _, found, err := s.holds.HoldForJob(ctx, start.JobID); err != nil {
		return err
	} else if found {
		return s.matchExistingHold(start, prior)
	}
	if !start.AccessChecked {
		if err := s.CheckModelAccess(ctx, start.Plan, start.Kind, start.Calls); err != nil {
			return err
		}
	}

	now := s.now()
	costMicrousd, err := s.worstCaseMicrousd(start.Calls)
	if err != nil {
		return err
	}
	required := 0
	var frozenRate plan.RateSnapshot
	if s.rates == nil {
		required = plan.Charge(costMicrousd)
	} else if start.Approval == nil {
		if costMicrousd == 0 {
			required = 0
		} else {
			var err error
			frozenRate, err = s.SelectRate(ctx)
			if err != nil {
				return err
			}
			required, err = plan.ChargeAt(costMicrousd, frozenRate)
			if err != nil {
				return ErrPricingUnavailable
			}
		}
	}
	var approved *int
	policyVersion := 0
	// Work the root marked as needing an approval may not start without one, and the
	// admission row it writes carries that ceiling — which is what every later gate reads
	// instead of asking which product this is.
	if s.requiresApproval(start.Kind) {
		if start.Approval == nil {
			return ErrApprovalRequired
		}
		var err error
		frozenRate = start.Approval.Rate
		var approvedCost int64
		approvedCost, err = ReservationCost(start.Approval.Calls)
		if err != nil {
			return err
		}
		if approvedCost == 0 && s.rates != nil {
			required = 0
		} else if frozenRate.Valid() {
			required, err = ReservationCreditsAt(start.Approval.Calls, frozenRate)
		} else if s.rates != nil {
			return ErrRateUnavailable
		} else {
			required, err = ReservationCredits(start.Approval.Calls)
		}
		if err != nil {
			return err
		}
		cap := start.Approval.ApprovedMaxCredits
		if cap < 0 {
			return ErrApprovalRequired
		}
		approved = &cap
		policyVersion = start.Approval.CancellationPolicyVersion
		if policyVersion < 0 || policyVersion > 1 {
			return ErrApprovalRequired
		}
	}

	return s.commitHold(ctx, start, required, frozenRate, approved, policyVersion, now)
}

func (s *Service) commitHold(ctx context.Context, start Start, required int, frozenRate plan.RateSnapshot, approved *int, policyVersion int, now time.Time) error {
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		if approved != nil && required > *approved {
			return &CreditCeilingError{Required: required, Approved: *approved}
		}
		// A committed reservation may lose its response. Check inside the same
		// writer transaction before renewing or spending any lot a second time.
		prior, _, found, err := tx.HoldForJob(ctx, start.JobID)
		if err != nil {
			return err
		}
		if found {
			if start.Approval != nil && len(start.Approval.Units) > 0 {
				u, err := unitStore(tx)
				if err != nil {
					return err
				}
				id, calls, err := u.UnitAdmission(ctx, start.JobID)
				if err != nil || id != start.Approval.UnitQuoteID || unitCallsDigest(calls) != unitCallsDigest(start.Approval.Units) || prior.UserID != start.UserID || prior.Kind != start.Kind || prior.HoldCredits != required {
					return ErrUnitApproval
				}
				return nil
			}
			return s.matchExistingHold(start, prior)
		}
		renewsAt, err := s.renew(ctx, tx, start.UserID, start.Plan, now)
		if err != nil {
			return err
		}

		// Master is exempt from the balance but its hold is still recorded: unlimited spend
		// is exactly the account whose spend the operator most wants to be able to read.
		var debits []LotDebit
		var eligible []Lot
		if !plan.Unlimited(start.Plan) {
			eligible, err = tx.LotsInConsumptionOrder(ctx, start.UserID, now)
			if err != nil {
				return err
			}
			debits, err = s.spend(ctx, tx, start.UserID, required, now, renewsAt)
			if err != nil {
				return err
			}
		}

		admission := Admission{
			UserID: start.UserID, Kind: start.Kind, JobID: start.JobID,
			HoldCredits: required, CreatedAt: now, ApprovedMaxCredits: approved,
			CancellationPolicyVersion: policyVersion,
			Rate:                      frozenRate,
		}
		if s.modelGrades {
			admission.AdmittedPlan = start.Plan
			for _, call := range start.Calls {
				if call.Count > 0 && call.Units == nil {
					info, _ := s.models.Lookup(call.Ref)
					admission.AdmittedModels = append(admission.AdmittedModels, AdmittedModel{Ref: call.Ref, Stage: call.Stage, Grade: info.Levels[call.Stage]})
				}
			}
		}
		if start.Approval != nil && len(start.Approval.Units) > 0 {
			admission.AdmittedPlan = start.Plan
		}
		if start.Plan != plan.Free && !plan.Unlimited(start.Plan) {
			coverage, found, err := s.anchors.CoverageFor(ctx, start.UserID, now)
			if err != nil {
				return err
			}
			if found {
				admission.CoverageID = coverage.ID
				dailyStart, _ := plan.DailyWindow(coverage.Anchor, now)
				benefitStart, _ := plan.BenefitWindow(coverage.Anchor, now)
				admission.DailyWindowStart, admission.BenefitWindowStart = &dailyStart, &benefitStart
			}
		}
		if err := tx.InsertAdmission(ctx, admission); err != nil {
			return err
		}
		if start.Approval != nil && len(start.Approval.Units) > 0 {
			u, err := unitStore(tx)
			if err != nil {
				return err
			}
			q, err := u.GetUnitQuote(ctx, start.UserID, start.Approval.UnitQuoteID)
			if err != nil || !s.now().Before(q.ExpiresAt) || q.Kind != start.Kind || q.Digest != unitCallsDigest(start.Approval.Units) || q.MaxCredits != required {
				return ErrUnitApproval
			}
			ok, err := u.ConsumeUnitQuote(ctx, q.ID, start.JobID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrUnitApproval
			}
			if err := u.SaveUnitAdmission(ctx, start.JobID, q.ID, start.Approval.Units); err != nil {
				return err
			}
		}
		if err := tx.InsertHoldDebits(ctx, start.JobID, debits); err != nil {
			return err
		}
		return tx.InsertEligibleLots(ctx, start.JobID, eligible)
	})
}

// AdmissionForJob reads the durable rights of an open job. A worker never takes
// a plan or free classification from its request payload.
func (s *Service) AdmissionForJob(ctx context.Context, jobID string) (Admission, bool, error) {
	admission, _, found, err := s.holds.HoldForJob(ctx, jobID)
	return admission, found, err
}

// spend takes credits out of the account's lots in consumption order, refusing before it
// writes anything if they do not cover the amount.
func (s *Service) spend(
	ctx context.Context, tx Storage, userID string, required int, now, renewsAt time.Time,
) ([]LotDebit, error) {
	lots, err := tx.LotsInConsumptionOrder(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	available := 0
	for _, lot := range lots {
		available += lot.Remaining
	}
	if available < required {
		return nil, &plan.InsufficientCreditsError{
			Required: required, Balance: available, RenewsAt: renewsAt,
		}
	}

	debits := make([]LotDebit, 0, len(lots))
	left := required
	for _, lot := range lots {
		if left == 0 {
			break
		}
		take := min(lot.Remaining, left)
		if take == 0 {
			continue
		}
		if err := tx.SpendFromLot(ctx, lot.ID, take); err != nil {
			return nil, err
		}
		debits = append(debits, LotDebit{LotID: lot.ID, Credits: take,
			OriginCoverageID: lot.CoverageID, OriginWindowStart: lot.WindowStart})
		left -= take
	}
	return debits, nil
}

// renewal is the coverage whose current windows renew opens at an instant, if any, and
// when the balance it produces next renews.
type renewal struct {
	coverage Coverage
	open     bool
	renewsAt time.Time
}

func (s *Service) renewalAt(ctx context.Context, userID string, acting plan.Plan, now time.Time) (renewal, error) {
	if plan.Unlimited(acting) || acting == plan.Free {
		return renewal{}, nil
	}
	coverage, found, err := s.anchors.CoverageFor(ctx, userID, now)
	if err != nil {
		return renewal{}, err
	}
	if !found || coverage.ID == "" || now.Before(coverage.Anchor) ||
		(!coverage.End.IsZero() && !now.Before(coverage.End)) {
		return renewal{}, nil
	}
	_, dailyEnd := plan.DailyWindow(coverage.Anchor, now)
	return renewal{coverage: coverage, open: true, renewsAt: earliest(dailyEnd, coverage.End)}, nil
}

// renew opens only the current eligible daily and monthly benefit windows. A missed
// window is never replayed and an absent paid coverage never creates a free grant.
func (s *Service) renew(
	ctx context.Context, tx Storage, userID string, acting plan.Plan, now time.Time,
) (time.Time, error) {
	r, err := s.renewalAt(ctx, userID, acting, now)
	if err != nil || !r.open {
		return time.Time{}, err
	}
	if err := s.openBenefits(ctx, tx, userID, r.coverage, now, "lazy", ""); err != nil {
		return time.Time{}, err
	}
	return r.renewsAt, nil
}

// renewalWrites reports whether renew would write anything for r. It reads, on the read
// pool, exactly the facts renew acts on — the current daily and monthly grant lots, an
// open legacy monthly lot to expire, the coverage's export window — and answers true
// whenever it cannot tell, so the write transaction stays the default.
func (s *Service) renewalWrites(ctx context.Context, r renewal, userID string, now time.Time) bool {
	if !r.open {
		return false
	}
	grants, export, err := s.benefitWindows(userID, r.coverage, now, "lazy", "")
	if err != nil {
		return true
	}
	if open, err := s.renewals.LegacyMonthlyLotOpen(ctx, userID, now); err != nil || open {
		return true
	}
	if opened, err := s.renewals.WindowGrantsOpened(ctx, grants); err != nil || !opened {
		return true
	}
	opened, err := s.renewals.ExportWindowOpened(ctx, export)
	return err != nil || !opened
}

func earliest(a, b time.Time) time.Time {
	if !b.IsZero() && b.Before(a) {
		return b
	}
	return a
}

func grantWindowID(kind LotKind, userID, coverageID string, start time.Time) string {
	return string(kind) + ":" + userID + ":" + coverageID + ":" + start.UTC().Format(time.RFC3339Nano)
}

func (s *Service) openBenefits(ctx context.Context, tx Storage, userID string, coverage Coverage, at time.Time, cause, correlation string) error {
	grants, export, err := s.benefitWindows(userID, coverage, at, cause, correlation)
	if err != nil {
		return err
	}
	if err := tx.ExpireLegacyMonthlyLots(ctx, userID, at); err != nil {
		return err
	}
	for _, grant := range grants {
		if _, err := tx.InsertLotIfAbsent(ctx, grant); err != nil {
			return err
		}
	}
	return tx.OpenExportWindow(ctx, export)
}

// benefitWindows is what openBenefits opens for a coverage at an instant: the current
// daily and monthly grant lots (a zero grant opens nothing) and the export window beside
// them. It writes nothing, so the renewal probe reads the very ids and ends it would open.
func (s *Service) benefitWindows(userID string, coverage Coverage, at time.Time, cause, correlation string) ([]Lot, ExportWindow, error) {
	if coverage.ID == "" || !coverage.Tier.Valid() || at.Before(coverage.Anchor) ||
		(!coverage.End.IsZero() && !at.Before(coverage.End)) {
		return nil, ExportWindow{}, errors.New("open benefits: invalid coverage")
	}
	dailyTier := coverage.DailyTier
	if !dailyTier.Valid() {
		dailyTier = coverage.Tier
	}
	dailyOffer, ok := plan.CommercialOffer(dailyTier)
	if !ok {
		return nil, ExportWindow{}, errors.New("open benefits: invalid daily tier")
	}
	bonusOffer, ok := plan.CommercialOffer(coverage.Tier)
	if !ok {
		return nil, ExportWindow{}, errors.New("open benefits: invalid benefit tier")
	}
	dailyStart, dailyEnd := plan.DailyWindow(coverage.Anchor, at)
	bonusStart, bonusEnd := plan.BenefitWindow(coverage.Anchor, at)
	var grants []Lot
	for _, grant := range []struct {
		kind       LotKind
		start, end time.Time
		amount     int
	}{
		{LotDaily, dailyStart, dailyEnd, dailyOffer.DailyCredits},
		{LotMonthly, bonusStart, bonusEnd, bonusOffer.MonthlyBonus},
	} {
		if grant.amount == 0 {
			continue
		}
		end := earliest(grant.end, coverage.End)
		start := grant.start
		grants = append(grants, Lot{
			ID:     grantWindowID(grant.kind, userID, coverage.ID, start),
			UserID: userID, Kind: grant.kind, CoverageID: coverage.ID,
			WindowStart: &start, IssuanceCause: cause, CorrelationID: correlation,
			Granted: grant.amount, Remaining: grant.amount, ExpiresAt: &end, CreatedAt: s.now(),
		})
	}
	return grants, ExportWindow{UserID: userID, CoverageID: coverage.ID,
		Start: bonusStart, End: earliest(bonusEnd, coverage.End), Allowance: bonusOffer.ServerExports}, nil
}

// OpenCoverage grants the first current day and month after a confirmed payment or
// support assignment. Its caller joins this write to tier and export-window writes.
func (s *Service) OpenCoverage(ctx context.Context, userID string, coverage Coverage, at time.Time, correlation string) error {
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		return s.openBenefits(ctx, tx, userID, coverage, at, "coverage", correlation)
	})
}

// AddUpgradeBonus preserves the day's old tier while adding only the prorated monthly
// difference. A distinct correlated lot makes a repeated payment callback harmless.
func (s *Service) AddUpgradeBonus(ctx context.Context, userID string, old Coverage, at time.Time, credits int, correlation string) error {
	if credits < 0 || correlation == "" {
		return errors.New("upgrade bonus: invalid amount or correlation")
	}
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		if err := s.openBenefits(ctx, tx, userID, old, at, "lazy", ""); err != nil {
			return err
		}
		if credits == 0 {
			return nil
		}
		start, end := plan.BenefitWindow(old.Anchor, at)
		end = earliest(end, old.End)
		_, err := tx.InsertLotIfAbsent(ctx, Lot{
			ID:     "monthly-upgrade:" + userID + ":" + old.ID + ":" + correlation,
			UserID: userID, Kind: LotMonthly, CoverageID: old.ID, WindowStart: &start,
			IssuanceCause: "upgrade", CorrelationID: correlation,
			Granted: credits, Remaining: credits, ExpiresAt: &end, CreatedAt: at,
		})
		return err
	})
}

func monthlyLotID(userID string, start time.Time) string {
	return "monthly:" + userID + ":" + start.In(time.FixedZone("Asia/Seoul", 9*60*60)).Format(time.DateOnly)
}

// worstCaseMicrousd prices every call the work will make at its largest possible shape.
//
// It reuses the ledger's own cost resolution, so an estimate and the row it will later be
// settled against can never be priced by two different rules. A model with no published
// price resolves to zero, which is correct for the one such model in the registry: it is
// free.
func (s *Service) worstCaseMicrousd(calls []PlannedCall) (int64, error) {
	var total int64
	for _, call := range calls {
		count := max(call.Count, 1)
		completionTokens := call.CompletionTokens
		if completionTokens <= 0 {
			completionTokens = s.maxCompletionTokens
		}
		info, found := s.models.Lookup(call.Ref)
		if !found {
			continue
		}
		cost := llm.ResolveCost(llm.CostInput{
			PromptTokens:        holdInputTokens,
			CompletionTokens:    completionTokens,
			InputUSDPerMillion:  info.InputUSDPerMillion,
			OutputUSDPerMillion: info.OutputUSDPerMillion,
		})
		if cost.Microusd < 0 || cost.Microusd > (math.MaxInt64-total)/int64(count) {
			return 0, ErrPricingUnavailable
		}
		total += cost.Microusd * int64(count)
	}
	return total, nil
}

// Settle reconciles a finished job's hold against what its calls actually cost, returning
// the remainder to the lots the hold came from.
//
// Refunding to the same lots rather than re-deriving them from the consumption order
// matters: by the time a job ends, a lot may have expired or a new one opened, and a
// refund into the wrong lot would quietly move credits between expiry dates.
//
// It is idempotent on the open-hold predicate, so a terminal transition that runs twice —
// a retry, or the boot sweep meeting a job that just finished — cannot refund twice.
func (s *Service) Settle(ctx context.Context, jobID string, outcome TerminalOutcome) error {
	return s.SettleCause(ctx, jobID, outcome, "unknown")
}

// SettleCause keeps the job owner's normalized terminal cause beside the once-only
// debit. A failed job without explicit provider evidence is an unknown fault.
func (s *Service) SettleCause(ctx context.Context, jobID string, outcome TerminalOutcome, cause string) error {
	if outcome != OutcomeSucceeded && outcome != OutcomeFailed && outcome != OutcomeCancelled {
		return ErrSettlementOutcome
	}
	if jobID == "" {
		return nil
	}
	now := s.now()

	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		admission, debits, found, err := tx.HoldForJob(ctx, jobID)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		if outcome == OutcomeCancelled && !s.cancellable(admission) {
			return ErrSettlementOutcome
		}

		cost, err := tx.CostForJob(ctx, jobID)
		if err != nil {
			return err
		}
		unitActual, unitWork, err := unitCharge(ctx, tx, admission, cost.ConfirmedMicrousd)
		if err != nil {
			return err
		}
		// An admission frozen before the FX migration retains its original
		// reservation policy. New all-free admissions have a zero hold instead.
		legacy := !unitWork && (s.rates == nil || !admission.Rate.Valid() && admission.HoldCredits > 0)
		actual := 0
		if legacy {
			actual = plan.Charge(cost.TotalMicrousd)
		}
		settlement := Settlement{Cause: cause}
		if !legacy {
			actual = 0
			if unitWork {
				actual = unitActual
			} else if admission.Rate.Valid() {
				actual, err = plan.ChargeAt(cost.ConfirmedMicrousd, admission.Rate)
				if err != nil {
					return err
				}
			} // An all-free admission cannot gain a retrospective paid charge.
			if admission.ApprovedMaxCredits != nil {
				actual = min(actual, admission.HoldCredits, *admission.ApprovedMaxCredits)
			}
			zero := 0
			confirmed := actual
			settlement.Reason, settlement.ConfirmedCharge, settlement.CancellationFee = outcome, &confirmed, &zero
		}
		// Approved work reserved its complete run up front. Never debit another lot for
		// provider overage: raw cost remains in the ledger, user credit is capped.
		if legacy && admission.ApprovedMaxCredits != nil {
			ceiling := min(admission.HoldCredits, *admission.ApprovedMaxCredits)
			actual = boundedCharge(cost.ConfirmedMicrousd, ceiling)
			if outcome == OutcomeFailed && cost.ConfirmedMicrousd == 0 {
				// No confirmed billable work: waive even the infrastructure base. Missing
				// usage stays unknown in the ledger; it is not a reported zero supplier bill.
				actual = 0
			}
			confirmed, fee := cancelledCharge(cost.ConfirmedMicrousd, ceiling)
			if outcome == OutcomeCancelled {
				actual = confirmed + fee
			} else {
				fee = 0
			}
			settlement.Reason, settlement.ConfirmedCharge, settlement.CancellationFee = outcome, &confirmed, &fee
		}

		switch {
		case actual < admission.HoldCredits:
			if err := refund(ctx, tx, debits, admission.HoldCredits-actual); err != nil {
				return err
			}
		case actual > admission.HoldCredits && len(debits) > 0:
			// The estimate was too low. Take what the lots can still give and stop there:
			// the balance floor is absolute, so the difference is ours, not a debt the
			// account carries into next month.
			//
			// The `len(debits) > 0` guard is what keeps an exempt tier exempt. A hold is
			// never zero (Charge has a per-request base), so a recorded hold that spent no
			// lot can only be master's — and charging its overrun here would drain a bonus
			// lot Hold deliberately left alone.
			spent, err := s.spendUpTo(ctx, tx, jobID, actual-admission.HoldCredits)
			if err != nil {
				return err
			}
			if !legacy {
				actual = admission.HoldCredits + spent
				settlement.ConfirmedCharge = &actual
			}
		}

		settlement.Credits = actual
		if !legacy && outcome == OutcomeFailed && cause != "provider" && actual > 0 && len(debits) > 0 {
			compensation := actual/2 + actual%2
			expires := now.AddDate(0, 0, 7)
			lotID := "compensation:" + jobID
			if _, err := tx.InsertLotIfAbsent(ctx, Lot{ID: lotID, UserID: admission.UserID,
				Kind: LotCompensation, Granted: compensation, Remaining: compensation,
				ExpiresAt: &expires, CreatedAt: now, IssuanceCause: "service_fault",
				CorrelationID: jobID}); err != nil {
				return err
			}
			settlement.CompensationCredits = compensation
			settlement.CompensationLotID = lotID
			settlement.CompensationExpiresAt = &expires
		}
		// The lots a settlement may still draw from are spent with it, as Release drops them.
		if err := tx.DeleteEligibleLotsForJob(ctx, jobID); err != nil {
			return err
		}
		return tx.MarkSettled(ctx, jobID, settlement, now)
	})
}

// refund returns credits to the lots they were taken from, newest debit last, so a lot
// can never be credited past what it granted.
func refund(ctx context.Context, tx Storage, debits []LotDebit, amount int) error {
	left := amount
	for _, debit := range debits {
		if left == 0 {
			break
		}
		give := min(debit.Credits, left)
		if err := tx.RefundToLot(ctx, debit.LotID, give); err != nil {
			return err
		}
		left -= give
	}
	return nil
}

// spendUpTo takes as much of amount as the lots hold, and reports what it took. Unlike
// spend it never refuses: settlement is charging for work already done, and the only
// question left is how much of it the balance can absorb.
func (s *Service) spendUpTo(
	ctx context.Context, tx Storage, jobID string, amount int,
) (int, error) {
	lots, err := tx.EligibleLotsForJob(ctx, jobID)
	if err != nil {
		return 0, err
	}
	left := amount
	for _, lot := range lots {
		if left == 0 {
			break
		}
		take := min(lot.Remaining, left)
		if take == 0 {
			continue
		}
		if err := tx.SpendFromLot(ctx, lot.ID, take); err != nil {
			return 0, err
		}
		left -= take
	}
	return amount - left, nil
}

// Release drops the hold for a job that was admitted but never created. Enqueue holds
// before it inserts — a refusal must leave no job row — so the rare failure between the
// two would otherwise leave the account charged for a start it never got.
func (s *Service) Release(ctx context.Context, jobID string) error {
	if jobID == "" {
		return nil
	}
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		admission, debits, found, err := tx.HoldForJob(ctx, jobID)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		if err := refund(ctx, tx, debits, admission.HoldCredits); err != nil {
			return err
		}
		if err := tx.DeleteEligibleLotsForJob(ctx, jobID); err != nil {
			return err
		}
		return tx.DeleteAdmissionForJob(ctx, jobID)
	})
}

// OpenHolds lists jobs whose hold has not been settled. The caller — the composition root
// at boot — asks the job context which of them are terminal and settles those; this
// context never reads another context's tables.
func (s *Service) OpenHolds(ctx context.Context) ([]string, error) {
	return s.holds.UnsettledHoldJobs(ctx)
}

// Record persists one completed provider call, pricing it by the shared reported →
// estimated → unavailable precedence. A model that has since left the registry still
// records its tokens; only its estimate is lost.
func (s *Service) Record(ctx context.Context, call Call) error {
	info, _ := s.models.Lookup(call.Model)
	// A call made under a reservation is priced at the frozen policy the reservation
	// admitted, so a catalog change between approval and call cannot move the bill.
	{
		if policy, ok := frozenCallPrice(ctx, call); ok {
			info.InputUSDPerMillion, info.OutputUSDPerMillion = policy.InputUSDPerMillion, policy.OutputUSDPerMillion
			if policy.Pricing.Version != 0 && !policy.Pricing.AggregateUsageSufficient {
				// An admission upper bound is not a measured multimodal/cache bill.
				// ResolveCost still preserves authoritative reported cost, including 0.
				info.InputUSDPerMillion, info.OutputUSDPerMillion = "", ""
			}
		}
	}
	cost := llm.ResolveCost(llm.CostInput{
		PromptTokens:        int64(call.Usage.PromptTokens),
		CompletionTokens:    int64(call.Usage.CompletionTokens),
		ReportedMicrousd:    call.Usage.CostMicrousd,
		Reported:            call.Usage.CostReported,
		InputUSDPerMillion:  info.InputUSDPerMillion,
		OutputUSDPerMillion: info.OutputUSDPerMillion,
	})

	return s.charges.InsertEvent(ctx, Event{
		UserID:             call.UserID,
		Kind:               call.Kind,
		JobID:              call.JobID,
		Stage:              call.Stage,
		Model:              call.Model.String(),
		PromptTokens:       int64(call.Usage.PromptTokens),
		CompletionTokens:   int64(call.Usage.CompletionTokens),
		ReasoningTokens:    int64(call.Usage.ReasoningTokens),
		ReasoningTruncated: call.ReasoningTruncated,
		CostMicrousd:       cost.Microusd,
		CostSource:         cost.Source,
		CreatedAt:          s.now(),
	})
}

// RecordCall writes the ledger row for one completed provider call, attributed to the
// work in context.
//
// A successful call is always recorded, even when the provider reported nothing: the row
// is the evidence the call happened. A FAILED call is recorded only when it reported
// usage (QUOTA-21), since those tokens were bought — because a call that never
// reached a model has nothing to account for.
// `stage` is the stage the CALL named for itself, in the llm boundary's stable form. It is
// preferred over StageFor because it is a fact rather than an inference: StageFor could only
// tell observe from write by comparing refs, and gave up when one model served both. A call
// that names none still falls back to it.
func (s *Service) RecordCall(ctx context.Context, ref llm.ModelRef, stage string, u llm.Usage, callErr error) error {
	work, ok := WorkFromContext(ctx)
	if !ok {
		return nil
	}
	if callErr != nil && u.PromptTokens == 0 && u.CompletionTokens == 0 && !u.CostReported {
		return nil
	}
	if s.requiresApproval(work.Kind) {
		// A provider can report paid usage while worker shutdown cancels its call.
		// Preserve that evidence for boot settlement without permitting another call.
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		ctx = recordCtx
	}
	if stage == "" {
		stage = work.StageFor(ref)
	}
	return s.Record(ctx, Call{
		UserID: work.UserID, Kind: work.Kind, JobID: work.JobID,
		Stage: stage, Model: ref, Usage: u,
		ReasoningTruncated: errors.As(callErr, new(*llm.TruncatedError)),
	})
}

// ReasoningSpendByModel is the recent reasoning-vs-completion split per model for one
// stage. It is the ledger's published answer to "is this model honoring its effort", read
// by the curation surface through its own port — the catalog never touches usage_events.
func (s *Service) ReasoningSpendByModel(ctx context.Context, stage string) ([]ReasoningSpend, error) {
	return s.charges.ReasoningSpend(ctx, stage, s.now().Add(-ReasoningSpendWindow))
}

// RecentPostFigures is every (stage, model)'s upper-median per-post credits over the last
// PostFigureWindow, each with the posts and accounts behind it (QUOTA-64). A row converts at
// the rate its own job was admitted at; a row whose rate or cost cannot be converted is left
// out rather than guessed. The floor is the caller's to apply through Eligible.
func (s *Service) RecentPostFigures(ctx context.Context) (map[StageModel]RecentPostFigure, error) {
	rows, err := s.charges.PostStageCosts(ctx, s.now().Add(-plan.PostFigureWindow))
	if err != nil {
		return nil, err
	}
	credits := map[StageModel][]int{}
	accounts := map[StageModel]map[string]bool{}
	for _, row := range rows {
		charged, err := plan.ChargeAt(row.CostMicrousd, row.Rate)
		if err != nil {
			continue
		}
		key := StageModel{Stage: row.Stage, Model: row.Model}
		credits[key] = append(credits[key], charged)
		if accounts[key] == nil {
			accounts[key] = map[string]bool{}
		}
		accounts[key][row.UserID] = true
	}
	out := make(map[StageModel]RecentPostFigure, len(credits))
	for key, values := range credits {
		out[key] = RecentPostFigure{Credits: plan.UpperMedian(values), Posts: len(values), Accounts: len(accounts[key])}
	}
	return out, nil
}

// BalanceFor reports what the account may spend, renewing the monthly grant first so a
// balance read at the boundary is never one grant behind.
//
// Most reads land inside windows already granted, so it first asks the read pool whether
// renewing would write anything; only a read that would grant, extend or expire something
// takes the writer. The answer is the same either way.
func (s *Service) BalanceFor(ctx context.Context, userID string, acting plan.Plan) (Balance, error) {
	now := s.now()
	if r, err := s.renewalAt(ctx, userID, acting, now); err == nil && !s.renewalWrites(ctx, r, userID, now) {
		return s.balanceAt(ctx, s.renewals.ReadLotsInConsumptionOrder, userID, acting, now, r.renewsAt)
	}

	var balance Balance
	err := s.tx.InWriteTx(ctx, func(tx Storage) error {
		renewsAt, err := s.renew(ctx, tx, userID, acting, now)
		if err != nil {
			return err
		}
		balance, err = s.balanceAt(ctx, tx.LotsInConsumptionOrder, userID, acting, now, renewsAt)
		return err
	})
	if err != nil {
		return Balance{}, err
	}
	return balance, nil
}

// balanceAt is the balance once its windows are renewed, read through lots.
func (s *Service) balanceAt(
	ctx context.Context, lots func(context.Context, string, time.Time) ([]Lot, error),
	userID string, acting plan.Plan, now, renewsAt time.Time,
) (Balance, error) {
	balance := Balance{RenewsAt: renewsAt, Unlimited: plan.Unlimited(acting)}
	if balance.Unlimited {
		return balance, nil
	}
	if acting != plan.Free {
		coverage, found, err := s.anchors.CoverageFor(ctx, userID, now)
		if err != nil {
			return Balance{}, err
		}
		if found && !now.Before(coverage.Anchor) && (coverage.End.IsZero() || now.Before(coverage.End)) {
			balance.CoverageID, balance.CoverageEnd = coverage.ID, coverage.End
			dailyTier := coverage.DailyTier
			if !dailyTier.Valid() {
				dailyTier = coverage.Tier
			}
			if offer, ok := plan.CommercialOffer(dailyTier); ok {
				balance.DailyGrant = offer.DailyCredits
			}
			if offer, ok := plan.CommercialOffer(coverage.Tier); ok {
				balance.MonthlyBonus = offer.MonthlyBonus
			}
			_, balance.DailyResetsAt = plan.DailyWindow(coverage.Anchor, now)
			balance.BenefitStart, balance.BenefitEnd = plan.BenefitWindow(coverage.Anchor, now)
			balance.BonusResetsAt = earliest(balance.BenefitEnd, coverage.End)
			balance.DailyResetsAt = earliest(balance.DailyResetsAt, coverage.End)
		}
	}

	found, err := lots(ctx, userID, now)
	if err != nil {
		return Balance{}, err
	}
	balance.Lots = found
	for _, lot := range found {
		balance.Credits += lot.Remaining
	}
	return balance, nil
}

// SpendableCredits is the current spendable balance for a model picker. It uses the
// same lazy grant as GetMyPlan, so the first read after a daily boundary does not show
// yesterday's expired balance.
func (s *Service) SpendableCredits(ctx context.Context, userID string, acting plan.Plan) (int, bool, error) {
	balance, err := s.BalanceFor(ctx, userID, acting)
	if err != nil {
		return 0, false, err
	}
	return balance.Credits, balance.Unlimited, nil
}

// CreditsFor is what one piece of work would hold, for a surface that must show a price
// before anything is started.
func (s *Service) CreditsFor(calls []PlannedCall) int {
	cost, err := s.worstCaseMicrousd(calls)
	if err != nil {
		return -1
	}
	if s.rates == nil {
		return plan.Charge(cost)
	}
	if cost == 0 {
		return 0
	}
	rate, err := s.SelectRate(context.Background())
	if err != nil {
		return -1 // Advisory picker estimate is unavailable; admission still refuses.
	}
	credits, err := plan.ChargeAt(cost, rate)
	if err != nil {
		return -1
	}
	return credits
}

// EnsureMonthlyLot opens the tier's monthly grant if the account has none that is
// current. Provisioning calls it so a new account can spend immediately rather than on
// whatever request happens to renew it first.
func (s *Service) EnsureMonthlyLot(ctx context.Context, userID string, acting plan.Plan) error {
	now := s.now()
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		_, err := s.renew(ctx, tx, userID, acting, now)
		return err
	})
}

// TopUpMonthlyLot raises the account's current monthly grant by `credits`.
//
// It is the credit half of a tier upgrade (QUOTA-35): someone who pays the difference this
// minute must be able to spend it this minute, so the lot they already hold grows rather
// than a second monthly lot opening beside it — ActiveMonthlyLot expects exactly one.
//
// An account with no current monthly lot is a no-op, not an error: its next request opens
// one at the new tier's size anyway.
func (s *Service) TopUpMonthlyLot(ctx context.Context, userID string, credits int) error {
	if credits <= 0 {
		return fmt.Errorf("top up: credits must be positive")
	}
	return s.tx.InWriteTx(ctx, func(tx Storage) error {
		lot, found, err := tx.ActiveMonthlyLot(ctx, userID, s.now())
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		return tx.RaiseLot(ctx, lot.ID, credits)
	})
}

// Grant opens a bonus lot. expiresAt may be nil, for a grant that does not expire.
func (s *Service) Grant(ctx context.Context, userID string, credits int, expiresAt *time.Time) error {
	if credits <= 0 {
		return fmt.Errorf("grant: credits must be positive")
	}
	return s.lots.InsertLot(ctx, Lot{
		ID: s.newID(), UserID: userID, Kind: LotBonus,
		Granted: credits, Remaining: credits,
		ExpiresAt: expiresAt, CreatedAt: s.now(),
	})
}
