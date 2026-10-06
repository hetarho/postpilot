package main

import (
	"context"
	"errors"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice"
)

type voiceCandidateDispatch interface {
	AuthorizeDispatch(context.Context, string, string) error
}

// The conditional dispatch writer serializes cancellation with the only provider call.
// The metered registry then uses the same frozen admission as every paid completion.
type voiceCandidateModels struct {
	registry meteredRegistry
	dispatch voiceCandidateDispatch
}

func (a voiceCandidateModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return a.registry.Lookup(ref)
}
func (a voiceCandidateModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	work, ok := usage.WorkFromContext(ctx)
	if !ok || work.Kind != job.KindWritingVoiceCandidates {
		return llm.Response{}, job.ErrDispatchRefused
	}
	if err := a.dispatch.AuthorizeDispatch(ctx, work.UserID, work.JobID); err != nil {
		return llm.Response{}, err
	}
	if err := ctx.Err(); err != nil {
		return llm.Response{}, err
	}
	return a.registry.Complete(ctx, ref, request)
}

type voiceCandidateJobs struct{ queue *job.Queue }

func (a voiceCandidateJobs) EnqueueCandidates(ctx context.Context, in voice.CandidateJobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: job.KindWritingVoiceCandidates, UserID: in.UserID, WriteModel: in.WriteModel, Payload: in.Payload, CancellationPolicyVersion: 1, PricingCalls: []job.PlannedCall{{Ref: in.WriteModel, Stage: llm.StageNameWrite, Count: 1, CompletionTokens: in.CompletionTokens, PromptTokens: in.PromptTokens}}})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.CandidatesRunningError{ActiveID: active.ActiveID}
	}
	return id, err
}
func (a voiceCandidateJobs) CandidateResult(ctx context.Context, userID, id string) (voice.CandidateJob, error) {
	found, err := a.queue.Result(ctx, userID, id)
	if errors.Is(err, job.ErrNotFound) {
		return voice.CandidateJob{}, voice.ErrCandidateNotFound
	}
	if err != nil {
		return voice.CandidateJob{}, err
	}
	if found.Kind != job.KindWritingVoiceCandidates || len(found.Subjects) != 0 {
		return voice.CandidateJob{}, voice.ErrCandidateNotFound
	}
	return voice.CandidateJob{ID: found.ID, Status: found.Status, WriteModel: found.WriteModel, Payload: found.Payload}, nil
}
func (a voiceCandidateJobs) LatestCandidates(ctx context.Context, userID, status string) (*voice.CandidateJob, error) {
	found, err := a.queue.LatestOwnedKind(ctx, userID, job.KindWritingVoiceCandidates, status)
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.CandidateJob{ID: found.ID, Status: found.Status, WriteModel: found.WriteModel, Payload: found.Payload}, nil
}
func (a voiceCandidateJobs) SaveCandidateResult(ctx context.Context, id string, payload []byte) error {
	return a.queue.SaveResult(ctx, id, payload)
}
func (a voiceCandidateJobs) CancelCandidates(ctx context.Context, userID, id string) error {
	if _, err := a.CandidateResult(ctx, userID, id); err != nil {
		return err
	}
	_, err := a.queue.CancelOwned(ctx, userID, id)
	if errors.Is(err, job.ErrNotFound) {
		return voice.ErrCandidateNotFound
	}
	return err
}

var _ voice.CandidateJobs = voiceCandidateJobs{}
var _ voice.Models = voiceCandidateModels{}

// Candidate estimates cover the same conservative prompt reservation as admission.
// The existing catalog estimator's edit allowance makes the displayed figure conservative.
type voiceCandidateEstimates struct{ rates estimateRates }

func (a voiceCandidateEstimates) CallCredits(ctx context.Context, info llm.ModelInfo, promptTokens, completionTokens int64) (int, bool) {
	return (templateEstimates{rates: a.rates}).CallCredits(ctx, info, usage.HoldPromptTokenBound(promptTokens), completionTokens)
}
