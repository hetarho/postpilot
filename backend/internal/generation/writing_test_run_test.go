package generation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

type runTestRights struct{ models *fakeModels }

func (r runTestRights) PrepareWritingTestModel(_ context.Context, _, stage string, ref llm.ModelRef) (WritingTestModel, error) {
	info, found := r.models.Resolve(ref)
	if !found || info.Disabled || !info.ServesStage(stage) {
		return WritingTestModel{}, llm.ErrModelUnavailable
	}
	return WritingTestModel{Info: info, PromptTokens: 30000}, nil
}

func runTestSnapshot(t *testing.T, factor, stage string, count int, images []Image) WritingTestSnapshot {
	t.Helper()
	if factor != "model" {
		stage = ""
	}
	common := writingTestCommon{
		Version: writingTestSnapshotVersion, Factor: factor, ModelStage: stage,
		Post:    PostInput{UserID: "alice", Memo: "Explicit frozen material", TargetLanguage: LanguageEnglish, TagCount: 4, Images: images},
		Profile: Profile{NoVoice: true}, ObserveModel: observeRef, BatchSize: 2,
		ObserveCompletionTokens: 512, ObservePromptTokens: 30000, WritePromptTokens: 30000, ObserveStructuredOutput: true,
		Reasoning: testReasoningPolicy, PromptVersion: writingTestPromptVersion, SchemaVersion: writingTestSchemaVersion(), AssignmentsHash: "frozen-assignments",
		Prepared: len(images) == 0,
	}
	var variants []writingTestVariant
	for index := 0; index < count; index++ {
		variant := writingTestVariant{Revision: "revision", SemanticKey: fmt.Sprintf("entrant-%d", index),
			Snapshot:     writeSnapshot{TargetLanguage: LanguageEnglish, Post: common.Post, Profile: common.Profile, SnapshotOnly: true},
			ObserveModel: observeRef, WriteModel: writeRef, ObservePromptTokens: 30000, ObserveCompletionTokens: 512,
			WritePromptTokens: 30000, WriteCompletionTokens: 1024 + index, ObserveStructuredOutput: true, WriteStructuredOutput: true,
		}
		if factor == "model" && stage == "write" {
			variant.WriteModel = llm.ModelRef{ProviderID: "provider", ModelID: fmt.Sprintf("writer-%d", index)}
		} else {
			variant.WriteCompletionTokens = 1024
		}
		if factor == "model" && stage == "observe" {
			variant.ObserveModel = llm.ModelRef{ProviderID: "provider", ModelID: fmt.Sprintf("observer-%d", index)}
		}
		variants = append(variants, variant)
	}
	return encodeRunTestSnapshot(t, common, variants)
}
func encodeRunTestSnapshot(t *testing.T, common writingTestCommon, variants []writingTestVariant) WritingTestSnapshot {
	t.Helper()
	data, err := encodeWritingTestCommon(common)
	if err != nil {
		t.Fatal(err)
	}
	out := WritingTestSnapshot{Common: data, PromptVersion: common.PromptVersion, AssignmentsHash: common.AssignmentsHash}
	for _, variant := range variants {
		raw, err := encodeWritingTestVariant(variant)
		if err != nil {
			t.Fatal(err)
		}
		out.Variants = append(out.Variants, raw)
	}
	out.Hash = writingTestHash(out.Common, out.Variants)
	return out
}
func runTestFactory(models *fakeModels, posts *fakePosts) *WritingTestFactory {
	for index := 0; index < 16; index++ {
		info := models.infos[writeRef]
		info.Ref = llm.ModelRef{ProviderID: "provider", ModelID: fmt.Sprintf("writer-%d", index)}
		models.infos[info.Ref] = info
	}
	service := NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 2, testReasoningPolicy, testBudget, testDeps())
	service.videos = &fakeLinker{}
	return &WritingTestFactory{service: service, deps: WritingTestFactoryDeps{Models: runTestRights{models}}}
}
func runTestAnswer() llm.Response {
	return llm.Response{Text: `{"title":"Complete post","summary":"Summary","tags":["a","b","c","d","e"],"blocks":[{"type":"TEXT","content":"Complete body"},{"type":"IMAGE","file":"one.jpg","alt":"Photo"},{"type":"IMAGE","file":"foreign.jpg"},{"type":"VIDEO","file":"clip.mp4"}],"storyline":[{"text":"Whole direct plan","files":["one.jpg","clip.mp4","foreign.jpg"]}],"nouns":["body"]}`, Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 7, CostMicrousd: 3, CostReported: true}}
}
func checkpointStore() (map[int]WritingTestCheckpoint, SaveWritingTestCheckpoint) {
	stored := map[int]WritingTestCheckpoint{}
	return stored, func(_ context.Context, checkpoint WritingTestCheckpoint) error {
		stored[checkpoint.Index] = cloneWritingTestCheckpoint(checkpoint)
		return nil
	}
}

