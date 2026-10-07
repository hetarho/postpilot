package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/voice"
)

type CandidateHandler struct{ service *voice.CandidateService }

func NewCandidateHandler(service *voice.CandidateService) *CandidateHandler {
	return &CandidateHandler{service: service}
}

func (h *CandidateHandler) EstimateWritingVoiceCandidates(ctx context.Context, req *connect.Request[postpilotv1.EstimateWritingVoiceCandidatesRequest]) (*connect.Response[postpilotv1.EstimateWritingVoiceCandidatesResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	count, countErr := voice.NormalizeCandidateCount(int(req.Msg.GetCandidateCount()))
	if countErr != nil {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid candidate count", postpilotv1.FailureReason_AUTHORING_CANDIDATE_COUNT_INVALID, nil)
	}
	estimate, err := h.service.Estimate(ctx, fromProtoRef(req.Msg.GetWriteModel()), count)
	if err != nil {
		return nil, candidateConnectError("estimate writing styles", err)
	}
	out := &postpilotv1.EstimateWritingVoiceCandidatesResponse{Free: estimate.Free}
	if estimate.Available {
		credits := int32(estimate.Credits)
		out.Credits = &credits
	}
	return connect.NewResponse(out), nil
}
func (h *CandidateHandler) StartWritingVoiceCandidates(ctx context.Context, req *connect.Request[postpilotv1.StartWritingVoiceCandidatesRequest]) (*connect.Response[postpilotv1.StartWritingVoiceCandidatesResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	count, countErr := voice.NormalizeCandidateCount(int(req.Msg.GetCandidateCount()))
	if countErr != nil {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid candidate count", postpilotv1.FailureReason_AUTHORING_CANDIDATE_COUNT_INVALID, nil)
	}
	id, err := h.service.Start(ctx, user, fromProtoRef(req.Msg.GetWriteModel()), count)
	if err != nil {
		return nil, candidateConnectError("start writing styles", err)
	}
	return connect.NewResponse(&postpilotv1.StartWritingVoiceCandidatesResponse{JobId: id}), nil
}
func (h *CandidateHandler) GetWritingVoiceCandidates(ctx context.Context, req *connect.Request[postpilotv1.GetWritingVoiceCandidatesRequest]) (*connect.Response[postpilotv1.GetWritingVoiceCandidatesResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	batch, err := h.service.Get(ctx, user, req.Msg.GetJobId())
	if err != nil {
		return nil, candidateConnectError("get writing styles", err)
	}
	return connect.NewResponse(&postpilotv1.GetWritingVoiceCandidatesResponse{JobId: batch.JobID, Candidates: protoWritingCandidates(batch.Candidates)}), nil
}
func (h *CandidateHandler) GetLatestWritingVoiceCandidates(ctx context.Context, _ *connect.Request[postpilotv1.GetLatestWritingVoiceCandidatesRequest]) (*connect.Response[postpilotv1.GetLatestWritingVoiceCandidatesResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	latest, err := h.service.Latest(ctx, user)
	if err != nil {
		return nil, candidateConnectError("get latest writing styles", err)
	}
	return connect.NewResponse(&postpilotv1.GetLatestWritingVoiceCandidatesResponse{JobId: latest.JobID, ResultJobId: latest.ResultJobID, Candidates: protoWritingCandidates(latest.Candidates)}), nil
}
func (h *CandidateHandler) CancelWritingVoiceCandidates(ctx context.Context, req *connect.Request[postpilotv1.CancelWritingVoiceCandidatesRequest]) (*connect.Response[postpilotv1.CancelWritingVoiceCandidatesResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.Cancel(ctx, user, req.Msg.GetJobId()); err != nil {
		return nil, candidateConnectError("cancel writing styles", err)
	}
	return connect.NewResponse(&postpilotv1.CancelWritingVoiceCandidatesResponse{}), nil
}
func (h *CandidateHandler) AdoptWritingVoiceCandidate(ctx context.Context, req *connect.Request[postpilotv1.AdoptWritingVoiceCandidateRequest]) (*connect.Response[postpilotv1.AdoptWritingVoiceCandidateResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	adopted, err := h.service.Adopt(ctx, user, req.Msg.GetJobId(), req.Msg.GetCandidateId(), req.Msg.GetMakeDefault())
	if err != nil {
		return nil, candidateConnectError("adopt writing style", err)
	}
	return connect.NewResponse(&postpilotv1.AdoptWritingVoiceCandidateResponse{Voice: toProtoVoice(adopted)}), nil
}
func protoWritingCandidates(values []voice.WritingCandidate) []*postpilotv1.WritingVoiceCandidate {
	out := make([]*postpilotv1.WritingVoiceCandidate, 0, len(values))
	for _, value := range values {
		out = append(out, &postpilotv1.WritingVoiceCandidate{Id: value.ID, Name: value.Name, Description: value.Description, Sample: value.Sample})
	}
	return out
}
func candidateConnectError(op string, err error) error {
	var running *voice.CandidatesRunningError
	switch {
	case errors.Is(err, voice.ErrCandidateCount):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid candidate count", postpilotv1.FailureReason_AUTHORING_CANDIDATE_COUNT_INVALID, nil)
	case errors.Is(err, voice.ErrCandidateNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "writing style not found", postpilotv1.FailureReason_WRITING_VOICE_CANDIDATE_NOT_FOUND, nil)
	case errors.Is(err, voice.ErrCandidatesNotReady):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "writing styles not ready", postpilotv1.FailureReason_WRITING_VOICE_CANDIDATES_NOT_READY, nil)
	case errors.Is(err, voice.ErrCandidateModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "writing model required", postpilotv1.FailureReason_WRITING_VOICE_CANDIDATE_MODEL_REQUIRED, nil)
	case errors.As(err, &running):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "writing styles already running", postpilotv1.FailureReason_WRITING_VOICE_CANDIDATES_RUNNING, map[string]string{"active_job_id": running.ActiveID})
	default:
		return toConnectError(op, err)
	}
}

var _ postpilotv1connect.WritingVoiceCandidateServiceHandler = (*CandidateHandler)(nil)
