package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/billing"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func (h *Handler) RequestRefund(ctx context.Context, req *connect.Request[postpilotv1.RequestRefundRequest]) (*connect.Response[postpilotv1.RequestRefundResponse], error) {
	owner, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, authRequired()
	}
	item, err := h.service.RequestRefund(ctx, owner, req.Msg.GetOrderId(), req.Msg.GetReason())
	if err != nil {
		return nil, refundError(err)
	}
	return connect.NewResponse(&postpilotv1.RequestRefundResponse{Refund: toProtoRefund(item)}), nil
}

func (h *Handler) ListMyRefunds(ctx context.Context, _ *connect.Request[postpilotv1.ListMyRefundsRequest]) (*connect.Response[postpilotv1.ListMyRefundsResponse], error) {
	owner, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, authRequired()
	}
	items, err := h.service.RefundRequests(ctx, owner)
	if err != nil {
		return nil, refundError(err)
	}
	response := &postpilotv1.ListMyRefundsResponse{}
	for _, item := range items {
		response.Refunds = append(response.Refunds, toProtoRefund(item))
	}
	return connect.NewResponse(response), nil
}

// The authentication interceptor restricts these procedures to master. No
// request carries an owner ID; the service loads each request from storage.
func (h *Handler) ListRefundReviews(ctx context.Context, _ *connect.Request[postpilotv1.ListRefundReviewsRequest]) (*connect.Response[postpilotv1.ListRefundReviewsResponse], error) {
	if _, ok := auth.UserFromContext(ctx); !ok {
		return nil, authRequired()
	}
	items, err := h.service.RefundRequests(ctx, "")
	if err != nil {
		return nil, refundError(err)
	}
	response := &postpilotv1.ListRefundReviewsResponse{}
	for _, item := range items {
		response.Refunds = append(response.Refunds, toProtoRefund(item))
	}
	return connect.NewResponse(response), nil
}

func (h *Handler) ReviewRefund(ctx context.Context, req *connect.Request[postpilotv1.ReviewRefundRequest]) (*connect.Response[postpilotv1.ReviewRefundResponse], error) {
	reviewer, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, authRequired()
	}
	item, err := h.service.ReviewRefund(ctx, reviewer, req.Msg.GetRequestId(), req.Msg.GetOutcome(), int(req.Msg.GetReviewedAmountKrw()))
	if err != nil {
		return nil, refundError(err)
	}
	return connect.NewResponse(&postpilotv1.ReviewRefundResponse{Refund: toProtoRefund(item)}), nil
}

func (h *Handler) ReconcileRefund(ctx context.Context, req *connect.Request[postpilotv1.ReconcileRefundRequest]) (*connect.Response[postpilotv1.ReconcileRefundResponse], error) {
	if _, ok := auth.UserFromContext(ctx); !ok {
		return nil, authRequired()
	}
	if err := h.service.ReconcileRefund(ctx, req.Msg.GetRequestId()); err != nil {
		return nil, refundError(err)
	}
	item, err := h.service.RefundRequest(ctx, req.Msg.GetRequestId())
	if err != nil {
		return nil, refundError(err)
	}
	return connect.NewResponse(&postpilotv1.ReconcileRefundResponse{Refund: toProtoRefund(item)}), nil
}

func toProtoRefund(item billing.RefundRequest) *postpilotv1.BillingRefundRequest {
	result := &postpilotv1.BillingRefundRequest{
		Id: item.ID, UserId: item.UserID, Reason: item.Reason, Status: item.Status,
		RequestedAt: instant(item.RequestedAt), ReviewedBy: item.ReviewedBy,
		ReviewedAmountKrw: int64(item.ReviewedAmountKRW), ConfirmedAmountKrw: int64(item.ConfirmedAmountKRW),
		PriorRefundedKrw: int64(item.PriorRefundedKRW),
		DispositionJson:  item.DispositionJSON,
		Payment: &postpilotv1.RefundPayment{OrderId: item.OrderID, Kind: item.Payment.Kind,
			ProviderPaymentKey: item.Payment.PaymentKey, ChargedKrw: int64(item.Payment.KRW),
			ChargedAt: instant(item.Payment.ChargedAt), CoverageId: item.Payment.CoverageID,
			PackLotId: item.Payment.PackLotID},
		Evidence: &postpilotv1.RefundEvidence{PaidModelJobs: int32(item.Evidence.PaidModelJobs),
			CreditsUsed: int32(item.Evidence.CreditsUsed), CreditsReserved: int32(item.Evidence.CreditsReserved),
			ServerExportsUsed:      int32(item.Evidence.ServerExportsUsed),
			ServerExportsReserved:  int32(item.Evidence.ServerExportsReserved),
			FundedCreditsRemaining: int32(item.Evidence.FundedCreditsRemaining),
			FundedExportsRemaining: int32(item.Evidence.FundedExportsRemaining)},
	}
	if item.ReviewedAt != nil {
		result.ReviewedAt = instant(*item.ReviewedAt)
	}
	if item.ConfirmedAt != nil {
		result.ConfirmedAt = instant(*item.ConfirmedAt)
	}
	return result
}

// refundError names the refusal the reviewing operator acts on, the provider's (REFUND_FAILED),
// before anything joined to it, so a store failure that followed still reads as that refusal.
// The rest stay UNKNOWN_FAILURE behind their Connect codes.
func refundError(err error) error {
	switch {
	case errors.Is(err, billing.ErrRefundFailed):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "provider refused the refund", postpilotv1.FailureReason_REFUND_FAILED, nil)
	case errors.Is(err, billing.ErrRefundNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "refund payment or request not found", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	case errors.Is(err, billing.ErrInvalidRefundRequest), errors.Is(err, billing.ErrRefundAmount):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid refund request or amount", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	case errors.Is(err, billing.ErrRefundConflict), errors.Is(err, billing.ErrRefundActiveUse), errors.Is(err, billing.ErrRefundDependentPayment):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "refund state or active use prevents review", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	case errors.Is(err, billing.ErrRefundProviderPending):
		return rpcserver.NewAppError(connect.CodeUnavailable, "provider outcome is still pending", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	case errors.Is(err, billing.ErrUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "refund review unavailable", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	default:
		slog.Error("refund request failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "could not process refund", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
}
