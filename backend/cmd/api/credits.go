package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voucher"
)

// usageAnchors is the composition seam between the credit ledger, subscriptions and
// account identity. It prefers an active subscription without teaching either context
// about the other's persistence.
type usageAnchors struct {
	auth    *auth.Service
	billing interface {
		AnchorFor(context.Context, string) (time.Time, bool, error)
		CoverageAt(context.Context, string, time.Time) (billing.Coverage, bool, error)
	}
	// late resolves the billing side at call time: the ledger is constructed before
	// billing (billing spends through it), so the anchors it is built with can only
	// name where billing will be.
	late *contexts
}

func (a usageAnchors) CoverageFor(ctx context.Context, userID string, at time.Time) (usage.Coverage, bool, error) {
	billingService := a.billing
	if billingService == nil && a.late != nil {
		billingService = a.late.billing
	}
	if billingService != nil {
		coverage, found, err := billingService.CoverageAt(ctx, userID, at)
		if err != nil || found {
			return usage.Coverage{ID: coverage.ID, Anchor: coverage.Anchor, End: coverage.End,
				Tier: coverage.Tier, DailyTier: coverage.DailyTier}, found, err
		}
	}
	return usage.Coverage{}, false, nil
}

func (a usageAnchors) AnchorFor(ctx context.Context, userID string) (time.Time, error) {
	billing := a.billing
	if billing == nil && a.late != nil && a.late.billing != nil {
		billing = a.late.billing
	}
	if billing != nil {
		anchor, found, err := billing.AnchorFor(ctx, userID)
		if err != nil {
			return time.Time{}, err
		}
		if found {
			return anchor, nil
		}
	}
	return a.auth.CreatedAt(ctx, userID)
}

// throttledRoute puts the per-IP window in front of a plain HTTP route.
//
// It refuses BEFORE the handler runs, which is the point: the webhook's first act is an
// outbound call to the payment provider, so a refusal that happened afterwards would already
// have spent what it was meant to protect. The client IP is resolved exactly as the auth
// interceptor resolves it, so the configured ingress header means one thing across the
// process.
type meteredRegistry struct {
	*llm.Registry
	ledger *usage.Service
}

func (m meteredRegistry) Complete(ctx context.Context, ref llm.ModelRef, req llm.Request) (llm.Response, error) {
	// The clip context owns the execution-policy rule; the registry is not touched
	// until the call is admitted, so a refusal costs no provider call.
	ctx, err := clipapp.AdmitExecution(ctx, ref, req)
	if err != nil {
		return llm.Response{}, err
	}
	if work, ok := usage.WorkFromContext(ctx); ok && m.ledger != nil {
		admission, found, err := m.ledger.AdmissionForJob(ctx, work.JobID)
		if err != nil {
			return llm.Response{}, err
		}
		if !found || admission.UserID != work.UserID {
			return llm.Response{}, llm.ErrModelUnavailable
		}
		calls := make([]llm.AdmittedCall, 0, len(admission.AdmittedModels))
		for _, model := range admission.AdmittedModels {
			calls = append(calls, llm.AdmittedCall{Ref: model.Ref, Stage: model.Stage, Grade: model.Grade})
		}
		ctx = llm.WithAdmittedCalls(ctx, calls)
	}
	response, err := m.Registry.Complete(ctx, ref, req)
	// A ledger failure never fails the user's work: the tokens are already spent, and the
	// budget it protects is a soft cap enforced at the NEXT admission.
	// req.Stage is what the CALL said it was for, so the ledger records the stage as a fact
	// instead of inferring it from the ref — which is what made a write call whose model also
	// served observation indistinguishable from an observation call.
	if recordErr := m.ledger.RecordCall(ctx, ref, req.Stage, response.Usage, err); recordErr != nil {
		slog.Error("usage ledger write failed", "model", ref.String(), "err", recordErr)
	}
	return response, err
}

