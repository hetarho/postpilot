package rpc

import (
	"connectrpc.com/connect"
	"context"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/template"
)

func (h *Handler) EstimateTemplateHeadingSuggestions(ctx context.Context, req *connect.Request[postpilotv1.EstimateTemplateHeadingSuggestionsRequest]) (*connect.Response[postpilotv1.EstimateTemplateHeadingSuggestionsResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	estimate, err := h.headings.Estimate(ctx, modelRefValue(req.Msg.GetWriteModel()))
	if err != nil {
		return nil, toConnectError("estimate heading suggestions", err)
	}
	out := &postpilotv1.EstimateTemplateHeadingSuggestionsResponse{Free: estimate.Free}
	if estimate.Available && !estimate.Free {
		credits := int32(estimate.Credits)
		out.Credits = &credits
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) StartTemplateHeadingSuggestions(ctx context.Context, req *connect.Request[postpilotv1.StartTemplateHeadingSuggestionsRequest]) (*connect.Response[postpilotv1.StartTemplateHeadingSuggestionsResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	sections := make([]template.HeadingSection, len(req.Msg.GetSections()))
	for i, section := range req.Msg.GetSections() {
		sections[i] = template.HeadingSection{Role: section.GetRole(), Description: section.GetDescription()}
	}
	id, err := h.headings.Start(ctx, user, template.HeadingSuggestionStart{WriteModel: modelRefValue(req.Msg.GetWriteModel()), TargetKind: req.Msg.GetTargetKind(), TargetPath: req.Msg.GetTargetPath(), Sections: sections})
	if err != nil {
		return nil, toConnectError("start heading suggestions", err)
	}
	return connect.NewResponse(&postpilotv1.StartTemplateHeadingSuggestionsResponse{JobId: id}), nil
}

func (h *Handler) GetTemplateHeadingSuggestions(ctx context.Context, req *connect.Request[postpilotv1.GetTemplateHeadingSuggestionsRequest]) (*connect.Response[postpilotv1.GetTemplateHeadingSuggestionsResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := h.headings.Result(ctx, user, req.Msg.GetJobId())
	if err != nil {
		return nil, toConnectError("get heading suggestions", err)
	}
	out := make([]*postpilotv1.TemplateHeadingChoices, len(groups))
	for i, headings := range groups {
		out[i] = &postpilotv1.TemplateHeadingChoices{Headings: headings}
	}
	return connect.NewResponse(&postpilotv1.GetTemplateHeadingSuggestionsResponse{Sections: out}), nil
}
