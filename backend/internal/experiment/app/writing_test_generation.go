package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

// WritingTestGeneration adapts opaque experiment snapshots to their owner.
// Source resolution remains generation's required owned ports, before admission.
type WritingTestGeneration struct {
	factory *generation.WritingTestFactory
	store   interface {
		GetTest(context.Context, string, string) (experiment.WritingTest, error)
		PreparedTestWork(context.Context, string, string) (experiment.TestExecutionWork, error)
	}
}

func NewWritingTestGeneration(factory *generation.WritingTestFactory, store interface {
	GetTest(context.Context, string, string) (experiment.WritingTest, error)
	PreparedTestWork(context.Context, string, string) (experiment.TestExecutionWork, error)
}) *WritingTestGeneration {
	if factory == nil || store == nil {
		panic("experiment/app: writing test generation requires factory and owned state")
	}
	return &WritingTestGeneration{factory: factory, store: store}
}

func (a *WritingTestGeneration) PrepareWritingTest(ctx context.Context, r experiment.TestStart, variants []experiment.FrozenTestVariant) (result experiment.TestPlan, err error) {
	defer func() { err = WritingTestPreparationError(err) }()
	if err := experiment.ValidateTestShape(r); err != nil {
		return experiment.TestPlan{}, err
	}
	if len(variants) != r.Count {
		return experiment.TestPlan{}, experiment.ErrTestEntrant
	}
	material := generation.WritingTestMaterialRequest{Material: r.Input.Material, Fictional: r.Input.Fictional, AttachmentIDs: append([]string(nil), r.Input.AttachmentIDs...),
		ObserveModel: llm.ModelRef{ProviderID: r.Input.ObserveModel.ProviderID, ModelID: r.Input.ObserveModel.ModelID}, WriteModel: llm.ModelRef{ProviderID: r.Input.WriteModel.ProviderID, ModelID: r.Input.WriteModel.ModelID},
		VoiceID: r.Input.VoiceID, TemplateID: r.Input.TemplateID, GuidelineSlotID: r.Input.GuidelineSlotID, TargetLanguage: generation.Language(r.Input.TargetLanguage),
		TagCount: r.Input.TagCount, UseMemory: r.Input.UseMemory, QualityRules: append([]string(nil), r.Input.QualityRules...)}
	if r.Input.TargetLength > 0 {
		length := r.Input.TargetLength
		material.TargetLength = &length
	}
	for _, v := range r.Input.TemplateAnswers {
		material.TemplateAnswers = append(material.TemplateAnswers, generation.TemplateAnswer{Label: v.Label, Text: v.Answer, Enabled: v.Enabled})
	}
	common, err := generation.EncodeWritingTestMaterialRequest(material)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	request := generation.WritingTestSnapshotRequest{UserID: r.UserID, SourcePostSlug: r.Input.SourcePostSlug, InputRevision: r.Input.InputRevision, ContentRevision: r.Input.ContentRevision,
		Factor: string(r.Factor), ModelStage: string(r.ModelStage), Count: r.Count, CommonMaterial: common}
	for index, v := range variants {
		if v.Reference != r.Entrants[index] {
			return experiment.TestPlan{}, experiment.ErrTestEntrant
		}
		raw, err := generation.EncodeWritingTestReference(toGenerationTestReference(v.Reference))
		if err != nil {
			return experiment.TestPlan{}, err
		}
		request.Variants = append(request.Variants, raw)
	}
	snapshot, err := a.factory.FreezeWritingTest(ctx, request)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	description, err := generation.DescribeWritingTestSnapshot(snapshot)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	plan := experiment.TestPlan{Snapshot: experiment.TestSnapshot{Common: snapshot.Common, Hash: snapshot.Hash, PromptVersion: snapshot.PromptVersion, AssignmentsHash: snapshot.AssignmentsHash}}
	for index, v := range description.Variants {
		plan.Snapshot.Variants = append(plan.Snapshot.Variants, experiment.FrozenTestVariant{Reference: variants[index].Reference, Content: snapshot.Variants[index], Revision: v.Revision, SemanticKey: v.SemanticKey, Synthetic: v.Synthetic, Label: variants[index].Label})
	}
	calls, err := a.factory.PlanWritingTest(snapshot)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	if err := a.factory.CheckWritingTestAccess(ctx, snapshot, calls); err != nil {
		return experiment.TestPlan{}, err
	}
	plan.Calls = fromGenerationTestCalls(calls)
	return plan, nil
}

