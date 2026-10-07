package generation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/postpilot/backend/internal/llm"
)

// PrepareWritingTestInput stores shared observations only through the test-owned callback.
// The source post is never read or written during execution of a frozen test.
func (f *WritingTestFactory) PrepareWritingTestInput(ctx context.Context, snapshot WritingTestSnapshot, options WritingTestRunOptions) (WritingTestRunResult, error) {
	if err := ctx.Err(); err != nil {
		return WritingTestRunResult{}, err
	}
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		return WritingTestRunResult{}, err
	}
	if err := checkWritingTestVersions(common); err != nil {
		return WritingTestRunResult{}, err
	}
	if options.SaveCheckpoint == nil {
		return WritingTestRunResult{}, ErrWritingTestCheckpointRequired
	}
	targets, seed := frozenObserveSelection(common.Post.Images, common.ObserveFiles, common.Observations)
	if common.Prepared || observerWritingTest(common) {
		targets = nil
		seed = common.Observations
	}
	batches := writingTestBatches(targets, common.BatchSize)
	checkpoint := writingTestInitialCheckpoint(snapshot, common, -1, nil, seed)
	if options.Checkpoint != nil {
		checkpoint, err = acceptWritingTestCheckpoint(checkpoint, *options.Checkpoint, batches)
		if err != nil {
			return WritingTestRunResult{}, err
		}
	}
	result := WritingTestRunResult{Checkpoint: checkpoint}
	if checkpoint.Prepared {
		if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(checkpoint)); err != nil {
			return result, err
		}
		result.Replayed = true
		return result, nil
	}
	if err := writingTestResumeAllowed(checkpoint, options.RetryFailed); err != nil {
		return result, err
	}
	// A currently unusable writer refuses before paying for shared eyesight.
	for _, variant := range variants {
		if err := f.checkWritingTestWriter(ctx, common.Post.UserID, variant); err != nil {
			return result, err
		}
	}
	if len(batches) > checkpoint.CompletedObserveCalls {
		if common.ObservePromptTokens <= 0 || common.ObserveCompletionTokens <= 0 {
			return result, ErrWritingTestMaterial
		}
		if err := f.checkWritingTestObserver(ctx, common.Post.UserID, common.ObserveModel, targets, common.ObserveStructuredOutput); err != nil {
			return result, err
		}
	}
	worker := f.writingTestWorker(common, nil)
	return runWritingTestObservation(ctx, worker, common.Post, batches, checkpoint, options)
}

