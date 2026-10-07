package main

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
)

// generationInspectionSelections reads existing eligible selections without
// initializing absent defaults, changing settings or querying live endpoints.
type generationInspectionSelections struct{ service *provider.Service }

type postRequestInspectionReader struct{ service *generation.Service }

func (a postRequestInspectionReader) ReadPostRequestInspection(ctx context.Context, userID, slug, stage string, status llm.InspectionStatus) (llm.RequestInspection, error) {
	value, err := a.service.ReadPostRequestInspection(ctx, userID, slug, stage, status)
	switch {
	case errors.Is(err, generation.ErrNotFound):
		err = post.ErrNotFound
	case errors.Is(err, generation.ErrForbidden):
		err = post.ErrForbidden
	}
	return value, err
}

type generationInspectionModels struct {
	registry   *llm.Registry
	selections generationInspectionSelections
}

func (a generationInspectionModels) PreparePostRequest(ctx context.Context, userID string, ref llm.ModelRef, request llm.Request) (llm.RequestInspection, error) {
	selected, eligible, err := a.selections.ModelForInspection(ctx, userID, request.Stage)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	if !eligible || selected != ref {
		return llm.RequestInspection{}, llm.ErrModelUnavailable
	}
	info, found := a.registry.Lookup(ref)
	if !found || info.Disabled || !info.ServesStage(request.Stage) {
		return llm.RequestInspection{}, llm.ErrModelUnavailable
	}
	ctx = llm.WithAdmittedCalls(ctx, []llm.AdmittedCall{{Ref: ref, Stage: request.Stage, Grade: info.Levels[request.Stage]}})
	return a.registry.Prepare(ctx, ref, request)
}

func (a generationInspectionSelections) ModelForInspection(ctx context.Context, userID, stage string) (llm.ModelRef, bool, error) {
	var purpose provider.Stage
	switch stage {
	case "observe":
		purpose = provider.StageObserve
	case "plan", "write", "revise":
		purpose = provider.StageWrite
	default:
		return llm.ModelRef{}, false, nil
	}
	selection, found, err := a.service.SelectionForInspection(ctx, userID, purpose)
	if err != nil {
		return llm.ModelRef{}, false, err
	}
	return selection.Ref, found && !selection.Missing && selection.UnavailableReason == "", nil
}

var _ generation.RequestInspectionSelections = generationInspectionSelections{}
var _ generation.RequestInspectionModels = generationInspectionModels{}
var _ post.RequestInspectionReader = postRequestInspectionReader{}

type authoringInspectionModels struct {
	registry *llm.Registry
	models   *provider.Service
}

func (a authoringInspectionModels) PrepareAuthoringRequest(ctx context.Context, userID string, ref llm.ModelRef, request llm.Request) (llm.RequestInspection, error) {
	info, eligible, err := a.models.ModelForInspection(ctx, userID, provider.StageWrite, ref)
	if err != nil {
		return llm.RequestInspection{}, err
	}
	if !eligible || request.Stage != llm.StageNameWrite {
		return llm.RequestInspection{}, llm.ErrModelUnavailable
	}
	ctx = llm.WithAdmittedCalls(ctx, []llm.AdmittedCall{{Ref: ref, Stage: request.Stage, Grade: info.Levels[request.Stage]}})
	return a.registry.Prepare(ctx, ref, request)
}

var _ authoring.RequestInspectionModels = authoringInspectionModels{}
var _ authoring.RequestInspectionSelections = generationInspectionSelections{}