func TestWritingTestSharedObservationAndSixteenCompleteOutputsNeverWriteTheSource(t *testing.T) {
	for _, factor := range []string{"model", "voice", "template", "guideline"} {
		t.Run(factor, func(t *testing.T) {
			models := newFakeModels()
			posts := &fakePosts{}
			factory := runTestFactory(models, posts)
			snapshot := runTestSnapshot(t, factor, "write", 16, []Image{photo("one.jpg"), photo("two.jpg"), photo("three.jpg")})
			_, variants, _ := decodeWritingTestSnapshot(snapshot)
			for _, variant := range variants {
				info := models.infos[writeRef]
				info.Ref = variant.WriteModel
				models.infos[variant.WriteModel] = info
			}
			stored, save := checkpointStore()
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				stage := request.Stage
				index := -1
				if stage == "write" {
					index = len(models.calls) - 3
				}
				if stored[index].InFlightStage != stage {
					t.Fatalf("provider issued without its durable checkpoint: index=%d stored=%+v", index, stored)
				}
				if stage == "observe" {
					if request.MaxTokens != 512 {
						t.Fatalf("observe budget drift: %d", request.MaxTokens)
					}
					return observationAnswer(request), nil
				}
				return runTestAnswer(), nil
			}
			prepared, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{SaveCheckpoint: save})
			if err != nil || prepared.Checkpoint.CompletedObserveCalls != 2 {
				t.Fatalf("prepared=%+v err=%v", prepared, err)
			}
			// Changing deployment budget/reasoning cannot change the admitted execution.
			factory.service.budget = fakeBudget{observe: 99, floor: 99, perChar: 1, ceiling: 99}
			factory.service.reasoning = ReasoningPolicy{Observe: llm.ReasoningHigh, Write: llm.ReasoningMax}
			var first llm.Request
			for index := 0; index < 16; index++ {
				result, err := factory.RunWritingTestCandidate(context.Background(), snapshot, index, &prepared.Checkpoint, WritingTestRunOptions{SaveCheckpoint: save})
				if err != nil {
					t.Fatal(err)
				}
				if result.ContentLanguage != LanguageEnglish || result.Answer.Storyline == nil || len(result.Answer.Storyline.Paragraphs) != 1 || len(result.Answer.Content.Tags) != 4 || len(result.Answer.Content.Blocks) != 2 || len(result.Answer.Storyline.MadeWith) != 3 {
					t.Fatalf("incomplete/unfiltered full output: %+v", result)
				}
				call := models.calls[len(models.calls)-1]
				if call.request.MaxTokens != variants[index].WriteCompletionTokens || call.request.Reasoning != testReasoningPolicy.Write || !reflect.DeepEqual(call.request.JSONSchema, WriteAnswerSchema()) {
					t.Fatalf("frozen execution drift: %+v", call.request)
				}
				if index == 0 {
					first = call.request
				} else if call.request.System != first.System || !reflect.DeepEqual(call.request.Messages, first.Messages) {
					t.Fatal("nonvaried prompt bytes changed")
				}
			}
			if len(models.calls) != 18 || posts.reads != 0 || len(posts.observationWrites) != 0 || len(posts.contents) != 0 || len(posts.storylines) != 0 {
				t.Fatalf("call/source mutation: calls=%d posts=%+v", len(models.calls), posts)
			}
		})
	}
}

