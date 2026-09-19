package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/voice"
)

type experimentVoices struct{ service *voice.Service }

func (a experimentVoices) ActiveVoice(ctx context.Context, userID, voiceID string) error {
	found, err := a.service.GetVoice(ctx, userID, voiceID)
	switch {
	case errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		return experiment.ErrVoiceUnavailable
	case err != nil:
		return err
	case found.Deleted():
		return experiment.ErrVoiceUnavailable
	}
	return nil
}

type experimentJobs struct{ queue *job.Queue }

func (a experimentJobs) EnqueueExperiment(ctx context.Context, request experiment.JobRequest) (string, error) {
	targetLanguage := ""
	if request.TargetLanguage != nil {
		targetLanguage = request.TargetLanguage.String()
	}
	subjects, guards := postVoiceWork(job.KindModelExperiment, request.UserID, request.PostSlug, request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindModelExperiment, UserID: request.UserID, Subjects: subjects, Guards: guards,
		TargetLanguage: targetLanguage, Payload: []byte(request.ExperimentID), ExtraModels: request.Models,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &experiment.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", experiment.ErrVoiceUnavailable
	}
	return id, err
}

func (a experimentJobs) HasRunnableExperiment(ctx context.Context, experimentID string) (bool, error) {
	return a.queue.HasActiveFor(ctx, job.Subject{Dimension: experiment.JobSubject, ID: experimentID}, job.Filter{})
}

// postExperiments reaches the experiment context from post and generation, both of which
// are constructed before it; the lookup happens at call time, after buildContexts has
// finished, so the adapter names where the service will be rather than holding a nil.
type postExperiments struct{ app *contexts }

func (a postExperiments) service() *experiment.Service { return a.app.experiment }

func (a postExperiments) PendingForPost(ctx context.Context, userID, slug string) (string, error) {
	found, err := a.service().PendingForPost(ctx, userID, slug)
	if err != nil || found == nil {
		return "", err
	}
	return found.ID, nil
}

func (a postExperiments) PurgePost(ctx context.Context, userID, slug string) error {
	return a.service().PurgePost(ctx, userID, slug)
}

type experimentCatalog struct {
	selections *provider.Service
	registry   meteredRegistry
	plans      *auth.Service
}

func (a experimentCatalog) Resolve(ref experiment.ModelRef) (experiment.Model, bool) {
	info, ok := a.registry.Lookup(llmRef(ref))
	if !ok {
		return experiment.Model{}, false
	}
	return experiment.Model{
		Ref: ref, Label: info.Label, Vision: info.Vision, Enabled: !info.Disabled,
		Stages:             info.Stages,
		InputUSDPerMillion: info.InputUSDPerMillion, OutputUSDPerMillion: info.OutputUSDPerMillion,
	}, true
}

func (a experimentCatalog) Adopt(ctx context.Context, userID string, stage experiment.Stage, ref experiment.ModelRef) error {
	_, err := a.selections.SaveSelection(ctx, userID, provider.Stage(stage), llmRef(ref))
	return err
}

func (a experimentCatalog) Active(ctx context.Context, userID string, stage experiment.Stage) (experiment.ModelRef, bool, error) {
	selections, err := a.selections.GetSelections(ctx, userID)
	if err != nil {
		return experiment.ModelRef{}, false, err
	}
	for _, selection := range selections {
		if string(selection.Stage) == string(stage) && !selection.Missing {
			return experiment.ModelRef{ProviderID: selection.Ref.ProviderID, ModelID: selection.Ref.ModelID}, true, nil
		}
	}
	return experiment.ModelRef{}, false, nil
}

func (a experimentCatalog) Recommended(stage experiment.Stage, ref experiment.ModelRef) bool {
	for _, set := range a.registry.RecommendationSets() {
		for _, selection := range set.Selections {
			if selection.Stage != string(stage) {
				continue
			}
			candidate := llmRef(ref)
			if selection.Active == candidate || selection.CandidateA == candidate || selection.CandidateB == candidate {
				return true
			}
		}
	}
	return false
}

type experimentRunner struct {
	generation *generation.Service
	voice      *voice.Service
}