// jobAdmission is the plan gate at the job-enqueue seam. It is the one place that joins
// the three things the decision needs and that no single context holds: the acting plan
// (from the request context), the floors of the refs the job will run (from the registry),
// and the account's ledger position (from usage).
type jobAdmission struct {
	ledger   *usage.Service
	registry *llm.Registry
	plans    *auth.Service
	jobs     interface {
		GetByID(context.Context, string) (job.Job, error)
	}
}

// clipAdmission is the charged clip path's hold: the clip context states its approved
// ceiling and priced lines in its own words, and this is where they become the ledger's.
type clipAdmission struct{ jobAdmission }

func (a clipAdmission) Hold(ctx context.Context, hold clipapp.Hold) error {
	reservation := &usage.Reservation{
		ApprovedMaxCredits:        hold.Reservation.ApprovedMaxCredits,
		CancellationPolicyVersion: hold.Reservation.CancellationPolicyVersion,
		Rate:                      hold.Reservation.Rate,
	}
	for _, c := range hold.Reservation.Calls {
		reservation.Calls = append(reservation.Calls, usage.PricedCall{Policy: c.Policy, Count: c.Count})
	}
	return a.hold(ctx, job.Start{UserID: hold.UserID, Kind: hold.Kind, JobID: hold.JobID, Calls: hold.Calls}, reservation, hold.AccessChecked)
}

// CheckAccess is the clip guard's model-access check, run on the non-transaction ledger
// before the hold's writer transaction opens: a free path's live qualification fetches
// the provider's endpoint document, which no write transaction may wait on (ARCH-10).
func (a clipAdmission) CheckAccess(ctx context.Context, hold clipapp.Hold) error {
	acting, err := a.actingPlan(ctx, hold.UserID, hold.Kind)
	if err != nil {
		return err
	}
	return a.ledger.CheckModelAccess(ctx, acting, hold.Kind, plannedCalls(hold.Calls))
}

func (a jobAdmission) Hold(ctx context.Context, start job.Start) error {
	return a.hold(ctx, start, nil, false)
}

// hold admits the start on the ledger. accessChecked is true only on the clip path, whose
// guard already ran the same plan's access check over the same calls before its transaction.
func (a jobAdmission) hold(ctx context.Context, start job.Start, clipReservation *usage.Reservation, accessChecked bool) error {
	acting, err := a.actingPlan(ctx, start.UserID, start.Kind)
	if err != nil {
		return err
	}
	return a.ledger.Hold(ctx, usage.Start{
		UserID: start.UserID, Plan: acting, Kind: start.Kind, JobID: start.JobID, Calls: plannedCalls(start.Calls),
		Approval: clipReservation, AccessChecked: accessChecked,
	})
}

// actingPlan is the tier an admission is decided under: the current stored tier, even when
// a long-lived session still carries the tier from before a downgrade.
func (a jobAdmission) actingPlan(ctx context.Context, userID, kind string) (plan.Plan, error) {
	acting, err := a.plans.PlanOf(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("hold %s: resolve acting plan: %w", kind, err)
	}
	return acting, nil
}

// plannedCalls is the queue's planned calls in the ledger's words, so the access check
// and the hold price exactly the same calls.
func plannedCalls(planned []job.PlannedCall) []usage.PlannedCall {
	calls := make([]usage.PlannedCall, 0, len(planned))
	for _, call := range planned {
		calls = append(calls, usage.PlannedCall{
			Ref: parseRegistryRef(call.Ref), Stage: call.Stage, Count: call.Count, CompletionTokens: int64(call.CompletionTokens),
			PromptTokens: int64(call.PromptTokens),
		})
	}
	return calls
}

func (a jobAdmission) Release(ctx context.Context, jobID string) {
	if err := a.ledger.Release(ctx, jobID); err != nil {
		slog.Error("release hold failed", "job", jobID, "err", err)
	}
}

