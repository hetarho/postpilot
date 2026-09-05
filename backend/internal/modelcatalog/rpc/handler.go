// Package rpc is the model-catalog context's transport edge: proto ↔ domain mapping and
// the translation of domain errors into Connect codes.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// Handler implements postpilotv1connect.ModelCatalogServiceHandler.
//
// It carries no authorization of its own: every procedure is in the interceptor's
// master-only set, so a non-master call never reaches these functions. Keeping the check
// there is what makes "which procedures are privileged" answerable by reading one map.
type Handler struct {
	svc *modelcatalog.Service
}

func NewHandler(svc *modelcatalog.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListCatalog(ctx context.Context, req *connect.Request[postpilotv1.ListCatalogRequest]) (*connect.Response[postpilotv1.ListCatalogResponse], error) {
	// An unknown purpose is refused rather than silently listed with no effort: a tab that
	// asked for the wrong slug would otherwise show every override as cleared.
	purpose := modelcatalog.Purpose("")
	if raw := req.Msg.GetPurpose(); raw != "" {
		parsed, err := modelcatalog.ParsePurpose(raw)
		if err != nil {
			return nil, toConnectError("list catalog", err)
		}
		purpose = parsed
	}
	browse, err := h.svc.Browse(ctx, req.Msg.GetRefresh(), purpose)
	if err != nil {
		return nil, toConnectError("list catalog", err)
	}
	entries := make([]*postpilotv1.CatalogEntry, 0, len(browse.Entries))
	for _, entry := range browse.Entries {
		entries = append(entries, toProtoEntry(entry))
	}
	fetchedAt := ""
	if !browse.FetchedAt.IsZero() {
		fetchedAt = browse.FetchedAt.UTC().Format(time.RFC3339)
	}
	return connect.NewResponse(&postpilotv1.ListCatalogResponse{
		Entries: entries, FetchedAt: fetchedAt,
		FromCache: browse.FromCache, FetchError: browse.FetchError,
	}), nil
}

func (h *Handler) SetModelPurpose(ctx context.Context, req *connect.Request[postpilotv1.SetModelPurposeRequest]) (*connect.Response[postpilotv1.SetModelPurposeResponse], error) {
	modelID := req.Msg.GetModelId()
	if modelID == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a model id is required", "MODEL_ID_REQUIRED", nil)
	}
	purpose := modelcatalog.Purpose(req.Msg.GetPurpose())
	model, err := h.svc.SetPurpose(ctx, modelID, purpose, req.Msg.GetRegistered())
	if err != nil {
		return nil, toConnectError("set model purpose", err)
	}
	// The answer is projected for the purpose that was written, so the row the client
	// re-renders carries that tab's effort and not another's.
	return connect.NewResponse(&postpilotv1.SetModelPurposeResponse{Entry: toProtoEntry(modelcatalog.EntryOf(model, purpose))}), nil
}

func (h *Handler) UpdateModel(ctx context.Context, req *connect.Request[postpilotv1.UpdateModelRequest]) (*connect.Response[postpilotv1.UpdateModelResponse], error) {
	modelID := req.Msg.GetModelId()
	if modelID == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a model id is required", "MODEL_ID_REQUIRED", nil)
	}
	patch := modelcatalog.Patch{Purpose: modelcatalog.Purpose(req.Msg.GetPurpose())}
	if req.Msg.ReasoningEffort != nil {
		effort := llm.ReasoningEffort(req.Msg.GetReasoningEffort())
		patch.Reasoning = &effort
	}
	model, err := h.svc.Update(ctx, modelID, patch)
	if err != nil {
		return nil, toConnectError("update model", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateModelResponse{Entry: toProtoEntry(modelcatalog.EntryOf(model, patch.Purpose))}), nil
}

func toProtoEntry(e modelcatalog.Entry) *postpilotv1.CatalogEntry {
	purposes := make([]string, 0, len(e.Purposes))
	for _, purpose := range e.Purposes {
		purposes = append(purposes, string(purpose))
	}
	return &postpilotv1.CatalogEntry{
		ModelId:             e.ModelID,
		ProviderSlug:        e.ProviderSlug,
		Label:               e.Label,
		Description:         e.Description,
		Vision:              e.Vision,
		StructuredOutput:    e.StructuredOutput,
		ImageOutput:         e.ImageOutput,
		VideoOutput:         e.VideoOutput,
		ContextTokens:       e.ContextTokens,
		InputUsdPerMillion:  e.InputUSDPerMillion,
		OutputUsdPerMillion: e.OutputUSDPerMillion,
		Curated:             e.Curated,
		Purposes:            purposes,
		Listed:              e.Listed,
		ReasoningEffort:     string(e.Reasoning),
		SourceCreatedAt:     e.SourceCreatedAt,
		ReasoningSpend:      toProtoReasoningSpend(e.ReasoningSpend),

		Reasons:                e.Reasons,
		ReasoningEfforts:       e.Efforts,
		ReasoningDefaultEffort: e.DefaultEffort,
		ReasoningMandatory:     e.Mandatory,
		ReasoningNativeEffort:  e.NativeEffort,
		ReasoningMaxTokens:     e.MaxTokens,
		ReasoningDrifted:       e.ReasoningDrifted,
		ReasoningKnown:         e.ReasoningKnown,
	}
}

func toProtoReasoningSpend(spend *modelcatalog.ReasoningSpend) *postpilotv1.ReasoningSpend {
	if spend == nil {
		return nil
	}
	return &postpilotv1.ReasoningSpend{
		Calls: spend.Calls, ReasoningTokens: spend.ReasoningTokens,
		CompletionTokens: spend.CompletionTokens,
	}
}

func toConnectError(op string, err error) error {
	switch {
	case errors.Is(err, modelcatalog.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "model not found", "MODEL_NOT_FOUND", nil)
	case errors.Is(err, modelcatalog.ErrInvalidReasoning):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "unknown reasoning effort", "MODEL_REASONING_INVALID", nil)
	case errors.Is(err, modelcatalog.ErrUnknownPurpose):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "unknown purpose", "MODEL_PURPOSE_INVALID", nil)
	case errors.Is(err, modelcatalog.ErrPurposeIneligible):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "model not capable of this purpose", "MODEL_PURPOSE_INELIGIBLE", nil)
	case errors.Is(err, modelcatalog.ErrPurposeNotRegistered):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "model is not registered to this purpose", "MODEL_PURPOSE_NOT_REGISTERED", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

var _ postpilotv1connect.ModelCatalogServiceHandler = (*Handler)(nil)
