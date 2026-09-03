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
	browse, err := h.svc.Browse(ctx, req.Msg.GetRefresh())
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

func (h *Handler) EnableModel(ctx context.Context, req *connect.Request[postpilotv1.EnableModelRequest]) (*connect.Response[postpilotv1.EnableModelResponse], error) {
	modelID := req.Msg.GetModelId()
	if modelID == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a model id is required", "MODEL_ID_REQUIRED", nil)
	}
	model, err := h.svc.Enable(ctx, modelID)
	if err != nil {
		return nil, toConnectError("enable model", err)
	}
	return connect.NewResponse(&postpilotv1.EnableModelResponse{Entry: toProtoEntry(entryOf(model))}), nil
}

func (h *Handler) UpdateModel(ctx context.Context, req *connect.Request[postpilotv1.UpdateModelRequest]) (*connect.Response[postpilotv1.UpdateModelResponse], error) {
	modelID := req.Msg.GetModelId()
	if modelID == "" {
		return nil, rpcserver.NewAppError(connect.CodeInvalidArgument, "a model id is required", "MODEL_ID_REQUIRED", nil)
	}
	patch := modelcatalog.Patch{}
	if req.Msg.Enabled != nil {
		enabled := req.Msg.GetEnabled()
		patch.Enabled = &enabled
	}
	if req.Msg.ReasoningEffort != nil {
		effort := llm.ReasoningEffort(req.Msg.GetReasoningEffort())
		patch.Reasoning = &effort
	}
	model, err := h.svc.Update(ctx, modelID, patch)
	if err != nil {
		return nil, toConnectError("update model", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateModelResponse{Entry: toProtoEntry(entryOf(model))}), nil
}

// entryOf presents a written row in the same shape the browse list uses, so the client
// patches its cache from the answer instead of refetching the whole catalog. The upstream
// description is absent because a write does not read the provider.
func entryOf(m modelcatalog.Model) modelcatalog.Entry {
	return modelcatalog.Entry{
		Candidate: modelcatalog.Candidate{
			ModelID: m.ModelID, ProviderSlug: m.ProviderSlug, Label: m.Label,
			Vision: m.Vision, StructuredOutput: m.StructuredOutput,
			ContextTokens:      m.ContextTokens,
			InputUSDPerMillion: m.InputUSDPerMillion, OutputUSDPerMillion: m.OutputUSDPerMillion,
		},
		Curated: true, Enabled: m.Enabled,
		Reasoning: m.Reasoning, Listed: m.Listed,
	}
}

func toProtoEntry(e modelcatalog.Entry) *postpilotv1.CatalogEntry {
	return &postpilotv1.CatalogEntry{
		ModelId:             e.ModelID,
		ProviderSlug:        e.ProviderSlug,
		Label:               e.Label,
		Description:         e.Description,
		Vision:              e.Vision,
		StructuredOutput:    e.StructuredOutput,
		ContextTokens:       e.ContextTokens,
		InputUsdPerMillion:  e.InputUSDPerMillion,
		OutputUsdPerMillion: e.OutputUSDPerMillion,
		Curated:             e.Curated,
		Enabled:             e.Enabled,
		Listed:              e.Listed,
		ReasoningEffort:     string(e.Reasoning),
		SourceCreatedAt:     e.SourceCreatedAt,
	}
}

func toConnectError(op string, err error) error {
	switch {
	case errors.Is(err, modelcatalog.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "model not found", "MODEL_NOT_FOUND", nil)
	case errors.Is(err, modelcatalog.ErrInvalidReasoning):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "unknown reasoning effort", "MODEL_REASONING_INVALID", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

var _ postpilotv1connect.ModelCatalogServiceHandler = (*Handler)(nil)
