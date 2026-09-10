package usage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	store   Store
	models  Models
	anchors Anchors

	// maxCompletionTokens is the same cap the registry sends on a call that sets none. It is
	// only a fallback for a planned call whose caller did not declare a stage budget.
	maxCompletionTokens int64

	// now and newID are seams for tests in this package, not configuration: every window
	// is a calendar boundary, so the only way to exercise one is to move the clock.
	now   func() time.Time
	newID func() string
}

func NewService(store Store, models Models, maxCompletionTokens int64) *Service {
	return &Service{
		store: store, models: models,
		maxCompletionTokens: maxCompletionTokens,
		now:                 time.Now, newID: newID,
	}
}

// SetAnchors attaches the account-specific monthly-window resolver. It is required for
// every path that renews a balance; an unwired ledger fails instead of silently reverting
// to a calendar month.
func (s *Service) SetAnchors(anchors Anchors) { s.anchors = anchors }

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
	_, err := s.store.InsertLotIfAbsent(ctx, Lot{
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
	return s.store.InWriteTx(ctx, func(tx Store) error {
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
	lot, found, err := s.store.ActiveMonthlyLot(ctx, userID, s.now())
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return s.store.RaiseLot(ctx, lot.ID, credits)
}

func (s *Service) OpenPurchasedLot(ctx context.Context, userID string, credits int) (string, error) {
	if userID == "" || credits <= 0 {
		return "", errors.New("open purchased lot: user and positive credits are required")
	}
	id := "purchased:" + s.newID()
	err := s.store.InsertLot(ctx, Lot{ID: id, UserID: userID, Kind: LotPurchased, Granted: credits, Remaining: credits, CreatedAt: s.now()})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) VoidUntouchedLot(ctx context.Context, lotID string) error {
	ok, err := s.store.VoidUntouchedLot(ctx, lotID)
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
	return s.store.LotUntouched(ctx, lotID)
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
	ids, err := s.store.UntouchedPurchasedLots(ctx, lotIDs)
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
	ok, err := s.store.RestoreLot(ctx, lotID, credits)
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
	return s.store.InsertLotIfAbsent(ctx, Lot{ID: id, UserID: userID, Kind: LotBonus, Granted: credits, Remaining: credits, CreatedAt: s.now()})
}

// Hold reserves the credits one piece of LLM work could cost, and records the start.
//
// Reserving up front rather than charging afterwards is what bounds the account: cost is
// only knowable after a call returns, so the only honest guarantee is that nothing starts
// that the balance cannot already cover. The reservation and the admission row are one
// transaction, which is what makes that guarantee survive two concurrent starts reading
// the same balance.
//
// The overshoot the old budget axes accepted has not disappeared — a call may still cost
// more than its estimate — but it is absorbed by the hold rather than by the account.
func (s *Service) Hold(ctx context.Context, start Start) error {
	if start.UserID == "" || start.Kind == "" || start.JobID == "" {
		return fmt.Errorf("hold: user, kind and job id are required")
	}
	if !start.Plan.Valid() {
		return fmt.Errorf("hold: acting plan is unknown")
	}

	now := s.now()
	required := plan.Charge(s.worstCaseMicrousd(start.Calls))
	var approved *int
	if start.Kind == "generate_clip" {
		if start.Clip == nil {
			return ErrClipApproval
		}
		var err error
		required, err = ClipCredits(start.Clip.Calls)
		if err != nil {
			return err
		}
		cap := start.Clip.ApprovedMaxCredits
		if cap < 0 {
			return ErrClipApproval
		}
		approved = &cap
	}

	return s.store.InWriteTx(ctx, func(tx Store) error {
		if approved != nil && required > *approved {
			return &CreditCeilingError{Required: required, Approved: *approved}
		}
		renewsAt, err := s.renew(ctx, tx, start.UserID, start.Plan, now)
		if err != nil {
			return err
		}

		// Master is exempt from the balance but its hold is still recorded: unlimited spend
		// is exactly the account whose spend the operator most wants to be able to read.
		var debits []LotDebit
		if !plan.Unlimited(start.Plan) {
			debits, err = s.spend(ctx, tx, start.UserID, required, now, renewsAt)
			if err != nil {
				return err
			}
		}

		if err := tx.InsertAdmission(ctx, Admission{
			UserID: start.UserID, Kind: start.Kind, JobID: start.JobID,
			HoldCredits: required, CreatedAt: now, ApprovedMaxCredits: approved,
		}); err != nil {
			return err
		}
		return tx.InsertHoldDebits(ctx, start.JobID, debits)
	})
}

// spend takes credits out of the account's lots in consumption order, refusing before it
// writes anything if they do not cover the amount.
func (s *Service) spend(
	ctx context.Context, tx Store, userID string, required int, now, renewsAt time.Time,
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
		debits = append(debits, LotDebit{LotID: lot.ID, Credits: take})
		left -= take
	}
	return debits, nil
}

// renew opens the tier's monthly lot when the previous one has lapsed, and reports the
// instant the next grant opens.
//
// It runs on access rather than on a timer so a balance is correct on the first request
// after a boundary, whether or not anything was running when the boundary passed.
func (s *Service) renew(
	ctx context.Context, tx Store, userID string, acting plan.Plan, now time.Time,
) (time.Time, error) {
	if s.anchors == nil {
		return time.Time{}, errors.New("usage: monthly anchors are not wired")
	}
	anchor, err := s.anchors.AnchorFor(ctx, userID)
	if err != nil {
		return time.Time{}, fmt.Errorf("resolve monthly anchor: %w", err)
	}
	start, end := plan.AnchorWindow(anchor, now)

	if plan.Unlimited(acting) {
		return end, nil
	}

	current, found, err := tx.ActiveMonthlyLot(ctx, userID, now)
	if err != nil {
		return time.Time{}, err
	}
	if found && current.ExpiresAt != nil {
		return *current.ExpiresAt, nil
	}

	// The lapsed lot is not deleted: it is already excluded from every balance by its own
	// expiry, and keeping it leaves the grant history readable.
	granted := plan.MonthlyCredits(acting)
	if _, err := tx.InsertLotIfAbsent(ctx, Lot{
		ID: monthlyLotID(userID, start), UserID: userID, Kind: LotMonthly,
		Granted: granted, Remaining: granted,
		ExpiresAt: &end, CreatedAt: now,
	}); err != nil {
		return time.Time{}, err
	}
	return end, nil
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
func (s *Service) worstCaseMicrousd(calls []PlannedCall) int64 {
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
		total += cost.Microusd * int64(count)
	}
	return total
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
	if outcome != OutcomeSucceeded && outcome != OutcomeFailed {
		return ErrSettlementOutcome
	}
	if jobID == "" {
		return nil
	}
	now := s.now()

	return s.store.InWriteTx(ctx, func(tx Store) error {
		admission, debits, found, err := tx.HoldForJob(ctx, jobID)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}

		cost, err := tx.CostForJob(ctx, jobID)
		if err != nil {
			return err
		}
		actual := plan.Charge(cost.TotalMicrousd)
		// A clip reserves its complete run after probing. Never debit another lot for
		// provider overage: raw cost remains in the ledger, user credit is capped.
		if admission.Kind == "generate_clip" {
			ceiling := admission.HoldCredits
			if admission.ApprovedMaxCredits != nil {
				ceiling = min(ceiling, *admission.ApprovedMaxCredits)
			}
			actual = boundedClipCharge(cost.ConfirmedMicrousd, ceiling)
			if outcome == OutcomeFailed && cost.ConfirmedMicrousd == 0 {
				// No confirmed billable work: waive even the infrastructure base. Missing
				// usage stays unknown in the ledger; it is not a reported zero supplier bill.
				actual = 0
			}
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
			if _, err := s.spendUpTo(ctx, tx, admission.UserID, actual-admission.HoldCredits, now); err != nil {
				return err
			}
		}

		return tx.MarkSettled(ctx, jobID, actual, now)
	})
}

// refund returns credits to the lots they were taken from, newest debit last, so a lot
// can never be credited past what it granted.
func refund(ctx context.Context, tx Store, debits []LotDebit, amount int) error {
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
	ctx context.Context, tx Store, userID string, amount int, now time.Time,
) (int, error) {
	lots, err := tx.LotsInConsumptionOrder(ctx, userID, now)
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
	return s.store.InWriteTx(ctx, func(tx Store) error {
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
		return tx.DeleteAdmissionForJob(ctx, jobID)
	})
}

// OpenHolds lists jobs whose hold has not been settled. The caller — the composition root
// at boot — asks the job context which of them are terminal and settles those; this
// context never reads another context's tables.
func (s *Service) OpenHolds(ctx context.Context) ([]string, error) {
	return s.store.UnsettledHoldJobs(ctx)
}

// Record persists one completed provider call, pricing it by the shared reported →
// estimated → unavailable precedence. A model that has since left the registry still
// records its tokens; only its estimate is lost.
func (s *Service) Record(ctx context.Context, call Call) error {
	info, _ := s.models.Lookup(call.Model)
	if call.Kind == "generate_clip" {
		if policy, ok := frozenCallPrice(ctx, call); ok {
			info.InputUSDPerMillion, info.OutputUSDPerMillion = policy.InputUSDPerMillion, policy.OutputUSDPerMillion
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

	return s.store.InsertEvent(ctx, Event{
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
// usage — job 23 preserves that, and those tokens were bought — because a call that never
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
	if work.Kind == "generate_clip" {
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
	return s.store.ReasoningSpend(ctx, stage, s.now().Add(-ReasoningSpendWindow))
}

// BalanceFor reports what the account may spend, renewing the monthly grant first so a
// balance read at the boundary is never one grant behind.
func (s *Service) BalanceFor(ctx context.Context, userID string, acting plan.Plan) (Balance, error) {
	now := s.now()

	var balance Balance
	err := s.store.InWriteTx(ctx, func(tx Store) error {
		renewsAt, err := s.renew(ctx, tx, userID, acting, now)
		if err != nil {
			return err
		}
		balance.RenewsAt = renewsAt
		balance.Unlimited = plan.Unlimited(acting)
		if balance.Unlimited {
			return nil
		}

		lots, err := tx.LotsInConsumptionOrder(ctx, userID, now)
		if err != nil {
			return err
		}
		balance.Lots = lots
		for _, lot := range lots {
			balance.Credits += lot.Remaining
		}
		return nil
	})
	if err != nil {
		return Balance{}, err
	}
	return balance, nil
}

// SpendableCredits is the balance WITHOUT renewing it.
//
// It exists for read-only surfaces — a model picker asking what the caller can afford — that
// would otherwise take the single SQLite writer lock on every listing just to open a lot
// they are not about to spend from. The renewal still happens on the very next hold or
// balance read, so the worst this can be is one listing that shows a lapsed balance in the
// seconds after a month boundary.
func (s *Service) SpendableCredits(ctx context.Context, userID string, acting plan.Plan) (int, bool, error) {
	if plan.Unlimited(acting) {
		return 0, true, nil
	}
	lots, err := s.store.LotsInConsumptionOrder(ctx, userID, s.now())
	if err != nil {
		return 0, false, err
	}
	credits := 0
	for _, lot := range lots {
		credits += lot.Remaining
	}
	return credits, false, nil
}

// CreditsFor is what one piece of work would hold, for a surface that must show a price
// before anything is started.
func (s *Service) CreditsFor(calls []PlannedCall) int {
	return plan.Charge(s.worstCaseMicrousd(calls))
}

// EnsureMonthlyLot opens the tier's monthly grant if the account has none that is
// current. Provisioning calls it so a new account can spend immediately rather than on
// whatever request happens to renew it first.
func (s *Service) EnsureMonthlyLot(ctx context.Context, userID string, acting plan.Plan) error {
	now := s.now()
	return s.store.InWriteTx(ctx, func(tx Store) error {
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
	return s.store.InWriteTx(ctx, func(tx Store) error {
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
	return s.store.InsertLot(ctx, Lot{
		ID: s.newID(), UserID: userID, Kind: LotBonus,
		Granted: credits, Remaining: credits,
		ExpiresAt: expiresAt, CreatedAt: s.now(),
	})
}