func TestWritingTestObserverPipelinesAreIndependentAndUseOneFixedFullWriter(t *testing.T) {
	models := newFakeModels()
	posts := &fakePosts{}
	factory := runTestFactory(models, posts)
	snapshot := runTestSnapshot(t, "model", "observe", 2, []Image{photo("one.jpg"), clip("clip.mp4")})
	common, variants, _ := decodeWritingTestSnapshot(snapshot)
	for _, variant := range variants {
		info := models.infos[observeRef]
		info.Ref = variant.ObserveModel
		info.VideoInput = true
		info.VideoDelivery.SignedVideoURL = true
		models.infos[variant.ObserveModel] = info
	}
	stored, save := checkpointStore()
	models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.Stage == "observe" {
			answer := observationAnswer(request)
			answer.Text = strings.ReplaceAll(answer.Text, "seen", ref.ModelID)
			return answer, nil
		}
		if ref != writeRef {
			t.Fatal("observer test changed the fixed writer")
		}
		return runTestAnswer(), nil
	}
	for index := 0; index < 2; index++ {
		result, err := factory.RunWritingTestCandidate(context.Background(), snapshot, index, nil, WritingTestRunOptions{SaveCheckpoint: save})
		if err != nil || result.Checkpoint.CompletedObserveCalls != 2 || len(result.Answer.Content.Blocks) != 3 || len(result.Answer.Storyline.MadeWith) != 2 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	if len(models.calls) != 6 || len(stored) != 2 || common.Post.UserID != "alice" {
		t.Fatalf("calls=%d stored=%+v", len(models.calls), stored)
	}
	if models.calls[2].request.Messages[0].Parts[0].Text == models.calls[5].request.Messages[0].Parts[0].Text {
		t.Fatal("observer consequences were reused across entrants")
	}
	for _, pair := range [][2]int{{0, 3}, {1, 4}} {
		left, right := models.calls[pair[0]].request, models.calls[pair[1]].request
		if left.System != right.System || !reflect.DeepEqual(left.Messages, right.Messages) {
			t.Fatal("observer inputs differ beyond their model ref")
		}
	}
	if posts.reads != 0 || len(posts.observationWrites) != 0 || len(posts.contents) != 0 {
		t.Fatal("test observation changed its source")
	}
}

func TestWritingTestCheckpointsFenceFailedOnlyRetryAndSuccessfulReplay(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "observe", 2, []Image{photo("one.jpg"), photo("two.jpg"), photo("three.jpg")})
	_, variants, _ := decodeWritingTestSnapshot(snapshot)
	for _, variant := range variants {
		info := models.infos[observeRef]
		info.Ref = variant.ObserveModel
		models.infos[variant.ObserveModel] = info
	}
	stored, save := checkpointStore()
	fail := true
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.Stage == "observe" {
			if len(models.calls) == 2 && fail {
				return llm.Response{Usage: llm.Usage{PromptTokens: 9}}, llm.ErrRateLimited
			}
			return observationAnswer(request), nil
		}
		return runTestAnswer(), nil
	}
	failed, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if !errors.Is(err, llm.ErrRateLimited) || failed.Checkpoint.CompletedObserveCalls != 1 || failed.Checkpoint.FailedStage != "observe" || failed.Usage.PromptTokens != 9 {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &failed.Checkpoint, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestRetryRequired) || len(models.calls) != 2 {
		t.Fatalf("implicit retry: %v", err)
	}
	calls, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0})
	if err != nil || len(calls) != 2 || calls[0].Count != 1 || calls[1].Count != 1 {
		t.Fatalf("retry plan=%+v err=%v", calls, err)
	}
	fail = false
	finished, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &failed.Checkpoint, RetryFailed: true, SaveCheckpoint: save})
	if err != nil || len(models.calls) != 4 || finished.Checkpoint.Answer == nil {
		t.Fatalf("finished=%+v err=%v", finished, err)
	}
	replayed, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &finished.Checkpoint, SaveCheckpoint: save})
	if err != nil || !replayed.Replayed || replayed.Usage != (CandidateUsage{}) || len(models.calls) != 4 {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	if _, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0}); !errors.Is(err, ErrWritingTestRetryRequired) {
		t.Fatalf("successful entrant retry: %v", err)
	}
}

func TestWritingTestRefusesUncertainAndForeignCheckpointsAndRejectedPersistenceBeforeCalls(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, nil)
	_, save := checkpointStore()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return runTestAnswer(), nil }
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{}); !errors.Is(err, ErrWritingTestCheckpointRequired) {
		t.Fatal(err)
	}
	refused := errors.New("cancelled owner fence")
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: func(context.Context, WritingTestCheckpoint) error { return refused }}); !errors.Is(err, refused) || len(models.calls) != 0 {
		t.Fatalf("pre-call fence err=%v calls=%d", err, len(models.calls))
	}
	common, variants, _ := decodeWritingTestSnapshot(snapshot)
	checkpoint := writingTestInitialCheckpoint(snapshot, common, 0, &variants[0], nil)
	checkpoint.Prepared = true
	checkpoint.InFlightStage = "write"
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &checkpoint, RetryFailed: true, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestExecutionUncertain) {
		t.Fatal(err)
	}
	checkpoint.InFlightStage = ""
	checkpoint.Index = 1
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &checkpoint, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestCheckpointInvalid) {
		t.Fatal(err)
	}
	if len(models.calls) != 0 {
		t.Fatal("refused checkpoint issued work")
	}
}

