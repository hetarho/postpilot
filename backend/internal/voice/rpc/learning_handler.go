package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

type LearningHandler struct{ service *voice.Service }

func NewLearningHandler(service *voice.Service) *LearningHandler {
	return &LearningHandler{service: service}
}

func (h *LearningHandler) LearnFromFinalizedPost(ctx context.Context, req *connect.Request[postpilotv1.LearnFromFinalizedPostRequest]) (*connect.Response[postpilotv1.LearnFromFinalizedPostResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	event, jobID, reused, err := h.service.LearnFromFinalizedPost(ctx, userID, req.Msg.GetPostSlug(), learningModel(req.Msg.GetAnalyzeModel()))
	if err != nil {
		return nil, learningError("learn from finalized post", err)
	}
	return connect.NewResponse(&postpilotv1.LearnFromFinalizedPostResponse{Event: toProtoLearningEvent(event), JobId: jobID, Reused: reused}), nil
}
func (h *LearningHandler) RetryVoiceLearning(ctx context.Context, req *connect.Request[postpilotv1.RetryVoiceLearningRequest]) (*connect.Response[postpilotv1.RetryVoiceLearningResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	event, jobID, err := h.service.RetryLearning(ctx, userID, req.Msg.GetEventId(), learningModel(req.Msg.GetAnalyzeModel()))
	if err != nil {
		return nil, learningError("retry voice learning", err)
	}
	return connect.NewResponse(&postpilotv1.RetryVoiceLearningResponse{Event: toProtoLearningEvent(event), JobId: jobID}), nil
}
func (h *LearningHandler) GetVoiceLearningEvent(ctx context.Context, req *connect.Request[postpilotv1.GetVoiceLearningEventRequest]) (*connect.Response[postpilotv1.GetVoiceLearningEventResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	event, err := h.service.GetLearningEvent(ctx, userID, req.Msg.GetEventId())
	if err != nil {
		return nil, learningError("get voice learning", err)
	}
	return connect.NewResponse(&postpilotv1.GetVoiceLearningEventResponse{Event: toProtoLearningEvent(event)}), nil
}
func (h *LearningHandler) GiveSentenceFeedback(ctx context.Context, req *connect.Request[postpilotv1.GiveSentenceFeedbackRequest]) (*connect.Response[postpilotv1.GiveSentenceFeedbackResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	reason := ""
	switch req.Msg.GetReason() {
	case postpilotv1.VoiceFeedbackReason_VOICE_FEEDBACK_REASON_VOCABULARY:
		reason = "vocabulary"
	case postpilotv1.VoiceFeedbackReason_VOICE_FEEDBACK_REASON_ENDING:
		reason = "ending"
	case postpilotv1.VoiceFeedbackReason_VOICE_FEEDBACK_REASON_LENGTH:
		reason = "length"
	case postpilotv1.VoiceFeedbackReason_VOICE_FEEDBACK_REASON_STRUCTURE:
		reason = "structure"
	}
	id, err := h.service.GiveFeedback(ctx, userID, req.Msg.GetPostSlug(), req.Msg.GetSentenceRef(), reason, req.Msg.GetAuthoredText(), req.Msg.GetSatisfaction())
	if err != nil {
		// Ownership and voice refusals keep their codes; every other failure is a rule of the
		// feedback itself, which the client shows verbatim.
		switch {
		case errors.Is(err, voice.ErrPostNotFound), errors.Is(err, voice.ErrForbidden), errors.Is(err, voice.ErrVoiceNotFound),
			errors.Is(err, voice.ErrVoiceDeleted), errors.Is(err, voice.ErrBaselineVoiceMismatch):
			return nil, learningError("give sentence feedback", err)
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	return connect.NewResponse(&postpilotv1.GiveSentenceFeedbackResponse{FeedbackId: id}), nil
}
func (h *LearningHandler) SetVoiceRuleStatus(ctx context.Context, req *connect.Request[postpilotv1.SetVoiceRuleStatusRequest]) (*connect.Response[postpilotv1.SetVoiceRuleStatusResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	status := voice.RuleStatus("")
	switch req.Msg.GetStatus() {
	case postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_CANDIDATE:
		status = voice.RuleCandidate
	case postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_ACTIVE:
		status = voice.RuleActive
	case postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_RETIRED:
		status = voice.RuleRetired
	case postpilotv1.VoiceRuleStatus_VOICE_RULE_STATUS_REJECTED:
		status = voice.RuleRejected
	}
	profile, err := h.service.ChangeRuleStatus(ctx, userID, req.Msg.GetRuleId(), status)
	if err != nil {
		return nil, learningError("set voice rule status", err)
	}
	return connect.NewResponse(&postpilotv1.SetVoiceRuleStatusResponse{Profile: toProtoProfile(profile)}), nil
}
func (h *LearningHandler) ListRuleConfirmations(ctx context.Context, req *connect.Request[postpilotv1.ListRuleConfirmationsRequest]) (*connect.Response[postpilotv1.ListRuleConfirmationsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.service.Confirmations(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, learningError("list rule confirmations", err)
	}
	out := make([]*postpilotv1.VoiceRuleConfirmation, 0, len(items))
	for _, item := range items {
		out = append(out, &postpilotv1.VoiceRuleConfirmation{Id: item.ID, RuleId: item.RuleID, ExistingStatement: item.ExistingStatement, ProposedStatement: item.ProposedStatement, Status: item.Status, CreatedAt: item.CreatedAt.UTC().Format(timeLayout)})
	}
	return connect.NewResponse(&postpilotv1.ListRuleConfirmationsResponse{Confirmations: out}), nil
}
func (h *LearningHandler) ResolveRuleConfirmation(ctx context.Context, req *connect.Request[postpilotv1.ResolveRuleConfirmationRequest]) (*connect.Response[postpilotv1.ResolveRuleConfirmationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.service.ResolveConfirmation(ctx, userID, req.Msg.GetConfirmationId(), req.Msg.GetReplace())
	if err != nil {
		return nil, learningError("resolve rule confirmation", err)
	}
	return connect.NewResponse(&postpilotv1.ResolveRuleConfirmationResponse{Profile: toProtoProfile(profile)}), nil
}

func learningModel(ref *postpilotv1.ModelRef) llm.ModelRef {
	if ref == nil {
		return llm.ModelRef{}
	}
	return llm.ModelRef{ProviderID: ref.GetProviderId(), ModelID: ref.GetModelId()}
}
func toProtoLearningEvent(e voice.LearningEvent) *postpilotv1.VoiceLearningEvent {
	processed := ""
	if e.ProcessedAt != nil {
		processed = e.ProcessedAt.UTC().Format(timeLayout)
	}
	return &postpilotv1.VoiceLearningEvent{Id: e.ID, PostSlug: e.PostSlug, BaselineRevision: e.BaselineRevision, Status: e.Status, JobId: e.JobID, Error: e.Error, CreatedAt: e.CreatedAt.UTC().Format(timeLayout), ProcessedAt: processed, VoiceId: e.VoiceID}
}
func learningError(op string, err error) error {
	switch {
	case errors.Is(err, voice.ErrLearningNotFound), errors.Is(err, voice.ErrRuleNotFound), errors.Is(err, voice.ErrConfirmationNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, voice.ErrPostNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, voice.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, voice.ErrAnalyzeModelRequired):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, voice.ErrInvalidLifecycle):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return toConnectError(op, err)
	}
}

var _ postpilotv1connect.VoiceLearningServiceHandler = (*LearningHandler)(nil)
