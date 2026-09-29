// Package rpc is the plan context's transport edge. It owns the Plan enum mapping that
// every other edge needs — auth's session probe, the model catalog, the admin screen —
// so the ladder has exactly one proto↔domain translation.
package rpc

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// planWire is the one proto↔domain table for the ladder. It is a table rather than a switch
// with a default branch (ARCH-3): TestPlanMappingCoversGeneratedEnum walks the generated enum
// and the domain ladder against it, so a tier added on either side fails a test instead of
// travelling as UNSPECIFIED.
var planWire = map[plan.Plan]postpilotv1.Plan{
	plan.Free:   postpilotv1.Plan_PLAN_FREE,
	plan.Light:  postpilotv1.Plan_PLAN_LIGHT,
	plan.Basic:  postpilotv1.Plan_PLAN_BASIC,
	plan.Pro:    postpilotv1.Plan_PLAN_PRO,
	plan.Max:    postpilotv1.Plan_PLAN_MAX,
	plan.Master: postpilotv1.Plan_PLAN_MASTER,
}

// ToProto maps a domain plan onto the wire enum. A plan outside the table is UNSPECIFIED
// rather than a tier: a client that cannot read the value must not be told it is free.
func ToProto(p plan.Plan) postpilotv1.Plan {
	return planWire[p]
}

// FromProto maps the wire enum inward. UNSPECIFIED and unknown values are refused rather
// than defaulted, so an old client cannot set a tier by omission.
func FromProto(wire postpilotv1.Plan) (plan.Plan, bool) {
	for p, mapped := range planWire {
		if mapped == wire {
			return p, true
		}
	}
	return "", false
}

// Balance is what this edge publishes about an account's credits, declared by the consumer:
// the ledger's own shape stops at the adapter that wires the two (ARCH-7), so a lot column
// the ledger adds does not reach the wire by accident.
type Balance struct {
	// Credits is the sum of what the unexpired lots still hold. Zero for an unlimited
	// account, which reads Unlimited instead.
	Credits   int
	Unlimited bool
	Lots      []Lot
	// RenewsAt is when the next monthly grant opens: every refusal names it, so a user is
	// never told "later" without being told when.
	RenewsAt                              time.Time
	DailyGrant, MonthlyBonus              int
	DailyResetsAt, BonusResetsAt          time.Time
	CoverageID                            string
	CoverageEnd, BenefitStart, BenefitEnd time.Time
}

// Lot is one grant as the plan screen shows it.
type Lot struct {
	Kind      string
	Granted   int
	Remaining int
	// ExpiresAt is nil for a grant that does not expire.
	ExpiresAt     *time.Time
	CoverageID    string
	WindowStart   *time.Time
	IssuanceCause string
}

// Ledger is the balance this handler reports. Declared here by its consumer; the usage
// context implements it through an adapter in the composition root.
type Ledger interface {
	BalanceFor(ctx context.Context, userID string, acting plan.Plan) (Balance, error)
}

// ExportReader is the clip-owned counter viewed by the plan response. The
// plan context depends on this narrow consumer port, not clip storage.
type ExportReader interface {
	Current(ctx context.Context, userID string, at time.Time) (ExportBalance, bool, error)
}

type ExportBalance struct {
	CoverageID                string
	StartsAt, EndsAt          time.Time
	Allowance, Used, Reserved int
}

// EstimatorCombo is one priced combo as this edge publishes it. It is declared here so the
// context that owns the assignment never learns the wire shape, and the composition root
// maps between the two.
type EstimatorCombo struct {
	Combo             string
	ObserveLabel      string
	WriteLabel        string
	PerPhotoMilli     int
	PerVideoMilli     int
	Per1000CharsMilli int
	PerPostBaseMilli  int
	ClipRates         *plan.ClipRates
}

// Estimator publishes the operator's priced combos (QUOTA-40). Declared here by its
// consumer; the model catalog implements it, because the assignment is a curation decision.
type Estimator interface {
	ComboRates(ctx context.Context) ([]EstimatorCombo, error)
}

// Handler implements postpilotv1connect.PlanServiceHandler.
type Handler struct {
	ledger    Ledger
	estimator Estimator
	exports   ExportReader
}

func NewHandler(ledger Ledger, estimator Estimator) *Handler {
	return &Handler{ledger: ledger, estimator: estimator}
}

func (h *Handler) WithExports(exports ExportReader) *Handler { h.exports = exports; return h }