func TestWritingTestRefusesUnsupportedVideoBeforeAnyPhotoCall(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, []Image{photo("one.jpg"), clip("clip.mp4")})
	_, save := checkpointStore()
	if _, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{SaveCheckpoint: save}); !errors.Is(err, ErrVideoUnsupported) || len(models.calls) != 0 {
		t.Fatalf("video refusal err=%v calls=%d", err, len(models.calls))
	}
}

func TestWritingTestLostResultCheckpointDoesNotReplayTheIssuedWriter(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, nil)
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return runTestAnswer(), nil }
	stored := map[int]WritingTestCheckpoint{}
	lost := errors.New("result checkpoint unavailable")
	save := func(_ context.Context, checkpoint WritingTestCheckpoint) error {
		if checkpoint.Answer != nil {
			return lost
		}
		stored[checkpoint.Index] = cloneWritingTestCheckpoint(checkpoint)
		return nil
	}
	result, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if !errors.Is(err, lost) || result.Usage.PromptTokens != 11 || result.Checkpoint.InFlightStage != "write" || len(models.calls) != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, len(models.calls))
	}
	checkpoint := stored[0]
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &checkpoint, RetryFailed: true, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestExecutionUncertain) || len(models.calls) != 1 {
		t.Fatalf("issued call replayed: err=%v calls=%d", err, len(models.calls))
	}
}

func TestWritingTestFrozenReuseAndWriterRetryNeverReobserve(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, []Image{photo("one.jpg")})
	common, variants, _ := decodeWritingTestSnapshot(snapshot)
	files := []string{}
	common.ObserveFiles, common.Observations = &files, []Observation{{File: "one.jpg", Scene: "Frozen carried fact", Model: "old/observer"}}
	snapshot = encodeRunTestSnapshot(t, common, variants)
	stored, save := checkpointStore()
	failed := true
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.Stage != "write" {
			t.Fatal("reused material was observed")
		}
		if !strings.Contains(request.Messages[0].Parts[0].Text, "Frozen carried fact") {
			t.Fatal("frozen reuse vanished from writer")
		}
		if failed {
			return llm.Response{Usage: llm.Usage{CompletionTokens: 2}}, llm.ErrBadOutput
		}
		return runTestAnswer(), nil
	}
	result, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if !errors.Is(err, llm.ErrBadOutput) || result.Checkpoint.FailedStage != "write" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	calls, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0})
	if err != nil || len(calls) != 1 || calls[0].Stage != "write" || calls[0].Count != 1 {
		t.Fatalf("retry plan=%+v err=%v", calls, err)
	}
	failed = false
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &result.Checkpoint, RetryFailed: true, SaveCheckpoint: save}); err != nil || len(models.calls) != 2 {
		t.Fatalf("retry err=%v calls=%d", err, len(models.calls))
	}
}

func TestWritingTestRequestSchemasRemainFrozenAcrossCatalogChanges(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, []Image{photo("one.jpg")})
	common, variants, _ := decodeWritingTestSnapshot(snapshot)
	common.ObserveStructuredOutput = false
	for index := range variants {
		variants[index].ObserveStructuredOutput, variants[index].WriteStructuredOutput = false, false
	}
	snapshot = encodeRunTestSnapshot(t, common, variants)
	_, save := checkpointStore()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.JSONSchema != nil {
			t.Fatal("new catalog schema capability changed the frozen request")
		}
		if request.Stage == "observe" {
			return observationAnswer(request), nil
		}
		return runTestAnswer(), nil
	}
	prepared, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, &prepared.Checkpoint, WritingTestRunOptions{SaveCheckpoint: save}); err != nil || len(models.calls) != 2 {
		t.Fatalf("err=%v calls=%d", err, len(models.calls))
	}
}