func (a jobAdmission) Settle(ctx context.Context, jobID, terminalStatus string) {
	var outcome usage.TerminalOutcome
	cause := "unknown"
	switch terminalStatus {
	case job.StatusDone:
		outcome = usage.OutcomeSucceeded
	case job.StatusFailed:
		outcome = usage.OutcomeFailed
		if a.jobs != nil {
			found, err := a.jobs.GetByID(ctx, jobID)
			if err == nil && found.Failure != nil {
				cause = failureSettlementCause(found.Failure.Reason)
			}
		}
	case job.StatusCancelled:
		outcome = usage.OutcomeCancelled
	}
	if terminalStatus == job.StatusDone {
		cause = "completed"
	} else if terminalStatus == job.StatusCancelled {
		cause = "owner_cancelled"
	}
	if err := a.ledger.SettleCause(ctx, jobID, outcome, cause); err != nil {
		slog.Error("settle hold failed", "job", jobID, "err", err)
	}
}

func failureSettlementCause(reason string) string {
	switch reason {
	case llm.FailureReasonModelUnavailable, llm.FailureReasonModelRateLimited, llm.FailureReasonModelUnsupported:
		return "provider"
	case job.FailureReasonInterrupted, job.FailureReasonPanicked, job.FailureReasonHandlerMissing:
		return "service"
	default:
		return "unknown"
	}
}

func (a jobAdmission) OpenHolds(ctx context.Context) ([]string, error) {
	return a.ledger.OpenHolds(ctx)
}

// providerCredits lets the model picker price a choice with the same estimator the gate
// applies when the work starts, so what a user is shown and what they are charged cannot
// be computed two different ways.
type providerCredits struct {
	ledger *usage.Service
	plans  *auth.Service
	budget config.LLMCompletionBudget
}

func (a providerCredits) ForCalls(calls []provider.PlannedCall) int {
	priced := make([]usage.PlannedCall, 0, len(calls))
	for _, call := range calls {
		completionTokens := 0
		switch call.Stage {
		case provider.StageObserve:
			completionTokens = a.budget.Observation()
		case provider.StageWrite:
			completionTokens = a.budget.Write(nil, call.NativeEffort)
		}
		priced = append(priced, usage.PlannedCall{
			Ref: call.Ref, Count: call.Count, CompletionTokens: int64(completionTokens),
		})
	}
	return a.ledger.CreditsFor(priced)
}

func (a providerCredits) Balance(ctx context.Context, userID string) (int, bool, error) {
	acting, ok := auth.PlanFromContext(ctx)
	if !ok {
		stored, err := a.plans.PlanOf(ctx, userID)
		if err != nil {
			return 0, false, fmt.Errorf("resolve acting plan: %w", err)
		}
		acting = stored
	}
	return a.ledger.SpendableCredits(ctx, userID, acting)
}

func (a providerCredits) Tier(ctx context.Context, userID string) (plan.Plan, error) {
	return a.plans.PlanOf(ctx, userID)
}

var _ provider.Credits = providerCredits{}

// parseRegistryRef splits the stored "provider/model" form a job records. The model id may
// itself contain slashes (`anthropic/claude-opus-5` under the `openrouter` provider), so
// only the first separator is a boundary.
func parseRegistryRef(ref string) llm.ModelRef {
	providerID, modelID, ok := strings.Cut(ref, "/")
	if !ok {
		return llm.ModelRef{}
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}
}

// voucherCredits is the ledger as the voucher context asks for it (QUOTA-58). It translates
// the ledger's standing type into the voucher's own, so the voucher package imports no ledger.
type voucherCredits struct{ ledger *usage.Service }

func (c voucherCredits) OpenVoucherLot(ctx context.Context, userID string, credits int, expiresAt time.Time) (string, error) {
	return c.ledger.OpenVoucherLot(ctx, userID, credits, expiresAt)
}

func (c voucherCredits) ExpireVoucherLot(ctx context.Context, lotID string, at time.Time) error {
	return c.ledger.ExpireVoucherLot(ctx, lotID, at)
}

func (c voucherCredits) VoucherLotStandings(ctx context.Context, lotIDs []string, at time.Time) (map[string]voucher.LotStanding, error) {
	standings, err := c.ledger.VoucherLotStandings(ctx, lotIDs, at)
	if err != nil {
		return nil, err
	}
	out := make(map[string]voucher.LotStanding, len(standings))
	for id, standing := range standings {
		out[id] = voucher.LotStanding{Remaining: standing.Remaining, ExpiresAt: standing.ExpiresAt}
	}
	return out, nil
}

