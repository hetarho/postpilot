// Package rpc is the provider context's transport edge: proto ↔ domain mapping and the
// translation of domain errors into Connect codes.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
)

// Handler implements postpilotv1connect.ProviderServiceHandler.
type Handler struct {
	svc *provider.Service
}

// NewHandler returns the ProviderService implementation.
func NewHandler(svc *provider.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListModels(ctx context.Context, _ *connect.Request[postpilotv1.ListModelsRequest]) (*connect.Response[postpilotv1.ListModelsResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	models := h.svc.ListModels()
	out := make([]*postpilotv1.ModelInfo, 0, len(models))
	for _, m := range models {
		out = append(out, toProtoModel(m))
	}
	return connect.NewResponse(&postpilotv1.ListModelsResponse{Models: out}), nil
}

func (h *Handler) GetSelections(ctx context.Context, _ *connect.Request[postpilotv1.GetSelectionsRequest]) (*connect.Response[postpilotv1.GetSelectionsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	selections, err := h.svc.GetSelections(ctx, userID)
	if err != nil {
		return nil, toConnectError("get selections", err)
	}
	out := make([]*postpilotv1.Selection, 0, len(selections))
	for _, s := range selections {
		out = append(out, toProtoSelection(s))
	}
	return connect.NewResponse(&postpilotv1.GetSelectionsResponse{Selections: out}), nil
}

func (h *Handler) SaveSelection(ctx context.Context, req *connect.Request[postpilotv1.SaveSelectionRequest]) (*connect.Response[postpilotv1.SaveSelectionResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	stage, ok := fromProtoStage(req.Msg.GetStage())
	if !ok {
		if req.Msg.GetStage() == postpilotv1.Stage_STAGE_UNSPECIFIED {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("stage is required"))
		}
		// A newer client than this server: say so rather than "required".
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: %s", provider.ErrUnknownStage, req.Msg.GetStage()))
	}
	ref := llm.ModelRef{ProviderID: req.Msg.GetRef().GetProviderId(), ModelID: req.Msg.GetRef().GetModelId()}
	saved, err := h.svc.SaveSelection(ctx, userID, stage, ref)
	if err != nil {
		return nil, toConnectError("save selection", err)
	}
	return connect.NewResponse(&postpilotv1.SaveSelectionResponse{Selection: toProtoSelection(saved)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	return userID, nil
}

func toConnectError(op string, err error) error {
	switch {
	case errors.Is(err, provider.ErrUnknownStage):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, provider.ErrModelNotRegistered):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, provider.ErrModelDisabled), errors.Is(err, provider.ErrModelUnsuitable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		slog.Error(op+" failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New(op+" failed"))
	}
}

var stageToProto = map[provider.Stage]postpilotv1.Stage{
	provider.StageObserve: postpilotv1.Stage_STAGE_OBSERVE,
	provider.StageWrite:   postpilotv1.Stage_STAGE_WRITE,
	provider.StageAnalyze: postpilotv1.Stage_STAGE_ANALYZE,
}

func fromProtoStage(s postpilotv1.Stage) (provider.Stage, bool) {
	for stage, wire := range stageToProto {
		if wire == s {
			return stage, true
		}
	}
	return "", false
}

func toProtoModel(m llm.ModelInfo) *postpilotv1.ModelInfo {
	return &postpilotv1.ModelInfo{
		Ref:              &postpilotv1.ModelRef{ProviderId: m.Ref.ProviderID, ModelId: m.Ref.ModelID},
		Label:            m.Label,
		Vision:           m.Vision,
		StructuredOutput: m.StructuredOutput,
		Disabled:         m.Disabled,
		DisabledReason:   m.DisabledReason,
	}
}

func toProtoSelection(s provider.Selection) *postpilotv1.Selection {
	return &postpilotv1.Selection{
		Stage:   stageToProto[s.Stage],
		Ref:     &postpilotv1.ModelRef{ProviderId: s.Ref.ProviderID, ModelId: s.Ref.ModelID},
		Missing: s.Missing,
	}
}

var _ postpilotv1connect.ProviderServiceHandler = (*Handler)(nil)
