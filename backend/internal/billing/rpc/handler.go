package rpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/billing"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

type Handler struct{ service *billing.Service }

func NewHandler(service *billing.Service) *Handler { return &Handler{service: service} }

func (h *Handler) GetMyBilling(ctx context.Context, _ *connect.Request[postpilotv1.GetMyBillingRequest]) (*connect.Response[postpilotv1.GetMyBillingResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, authRequired()
	}
	view, err := h.service.GetMyBilling(ctx, userID)
	if err != nil {
		slog.Error("billing read failed", "user_id", userID, "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not read billing", "UNKNOWN_FAILURE", nil)
	}
	response := &postpilotv1.GetMyBillingResponse{CustomerKey: view.CustomerKey}
	if view.Subscription != nil {
		response.Subscription = toProtoSubscription(*view.Subscription)
	}
	if view.PaymentMethod != nil {
		response.PaymentMethod = &postpilotv1.BillingPaymentMethod{CardLabel: view.PaymentMethod.CardLabel, RegisteredAt: instant(view.PaymentMethod.RegisteredAt)}
	}
	for _, event := range view.History {
		response.History = append(response.History, toProtoEvent(event))
	}
	for _, purchase := range view.Purchases {
		response.Purchases = append(response.Purchases, toProtoPurchase(purchase))
	}
	return connect.NewResponse(response), nil
}

func (h *Handler) QuotePrice(ctx context.Context, req *connect.Request[postpilotv1.QuotePriceRequest]) (*connect.Response[postpilotv1.QuotePriceResponse], error) {
	if _, ok := auth.UserFromContext(ctx); !ok {
		return nil, authRequired()
	}
	tier, ok := planrpc.FromProto(req.Msg.GetPlan())
	term, termOK := termFromProto(req.Msg.GetTerm())
	if !ok || !termOK || (tier != plan.Basic && tier != plan.Pro && tier != plan.Max) {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid billing selection", "BILLING_SELECTION_INVALID", nil)
	}
	quote, err := h.service.QuotePrice(ctx, tier, term)
	if errors.Is(err, billing.ErrUnavailable) {
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition, "billing unavailable", "BILLING_UNAVAILABLE", nil)
	}
	if err != nil {
		slog.Error("billing quote failed", "user_id", func() string { id, _ := auth.UserFromContext(ctx); return id }(), "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "could not quote price", "UNKNOWN_FAILURE", nil)
	}
	return connect.NewResponse(&postpilotv1.QuotePriceResponse{UsdCents: int32(quote.USDCents), Krw: int64(quote.KRW), KrwPerUsdE4: quote.RatePerUSDE4, RateDate: quote.RateDate}), nil
}

func toProtoSubscription(value billing.Subscription) *postpilotv1.BillingSubscription {
	result := &postpilotv1.BillingSubscription{
		Plan: planrpc.ToProto(value.Tier), Term: termToProto(value.Term), AnchorAt: instant(value.AnchorAt),
		TermStart: instant(value.TermStart), TermEnd: instant(value.TermEnd), NextGrantAt: instant(value.NextGrantAt),
		AutoRenew: value.AutoRenew, Status: value.Status,
	}
	if value.ScheduledTier != nil {
		result.ScheduledPlan = planrpc.ToProto(*value.ScheduledTier)
	}
	if value.ScheduledTerm != nil {
		result.ScheduledTerm = termToProto(*value.ScheduledTerm)
	}
	return result
}

func toProtoEvent(value billing.Event) *postpilotv1.BillingEvent {
	result := &postpilotv1.BillingEvent{Id: value.ID, Kind: value.Kind, CreatedAt: instant(value.CreatedAt)}
	if value.Tier != nil {
		result.Plan = planrpc.ToProto(*value.Tier)
	}
	if value.Term != nil {
		result.Term = termToProto(*value.Term)
	}
	if value.Credits != nil {
		result.Credits = int32(*value.Credits)
	}
	if value.USDCents != nil {
		result.UsdCents = int32(*value.USDCents)
	}
	if value.KRWPerUSDE4 != nil {
		result.KrwPerUsdE4 = *value.KRWPerUSDE4
	}
	if value.RateDate != nil {
		result.RateDate = *value.RateDate
	}
	if value.KRW != nil {
		result.Krw = int64(*value.KRW)
	}
	if value.ProviderPaymentKey != nil {
		result.ProviderPaymentKey = *value.ProviderPaymentKey
	}
	if value.OrderID != nil {
		result.OrderId = *value.OrderID
	}
	if value.Note != nil {
		result.Note = *value.Note
	}
	return result
}

func toProtoPurchase(value billing.Purchase) *postpilotv1.BillingPurchase {
	result := &postpilotv1.BillingPurchase{Id: value.ID, LotId: value.LotID, Credits: int32(value.Credits), UsdCents: int32(value.USDCents), Krw: int64(value.KRW), ProviderPaymentKey: value.ProviderPaymentKey, OrderId: value.OrderID, ChargedAt: instant(value.ChargedAt)}
	if value.RefundedAt != nil {
		result.RefundedAt = instant(*value.RefundedAt)
	}
	return result
}

func termFromProto(value postpilotv1.Term) (billing.Term, bool) {
	switch value {
	case postpilotv1.Term_TERM_MONTHLY:
		return billing.TermMonthly, true
	case postpilotv1.Term_TERM_ANNUAL:
		return billing.TermAnnual, true
	default:
		return "", false
	}
}
func termToProto(value billing.Term) postpilotv1.Term {
	if value == billing.TermMonthly {
		return postpilotv1.Term_TERM_MONTHLY
	}
	if value == billing.TermAnnual {
		return postpilotv1.Term_TERM_ANNUAL
	}
	return postpilotv1.Term_TERM_UNSPECIFIED
}
func instant(value time.Time) string { return value.UTC().Format(time.RFC3339) }
func authRequired() error {
	return rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
}