func (a *WritingTestGeneration) PrepareFailedTestCandidates(ctx context.Context, r experiment.TestRetryQuoteRequest) (result experiment.TestPlan, err error) {
	defer func() { err = WritingTestPreparationError(err) }()
	if err := experiment.ValidateRetryQuoteShape(r); err != nil {
		return experiment.TestPlan{}, err
	}
	test, err := a.store.GetTest(ctx, r.UserID, r.TestID)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	if test.Revision != r.ExpectedRevision {
		return experiment.TestPlan{}, experiment.ErrTestRevisionConflict
	}
	work, err := a.store.PreparedTestWork(ctx, r.UserID, r.TestID)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	snapshot, err := restoreExecutionSnapshot(work)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	shared, err := decodeTestCheckpoint(work.SharedCheckpoint)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	checkpoints := map[int]generation.WritingTestCheckpoint{}
	var indices []int
	for _, id := range r.CandidateIDs {
		found := false
		for _, candidate := range test.Candidates {
			if candidate.ID == id && candidate.Status == string(experiment.TestCandidateFailed) {
				found = true
				break
			}
		}
		index, ok := work.CandidateIndices[id]
		if !found || !ok {
			return experiment.TestPlan{}, experiment.ErrTestEntrant
		}
		indices = append(indices, index)
		cp, err := decodeTestCheckpoint(work.CandidateCheckpoints[id])
		if err != nil {
			return experiment.TestPlan{}, err
		}
		if cp != nil {
			checkpoints[index] = *cp
		}
	}
	calls, err := a.factory.PlanWritingTestRetry(snapshot, shared, checkpoints, indices)
	if err != nil {
		return experiment.TestPlan{}, err
	}
	plan := work.Plan
	if len(calls) > 0 {
		if err := a.factory.CheckWritingTestAccess(ctx, snapshot, calls); err != nil {
			return experiment.TestPlan{}, err
		}
	}
	plan.Calls = fromGenerationTestCalls(calls)
	return plan, nil
}

func restoreExecutionSnapshot(work experiment.TestExecutionWork) (generation.WritingTestSnapshot, error) {
	variants := make([][]byte, len(work.Plan.Snapshot.Variants))
	for i, v := range work.Plan.Snapshot.Variants {
		variants[i] = v.Content
	}
	return generation.RestoreWritingTestSnapshot(work.Plan.Snapshot.Common, variants, work.Plan.Snapshot.Hash, work.Plan.Snapshot.PromptVersion)
}
func toGenerationTestReference(v experiment.TestEntrantRef) generation.WritingTestReference {
	return generation.WritingTestReference{SourceKind: v.SourceKind, Model: llm.ModelRef{ProviderID: v.Model.ProviderID, ModelID: v.Model.ModelID}, SettingKind: v.SettingKind, SettingID: v.SettingID, SettingRevision: v.SettingRevision,
		AuthoringSessionID: v.AuthoringSessionID, AuthoringCandidateID: v.AuthoringCandidateID, AuthoringRevision: v.AuthoringRevision}
}
func fromGenerationTestCalls(values []generation.WritingTestCall) []experiment.TestCall {
	out := make([]experiment.TestCall, len(values))
	for i, v := range values {
		out[i] = experiment.TestCall{Ref: experiment.ModelRef{ProviderID: v.Ref.ProviderID, ModelID: v.Ref.ModelID}, Stage: experiment.Stage(v.Stage), Count: v.Count, PromptTokens: v.PromptTokens, CompletionTokens: v.CompletionTokens}
	}
	return out
}

type testCheckpointWire struct {
	Version    int                              `json:"version"`
	Checkpoint generation.WritingTestCheckpoint `json:"checkpoint"`
}

