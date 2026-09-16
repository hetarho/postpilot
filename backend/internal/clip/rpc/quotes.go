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
	var admission *clip.ModelAdmissionError
	if errors.Is(err, llm.ErrUnsupported) && !errors.As(err, &admission) {
		return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition, "video input is required", "MODEL_VIDEO_UNSUPPORTED", map[string]string{"model": observe.String()})
	}
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.QuoteClipGenerationResponse{ReusedChunks: int32(q.Pricing.ReusedChunks), RemainingChunks: int32(q.Pricing.ObservationCalls), RenderOnly: q.Pricing.RenderOnly(), ResponseRetries: int32(q.Pricing.Plan.ResponseRetries), CancellationPolicy: &v1.ClipCancellationPolicy{Version: int32(q.Pricing.CancellationPolicyVersion), UnusedReservationNumerator: 1, UnusedReservationDenominator: 2, Rounding: "ceil"}, QuoteId: q.ID, MaxCredits: int32(q.Pricing.MaxCredits), ExpiresAt: q.ExpiresAt.UTC().Format(time.RFC3339Nano), PricedCalls: []*v1.ClipPricedCall{pricedCallProto(q.Pricing.Observe, "observe", q.Pricing.ObserveCalls(q.Pricing.ObservationCalls)), pricedCallProto(q.Pricing.Plan, "flow", q.Pricing.FlowCalls()), pricedCallProto(q.Pricing.Narration, "narration", q.Pricing.NarrationCalls())}}), nil
}

// QuoteClipRevision prices ONE written revision of the plan the owner is
// looking at: one writing call for a narration target, two where the footage is
// rewritten and the narration follows it (CLIP-131).
func (h *Handler) QuoteClipRevision(ctx context.Context, req *connect.Request[v1.QuoteClipRevisionRequest]) (*connect.Response[v1.QuoteClipRevisionResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	observe := llm.ModelRef{ProviderID: req.Msg.GetObserveModel().GetProviderId(), ModelID: req.Msg.GetObserveModel().GetModelId()}
	write := llm.ModelRef{ProviderID: req.Msg.GetWriteModel().GetProviderId(), ModelID: req.Msg.GetWriteModel().GetModelId()}
	q, err := h.generation.QuoteRevision(ctx, user, req.Msg.GetProjectId(), req.Msg.GetRequest(), req.Msg.GetTarget(), observe.String(), write.String())
	if err != nil {
		return nil, toConnectError(err)
	}
	project, err := h.service.GetProject(ctx, user, req.Msg.GetProjectId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.QuoteClipRevisionResponse{
		QuoteId: q.ID, MaxCredits: int32(q.Pricing.MaxCredits), ExpiresAt: q.ExpiresAt.UTC().Format(time.RFC3339Nano),
		ResponseRetries: int32(q.Pricing.Plan.ResponseRetries), PlanRevision: int32(project.EditPlanRevision),
		CancellationPolicy: &v1.ClipCancellationPolicy{Version: int32(q.Pricing.CancellationPolicyVersion), UnusedReservationNumerator: 1, UnusedReservationDenominator: 2, Rounding: "ceil"},
		PricedCalls:        []*v1.ClipPricedCall{pricedCallProto(q.Pricing.Plan, "flow", q.Pricing.FlowCalls()), pricedCallProto(q.Pricing.Narration, "narration", q.Pricing.NarrationCalls())},
	}), nil
}

// StartClipRevision accepts one approved revision. The actor comes from the
// session interceptor, never from the payload.
func (h *Handler) StartClipRevision(ctx context.Context, req *connect.Request[v1.StartClipRevisionRequest]) (*connect.Response[v1.StartClipRevisionResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	observe := llm.ModelRef{ProviderID: req.Msg.GetObserveModel().GetProviderId(), ModelID: req.Msg.GetObserveModel().GetModelId()}
	write := llm.ModelRef{ProviderID: req.Msg.GetWriteModel().GetProviderId(), ModelID: req.Msg.GetWriteModel().GetModelId()}
	approval := clip.QuoteApproval{QuoteID: req.Msg.GetQuoteId(), CancellationPolicyVersion: int(req.Msg.GetCancellationPolicyVersion())}
	if req.Msg.ApprovedMaxCredits != nil {
		credits := int(req.Msg.GetApprovedMaxCredits())
		approval.MaxCredits = &credits
	}
	id, err := h.generation.StartRevision(ctx, user, req.Msg.GetProjectId(), req.Msg.GetRequest(), req.Msg.GetTarget(), observe.String(), write.String(), approval)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.StartClipRevisionResponse{JobId: id}), nil
}

func pricedCallProto(p llm.CallPolicy, label string, count int) *v1.ClipPricedCall {
	return &v1.ClipPricedCall{Label: label, Model: &v1.ModelRef{ProviderId: p.Ref.ProviderID, ModelId: p.Ref.ModelID}, Stage: p.Stage, Calls: int32(count), PromptTokens: int32(p.InputTokenLimit()), CompletionTokens: int32(p.CompletionTokens), Reasoning: string(p.Reasoning), InputUsdPerMillion: p.InputUSDPerMillion, OutputUsdPerMillion: p.OutputUSDPerMillion}
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
	return &v1.ClipAccounting{CancellationPolicyVersion: int32(a.CancellationPolicyVersion), SettlementReason: a.SettlementReason, NominalReservedCredits: optionalInt32(a.NominalReservation), ConfirmedChargeCredits: optionalInt32(a.ConfirmedCharge), CancellationFeeCredits: optionalInt32(a.CancellationFee), ShadowConfirmedChargeCredits: optionalInt32(a.ShadowConfirmedCharge), ShadowCancellationFeeCredits: optionalInt32(a.ShadowCancellationFee), JobId: a.JobID, Status: a.Status, ApprovedMaxCredits: optionalInt32(a.ApprovedMax), ReservedCredits: optionalInt32(a.Reserved), FinalChargeCredits: optionalInt32(a.FinalCharge), RefundCredits: optionalInt32(a.Refund), ShadowChargeCredits: optionalInt32(a.ShadowCharge), Settled: a.Settled}
}
