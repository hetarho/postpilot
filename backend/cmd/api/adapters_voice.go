package main

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/voice"
)

type voiceModels struct {
	selections *provider.Service
	registry   meteredRegistry
	plans      *auth.Service
}

func (a voiceModels) AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error) {
	selections, err := a.selections.GetSelections(ctx, userID)
	if err != nil {
		return llm.ModelRef{}, false, err
	}
	for _, selection := range selections {
		if selection.Stage != provider.StageAnalyze || selection.Missing {
			continue
		}
		info, ok := a.registry.Lookup(selection.Ref)
		if !ok || info.Disabled {
			return llm.ModelRef{}, false, nil
		}
		return selection.Ref, true, nil
	}
	return llm.ModelRef{}, false, nil
}

func (a voiceModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return a.registry.Lookup(ref) }

func (a voiceModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

type voiceJobs struct{ queue *job.Queue }

func (a voiceJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	subjects, guards := postVoiceWork(job.KindAnalyzeVoice, request.UserID, "", request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: request.UserID, Subjects: subjects, Guards: guards, WriteModel: request.WriteModel,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", voice.ErrVoiceDeleted
	}
	return id, err
}

func (a voiceJobs) EnqueuePersonalization(ctx context.Context, request voice.PersonalizationJobRequest) (string, error) {
	subjects, guards := postVoiceWork(request.Kind, request.UserID, "", request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: request.Kind, UserID: request.UserID, Subjects: subjects, Guards: guards,
		WriteModel: request.Model, Payload: []byte(request.Payload),
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", voice.ErrVoiceDeleted
	}
	return id, err
}

func (a voiceJobs) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{Kind: kind})
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func (a voiceJobs) LatestForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.FinishedJob, error) {
	found, err := a.queue.LatestFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{Kind: kind})
	if err != nil || found == nil {
		return nil, err
	}
	finished := &voice.FinishedJob{ID: found.ID, Status: found.Status}
	if found.Failure != nil {
		finished.Failure = &voice.Failure{Reason: found.Failure.Reason, Params: found.Failure.Params, TechnicalDetail: found.Failure.TechnicalDetail}
	}
	return finished, nil
}

func (a voiceJobs) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	return a.queue.HasActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{})
}

// voiceExperiments adapts the experiment context's publishable-work guard for DeleteVoice.
type voiceExperiments struct{ service *experiment.Service }

func (a voiceExperiments) HasPublishableExperimentForVoice(ctx context.Context, userID, voiceID string) (bool, error) {
	return a.service.HasPublishableForVoice(ctx, userID, voiceID)
}