func encodeTestCheckpoint(v generation.WritingTestCheckpoint) ([]byte, error) {
	return json.Marshal(testCheckpointWire{Version: 1, Checkpoint: v})
}
func decodeTestCheckpoint(raw []byte) (*generation.WritingTestCheckpoint, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > 8<<20 {
		return nil, generation.ErrWritingTestCheckpointInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var w testCheckpointWire
	if err := d.Decode(&w); err != nil {
		return nil, generation.ErrWritingTestCheckpointInvalid
	}
	if w.Version != 1 || d.Decode(new(any)) != io.EOF {
		return nil, generation.ErrWritingTestCheckpointInvalid
	}
	return &w.Checkpoint, nil
}

func (a *WritingTestGeneration) PrepareTestInput(ctx context.Context, work experiment.TestExecutionWork, save func(context.Context, []byte) error, progress experiment.Progress) ([]byte, error) {
	snapshot, err := restoreExecutionSnapshot(work)
	if err != nil {
		return nil, err
	}
	cp, err := decodeTestCheckpoint(work.SharedCheckpoint)
	if err != nil {
		return nil, err
	}
	result, err := a.factory.PrepareWritingTestInput(ctx, snapshot, generation.WritingTestRunOptions{Checkpoint: cp, RetryFailed: true, Progress: generation.Progress(progress), SaveCheckpoint: func(ctx context.Context, v generation.WritingTestCheckpoint) error {
		raw, err := encodeTestCheckpoint(v)
		if err != nil {
			return err
		}
		return save(ctx, raw)
	}})
	if err != nil {
		return nil, err
	}
	return encodeTestCheckpoint(result.Checkpoint)
}
func (a *WritingTestGeneration) RunTestCandidate(ctx context.Context, work experiment.TestExecutionWork, id string, shared []byte, save func(context.Context, []byte) error, progress experiment.Progress) (experiment.TestExecutionResult, error) {
	snapshot, err := restoreExecutionSnapshot(work)
	if err != nil {
		return experiment.TestExecutionResult{}, err
	}
	index, ok := work.CandidateIndices[id]
	if !ok {
		return experiment.TestExecutionResult{}, experiment.ErrTestEntrant
	}
	cp, err := decodeTestCheckpoint(work.CandidateCheckpoints[id])
	if err != nil {
		return experiment.TestExecutionResult{}, err
	}
	common, err := decodeTestCheckpoint(shared)
	if err != nil {
		return experiment.TestExecutionResult{}, err
	}
	started := time.Now()
	result, runErr := a.factory.RunWritingTestCandidate(ctx, snapshot, index, common, generation.WritingTestRunOptions{Checkpoint: cp, RetryFailed: true, Progress: generation.Progress(progress), SaveCheckpoint: func(ctx context.Context, v generation.WritingTestCheckpoint) error {
		raw, err := encodeTestCheckpoint(v)
		if err != nil {
			return err
		}
		return save(ctx, raw)
	}})
	accounting := experiment.Usage{PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, CostMicrousd: result.Usage.CostMicrousd, LatencyMS: time.Since(started).Milliseconds(), CostSource: experiment.CostEstimated}
	var priorUsage *experiment.Usage
	for _, c := range work.Test.Candidates {
		if c.ID == id && c.Usage != nil {
			priorUsage = c.Usage
			accounting.PromptTokens += c.Usage.PromptTokens
			accounting.CompletionTokens += c.Usage.CompletionTokens
			accounting.CostMicrousd += c.Usage.CostMicrousd
			accounting.LatencyMS += c.Usage.LatencyMS
		}
	}
	if result.Usage.CostReported {
		accounting.CostSource = experiment.CostReported
	}
	accountRaw, err := json.Marshal(accounting)
	if result.Replayed {
		// Recovered paid checkpoints carry an answer but may have lost their
		// candidate-level accounting write. That usage is unknown here; the
		// separate original job ledger still owns its confirmed debit.
		if priorUsage == nil {
			accountRaw = nil
		} else {
			accountRaw, err = json.Marshal(priorUsage)
		}
	}
	out := experiment.TestExecutionResult{Accounting: accountRaw}
	if runErr != nil || err != nil {
		return out, errors.Join(runErr, err)
	}
	value := experiment.TestOutput{ContentLanguage: string(result.ContentLanguage), Nouns: result.Answer.Nouns, Content: experiment.TestOutputContent{Title: result.Answer.Content.Title, Summary: result.Answer.Content.Summary, Tags: result.Answer.Content.Tags}}
	for _, b := range result.Answer.Content.Blocks {
		value.Content.Blocks = append(value.Content.Blocks, experiment.TestOutputBlock{Type: string(b.Type), Content: b.Content, Level: b.Level, File: b.File, Alt: b.Alt, Caption: b.Caption, Items: b.Items, Files: b.Files, Layout: b.Layout})
	}
	if result.Answer.Storyline != nil {
		value.Storyline = &experiment.TestOutputStoryline{MadeWith: result.Answer.Storyline.MadeWith}
		for _, p := range result.Answer.Storyline.Paragraphs {
			value.Storyline.Paragraphs = append(value.Storyline.Paragraphs, experiment.TestOutputParagraph{Text: p.Text, Files: p.Files})
		}
	}
	out.Output, err = experiment.EncodeTestOutput(value)
	return out, err
}