func TestWritingTestSharedPartialRetryAndCheckpointMaterialValidation(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, []Image{photo("one.jpg"), photo("two.jpg"), photo("three.jpg")})
	stored, save := checkpointStore()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if len(models.calls) == 2 {
			return llm.Response{}, llm.ErrRateLimited
		}
		if request.Stage == "observe" {
			return observationAnswer(request), nil
		}
		return runTestAnswer(), nil
	}
	partial, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{SaveCheckpoint: save})
	if !errors.Is(err, llm.ErrRateLimited) || partial.Checkpoint.CompletedObserveCalls != 1 {
		t.Fatalf("partial=%+v err=%v", partial, err)
	}
	calls, err := factory.PlanWritingTestRetry(snapshot, &partial.Checkpoint, stored, []int{0, 1})
	if err != nil || len(calls) != 3 || calls[0].Stage != "observe" || calls[0].Count != 1 {
		t.Fatalf("remaining=%+v err=%v", calls, err)
	}
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestPreparationRequired) {
		t.Fatalf("unprepared writer: %v", err)
	}
	if _, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{Checkpoint: &partial.Checkpoint, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestRetryRequired) || len(models.calls) != 2 {
		t.Fatalf("implicit shared retry: %v", err)
	}
	prepared, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{Checkpoint: &partial.Checkpoint, RetryFailed: true, SaveCheckpoint: save})
	if err != nil || !prepared.Checkpoint.Prepared || len(models.calls) != 3 {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	corrupt := cloneWritingTestCheckpoint(prepared.Checkpoint)
	corrupt.Observations = append(corrupt.Observations, Observation{File: "foreign.jpg", Scene: "unrelated private fact"})
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, &corrupt, WritingTestRunOptions{SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestCheckpointInvalid) || len(models.calls) != 3 {
		t.Fatalf("foreign observations accepted: %v", err)
	}
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, &prepared.Checkpoint, WritingTestRunOptions{SaveCheckpoint: save}); err != nil || len(models.calls) != 4 {
		t.Fatalf("prepared writer err=%v calls=%d", err, len(models.calls))
	}
}

func TestWritingTestActualRequestBytesVaryOnlyTheChosenSettingAcrossFormats(t *testing.T) {
	for _, factor := range []string{"voice", "template", "guideline"} {
		for _, count := range []int{2, 4, 8, 16} {
			t.Run(fmt.Sprintf("%s/%d", factor, count), func(t *testing.T) {
				models := newFakeModels()
				factory := runTestFactory(models, &fakePosts{})
				snapshot := runTestSnapshot(t, factor, "", count, nil)
				common, variants, _ := decodeWritingTestSnapshot(snapshot)
				for index := range variants {
					marker := fmt.Sprintf("VARIED-%d", index)
					switch factor {
					case "voice":
						variants[index].Snapshot.Profile = Profile{Text: marker}
					case "template":
						variants[index].Snapshot.Post.Template = &TemplateBrief{Name: "Fixed display label", Body: marker}
					case "guideline":
						variants[index].Snapshot.Post.Guidelines = []string{"UNCHANGED-FIRST", marker, "UNCHANGED-LAST"}
					}
				}
				snapshot = encodeRunTestSnapshot(t, common, variants)
				_, save := checkpointStore()
				models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return runTestAnswer(), nil }
				var firstSystem, firstUser string
				for index := 0; index < count; index++ {
					if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, index, nil, WritingTestRunOptions{SaveCheckpoint: save}); err != nil {
						t.Fatal(err)
					}
					request := models.calls[index].request
					marker := fmt.Sprintf("VARIED-%d", index)
					user := request.Messages[0].Parts[0].Text
					if !strings.Contains(request.System+user, marker) {
						t.Fatalf("%s variant missing from actual request", factor)
					}
					system := strings.ReplaceAll(request.System, marker, "ONE-SETTING-SLOT")
					user = strings.ReplaceAll(user, marker, "ONE-SETTING-SLOT")
					if index == 0 {
						firstSystem, firstUser = system, user
					} else if system != firstSystem || user != firstUser {
						t.Fatal("unvaried prompt material changed")
					}
				}
				if len(models.calls) != count {
					t.Fatalf("calls=%d want=%d", len(models.calls), count)
				}
			})
		}
	}
}

func TestWritingTestSuccessfulReplayStillHonorsTheOwnerPurgeFence(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 2, nil)
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return runTestAnswer(), nil }
	_, save := checkpointStore()
	finished, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil {
		t.Fatal(err)
	}
	purged := errors.New("private test payload purged")
	result, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &finished.Checkpoint, SaveCheckpoint: func(context.Context, WritingTestCheckpoint) error { return purged }})
	if !errors.Is(err, purged) || result.Replayed || result.Answer.Content.Title != "" || len(models.calls) != 1 {
		t.Fatalf("purged replay=%+v err=%v calls=%d", result, err, len(models.calls))
	}
}