// billingCredits is the ledger as the billing context asks for it. The only thing it adds is
// the translation ARCH-7 wants at the boundary: the ledger's sentinel for "this lot is no
// longer whole" becomes billing's own, so the refund path matches an error it owns and the
// billing package imports no ledger at all.
type billingCredits struct {
	*usage.Service
	exports clip.ExportWindows
}

type supportAccounts struct {
	auth    *auth.Service
	billing *billing.Service
}

type usageExports struct{ clip.ExportWindows }

func (a usageExports) OpenExportWindow(ctx context.Context, window usage.ExportWindow) error {
	return a.ExportWindows.OpenExportWindow(ctx, clipExportWindow(window), "lazy")
}

// clipExportWindow is the ledger's export window in the clip context's words.
func clipExportWindow(window usage.ExportWindow) clip.ExportWindow {
	return clip.ExportWindow{UserID: window.UserID, CoverageID: window.CoverageID,
		Start: window.Start, End: window.End, Allowance: window.Allowance}
}

// ledgerCoverage is billing's coverage in the ledger's words.
func ledgerCoverage(coverage billing.Coverage) usage.Coverage {
	return usage.Coverage{ID: coverage.ID, Anchor: coverage.Anchor, End: coverage.End,
		Tier: coverage.Tier, DailyTier: coverage.DailyTier}
}

// ExportWindowOpened answers the ledger's renewal probe from clip's window read: opening
// changes nothing only when the window of this coverage and start already ends no sooner.
// A window the open would refuse, or any other current window, answers "it would write".
func (a usageExports) ExportWindowOpened(ctx context.Context, window usage.ExportWindow) (bool, error) {
	if window.UserID == "" || window.CoverageID == "" || !window.Start.Before(window.End) || window.Allowance < 0 {
		return false, nil
	}
	current, found, err := a.ExportWindows.CurrentExportWindow(ctx, window.UserID, window.Start)
	if err != nil || !found {
		return false, err
	}
	return current.CoverageID == window.CoverageID && current.Start.Equal(window.Start) && !current.End.Before(window.End), nil
}

func (a supportAccounts) ListUsers(ctx context.Context) ([]auth.User, error) {
	return a.auth.ListUsers(ctx)
}
func (a supportAccounts) SetUserPlan(ctx context.Context, userID string, target plan.Plan) error {
	return a.billing.AssignSupportTier(ctx, userID, target)
}

func (c billingCredits) OpenCoverage(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, correlationID string) error {
	if err := c.Service.OpenCoverage(ctx, userID, ledgerCoverage(coverage), at, correlationID); err != nil {
		return err
	}
	return c.openExport(ctx, userID, coverage, at, correlationID)
}

func (c billingCredits) AddUpgradeBonus(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, credits, exportDelta int, correlationID string) error {
	if err := c.Service.AddUpgradeBonus(ctx, userID, ledgerCoverage(coverage), at, credits, correlationID); err != nil {
		return err
	}
	if err := c.openExport(ctx, userID, coverage, at, ""); err != nil {
		return err
	}
	window, _ := usage.ExportWindowAt(userID, ledgerCoverage(coverage), at)
	return c.exports.RaiseExportWindow(ctx, clipExportWindow(window), exportDelta, correlationID)
}

func (c billingCredits) openExport(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, correlationID string) error {
	if c.exports == nil {
		return errors.New("billing export windows are not wired")
	}
	window, ok := usage.ExportWindowAt(userID, ledgerCoverage(coverage), at)
	if !ok {
		return errors.New("billing export window: invalid tier")
	}
	return c.exports.OpenExportWindow(ctx, clipExportWindow(window), correlationID)
}

func (c billingCredits) VoidUntouchedLot(ctx context.Context, lotID string) error {
	err := c.Service.VoidUntouchedLot(ctx, lotID)
	if errors.Is(err, usage.ErrLotTouched) {
		return billing.ErrLotTouched
	}
	return err
}
