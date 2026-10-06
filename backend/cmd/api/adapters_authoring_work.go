package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/usage"
)

type authoringModels struct {
	registry meteredRegistry
	dispatch voiceCandidateDispatch
}

func (a authoringModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return a.registry.Lookup(ref)
}
func (a authoringModels) Complete(ctx context.Context, ref llm.ModelRef, in llm.Request) (llm.Response, error) {
	work, ok := usage.WorkFromContext(ctx)
	if !ok || work.Kind != authoring.JobKind {
		return llm.Response{}, job.ErrDispatchRefused
	}
	if err := a.dispatch.AuthorizeDispatch(ctx, work.UserID, work.JobID); err != nil {
		return llm.Response{}, err
	}
	if err := ctx.Err(); err != nil {
		return llm.Response{}, err
	}
	return a.registry.Complete(ctx, ref, in)
}

type authoringJobs struct{ queue *job.Queue }

func (a authoringJobs) Enqueue(ctx context.Context, in authoring.JobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: authoring.JobKind, UserID: in.UserID, WriteModel: in.WriteModel, Payload: in.Payload, CancellationPolicyVersion: 1, PricingCalls: []job.PlannedCall{{Ref: in.WriteModel, Stage: llm.StageNameWrite, Count: 1, CompletionTokens: in.CompletionTokens, PromptTokens: in.PromptTokens}}})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", authoring.ErrBusy
	}
	return id, err
}
func (a authoringJobs) Get(ctx context.Context, user, id string) (authoring.Job, error) {
	found, err := a.queue.Result(ctx, user, id)
	if errors.Is(err, job.ErrNotFound) {
		return authoring.Job{}, authoring.ErrNotFound
	}
	if err != nil {
		return authoring.Job{}, err
	}
	if found.Kind != authoring.JobKind || len(found.Subjects) != 0 {
		return authoring.Job{}, authoring.ErrNotFound
	}
	reason := ""
	if found.Failure != nil {
		reason = found.Failure.Reason
	}
	return authoring.Job{ID: found.ID, Status: found.Status, WriteModel: found.WriteModel, Payload: found.Payload, FailureReason: reason}, nil
}
func (a authoringJobs) Latest(ctx context.Context, user string) (*authoring.Job, error) {
	found, err := a.queue.LatestOwnedKind(ctx, user, authoring.JobKind, "")
	if err != nil || found == nil {
		return nil, err
	}
	reason := ""
	if found.Failure != nil {
		reason = found.Failure.Reason
	}
	return &authoring.Job{ID: found.ID, Status: found.Status, WriteModel: found.WriteModel, Payload: found.Payload, FailureReason: reason}, nil
}
func (a authoringJobs) SaveResult(ctx context.Context, id string, payload []byte) error {
	return a.queue.SaveResult(ctx, id, payload)
}
func (a authoringJobs) Cancel(ctx context.Context, user, id string) error {
	if _, err := a.Get(ctx, user, id); err != nil {
		return err
	}
	_, err := a.queue.CancelOwned(ctx, user, id)
	if errors.Is(err, job.ErrNotFound) {
		return authoring.ErrNotFound
	}
	return err
}

type authoringBudget struct{ config config.LLMCompletionBudget }

func (a authoringBudget) CompletionCap(kind authoring.Kind, mode authoring.Mode, chars int, native bool) int {
	if mode == authoring.Recommend {
		chars = authoring.RecommendationOutputChars(kind)
	}
	return a.config.Revise(chars, nil, native)
}

type authoringEstimates struct{ rates estimateRates }

func (a authoringEstimates) CallCredits(ctx context.Context, info llm.ModelInfo, prompt, completion int64) (int, bool) {
	return (templateEstimates{rates: a.rates}).CallCredits(ctx, info, usage.HoldPromptTokenBound(prompt), completion)
}

var _ authoring.Models = authoringModels{}
var _ authoring.Jobs = authoringJobs{}
var _ authoring.Budget = authoringBudget{}
var _ authoring.Estimator = authoringEstimates{}
