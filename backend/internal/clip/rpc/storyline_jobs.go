package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func modelRefs(observe, write interface {
	GetProviderId() string
	GetModelId() string
}) (string, string) {
	return llm.ModelRef{ProviderID: observe.GetProviderId(), ModelID: observe.GetModelId()}.String(),
		llm.ModelRef{ProviderID: write.GetProviderId(), ModelID: write.GetModelId()}.String()
}

func approval(policy int32, quote string, max *int32) clip.QuoteApproval {
	var credits *int
	if max != nil {
		value := int(*max)
		credits = &value
	}
	return clip.QuoteApproval{CancellationPolicyVersion: int(policy), QuoteID: quote, MaxCredits: credits}
}

// generationError is a quote's or a start's refusal: a model that cannot watch video is named
// as such, everything else maps as the clip context's errors do.
func generationError(err error, observe string) error {
	var admission *clip.ModelAdmissionError
	if errors.Is(err, llm.ErrUnsupported) && !errors.As(err, &admission) {
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "video input is required", v1.FailureReason_MODEL_VIDEO_UNSUPPORTED, map[string]string{"model": observe})
	}
	return toConnectError(err)
}

// QuoteClipStoryline prices 스토리라인 먼저 and 다시 만들기 (CLIP-177).
func (h *Handler) QuoteClipStoryline(ctx context.Context, req *connect.Request[v1.QuoteClipStorylineRequest]) (*connect.Response[v1.QuoteClipGenerationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	observe, write := modelRefs(req.Msg.GetObserveModel(), req.Msg.GetWriteModel())
	q, err := h.generation.QuoteStoryline(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, observe, write)
	if err != nil {
		return nil, generationError(err, observe)
	}
	return connect.NewResponse(generationQuoteProto(ctx, h, user, req.Msg.ProjectId, q)), nil
}

// StartClipStoryline accepts an approved storyline call.
func (h *Handler) StartClipStoryline(ctx context.Context, req *connect.Request[v1.StartClipStorylineRequest]) (*connect.Response[v1.StartClipGenerationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(errors.New("clip generation unavailable"))
	}
	observe, write := modelRefs(req.Msg.GetObserveModel(), req.Msg.GetWriteModel())
	approved := approval(req.Msg.CancellationPolicyVersion, req.Msg.QuoteId, req.Msg.ApprovedMaxCredits)
	approved.AnalysisPreparationID = req.Msg.AnalysisPreparationId
	id, err := h.generation.StartStoryline(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, observe, write, approved)
	if err != nil {
		return nil, generationError(err, observe)
	}
	return connect.NewResponse(&v1.StartClipGenerationResponse{JobId: id}), nil
}

// QuoteClipStorylineRevision prices one storyline request (CLIP-181).
func (h *Handler) QuoteClipStorylineRevision(ctx context.Context, req *connect.Request[v1.QuoteClipStorylineRevisionRequest]) (*connect.Response[v1.QuoteClipGenerationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	observe, write := modelRefs(req.Msg.GetObserveModel(), req.Msg.GetWriteModel())
	q, err := h.generation.QuoteStorylineRevision(ctx, user, req.Msg.ProjectId, req.Msg.Request, observe, write)
	if err != nil {
		return nil, generationError(err, observe)
	}
	return connect.NewResponse(generationQuoteProto(ctx, h, user, req.Msg.ProjectId, q)), nil
}

// StartClipStorylineRevision accepts an approved storyline request.
func (h *Handler) StartClipStorylineRevision(ctx context.Context, req *connect.Request[v1.StartClipStorylineRevisionRequest]) (*connect.Response[v1.StartClipGenerationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(errors.New("clip generation unavailable"))
	}
	observe, write := modelRefs(req.Msg.GetObserveModel(), req.Msg.GetWriteModel())
	id, err := h.generation.StartStorylineRevision(ctx, user, req.Msg.ProjectId, req.Msg.Request, observe, write, approval(req.Msg.CancellationPolicyVersion, req.Msg.QuoteId, req.Msg.ApprovedMaxCredits))
	if err != nil {
		return nil, generationError(err, observe)
	}
	return connect.NewResponse(&v1.StartClipGenerationResponse{JobId: id}), nil
}

// storylineEdit is the owner's edit as the patch carries it; nil leaves the storyline alone.
func storylineEdit(m *v1.ClipStorylineEdit) *[]clip.StorylineParagraph {
	if m == nil {
		return nil
	}
	out := make([]clip.StorylineParagraph, 0, len(m.GetParagraphs()))
	for _, p := range m.GetParagraphs() {
		out = append(out, clip.StorylineParagraph{Text: p.GetText(), ObservationIDs: p.GetObservationIds()})
	}
	return &out
}