// RunWritingTestCandidate reuses the shared checkpoint for every non-observer factor.
// Observer entrants have independent checkpoints and always observe their own identical input.
func (f *WritingTestFactory) RunWritingTestCandidate(ctx context.Context, snapshot WritingTestSnapshot, index int, shared *WritingTestCheckpoint, options WritingTestRunOptions) (WritingTestRunResult, error) {
	if err := ctx.Err(); err != nil {
		return WritingTestRunResult{}, err
	}
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		return WritingTestRunResult{}, err
	}
	if err := checkWritingTestVersions(common); err != nil {
		return WritingTestRunResult{}, err
	}
	if options.SaveCheckpoint == nil {
		return WritingTestRunResult{}, ErrWritingTestCheckpointRequired
	}
	if index < 0 || index >= len(variants) {
		return WritingTestRunResult{}, ErrWritingTestReference
	}
	variant := variants[index]
	post := variant.Snapshot.Post
	var targets []Image
	var observations []Observation
	if observerWritingTest(common) {
		targets = post.Images
	} else {
		observations = common.Observations
		commonTargets, commonSeed := frozenObserveSelection(common.Post.Images, common.ObserveFiles, common.Observations)
		if !common.Prepared && len(commonTargets) > 0 {
			if shared == nil {
				return WritingTestRunResult{}, ErrWritingTestPreparationRequired
			}
			expected := writingTestInitialCheckpoint(snapshot, common, -1, nil, commonSeed)
			checked, err := acceptWritingTestCheckpoint(expected, *shared, writingTestBatches(commonTargets, common.BatchSize))
			if err != nil {
				return WritingTestRunResult{}, err
			}
			if !checked.Prepared || checked.InFlightStage != "" || checked.FailedStage != "" {
				return WritingTestRunResult{}, ErrWritingTestPreparationRequired
			}
			observations = checked.Observations
		}
	}
	batches := writingTestBatches(targets, common.BatchSize)
	checkpoint := writingTestInitialCheckpoint(snapshot, common, index, &variant, observations)
	if options.Checkpoint != nil {
		checkpoint, err = acceptWritingTestCheckpoint(checkpoint, *options.Checkpoint, batches)
		if err != nil {
			return WritingTestRunResult{}, err
		}
	}
	result := WritingTestRunResult{Checkpoint: checkpoint}
	if checkpoint.Answer != nil {
		if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(checkpoint)); err != nil {
			return result, err
		}
		result.Answer = cloneWritingTestAnswer(*checkpoint.Answer)
		result.ContentLanguage = post.TargetLanguage
		result.Replayed = true
		return result, nil
	}
	if err := writingTestResumeAllowed(checkpoint, options.RetryFailed); err != nil {
		return result, err
	}
	if err := f.checkWritingTestWriter(ctx, common.Post.UserID, variant); err != nil {
		return result, err
	}
	if len(batches) > checkpoint.CompletedObserveCalls {
		if variant.ObservePromptTokens <= 0 || variant.ObserveCompletionTokens <= 0 {
			return result, ErrWritingTestMaterial
		}
		if err := f.checkWritingTestObserver(ctx, common.Post.UserID, variant.ObserveModel, targets, variant.ObserveStructuredOutput); err != nil {
			return result, err
		}
	}
	worker := f.writingTestWorker(common, &variant)
	if !checkpoint.Prepared {
		result, err = runWritingTestObservation(ctx, worker, post, batches, checkpoint, options)
		if err != nil {
			return result, err
		}
		checkpoint = result.Checkpoint
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	checkpoint.InFlightStage, checkpoint.FailedStage = llm.StageNameWrite, ""
	if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(checkpoint)); err != nil {
		result.Checkpoint = checkpoint
		return result, err
	}
	result.Checkpoint = checkpoint
	progress := writingTestProgress(options.Progress)
	progress("write", 0, 1)
	ctx = withWritingTestRequestCapture(ctx, &checkpoint, options.SaveCheckpoint)
	post.Images = observedImages(post.Images, checkpoint.Observations)
	// Every test entrant returns its direct-path storyline; it never follows or updates a
	// source post's existing storyline.
	post.FollowStoryline = nil
	answer, usage, callErr := worker.writeCandidate(ctx, post, variant.Snapshot.Profile, checkpoint.Observations, variant.WriteModel)
	addWritingTestUsage(&result.Usage, usage)
	settled := cloneWritingTestCheckpoint(checkpoint)
	settled.InFlightStage = ""
	if callErr != nil {
		settled.FailedStage = llm.StageNameWrite
	} else {
		settled.Answer = &answer
	}
	if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(settled)); err != nil {
		return result, err
	}
	result.Checkpoint = settled
	if callErr != nil {
		return result, callErr
	}
	result.Answer, result.ContentLanguage = cloneWritingTestAnswer(answer), post.TargetLanguage
	progress("write", 1, 1)
	return result, nil
}

func (f *WritingTestFactory) WriteTestEntrant(ctx context.Context, snapshot WritingTestSnapshot, index int, shared *WritingTestCheckpoint, options WritingTestRunOptions) (WriteAnswer, error) {
	result, err := f.RunWritingTestCandidate(ctx, snapshot, index, shared, options)
	return result.Answer, err
}

func runWritingTestObservation(ctx context.Context, worker *Service, post PostInput, batches [][]Image, checkpoint WritingTestCheckpoint, options WritingTestRunOptions) (WritingTestRunResult, error) {
	result := WritingTestRunResult{Checkpoint: checkpoint}
	ctx = withWritingTestRequestCapture(ctx, &checkpoint, options.SaveCheckpoint)
	progress := writingTestProgress(options.Progress)
	total, done := 0, 0
	for index, batch := range batches {
		total += len(batch)
		if index < checkpoint.CompletedObserveCalls {
			done += len(batch)
		}
	}
	progress("observe", done, total)
	for index := checkpoint.CompletedObserveCalls; index < len(batches); index++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		checkpoint.InFlightStage, checkpoint.FailedStage = llm.StageNameObserve, ""
		if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(checkpoint)); err != nil {
			result.Checkpoint = checkpoint
			return result, err
		}
		result.Checkpoint = checkpoint
		observations, usage, callErr := worker.observeCandidate(ctx, post, batches[index], checkpoint.Observations, checkpoint.ObserveModel, func(string, int, int) {}, false)
		addWritingTestUsage(&result.Usage, usage)
		settled := cloneWritingTestCheckpoint(checkpoint)
		settled.InFlightStage = ""
		if callErr != nil {
			settled.FailedStage = llm.StageNameObserve
		} else {
			settled.Observations, settled.CompletedObserveCalls = observations, index+1
			settled.Prepared = index+1 == len(batches)
		}
		if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(settled)); err != nil {
			return result, err
		}
		result.Checkpoint, checkpoint = settled, settled
		if callErr != nil {
			return result, callErr
		}
		done += len(batches[index])
		progress("observe", done, total)
	}
	if !checkpoint.Prepared {
		checkpoint.Prepared = true
		if err := options.SaveCheckpoint(ctx, cloneWritingTestCheckpoint(checkpoint)); err != nil {
			return result, err
		}
		result.Checkpoint = checkpoint
	}
	return result, nil
}

