package rpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	authactor "github.com/postpilot/backend/internal/auth/rpc/actor"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func uploadTextProto(text clip.UploadText) *v1.ClipUploadText {
	return &v1.ClipUploadText{Body: text.Body, Tags: text.Tags, Edited: text.Edited, UpdatedAt: text.UpdatedAt.UTC().Format(time.RFC3339Nano), Revision: int32(text.Revision)}
}
func uploadTextError(err error) error {
	if errors.Is(err, clip.ErrUploadTextConfirmation) {
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "edited upload text needs confirmation", v1.FailureReason_CLIP_UPLOAD_TEXT_CONFIRMATION_REQUIRED, nil)
	}
	if errors.Is(err, clip.ErrUploadTextLimit) {
		limits := clip.UploadLimits()
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "upload text exceeds limits", v1.FailureReason_CLIP_UPLOAD_TEXT_LIMIT, map[string]string{"max": strconv.Itoa(limits.MaxChars), "tags": strconv.Itoa(limits.MaxTags)})
	}
	return toConnectError(err)
}
func (h *Handler) GetClipUploadTexts(ctx context.Context, req *connect.Request[v1.GetClipUploadTextsRequest]) (*connect.Response[v1.GetClipUploadTextsResponse], error) {
	user, err := authactor.ActingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	rows, err := h.generation.UploadTexts(ctx, user, req.Msg.ProjectId)
	if err != nil {
		return nil, toConnectError(err)
	}
	limits := clip.UploadLimits()
	out := &v1.GetClipUploadTextsResponse{Limits: &v1.ClipUploadTextLimits{MaxChars: int32(limits.MaxChars), MaxTags: int32(limits.MaxTags)}}
	if len(rows) > 0 {
		out.Text = uploadTextProto(rows[0])
	}
	return connect.NewResponse(out), nil
}
func (h *Handler) SaveClipUploadText(ctx context.Context, req *connect.Request[v1.SaveClipUploadTextRequest]) (*connect.Response[v1.SaveClipUploadTextResponse], error) {
	user, err := authactor.ActingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	text, err := h.generation.SaveUploadText(ctx, user, req.Msg.ProjectId, req.Msg.Body, req.Msg.Tags, int(req.Msg.ExpectedRevision))
	if err != nil {
		return nil, uploadTextError(err)
	}
	return connect.NewResponse(&v1.SaveClipUploadTextResponse{Text: uploadTextProto(text)}), nil
}
func (h *Handler) QuoteClipUploadText(ctx context.Context, req *connect.Request[v1.QuoteClipUploadTextRequest]) (*connect.Response[v1.QuoteClipGenerationResponse], error) {
	user, err := authactor.ActingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	write := llm.ModelRef{ProviderID: req.Msg.GetWriteModel().GetProviderId(), ModelID: req.Msg.GetWriteModel().GetModelId()}
	quote, err := h.generation.QuoteUploadText(ctx, user, req.Msg.ProjectId, write.String(), req.Msg.ConfirmEdited)
	if err != nil {
		return nil, uploadTextError(err)
	}
	return connect.NewResponse(&v1.QuoteClipGenerationResponse{QuoteId: quote.ID, MaxCredits: int32(quote.Pricing.MaxCredits), ExpiresAt: quote.ExpiresAt.UTC().Format(time.RFC3339Nano), CancellationPolicy: cancellationPolicyProto(quote.Pricing), PricedCalls: []*v1.ClipPricedCall{pricedCallProto(quote.Pricing.Plan, "upload_text", 1)}}), nil
}
func (h *Handler) StartClipUploadText(ctx context.Context, req *connect.Request[v1.StartClipUploadTextRequest]) (*connect.Response[v1.StartClipGenerationResponse], error) {
	user, err := authactor.ActingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	write := llm.ModelRef{ProviderID: req.Msg.GetWriteModel().GetProviderId(), ModelID: req.Msg.GetWriteModel().GetModelId()}
	id, err := h.generation.StartUploadText(ctx, user, req.Msg.ProjectId, write.String(), req.Msg.ConfirmEdited, approval(req.Msg.CancellationPolicyVersion, req.Msg.QuoteId, req.Msg.ApprovedMaxCredits))
	if err != nil {
		return nil, uploadTextError(err)
	}
	return connect.NewResponse(&v1.StartClipGenerationResponse{JobId: id}), nil
}