// GetMyPlan reports the caller's own tier and what it has left to spend.
//
// It reads the plan from the request context rather than from a payload — a tier in a
// message is a claim by the caller — and it publishes the grant table so the frontend
// renders numbers it never has to know.
//
// Reading a balance also renews it: the monthly grant opens on access, so a client that
// polls at the boundary sees the new lot rather than an expired one.
func (h *Handler) GetMyPlan(ctx context.Context, _ *connect.Request[postpilotv1.GetMyPlanRequest]) (*connect.Response[postpilotv1.GetMyPlanResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	acting, ok := auth.PlanFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}

	balance, err := h.ledger.BalanceFor(ctx, userID, acting)
	if err != nil {
		slog.Error("plan balance read failed", "user_id", userID, "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not read plan balance", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
	var serverWindow *postpilotv1.ServerExportWindow
	if acting != plan.Master && h.exports != nil {
		current, ok, readErr := h.exports.Current(ctx, userID, time.Now())
		if readErr != nil {
			slog.Error("server export balance read failed", "user_id", userID, "err", readErr)
			return nil, rpcserver.NewAppError(connect.CodeInternal, "could not read server export balance", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
		}
		if ok {
			serverWindow = &postpilotv1.ServerExportWindow{CoverageId: current.CoverageID,
				StartsAt: wireTime(current.StartsAt), EndsAt: wireTime(current.EndsAt),
				Allowance: int32(current.Allowance), Used: int32(current.Used), Reserved: int32(current.Reserved),
				Remaining: int32(max(0, current.Allowance-current.Used-current.Reserved))}
		} else {
			serverWindow = &postpilotv1.ServerExportWindow{EndsAt: wireTime(balance.RenewsAt)}
		}
	}

	lots := make([]*postpilotv1.CreditLot, 0, len(balance.Lots))
	for _, lot := range balance.Lots {
		expires := ""
		if lot.ExpiresAt != nil {
			expires = lot.ExpiresAt.UTC().Format(time.RFC3339)
		}
		lots = append(lots, &postpilotv1.CreditLot{
			Kind:          lot.Kind,
			Granted:       int32(lot.Granted),
			Remaining:     int32(lot.Remaining),
			ExpiresAt:     expires,
			CoverageId:    lot.CoverageID,
			WindowStart:   wireTimePointer(lot.WindowStart),
			IssuanceCause: lot.IssuanceCause,
		})
	}

	offers := make([]*postpilotv1.PlanOffer, 0, len(plan.Offers()))
	for _, offer := range plan.Offers() {
		offers = append(offers, &postpilotv1.PlanOffer{
			Plan:                 ToProto(offer.Plan),
			MonthlyCredits:       int32(offer.MonthlyCredits),
			PriceUsdCents:        int32(offer.PriceUSDCents),
			Recommended:          offer.Recommended,
			MonthlyKrw:           int32(offer.MonthlyKRW),
			AnnualKrw:            int32(offer.AnnualKRW),
			DailyCredits:         int32(offer.DailyCredits),
			MonthlyBonus:         int32(offer.MonthlyBonus),
			ModelCeiling:         offer.ModelCeiling,
			MonthlyServerExports: int32(offer.ServerExports),
		})
	}
	packs := make([]*postpilotv1.CreditPack, 0, len(plan.Packs()))
	for _, pack := range plan.Packs() {
		packs = append(packs, &postpilotv1.CreditPack{Id: pack.ID, PriceKrw: int32(pack.PriceKRW), Credits: int32(pack.Credits)})
	}

	// A comparison with no priced combo shows grants and prices and no post estimate. That
	// is a state the operator can fix, not a failure of this read, so a combo lookup that
	// fails is logged and answered as "none assigned" rather than failing GetMyPlan.
	var combos []*postpilotv1.EstimatorCombo
	priced, comboErr := h.estimator.ComboRates(ctx)
	if comboErr != nil {
		slog.Error("estimator combo read failed", "user_id", userID, "err", comboErr)
	}
	for _, combo := range priced {
		var clipRates *postpilotv1.ClipEstimatorRates
		if combo.ClipRates != nil {
			clipRates = &postpilotv1.ClipEstimatorRates{
				PerSourceMilli:       int32(combo.ClipRates.PerSource),
				PerOutputSecondMilli: int32(combo.ClipRates.PerOutputSecond),
				PerClipBaseMilli:     int32(combo.ClipRates.PerClipBase),
			}
		}
		combos = append(combos, &postpilotv1.EstimatorCombo{
			Combo:                 combo.Combo,
			ObserveLabel:          combo.ObserveLabel,
			WriteLabel:            combo.WriteLabel,
			PerPhotoMilli:         int32(combo.PerPhotoMilli),
			PerVideoMilli:         int32(combo.PerVideoMilli),
			PerThousandCharsMilli: int32(combo.Per1000CharsMilli),
			PerPostBaseMilli:      int32(combo.PerPostBaseMilli),
			ClipRates:             clipRates,
		})
	}
	var fxRate *postpilotv1.PlanFXRate
	fxUnavailable := false
	if source, ok := h.estimator.(interface {
		CurrentRate(context.Context) (plan.RateSnapshot, error)
	}); ok {
		rate, err := source.CurrentRate(ctx)
		if err != nil {
			fxUnavailable = true
		} else if rate.Valid() {
			fxRate = &postpilotv1.PlanFXRate{Source: rate.Source, PublicationDate: rate.PublicationDate,
				ReferenceE4: rate.ReferenceE4, AppliedE4: rate.AppliedE4, Temporary: rate.Temporary}
		}
	}

	return connect.NewResponse(&postpilotv1.GetMyPlanResponse{
		Plan:               ToProto(acting),
		Offers:             offers,
		EstimatorCombos:    combos,
		ClipSourceSeconds:  plan.EstimatorClipSourceSeconds,
		CreditPacks:        packs,
		FxRate:             fxRate,
		FxUnavailable:      fxUnavailable,
		ServerExportWindow: serverWindow,
		Balance: &postpilotv1.CreditBalance{
			Credits:            int32(balance.Credits),
			Unlimited:          balance.Unlimited,
			Lots:               lots,
			RenewsAt:           wireTime(balance.RenewsAt),
			DailyGrant:         int32(balance.DailyGrant),
			MonthlyBonus:       int32(balance.MonthlyBonus),
			DailyResetsAt:      wireTime(balance.DailyResetsAt),
			BonusResetsAt:      wireTime(balance.BonusResetsAt),
			CoverageId:         balance.CoverageID,
			CoverageEndsAt:     wireTime(balance.CoverageEnd),
			BenefitWindowStart: wireTime(balance.BenefitStart),
			BenefitWindowEnd:   wireTime(balance.BenefitEnd),
		},
	}), nil
}

func wireTime(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}
func wireTimePointer(at *time.Time) string {
	if at == nil {
		return ""
	}
	return wireTime(*at)
}

var _ postpilotv1connect.PlanServiceHandler = (*Handler)(nil)
