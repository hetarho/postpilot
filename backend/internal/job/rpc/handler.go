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
)

type Handler struct {
	queue *job.Queue
}

func NewHandler(queue *job.Queue) *Handler { return &Handler{queue: queue} }

func (h *Handler) GetGeneration(ctx context.Context, req *connect.Request[postpilotv1.GetGenerationRequest]) (*connect.Response[postpilotv1.GetGenerationResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	found, err := h.queue.Get(ctx, req.Msg.GetId(), userID)
	if err != nil {
		switch {
		case errors.Is(err, job.ErrNotFound):
			return nil, connect.NewError(connect.CodeNotFound, errors.New("job not found"))
		case errors.Is(err, job.ErrForbidden):
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("job belongs to another user"))
		default:
			slog.Error("get generation failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("get generation failed"))
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
		Error: found.Error, PostSlug: postSlug, ObserveModel: modelRef(found.ObserveModel),
		WriteModel: modelRef(found.WriteModel), CreatedAt: found.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: found.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func modelRef(value string) *postpilotv1.ModelRef {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return nil
	}
	return &postpilotv1.ModelRef{ProviderId: providerID, ModelId: modelID}
}
