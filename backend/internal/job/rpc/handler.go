// Package rpc is the job context's Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

type Handler struct {
	queue *job.Queue
}

func NewHandler(queue *job.Queue) *Handler { return &Handler{queue: queue} }

func (h *Handler) GetGeneration(ctx context.Context, req *connect.Request[postpilotv1.GetGenerationRequest]) (*connect.Response[postpilotv1.GetGenerationResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	found, err := h.queue.Get(ctx, req.Msg.GetId(), userID)
	if err != nil {
		switch {
		case errors.Is(err, job.ErrNotFound):
			return nil, rpcserver.NewAppError(connect.CodeNotFound, "job not found", "JOB_NOT_FOUND", nil)
		case errors.Is(err, job.ErrForbidden):
			return nil, rpcserver.NewAppError(connect.CodePermissionDenied, "job belongs to another user", "JOB_FORBIDDEN", nil)
		default:
			slog.Error("get generation failed", "err", err)
			return nil, rpcserver.NewAppError(connect.CodeInternal, "get generation failed", "UNKNOWN_FAILURE", nil)
		}
	}
	return connect.NewResponse(&postpilotv1.GetGenerationResponse{Job: ToProto(found)}), nil
}

// ToProto is shared with the post RPC adapter, which embeds an active job snapshot.
func ToProto(found *job.JobSummary) *postpilotv1.GenerationJob {
	if found == nil {
		return nil
	}
	postSlug := ""
	if found.PostSlug != nil {
		postSlug = *found.PostSlug
	}
	return &postpilotv1.GenerationJob{
		Id: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: int32(found.ProgressDone), ProgressTotal: int32(found.ProgressTotal),
		PostSlug: postSlug, ObserveModel: modelRef(found.ObserveModel),
		WriteModel: modelRef(found.WriteModel), TargetLanguage: languageToProto(found.TargetLanguage), CreatedAt: found.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: found.UpdatedAt.UTC().Format(time.RFC3339), Failure: failureToProto(found.Failure),
	}
}

func failureToProto(found *job.Failure) *postpilotv1.Failure {
	if found == nil || found.Reason == "" {
		return nil
	}
	params := make(map[string]string, len(found.Params))
	for key, value := range found.Params {
		params[key] = value
	}
	return &postpilotv1.Failure{Reason: found.Reason, Params: params, TechnicalDetail: found.TechnicalDetail}
}

func languageToProto(value string) postpilotv1.ContentLanguage {
	switch value {
	case "ko":
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	case "en":
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	default:
		return postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	}
}

func modelRef(value string) *postpilotv1.ModelRef {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return nil
	}
	return &postpilotv1.ModelRef{ProviderId: providerID, ModelId: modelID}
}
