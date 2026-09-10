package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func (h *Handler) QuoteClipGeneration(ctx context.Context, req *connect.Request[v1.QuoteClipGenerationRequest]) (*connect.Response[v1.QuoteClipGenerationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	observe := llm.ModelRef{ProviderID: req.Msg.GetObserveModel().GetProviderId(), ModelID: req.Msg.GetObserveModel().GetModelId()}
	write := llm.ModelRef{ProviderID: req.Msg.GetWriteModel().GetProviderId(), ModelID: req.Msg.GetWriteModel().GetModelId()}
	q, err := h.generation.Quote(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, observe.String(), write.String())
	if errors.Is(err, llm.ErrUnsupported) {
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition, "video input is required", "MODEL_VIDEO_UNSUPPORTED", map[string]string{"model": observe.String()})
	}
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.QuoteClipGenerationResponse{QuoteId: q.ID, MaxCredits: int32(q.Pricing.MaxCredits), ExpiresAt: q.ExpiresAt.UTC().Format(time.RFC3339Nano), PricedCalls: []*v1.ClipPricedCall{pricedCallProto(q.Pricing.Observe, q.Pricing.ObservationCalls), pricedCallProto(q.Pricing.Plan, 1)}}), nil
}

func pricedCallProto(p llm.CallPolicy, count int) *v1.ClipPricedCall {
	return &v1.ClipPricedCall{Model: &v1.ModelRef{ProviderId: p.Ref.ProviderID, ModelId: p.Ref.ModelID}, Stage: p.Stage, Calls: int32(count), PromptTokens: 30000, CompletionTokens: int32(p.CompletionTokens), Reasoning: string(p.Reasoning), InputUsdPerMillion: p.InputUSDPerMillion, OutputUsdPerMillion: p.OutputUSDPerMillion}
}

func optionalInt32(n *int) *int32 {
	if n == nil {
		return nil
	}
	value := int32(*n)
	return &value
}
func accountingProto(a *clip.Accounting) *v1.ClipAccounting {
	if a == nil {
		return nil
	}
	return &v1.ClipAccounting{JobId: a.JobID, Status: a.Status, ApprovedMaxCredits: optionalInt32(a.ApprovedMax), ReservedCredits: optionalInt32(a.Reserved), FinalChargeCredits: optionalInt32(a.FinalCharge), RefundCredits: optionalInt32(a.Refund), ShadowChargeCredits: optionalInt32(a.ShadowCharge), Settled: a.Settled}
}
