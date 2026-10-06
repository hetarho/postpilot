package rpc

import (
	"context"
	"strings"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

// --- 검증 (VOICE-43) ---

func (h *Handler) StartVoiceCheck(ctx context.Context, req *connect.Request[postpilotv1.StartVoiceCheckRequest]) (*connect.Response[postpilotv1.StartVoiceCheckResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	check, jobID, err := h.service.StartVoiceCheck(ctx, userID, req.Msg.GetVoiceId(), req.Msg.GetPromptKey(), fromProtoRef(req.Msg.GetModel()))
	if err != nil {
		return nil, toConnectError("start voice check", err)
	}
	return connect.NewResponse(&postpilotv1.StartVoiceCheckResponse{Check: toProtoCheck(check), JobId: jobID}), nil
}

func (h *Handler) ListVoiceChecks(ctx context.Context, req *connect.Request[postpilotv1.ListVoiceChecksRequest]) (*connect.Response[postpilotv1.ListVoiceChecksResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	checks, activeJobID, err := h.service.ListVoiceChecks(ctx, userID, req.Msg.GetVoiceId())
	if err != nil {
		return nil, toConnectError("list voice checks", err)
	}
	out := make([]*postpilotv1.VoiceCheck, 0, len(checks))
	for _, check := range checks {
		out = append(out, toProtoCheck(check))
	}
	return connect.NewResponse(&postpilotv1.ListVoiceChecksResponse{Checks: out, ActiveJobId: activeJobID}), nil
}

func (h *Handler) RetryVoiceCheck(ctx context.Context, req *connect.Request[postpilotv1.RetryVoiceCheckRequest]) (*connect.Response[postpilotv1.RetryVoiceCheckResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	check, jobID, err := h.service.RetryVoiceCheck(ctx, userID, req.Msg.GetCheckId(), fromProtoRef(req.Msg.GetModel()))
	if err != nil {
		return nil, toConnectError("retry voice check", err)
	}
	return connect.NewResponse(&postpilotv1.RetryVoiceCheckResponse{Check: toProtoCheck(check), JobId: jobID}), nil
}

func fromProtoRef(ref *postpilotv1.ModelRef) llm.ModelRef {
	return llm.ModelRef{ProviderID: ref.GetProviderId(), ModelID: ref.GetModelId()}
}

func toProtoCheck(check voice.CheckView) *postpilotv1.VoiceCheck {
	out := &postpilotv1.VoiceCheck{
		Id: check.ID, Answer: check.Answer, AnswerDeleted: check.AnswerDeleted, Status: toProtoCheckStatus(check.Status),
		Piece: check.Piece, Comparison: ToProtoComparisons(check.Comparison), Stale: check.Stale,
		CreatedAt: check.CreatedAt.UTC().Format(timeLayout),
	}
	if check.Prompt.Key != "" {
		out.Prompt = &postpilotv1.VoicePrompt{Key: check.Prompt.Key, Part: toProtoPart(check.Prompt.Part), Photo: check.Prompt.Photo, Text: check.Prompt.Text, Scene: check.Prompt.Scene, Hint: check.Prompt.Hint, Starter: check.Prompt.Starter}
	}
	if providerID, modelID, ok := strings.Cut(check.WriteModel, "/"); ok {
		out.WriteModel = &postpilotv1.ModelRef{ProviderId: providerID, ModelId: modelID}
	}
	if check.Failure != nil {
		out.Failure = &postpilotv1.Failure{Reason: check.Failure.Reason, Params: check.Failure.Params, TechnicalDetail: check.Failure.TechnicalDetail}
	}
	return out
}

func toProtoCheckStatus(status voice.CheckStatus) postpilotv1.VoiceCheckStatus {
	switch status {
	case voice.CheckQueued:
		return postpilotv1.VoiceCheckStatus_VOICE_CHECK_STATUS_QUEUED
	case voice.CheckRunning:
		return postpilotv1.VoiceCheckStatus_VOICE_CHECK_STATUS_RUNNING
	case voice.CheckDone:
		return postpilotv1.VoiceCheckStatus_VOICE_CHECK_STATUS_DONE
	case voice.CheckFailed:
		return postpilotv1.VoiceCheckStatus_VOICE_CHECK_STATUS_FAILED
	}
	return postpilotv1.VoiceCheckStatus_VOICE_CHECK_STATUS_UNSPECIFIED
}