func observerWritingTest(common writingTestCommon) bool {
	return common.Factor == "model" && common.ModelStage == llm.StageNameObserve
}

func writingTestBatches(targets []Image, size int) [][]Image {
	if size <= 0 {
		return nil
	}
	photos := photosOf(targets)
	var batches [][]Image
	for start := 0; start < len(photos); start += size {
		batches = append(batches, photos[start:min(start+size, len(photos))])
	}
	for _, video := range videosOf(targets) {
		batches = append(batches, []Image{video})
	}
	return batches
}

func writingTestInitialCheckpoint(snapshot WritingTestSnapshot, common writingTestCommon, index int, variant *writingTestVariant, seed []Observation) WritingTestCheckpoint {
	value := WritingTestCheckpoint{SnapshotHash: snapshot.Hash, Index: index, ObserveModel: common.ObserveModel, BatchSize: common.BatchSize, ObservePromptTokens: common.ObservePromptTokens, ObserveCompletionTokens: common.ObserveCompletionTokens, ObserveStructuredOutput: common.ObserveStructuredOutput, Reasoning: common.Reasoning, Observations: mapSlice(seed, func(o Observation) Observation { return fromSnapshotObservation(toSnapshotObservation(o)) })}
	if variant != nil {
		value.ObserveModel, value.WriteModel = variant.ObserveModel, variant.WriteModel
		value.ObservePromptTokens, value.ObserveCompletionTokens = variant.ObservePromptTokens, variant.ObserveCompletionTokens
		value.WritePromptTokens, value.WriteCompletionTokens = variant.WritePromptTokens, variant.WriteCompletionTokens
		value.ObserveStructuredOutput, value.WriteStructuredOutput = variant.ObserveStructuredOutput, variant.WriteStructuredOutput
	}
	return value
}

func acceptWritingTestCheckpoint(expected, stored WritingTestCheckpoint, batches [][]Image) (WritingTestCheckpoint, error) {
	left, right := expected, stored
	left.Observations, right.Observations = nil, nil
	left.CompletedObserveCalls, right.CompletedObserveCalls = 0, 0
	left.Prepared, right.Prepared = false, false
	left.InFlightStage, right.InFlightStage = "", ""
	left.FailedStage, right.FailedStage = "", ""
	left.Answer, right.Answer = nil, nil
	left.RequestInspections, right.RequestInspections = nil, nil
	for _, inspection := range stored.RequestInspections {
		if inspection.Validate() != nil || (inspection.Status != llm.InspectionCaptured && inspection.Status != llm.InspectionUnavailable) {
			return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
		}
	}
	if !reflect.DeepEqual(left, right) || stored.CompletedObserveCalls < 0 || stored.CompletedObserveCalls > len(batches) || (stored.Prepared && stored.CompletedObserveCalls != len(batches)) || (stored.Answer != nil && (!stored.Prepared || stored.Index < 0 || stored.InFlightStage != "" || stored.FailedStage != "")) {
		return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
	}
	for _, stage := range []string{stored.InFlightStage, stored.FailedStage} {
		if stage != "" && stage != llm.StageNameObserve && stage != llm.StageNameWrite {
			return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
		}
	}
	if stored.InFlightStage != "" && stored.FailedStage != "" {
		return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
	}
	if (stored.Index < 0 && (stored.InFlightStage == llm.StageNameWrite || stored.FailedStage == llm.StageNameWrite)) || (stored.Prepared && (stored.InFlightStage == llm.StageNameObserve || stored.FailedStage == llm.StageNameObserve)) {
		return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
	}
	allowed := make(map[string]Observation, len(expected.Observations))
	fresh := make(map[string]bool)
	for _, observation := range expected.Observations {
		allowed[observation.File] = observation
	}
	for _, batch := range batches[:stored.CompletedObserveCalls] {
		for _, image := range batch {
			fresh[image.Filename] = true
			allowed[image.Filename] = Observation{File: image.Filename, Model: expected.ObserveModel.String()}
		}
	}
	seen := make(map[string]bool, len(stored.Observations))
	for _, observation := range stored.Observations {
		original, ok := allowed[observation.File]
		if !ok || seen[observation.File] || (fresh[observation.File] && observation.Model != original.Model) || (!fresh[observation.File] && !reflect.DeepEqual(observation, original)) {
			return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
		}
		seen[observation.File] = true
	}
	if len(seen) != len(allowed) {
		return WritingTestCheckpoint{}, ErrWritingTestCheckpointInvalid
	}
	return cloneWritingTestCheckpoint(stored), nil
}

