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
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voucher"
)

type usageAnchors struct {
	auth    *auth.Service
	billing interface {
		AnchorFor(context.Context, string) (time.Time, bool, error)
	}
	// late resolves the billing side at call time: the ledger is constructed before
	// billing (billing spends through it), so the anchors it is built with can only
	// name where billing will be.
	late *contexts
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
}

// clipAdmission is the charged clip path's hold: the clip context states its approved
// ceiling and priced lines in its own words, and this is where they become the ledger's.
type clipAdmission struct{ jobAdmission }

func (a clipAdmission) Hold(ctx context.Context, hold clipapp.Hold) error {
	reservation := &usage.Reservation{
		ApprovedMaxCredits:        hold.Reservation.ApprovedMaxCredits,
		CancellationPolicyVersion: hold.Reservation.CancellationPolicyVersion,
	}
	for _, c := range hold.Reservation.Calls {
		reservation.Calls = append(reservation.Calls, usage.PricedCall{Policy: c.Policy, Count: c.Count})
	}
	return a.hold(ctx, job.Start{UserID: hold.UserID, Kind: hold.Kind, JobID: hold.JobID, Calls: hold.Calls}, reservation)
}

func (a jobAdmission) Hold(ctx context.Context, start job.Start) error {
	return a.hold(ctx, start, nil)
}

func (a jobAdmission) hold(ctx context.Context, start job.Start, clipReservation *usage.Reservation) error {
	// The request's own tier is preferred so one request is judged against one tier
	// throughout; a start made from a worker context has no session to read, and falls back
	// to the stored row, which is the same authority the interceptor resolved from.
	acting, ok := auth.PlanFromContext(ctx)
	if !ok {
		stored, err := a.plans.PlanOf(ctx, start.UserID)
		if err != nil {
			return fmt.Errorf("hold %s: resolve acting plan: %w", start.Kind, err)
		}
		acting = stored
	}
	calls := make([]usage.PlannedCall, 0, len(start.Calls))
	for _, call := range start.Calls {
		calls = append(calls, usage.PlannedCall{
			Ref: parseRegistryRef(call.Ref), Count: call.Count, CompletionTokens: int64(call.CompletionTokens),
		})
	}
	return a.ledger.Hold(ctx, usage.Start{
		UserID: start.UserID, Plan: acting, Kind: start.Kind, JobID: start.JobID, Calls: calls,
		Approval: clipReservation,
	})
}

func (a jobAdmission) Release(ctx context.Context, jobID string) {
	if err := a.ledger.Release(ctx, jobID); err != nil {
		slog.Error("release hold failed", "job", jobID, "err", err)
	}
}

func (a jobAdmission) Settle(ctx context.Context, jobID, terminalStatus string) {
	var outcome usage.TerminalOutcome
	switch terminalStatus {
	case job.StatusDone:
		outcome = usage.OutcomeSucceeded
	case job.StatusFailed:
		outcome = usage.OutcomeFailed
	case job.StatusCancelled:
		outcome = usage.OutcomeCancelled
	}
	if err := a.ledger.Settle(ctx, jobID, outcome); err != nil {
		slog.Error("settle hold failed", "job", jobID, "err", err)
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
type billingCredits struct{ billing.Credits }

func (c billingCredits) VoidUntouchedLot(ctx context.Context, lotID string) error {
	err := c.Credits.VoidUntouchedLot(ctx, lotID)
	if errors.Is(err, usage.ErrLotTouched) {
		return billing.ErrLotTouched
	}
	return err
}
