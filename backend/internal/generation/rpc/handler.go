// Package rpc is the generation context's Connect transport edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

type Handler struct{ service *generation.Service }

func NewHandler(service *generation.Service) *Handler { return &Handler{service: service} }

func (h *Handler) StartGeneration(ctx context.Context, req *connect.Request[postpilotv1.StartGenerationRequest]) (*connect.Response[postpilotv1.StartGenerationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.service.Start(ctx, generation.StartRequest{
		UserID: userID, PostSlug: req.Msg.GetPostSlug(),
		ObserveModel: modelRefValue(req.Msg.GetObserveModel()),
		WriteModel:   modelRefValue(req.Msg.GetWriteModel()),
		TargetLength: optionalTargetLength(req.Msg.TargetLength),
	})
	if err != nil {
		return nil, toConnectError("start generation", err)
	}
	return connect.NewResponse(&postpilotv1.StartGenerationResponse{JobId: id}), nil
}

func optionalTargetLength(value *int32) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func (h *Handler) StartRevision(ctx context.Context, req *connect.Request[postpilotv1.StartRevisionRequest]) (*connect.Response[postpilotv1.StartRevisionResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := h.service.StartRevision(ctx, generation.StartRevisionRequest{
		UserID: userID, PostSlug: req.Msg.GetPostSlug(), Instruction: req.Msg.GetInstruction(),
		SaveAsRule: req.Msg.GetSaveAsRule(), WriteModel: modelRefValue(req.Msg.GetWriteModel()),
	})
	if err != nil {
		return nil, toConnectError("start revision", err)
	}
	return connect.NewResponse(&postpilotv1.StartRevisionResponse{JobId: id}), nil
}

func (h *Handler) GetGeneration(ctx context.Context, req *connect.Request[postpilotv1.GetGenerationRequest]) (*connect.Response[postpilotv1.GetGenerationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	found, err := h.service.GetJob(ctx, req.Msg.GetId(), userID)
	if err != nil {
		return nil, toConnectError("get generation", err)
	}
	return connect.NewResponse(&postpilotv1.GetGenerationResponse{Job: toProtoJob(found)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

func toConnectError(op string, err error) error {
	// Plan refusals are matched by type here rather than mapped by each service: the
	// admission gate lives at one seam (job enqueue), so its two failures must translate
	// identically wherever they surface.
	var quota *plan.QuotaError
	if errors.As(err, &quota) {
		return rpcserver.AppErrorFrom(connect.CodeResourceExhausted, quota)
	}
	var locked *plan.ModelLockedError
	if errors.As(err, &locked) {
		return rpcserver.AppErrorFrom(connect.CodePermissionDenied, locked)
	}
	var active *generation.JobAlreadyInProgressError
	switch {
	case errors.Is(err, generation.ErrNotFound):
		if op == "get generation" {
			return rpcserver.NewAppError(connect.CodeNotFound, "generation job not found", "JOB_NOT_FOUND", nil)
		}
		return rpcserver.NewAppError(connect.CodeNotFound, "post not found", "POST_NOT_FOUND", nil)
	case errors.Is(err, generation.ErrForbidden):
		if op == "get generation" {
			return rpcserver.NewAppError(connect.CodePermissionDenied, "generation job belongs to another user", "JOB_FORBIDDEN", nil)
		}
		return rpcserver.NewAppError(connect.CodePermissionDenied, "post belongs to another user", "POST_FORBIDDEN", nil)
	case errors.Is(err, generation.ErrWriteModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "an enabled write model is required", "GENERATION_WRITE_MODEL_REQUIRED", nil)
	case errors.Is(err, generation.ErrObserveModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "an enabled photo observation model is required", "GENERATION_OBSERVE_MODEL_REQUIRED", nil)
	case errors.Is(err, generation.ErrLanguageRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post target language is required", "POST_TARGET_LANGUAGE_REQUIRED", nil)
	case errors.Is(err, generation.ErrContentLanguageRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "content language is required for revision", "CONTENT_LANGUAGE_REQUIRED", nil)
	case errors.Is(err, generation.ErrRevisionContentRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post content is required for revision", "REVISION_CONTENT_REQUIRED", nil)
	case errors.Is(err, generation.ErrVoiceDeleted):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post voice is deleted", "VOICE_DELETED", nil)
	case errors.Is(err, generation.ErrVoiceMismatch):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post voice changed after enqueue", "GENERATION_VOICE_MISMATCH", nil)
	case errors.Is(err, generation.ErrVoiceContentLanguageMismatch):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post content language does not match the voice source language", "VOICE_CONTENT_LANGUAGE_MISMATCH", nil)
	case errors.Is(err, generation.ErrVoiceRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post voice is required", "VOICE_REQUIRED", nil)
	case errors.Is(err, generation.ErrRevisionInstructionRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "revision instruction is required", "REVISION_INSTRUCTION_REQUIRED", nil)
	case errors.Is(err, generation.ErrRevisionInstructionTooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "revision instruction is too long", "REVISION_INSTRUCTION_TOO_LONG", map[string]string{
			"max": strconv.Itoa(generation.RevisionInstructionMaxChars),
		})
	case errors.Is(err, generation.ErrInvalidTargetLength):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "target length must be positive", "GENERATION_TARGET_LENGTH_INVALID", nil)
	case errors.As(err, &active):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "generation is already in progress", "GENERATION_ALREADY_RUNNING", activeJobParams(active.ActiveID))
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "generation request failed", "UNKNOWN_FAILURE", nil)
	}
}

func activeJobParams(id string) map[string]string {
	if id == "" || len(id) > 128 {
		return nil
	}
	for i := range len(id) {
		char := id[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' && char != '_' {
			return nil
		}
	}
	return map[string]string{"active_job_id": id}
}

func modelRefValue(ref *postpilotv1.ModelRef) string {
	if ref == nil || ref.GetProviderId() == "" || ref.GetModelId() == "" {
		return ""
	}
	return ref.GetProviderId() + "/" + ref.GetModelId()
}

func toProtoJob(found *generation.JobSummary) *postpilotv1.GenerationJob {
	if found == nil {
		return nil
	}
	return &postpilotv1.GenerationJob{
		Id: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: int32(found.ProgressDone), ProgressTotal: int32(found.ProgressTotal),
		PostSlug: found.PostSlug, ObserveModel: splitModelRef(found.ObserveModel),
		WriteModel: splitModelRef(found.WriteModel), TargetLanguage: languageToProto(found.TargetLanguage), CreatedAt: found.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: found.UpdatedAt.UTC().Format(time.RFC3339), Failure: failureToProto(found.Failure),
	}
}

func failureToProto(found *generation.Failure) *postpilotv1.Failure {
	if found == nil || found.Reason == "" {
		return nil
	}
	params := make(map[string]string, len(found.Params))
	for key, value := range found.Params {
		params[key] = value
	}
	return &postpilotv1.Failure{
		Reason: found.Reason, Params: params, TechnicalDetail: found.TechnicalDetail,
	}
}

func languageToProto(value generation.Language) postpilotv1.ContentLanguage {
	switch value {
	case generation.LanguageKorean:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case generation.LanguageEnglish:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}

func splitModelRef(value string) *postpilotv1.ModelRef {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return nil
	}
	return &postpilotv1.ModelRef{ProviderId: providerID, ModelId: modelID}
}

var _ postpilotv1connect.GenerationServiceHandler = (*Handler)(nil)