func writingTestResumeAllowed(checkpoint WritingTestCheckpoint, retry bool) error {
	if checkpoint.InFlightStage != "" {
		return ErrWritingTestExecutionUncertain
	}
	if checkpoint.FailedStage != "" && !retry {
		return ErrWritingTestRetryRequired
	}
	return nil
}

func writingTestProgress(progress Progress) Progress {
	if progress == nil {
		return func(string, int, int) {}
	}
	return progress
}

func addWritingTestUsage(out *CandidateUsage, usage llm.Usage) {
	out.PromptTokens += int64(usage.PromptTokens)
	out.CompletionTokens += int64(usage.CompletionTokens)
	if usage.CostReported {
		out.CostMicrousd += usage.CostMicrousd
		out.CostReported = true
	}
}

func cloneWritingTestAnswer(answer WriteAnswer) WriteAnswer {
	out := WriteAnswer{Content: fromSnapshotContent(toSnapshotContent(answer.Content)), Nouns: copyTexts(answer.Nouns), Origins: cloneOriginReview(answer.Origins), OriginCandidates: cloneContentOriginCandidates(answer.OriginCandidates)}
	if answer.Storyline != nil {
		out.Storyline = &Storyline{Origins: clonePlanOrigins(answer.Storyline.Origins), OriginCandidates: clonePlanOriginCandidates(answer.Storyline.OriginCandidates), MadeWith: copyTexts(answer.Storyline.MadeWith), Paragraphs: mapSlice(answer.Storyline.Paragraphs, func(p StorylineParagraph) StorylineParagraph {
			return StorylineParagraph{Text: p.Text, Files: copyTexts(p.Files)}
		})}
	}
	return out
}

func cloneWritingTestCheckpoint(value WritingTestCheckpoint) WritingTestCheckpoint {
	value.RequestInspections = mapSlice(value.RequestInspections, llm.CloneRequestInspection)
	value.Observations = mapSlice(value.Observations, func(o Observation) Observation { return fromSnapshotObservation(toSnapshotObservation(o)) })
	if value.Answer != nil {
		answer := cloneWritingTestAnswer(*value.Answer)
		value.Answer = &answer
	}
	return value
}

type writingTestFrozenBudget struct {
	CompletionBudget
	observe, write int
}

func (b writingTestFrozenBudget) Observation() int     { return b.observe }
func (b writingTestFrozenBudget) Write(*int, bool) int { return b.write }

type writingTestFrozenProfiles struct{ Profiles }

func (p writingTestFrozenProfiles) ValidateProfileSources(ctx context.Context, userID, voiceID string, sources []ProfileSource) error {
	if voiceID == "" && len(sources) == 0 {
		return nil
	}
	if checker, ok := p.Profiles.(FrozenProfileSources); ok {
		return checker.ValidateProfileSources(ctx, userID, voiceID, sources)
	}
	if len(sources) > 0 {
		return fmt.Errorf("frozen voice source validation is unavailable")
	}
	return nil
}

func (f *WritingTestFactory) writingTestWorker(common writingTestCommon, variant *writingTestVariant) *Service {
	worker := *f.service
	worker.fullWritingTest = true
	budget := writingTestFrozenBudget{CompletionBudget: worker.budget, observe: common.ObserveCompletionTokens}
	if variant != nil {
		budget.observe, budget.write = variant.ObserveCompletionTokens, variant.WriteCompletionTokens
	}
	worker.budget, worker.reasoning, worker.batchSize = budget, common.Reasoning, common.BatchSize
	worker.profiles = writingTestFrozenProfiles{Profiles: worker.profiles}
	models := writingTestFrozenModels{LLM: worker.models, legacy: common.Post.OriginProtocolVersion == 0, observeRef: common.ObserveModel, observeSchema: common.ObserveStructuredOutput}
	if variant != nil {
		models.observeRef, models.writeRef = variant.ObserveModel, variant.WriteModel
		models.observeSchema, models.writeSchema = variant.ObserveStructuredOutput, variant.WriteStructuredOutput
	}
	worker.models = models
	return &worker
}