func (a experimentRunner) Snapshot(ctx context.Context, request experiment.StartRequest) (experiment.Snapshot, error) {
	switch request.Stage {
	case experiment.StageWrite:
		content, err := a.generation.SnapshotWriteInput(ctx, request.UserID, request.PostSlug, llmRef(request.ObserveModel), request.TargetLength, request.ObserveFiles)
		targetLanguage := experiment.Language(generation.SnapshotTargetLanguage(content))
		var frozenTarget *experiment.Language
		if targetLanguage.Valid() {
			frozenTarget = &targetLanguage
		}
		return experiment.Snapshot{
			Content: content, PromptVersion: generation.WriteExperimentPromptVersion,
			VoiceID: generation.SnapshotVoice(content), TemplateName: generation.SnapshotTemplateName(content), TargetLanguage: frozenTarget,
		}, mapSnapshotError(err)
	case experiment.StageObserve:
		content, err := a.generation.SnapshotObserveInput(ctx, request.UserID, request.PostSlug)
		return experiment.Snapshot{Content: content, PromptVersion: generation.ObserveExperimentPromptVersion}, mapSnapshotError(err)
	case experiment.StageAnalyze:
		content, err := a.voice.SnapshotAnalysisInput(ctx, request.UserID, request.VoiceID)
		return experiment.Snapshot{Content: content, PromptVersion: voice.AnalyzeExperimentPromptVersion, VoiceID: request.VoiceID}, experimentVoiceError(err)
	default:
		return experiment.Snapshot{}, experiment.ErrInvalidStage
	}
}

func (a experimentRunner) PrepareWrite(ctx context.Context, found experiment.Experiment, progress experiment.Progress) (experiment.Snapshot, error) {
	content, err := a.generation.PrepareWriteInput(ctx, found.InputSnapshot, generation.Progress(progress))
	return experiment.Snapshot{Content: content, PromptVersion: generation.WriteExperimentPromptVersion, VoiceID: found.VoiceID, TemplateName: found.TemplateName, TargetLanguage: cloneExperimentLanguage(found.TargetLanguage)}, mapSnapshotError(err)
}

func cloneExperimentLanguage(value *experiment.Language) *experiment.Language {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (a experimentRunner) RunCandidate(ctx context.Context, found experiment.Experiment, candidate experiment.Candidate, progress experiment.Progress) (experiment.CandidateResult, error) {
	ref := llmRef(candidate.Model)
	switch found.Stage {
	case experiment.StageWrite:
		content, usage, err := a.generation.RunWriteCandidate(ctx, found.InputSnapshot, ref)
		encoded, encodeErr := json.Marshal(toOutputPost(content))
		if err == nil {
			err = encodeErr
		}
		return experiment.CandidateResult{Output: encoded, Usage: experimentUsage(usage.PromptTokens, usage.CompletionTokens, usage.CostMicrousd, usage.CostReported)}, err
	case experiment.StageObserve:
		// Candidate progress is emitted once by the experiment coordinator. Forwarding
		// each vision pipeline's batch progress would race two incompatible counters.
		observations, usage, err := a.generation.RunObserveCandidate(ctx, found.InputSnapshot, ref, func(string, int, int) {})
		encoded, encodeErr := json.Marshal(toOutputObservations(observations))
		if err == nil {
			err = encodeErr
		}
		return experiment.CandidateResult{Output: encoded, Usage: experimentUsage(usage.PromptTokens, usage.CompletionTokens, usage.CostMicrousd, usage.CostReported)}, mapSnapshotError(err)
	case experiment.StageAnalyze:
		styleguide, usage, err := a.voice.RunAnalyzeCandidate(ctx, found.InputSnapshot, ref)
		encoded, encodeErr := json.Marshal(styleguide)
		if err == nil {
			err = encodeErr
		}
		return experiment.CandidateResult{Output: encoded, Usage: experimentUsage(usage.PromptTokens, usage.CompletionTokens, usage.CostMicrousd, usage.CostReported)}, err
	default:
		return experiment.CandidateResult{}, experiment.ErrInvalidStage
	}
}

func (a experimentRunner) ApplyWinner(ctx context.Context, found experiment.Experiment, candidate experiment.Candidate, confirmStyleguide bool) error {
	switch found.Stage {
	case experiment.StageWrite:
		var value outputPost
		if err := json.Unmarshal(candidate.Output, &value); err != nil {
			return fmt.Errorf("decode write winner: %w", err)
		}
		return mapSnapshotError(a.generation.ApplyWriteWinner(ctx, found.UserID, found.PostSlug, fromOutputPost(value), found.InputSnapshot))
	case experiment.StageObserve:
		var values []outputObservation
		if err := json.Unmarshal(candidate.Output, &values); err != nil {
			return fmt.Errorf("decode observation winner: %w", err)
		}
		return a.generation.ApplyObservationWinner(ctx, found.UserID, found.PostSlug, fromOutputObservations(values))
	case experiment.StageAnalyze:
		if !confirmStyleguide {
			return experiment.ErrConfirmationRequired
		}
		var styleguide string
		if err := json.Unmarshal(candidate.Output, &styleguide); err != nil {
			return fmt.Errorf("decode styleguide winner: %w", err)
		}
		return experimentVoiceError(a.voice.ApplyStyleguideWinner(ctx, found.UserID, found.VoiceID, styleguide))
	default:
		return experiment.ErrInvalidStage
	}
}

type outputPost struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Tags    []string      `json:"tags"`
	Blocks  []outputBlock `json:"blocks"`
}
type outputBlock struct {
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Level   int32    `json:"level"`
	File    string   `json:"file"`
	Alt     string   `json:"alt"`
	Caption string   `json:"caption"`
	Items   []string `json:"items"`
}
type outputObservation struct {
	File          string   `json:"file"`
	Scene         string   `json:"scene"`
	Mood          string   `json:"mood"`
	VisibleText   string   `json:"visible_text"`
	Objects       []string `json:"objects"`
	PeoplePresent bool     `json:"people_present"`
	// Carried so applying an observation A/B winner does not blank the provenance of the
	// snapshot it replaces — which would make the next picker report every photo as observed
	// by an unrecorded model.
	Model string `json:"model,omitempty"`
}

