// Package rpc is the Connect surface of the voucher context.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/voucher"
)

// Handler serves VoucherService. The interceptor decides who may call what: GetVoucher is
// public, RedeemVoucher needs a session, and the operator procedures are master-only.
type Handler struct{ service *voucher.Service }

func NewHandler(service *voucher.Service) *Handler { return &Handler{service: service} }

func (h *Handler) GetVoucher(ctx context.Context, req *connect.Request[postpilotv1.GetVoucherRequest]) (*connect.Response[postpilotv1.GetVoucherResponse], error) {
	view, err := h.service.View(ctx, req.Msg.GetToken())
	if err != nil {
		return nil, voucherError("read", "", err)
	}
	return connect.NewResponse(&postpilotv1.GetVoucherResponse{
		Credits: int32(view.Credits), ValidityDays: int32(view.ValidityDays), Message: view.Message,
		State: toProtoState(view.State), LinkExpiresAt: instant(view.LinkExpiresAt),
	}), nil
}

func (h *Handler) RedeemVoucher(ctx context.Context, req *connect.Request[postpilotv1.RedeemVoucherRequest]) (*connect.Response[postpilotv1.RedeemVoucherResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, authRequired()
	}
	redemption, err := h.service.Redeem(ctx, userID, req.Msg.GetToken())
	if err != nil {
		return nil, voucherError("redeem", userID, err)
	}
	return connect.NewResponse(&postpilotv1.RedeemVoucherResponse{
		Credits: int32(redemption.Credits), CreditsExpireAt: instant(redemption.CreditsExpireAt),
	}), nil
}

func (h *Handler) IssueVoucher(ctx context.Context, req *connect.Request[postpilotv1.IssueVoucherRequest]) (*connect.Response[postpilotv1.IssueVoucherResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, authRequired()
	}
	request := voucher.Issue{
		Credits: int(req.Msg.GetCredits()), ValidityDays: int(req.Msg.GetValidityDays()),
		Message: req.Msg.GetMessage(),
	}
	if sale := req.Msg.GetSale(); sale != nil {
		request.Sale = &voucher.Sale{AmountKRW: sale.GetAmountKrw(), Payer: sale.GetPayerName()}
	}
	issued, err := h.service.Issue(ctx, userID, request)
	if err != nil {
		return nil, voucherError("issue", userID, err)
	}
	return connect.NewResponse(&postpilotv1.IssueVoucherResponse{
		Voucher: toProtoVoucher(voucher.Listed{Voucher: issued, State: voucher.StateRedeemable}),
	}), nil
}

func (h *Handler) ListVouchers(ctx context.Context, _ *connect.Request[postpilotv1.ListVouchersRequest]) (*connect.Response[postpilotv1.ListVouchersResponse], error) {
	userID, _ := auth.UserFromContext(ctx)
	listed, err := h.service.List(ctx)
	if err != nil {
		return nil, voucherError("list", userID, err)
	}
	response := &postpilotv1.ListVouchersResponse{}
	for _, item := range listed {
		response.Vouchers = append(response.Vouchers, toProtoVoucher(item))
	}
	for _, preset := range voucher.Presets() {
		response.Presets = append(response.Presets, &postpilotv1.VoucherPreset{
			Plan: planrpc.ToProto(preset.Plan), Credits: int32(preset.Credits), ValidityDays: int32(preset.Days),
		})
	}
	return connect.NewResponse(response), nil
}

func (h *Handler) RevokeVoucher(ctx context.Context, req *connect.Request[postpilotv1.RevokeVoucherRequest]) (*connect.Response[postpilotv1.RevokeVoucherResponse], error) {
	userID, _ := auth.UserFromContext(ctx)
	revoked, err := h.service.Revoke(ctx, req.Msg.GetId())
	if err != nil {
		return nil, voucherError("revoke", userID, err)
	}
	return connect.NewResponse(&postpilotv1.RevokeVoucherResponse{Voucher: toProtoVoucher(revoked)}), nil
}

// toProtoVoucher carries the token only while the link can still be redeemed: a spent,
// lapsed or revoked link has nothing left to copy.
func toProtoVoucher(item voucher.Listed) *postpilotv1.Voucher {
	out := &postpilotv1.Voucher{
		Id: item.ID, Credits: int32(item.Credits), ValidityDays: int32(item.ValidityDays),
		Message: item.Message, State: toProtoState(item.State),
		IssuedAt: instant(item.IssuedAt), LinkExpiresAt: instant(item.LinkExpiresAt),
		RedeemedBy: item.RedeemedBy, RemainingCredits: int32(item.RemainingCredits),
		CreditsExpireAt: instant(item.CreditsExpireAt),
	}
	if item.Sale != nil {
		out.Sale = &postpilotv1.VoucherSale{AmountKrw: item.Sale.AmountKRW, PayerName: item.Sale.Payer}
	}
	if item.RedeemedAt != nil {
		out.RedeemedAt = instant(*item.RedeemedAt)
	}
	if item.State == voucher.StateRedeemable {
		out.Token = item.Token
	}
	return out
}

func toProtoState(state voucher.State) postpilotv1.VoucherState {
	switch state {
	case voucher.StateRedeemable:
		return postpilotv1.VoucherState_VOUCHER_STATE_REDEEMABLE
	case voucher.StateRedeemed:
		return postpilotv1.VoucherState_VOUCHER_STATE_REDEEMED
	case voucher.StateExpired:
		return postpilotv1.VoucherState_VOUCHER_STATE_EXPIRED
	case voucher.StateRevoked:
		return postpilotv1.VoucherState_VOUCHER_STATE_REVOKED
	default:
		return postpilotv1.VoucherState_VOUCHER_STATE_UNSPECIFIED
	}
}

// voucherError maps the domain's refusals and logs anything else. Neither path names the
// token: it is the gift link's only secret.
func voucherError(action, userID string, err error) error {
	switch {
	case errors.Is(err, voucher.ErrInvalid):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "voucher issue is invalid", postpilotv1.FailureReason_VOUCHER_INVALID, nil)
	case errors.Is(err, voucher.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "voucher not found", postpilotv1.FailureReason_VOUCHER_NOT_FOUND, nil)
	case errors.Is(err, voucher.ErrRedeemed):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voucher already redeemed", postpilotv1.FailureReason_VOUCHER_REDEEMED, nil)
	case errors.Is(err, voucher.ErrExpired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voucher link expired", postpilotv1.FailureReason_VOUCHER_EXPIRED, nil)
	case errors.Is(err, voucher.ErrRevoked):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "voucher revoked", postpilotv1.FailureReason_VOUCHER_REVOKED, nil)
	default:
		slog.Error("voucher "+action+" failed", "user_id", userID, "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "could not "+action+" voucher", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
}

func authRequired() error {
	return rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
}

// instant formats a present instant as RFC3339 and an absent one as the empty string.
func instant(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
