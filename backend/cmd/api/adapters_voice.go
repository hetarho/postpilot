package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
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

func (a voiceModels) ModelEnabled(ref llm.ModelRef, stage string) bool {
	info, ok := a.registry.Lookup(ref)
	return ok && !info.Disabled && info.ServesStage(stage)
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
	subjects, guards := postVoiceWork(request.Kind, request.UserID, request.PostSlug, request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: request.Kind, UserID: request.UserID, Subjects: subjects, Guards: guards,
		WriteModel: request.Model, ExtraModels: request.ExtraModels, Payload: []byte(request.Payload),
		CallCounts: request.CallCounts,
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

func (a voiceJobs) IsPersonalizationJobActive(ctx context.Context, jobID, userID string) (bool, error) {
	if jobID == "" {
		return false, nil
	}
	found, err := a.queue.Get(ctx, jobID, userID)
	if errors.Is(err, job.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return found.Status == job.StatusQueued || found.Status == job.StatusRunning, nil
}

func (a voiceJobs) FailQueuedPersonalization(ctx context.Context, jobID, userID string, failure voice.Failure) (bool, error) {
	return a.queue.FailQueued(ctx, jobID, userID, job.Failure{Reason: failure.Reason, Params: failure.Params, TechnicalDetail: failure.TechnicalDetail})
}

func (a voiceJobs) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{Kind: kind})
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func (a voiceJobs) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	return a.queue.HasActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{})
}

// voiceExperiments adapts the experiment context's publishable-work guard for DeleteVoice.
type voiceExperiments struct{ service *experiment.Service }

func (a voiceExperiments) HasPublishableExperimentForVoice(ctx context.Context, userID, voiceID string) (bool, error) {
	return a.service.HasPublishableForVoice(ctx, userID, voiceID)
}

// postVoices adapts the voice directory for the post context: every owned voice, tombstones
// included, so a post keeps a name after its voice is deleted.
type voicePosts struct{ service *post.Service }

func (a voicePosts) LearningSnapshot(ctx context.Context, userID, slug string) (voice.FinalizationInput, error) {
	found, err := a.service.LearningSnapshot(ctx, userID, slug)
	if err != nil {
		switch {
		case errors.Is(err, post.ErrNotFound):
			return voice.FinalizationInput{}, voice.ErrPostNotFound
		case errors.Is(err, post.ErrForbidden):
			return voice.FinalizationInput{}, voice.ErrForbidden
		case errors.Is(err, post.ErrNoMachineBaseline), errors.Is(err, post.ErrPostNotFinalized):
			return voice.FinalizationInput{}, voice.ErrInvalidLifecycle
		default:
			return voice.FinalizationInput{}, err
		}
	}
	baseline, err := json.Marshal(postContentWire(found.MachineBaseline))
	if err != nil {
		return voice.FinalizationInput{}, err
	}
	current, err := json.Marshal(postContentWire(found.Current))
	if err != nil {
		return voice.FinalizationInput{}, err
	}
	return voice.FinalizationInput{PostSlug: found.PostSlug, UserID: found.UserID, VoiceID: found.VoiceID, BaselineVoiceID: found.MachineBaselineVoiceID, BaselineJSON: string(baseline), FinalJSON: string(current), Title: found.Current.Title, Tags: found.Current.Tags, BaselineRevision: found.BaselineRevision, ContentRevision: found.ContentRevision, TargetLength: found.TargetLength, ContentLanguage: voice.Language(found.ContentLanguage), VoiceSourceLanguage: voice.Language(found.VoiceSourceLanguage)}, nil
}

type postBlockWire struct {
	Type    string   `json:"type"`
	Content string   `json:"content,omitempty"`
	Level   int32    `json:"level,omitempty"`
	File    string   `json:"file,omitempty"`
	Alt     string   `json:"alt,omitempty"`
	Caption string   `json:"caption,omitempty"`
	Items   []string `json:"items,omitempty"`
}
type postContentJSONWire struct {
	Title   string          `json:"title"`
	Summary string          `json:"summary"`
	Tags    []string        `json:"tags"`
	Blocks  []postBlockWire `json:"blocks"`
}

func postContentWire(content post.PostContent) postContentJSONWire {
	out := postContentJSONWire{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		out.Blocks = append(out.Blocks, postBlockWire{Type: string(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return out
}
