// Package rpc is the generation context's Connect transport edge.
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
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/generation"
)

type Handler struct{ service *generation.Service }

func NewHandler(service *generation.Service) *Handler { return &Handler{service: service} }

func (h *Handler) StartGeneration(ctx context.Context, req *connect.Request[postpilotv1.StartGenerationRequest]) (*connect.Response[postpilotv1.StartGenerationResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	started, err := h.service.StartExperiment(ctx, generation.StartExperimentRequest{
		UserID: userID, PostSlug: req.Msg.GetPostSlug(),
		ObserveModel: modelRefValue(req.Msg.GetObserveModel()),
		WriteModelA:  modelRefValue(req.Msg.GetWriteModelA()),
		WriteModelB:  modelRefValue(req.Msg.GetWriteModelB()),
	})
	if err != nil {
		return nil, toConnectError("start generation", err)
	}
	return connect.NewResponse(&postpilotv1.StartGenerationResponse{JobId: started.JobID, ExperimentId: started.ExperimentID}), nil
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
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return userID, nil
}

func toConnectError(op string, err error) error {
	var active *generation.JobAlreadyInProgressError
	switch {
	case errors.Is(err, generation.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("not found"))
	case errors.Is(err, generation.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, errors.New("not yours"))
	case errors.Is(err, generation.ErrWriteModelRequired), errors.Is(err, generation.ErrObserveModelRequired):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(err.Error()))
	case errors.Is(err, generation.ErrRevisionContentRequired):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(err.Error()))
	case errors.Is(err, generation.ErrRevisionInstructionRequired), errors.Is(err, generation.ErrRevisionInstructionTooLong):
		return connect.NewError(connect.CodeInvalidArgument, errors.New(err.Error()))
	case errors.As(err, &active):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("generation already in progress: "+active.ActiveID))
	default:
		slog.Error(op+" failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New(op+" failed"))
	}
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
		Error: found.Error, PostSlug: found.PostSlug, ObserveModel: splitModelRef(found.ObserveModel),
		WriteModel: splitModelRef(found.WriteModel), CreatedAt: found.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: found.UpdatedAt.UTC().Format(time.RFC3339),
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