func toOutputPost(content generation.PostContent) outputPost {
	out := outputPost{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		out.Blocks = append(out.Blocks, outputBlock{Type: string(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return out
}
func fromOutputPost(value outputPost) generation.PostContent {
	out := generation.PostContent{Title: value.Title, Summary: value.Summary, Tags: value.Tags}
	for _, block := range value.Blocks {
		out.Blocks = append(out.Blocks, generation.Block{Type: generation.BlockType(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return out
}
func toOutputObservations(values []generation.Observation) []outputObservation {
	out := make([]outputObservation, 0, len(values))
	for _, value := range values {
		out = append(out, outputObservation{File: value.File, Scene: value.Scene, Mood: value.Mood, VisibleText: value.VisibleText, Objects: value.Objects, PeoplePresent: value.PeoplePresent, Model: value.Model})
	}
	return out
}
func fromOutputObservations(values []outputObservation) []generation.Observation {
	out := make([]generation.Observation, 0, len(values))
	for _, value := range values {
		out = append(out, generation.Observation{File: value.File, Scene: value.Scene, Mood: value.Mood, VisibleText: value.VisibleText, Objects: value.Objects, PeoplePresent: value.PeoplePresent, Model: value.Model})
	}
	return out
}
func experimentUsage(prompt, completion, cost int64, reported bool) experiment.UsageReport {
	return experiment.UsageReport{PromptTokens: prompt, CompletionTokens: completion, CostMicrousd: cost, CostReported: reported}
}
func experimentRef(value string) experiment.ModelRef {
	ref := parseLLMRef(value)
	return experiment.ModelRef{ProviderID: ref.ProviderID, ModelID: ref.ModelID}
}
func llmRef(ref experiment.ModelRef) llm.ModelRef {
	return llm.ModelRef{ProviderID: ref.ProviderID, ModelID: ref.ModelID}
}
func parseLLMRef(value string) llm.ModelRef {
	providerID, modelID, _ := strings.Cut(value, "/")
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}
}
func mapSnapshotError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, generation.ErrVideoUnsupported):
		var unsupported *generation.VideoUnsupportedError
		if errors.As(err, &unsupported) {
			return &experiment.VideoUnsupportedError{Model: unsupported.Model}
		}
		return experiment.ErrVideoUnsupported
	case errors.Is(err, generation.ErrVoiceDeleted), errors.Is(err, generation.ErrVoiceMismatch), errors.Is(err, generation.ErrVoiceRequired):
		return experiment.ErrVoiceUnavailable
	case strings.Contains(err.Error(), "read photo"):
		return experiment.ErrSnapshotUnavailable
	}
	return err
}

func experimentVoiceError(err error) error {
	switch {
	case errors.Is(err, voice.ErrVoiceDeleted), errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		return experiment.ErrVoiceUnavailable
	default:
		return err
	}
}

// creditBootstrap gives a freshly provisioned account the credits its tier is granted,
// so it can spend from its first request rather than from whichever one happens to renew
// it. It is idempotent for the same reason the voice bootstrap is: `adduser` may be rerun
// to repair an account, and a repair must not mint a second monthly lot.
