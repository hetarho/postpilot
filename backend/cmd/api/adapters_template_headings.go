package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type templateHeadingVoice struct{ service *voice.Service }

func (a templateHeadingVoice) DefaultVoiceStyle(ctx context.Context, user string) (string, bool, error) {
	voices, err := a.service.ListVoices(ctx, user)
	if err != nil {
		return "", false, err
	}
	for _, found := range voices {
		if !found.IsDefault || found.Deleted() || !found.Made {
			continue
		}
		profile, err := a.service.PromptProfileForTopic(ctx, user, found.ID, "", voice.LanguageKorean, "")
		if err != nil {
			return "", false, err
		}
		// No sample/excerpt is passed: this action needs style alone, never post material.
		return profile.Text, true, nil
	}
	return "", false, nil
}

type templateHeadingJobs struct{ queue *job.Queue }

func (a templateHeadingJobs) EnqueueHeadingSuggestions(ctx context.Context, request template.HeadingSuggestionJob) (string, error) {
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: template.HeadingSuggestionJobKind, UserID: request.UserID, WriteModel: request.WriteModel, Payload: request.Payload, CancellationPolicyVersion: templateRequestCancellationPolicy,
		PricingCalls: []job.PlannedCall{{Ref: request.WriteModel, Stage: llm.StageNameWrite, Count: request.Calls, CompletionTokens: request.CompletionTokens}},
	})
	if errors.Is(err, job.ErrActiveConflict) {
		return "", template.ErrRequestRunning
	}
	return id, err
}

func (a templateHeadingJobs) SaveHeadingSuggestions(ctx context.Context, id string, payload []byte) error {
	return a.queue.SaveResult(ctx, id, payload)
}

func (a templateHeadingJobs) HeadingSuggestionsPayload(ctx context.Context, user, id string) ([]byte, error) {
	found, err := a.queue.Result(ctx, user, id)
	if errors.Is(err, job.ErrNotFound) || errors.Is(err, job.ErrForbidden) {
		return nil, template.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if found.Kind != template.HeadingSuggestionJobKind {
		return nil, template.ErrNotFound
	}
	if found.Status != job.StatusDone {
		return nil, template.ErrRequestNotReady
	}
	return found.Payload, nil
}
