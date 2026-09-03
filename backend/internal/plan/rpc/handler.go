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
	"github.com/postpilot/backend/internal/usage"
)

// ToProto maps a domain plan onto the wire enum. An unknown plan becomes UNSPECIFIED
// rather than a tier: a client that cannot read the value must not be told it is free.
func ToProto(p plan.Plan) postpilotv1.Plan {
	switch p {
	case plan.Free:
		return postpilotv1.Plan_PLAN_FREE
	case plan.Basic:
		return postpilotv1.Plan_PLAN_BASIC
	case plan.Pro:
		return postpilotv1.Plan_PLAN_PRO
	case plan.Max:
		return postpilotv1.Plan_PLAN_MAX
	case plan.Master:
		return postpilotv1.Plan_PLAN_MASTER
	default:
		return postpilotv1.Plan_PLAN_UNSPECIFIED
	}
}

// FromProto maps the wire enum inward. UNSPECIFIED and unknown values are refused rather
// than defaulted, so an old client cannot set a tier by omission.
func FromProto(p postpilotv1.Plan) (plan.Plan, bool) {
	switch p {
	case postpilotv1.Plan_PLAN_FREE:
		return plan.Free, true
	case postpilotv1.Plan_PLAN_BASIC:
		return plan.Basic, true
	case postpilotv1.Plan_PLAN_PRO:
		return plan.Pro, true
	case postpilotv1.Plan_PLAN_MAX:
		return plan.Max, true
	case postpilotv1.Plan_PLAN_MASTER:
		return plan.Master, true
	default:
		return "", false
	}
}

// Ledger is the balance this handler reports. Declared here by its consumer; the usage
// context implements it.
type Ledger interface {
	BalanceFor(ctx context.Context, userID string, acting plan.Plan) (usage.Balance, error)
}

// Handler implements postpilotv1connect.PlanServiceHandler.
type Handler struct {
	ledger Ledger
}

func NewHandler(ledger Ledger) *Handler { return &Handler{ledger: ledger} }

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
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	acting, ok := auth.PlanFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}

	balance, err := h.ledger.BalanceFor(ctx, userID, acting)
	if err != nil {
		slog.Error("plan balance read failed", "user_id", userID, "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not read plan balance", "UNKNOWN_FAILURE", nil)
	}

	lots := make([]*postpilotv1.CreditLot, 0, len(balance.Lots))
	for _, lot := range balance.Lots {
		expires := ""
		if lot.ExpiresAt != nil {
			expires = lot.ExpiresAt.UTC().Format(time.RFC3339)
		}
		lots = append(lots, &postpilotv1.CreditLot{
			Kind:      string(lot.Kind),
			Granted:   int32(lot.Granted),
			Remaining: int32(lot.Remaining),
			ExpiresAt: expires,
		})
	}

	offers := make([]*postpilotv1.PlanOffer, 0, len(plan.Offers()))
	for _, offer := range plan.Offers() {
		offers = append(offers, &postpilotv1.PlanOffer{
			Plan:           ToProto(offer.Plan),
			MonthlyCredits: int32(offer.MonthlyCredits),
			PriceUsdCents:  int32(offer.PriceUSDCents),
		})
	}

	return connect.NewResponse(&postpilotv1.GetMyPlanResponse{
		Plan:   ToProto(acting),
		Offers: offers,
		Balance: &postpilotv1.CreditBalance{
			Credits:      int32(balance.Credits),
			Unlimited:    balance.Unlimited,
			Lots:         lots,
			RenewsAt:     balance.RenewsAt.UTC().Format(time.RFC3339),
			MonthlyGrant: int32(plan.MonthlyCredits(acting)),
		},
	}), nil
}

var _ postpilotv1connect.PlanServiceHandler = (*Handler)(nil)