func checkWritingTestVersions(common writingTestCommon) error {
	known := (common.PromptVersion == writingTestPromptVersion && common.SchemaVersion == writingTestSchemaVersion() && common.Post.OriginProtocolVersion == OriginProtocolVersion) || (common.PromptVersion == legacyWritingTestPromptVersion && common.SchemaVersion == legacyWritingTestSchemaVersion() && common.Post.OriginProtocolVersion == 0)
	if !known || !common.Reasoning.Observe.Valid() || !common.Reasoning.Write.Valid() {
		return ErrWritingTestMaterial
	}
	if observerWritingTest(common) && len(common.Post.Images) == 0 {
		return ErrWritingTestMaterial
	}
	return nil
}

type writingTestFrozenModels struct {
	LLM
	legacy                     bool
	observeRef, writeRef       llm.ModelRef
	observeSchema, writeSchema bool
}

func (m writingTestFrozenModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, found := m.LLM.Resolve(ref)
	if ref == m.writeRef {
		info.StructuredOutput = m.writeSchema
	} else if ref == m.observeRef {
		info.StructuredOutput = m.observeSchema
	}
	return info, found
}
func (m writingTestFrozenModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	if request.Stage == llm.StageNameObserve {
		request.JSONSchema = nil
		if m.observeSchema {
			if request.HasVideos() {
				request.JSONSchema = VideoObservationsSchema()
				if m.legacy {
					request.JSONSchema = LegacyVideoObservationsSchema()
				}
			} else {
				request.JSONSchema = ObservationsSchema()
				if m.legacy {
					request.JSONSchema = LegacyObservationsSchema()
				}
			}
		}
	}
	return m.LLM.Complete(ctx, ref, request)
}

func (f *WritingTestFactory) checkWritingTestWriter(ctx context.Context, userID string, variant writingTestVariant) error {
	if !variant.Snapshot.Post.TargetLanguage.Valid() {
		return ErrLanguageRequired
	}
	if variant.WriteCompletionTokens <= 0 || variant.WritePromptTokens <= 0 {
		return ErrWritingTestMaterial
	}
	if f.deps.Models == nil {
		return ErrWritingTestReference
	}
	model, err := f.deps.Models.PrepareWritingTestModel(ctx, userID, llm.StageNameWrite, variant.WriteModel)
	if err != nil {
		return err
	}
	if model.Info.Ref != variant.WriteModel || model.Info.Disabled || !model.Info.ServesStage(llm.StageNameWrite) {
		return ErrWriteModelRequired
	}
	current, err := f.service.requireWriteModel(variant.WriteModel.String())
	if err != nil {
		return err
	}
	if variant.WriteStructuredOutput && (!model.Info.StructuredOutput || !current.StructuredOutput) {
		return llm.ErrUnsupported
	}
	worker := f.writingTestWorker(writingTestCommon{}, nil)
	return worker.validateFrozenProfile(ctx, userID, variant.Snapshot.Post.Voice.ID, &variant.Snapshot.Profile)
}

func (f *WritingTestFactory) checkWritingTestObserver(ctx context.Context, userID string, ref llm.ModelRef, targets []Image, structured bool) error {
	if f.deps.Models == nil {
		return ErrWritingTestReference
	}
	model, err := f.deps.Models.PrepareWritingTestModel(ctx, userID, llm.StageNameObserve, ref)
	if err != nil {
		return err
	}
	if model.Info.Ref != ref || model.Info.Disabled || !model.Info.ServesStage(llm.StageNameObserve) || (len(photosOf(targets)) > 0 && !model.Info.Vision) {
		return ErrObserveModelRequired
	}
	current, found := f.service.models.Resolve(ref)
	if !found || current.Disabled || !current.ServesStage(llm.StageNameObserve) || (len(photosOf(targets)) > 0 && !current.Vision) {
		return ErrObserveModelRequired
	}
	if structured && (!model.Info.StructuredOutput || !current.StructuredOutput) {
		return llm.ErrUnsupported
	}
	if len(videosOf(targets)) > 0 && (!model.Info.VideoInput || !model.Info.VideoDelivery.SignedVideoURL || f.service.videos == nil) {
		return &VideoUnsupportedError{Model: ref.String()}
	}
	return f.service.refuseVideoBlindObserveModel(targets, ref)
}
