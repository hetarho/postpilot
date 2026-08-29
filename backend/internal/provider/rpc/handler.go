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

func (h *Handler) GetComparisonPairs(ctx context.Context, _ *connect.Request[postpilotv1.GetComparisonPairsRequest]) (*connect.Response[postpilotv1.GetComparisonPairsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	pairs, err := h.svc.GetComparisonPairs(ctx, userID)
	if err != nil {
		return nil, toConnectError("get comparison pairs", err)
	}
	out := make([]*postpilotv1.ComparisonPair, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, toProtoPair(pair))
	}
	return connect.NewResponse(&postpilotv1.GetComparisonPairsResponse{Pairs: out}), nil
}

func (h *Handler) SaveComparisonPair(ctx context.Context, req *connect.Request[postpilotv1.SaveComparisonPairRequest]) (*connect.Response[postpilotv1.SaveComparisonPairResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	stage, ok := fromProtoStage(req.Msg.GetStage())
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("stage is required"))
	}
	pair, err := h.svc.SaveComparisonPair(ctx, userID, stage, fromProtoRef(req.Msg.GetCandidateA()), fromProtoRef(req.Msg.GetCandidateB()))
	if err != nil {
		return nil, toConnectError("save comparison pair", err)
	}
	return connect.NewResponse(&postpilotv1.SaveComparisonPairResponse{Pair: toProtoPair(pair)}), nil
}

func (h *Handler) ListRecommendationSets(ctx context.Context, _ *connect.Request[postpilotv1.ListRecommendationSetsRequest]) (*connect.Response[postpilotv1.ListRecommendationSetsResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	sets := h.svc.RecommendationSets()
	out := make([]*postpilotv1.RecommendationSet, 0, len(sets))
	for _, set := range sets {
		out = append(out, toProtoRecommendation(set))
	}
	return connect.NewResponse(&postpilotv1.ListRecommendationSetsResponse{Sets: out}), nil
}

func (h *Handler) ApplyRecommendationSet(ctx context.Context, req *connect.Request[postpilotv1.ApplyRecommendationSetRequest]) (*connect.Response[postpilotv1.ApplyRecommendationSetResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	set, selections, pairs, err := h.svc.ApplyRecommendationSet(ctx, userID, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError("apply recommendation set", err)
	}
	outSelections := make([]*postpilotv1.Selection, 0, len(selections))
	for _, selection := range selections {
		outSelections = append(outSelections, toProtoSelection(selection))
	}
	outPairs := make([]*postpilotv1.ComparisonPair, 0, len(pairs))
	for _, pair := range pairs {
		outPairs = append(outPairs, toProtoPair(pair))
	}
	return connect.NewResponse(&postpilotv1.ApplyRecommendationSetResponse{
		Set: toProtoRecommendation(set), Selections: outSelections, Pairs: outPairs,
	}), nil
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
	case errors.Is(err, provider.ErrModelNotRegistered), errors.Is(err, provider.ErrRecommendationNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, provider.ErrModelDisabled), errors.Is(err, provider.ErrModelUnsuitable), errors.Is(err, provider.ErrDuplicateCandidates):
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
		Ref:                 &postpilotv1.ModelRef{ProviderId: m.Ref.ProviderID, ModelId: m.Ref.ModelID},
		Label:               m.Label,
		Vision:              m.Vision,
		StructuredOutput:    m.StructuredOutput,
		Disabled:            m.Disabled,
		DisabledReason:      m.DisabledReason,
		ContextTokens:       m.ContextTokens,
		InputUsdPerMillion:  m.InputUSDPerMillion,
		OutputUsdPerMillion: m.OutputUSDPerMillion,
		PricingCheckedAt:    m.PricingCheckedAt,
	}
}

func toProtoSelection(s provider.Selection) *postpilotv1.Selection {
	return &postpilotv1.Selection{
		Stage:   stageToProto[s.Stage],
		Ref:     &postpilotv1.ModelRef{ProviderId: s.Ref.ProviderID, ModelId: s.Ref.ModelID},
		Missing: s.Missing, Slot: slotToProto(s.Slot),
	}
}

func fromProtoRef(ref *postpilotv1.ModelRef) llm.ModelRef {
	if ref == nil {
		return llm.ModelRef{}
	}
	return llm.ModelRef{ProviderID: ref.GetProviderId(), ModelID: ref.GetModelId()}
}

func toProtoRef(ref llm.ModelRef) *postpilotv1.ModelRef {
	return &postpilotv1.ModelRef{ProviderId: ref.ProviderID, ModelId: ref.ModelID}
}

func slotToProto(slot provider.SelectionSlot) postpilotv1.SelectionSlot {
	switch slot {
	case provider.SlotCandidateA:
		return postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_A
	case provider.SlotCandidateB:
		return postpilotv1.SelectionSlot_SELECTION_SLOT_CANDIDATE_B
	default:
		return postpilotv1.SelectionSlot_SELECTION_SLOT_ACTIVE
	}
}

func toProtoPair(pair provider.ComparisonPair) *postpilotv1.ComparisonPair {
	return &postpilotv1.ComparisonPair{Stage: stageToProto[pair.Stage], CandidateA: toProtoSelection(pair.CandidateA), CandidateB: toProtoSelection(pair.CandidateB)}
}

func toProtoRecommendation(set provider.RecommendationSet) *postpilotv1.RecommendationSet {
	out := &postpilotv1.RecommendationSet{Id: set.ID, Label: set.Label}
	for _, selection := range set.Selections {
		out.Selections = append(out.Selections, &postpilotv1.RecommendationStageSelection{
			Stage: stageToProto[selection.Stage], Active: toProtoRef(selection.Active),
			CandidateA: toProtoRef(selection.CandidateA), CandidateB: toProtoRef(selection.CandidateB),
		})
	}
	return out
}

var _ postpilotv1connect.ProviderServiceHandler = (*Handler)(nil)
