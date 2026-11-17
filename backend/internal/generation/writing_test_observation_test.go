package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// testPhotoSources stands in for the test photo lookup: the owner's frozen test photos.
type testPhotoSources struct{ source WritingTestSource }

func (s *testPhotoSources) ResolveWritingTestSource(_ context.Context, user, slug string, _, _ int64, material WritingTestMaterialRequest) (WritingTestSource, error) {
	if user != "alice" || slug != "" || material.Output != WritingTestOutputObservation || len(material.PhotoIDs) != len(s.source.Attachments) {
		return WritingTestSource{}, ErrWritingTestMaterial
	}
	return s.source, nil
}

func observationTestSource(photos int) WritingTestSource {
	source := WritingTestSource{Post: PostInput{UserID: "alice", TargetLanguage: LanguageKorean}, Revision: "test-photos-v1"}
	for index := range photos {
		image := Image{ID: fmt.Sprintf("photo-%d", index), Filename: fmt.Sprintf("p%d.jpg", index), Key: fmt.Sprintf("tests/alice/photo-%d.jpg", index), Kind: AttachmentPhoto, ContentType: "image/jpeg", Width: 1024, Height: 768}
		source.Post.Images = append(source.Post.Images, image)
		source.Attachments = append(source.Attachments, WritingTestAttachment{ID: image.ID, Fingerprint: "bytes:" + image.Key, Image: image})
	}
	return source
}

func observationTestFixture(t *testing.T, photos int) (*WritingTestFactory, *plannerPorts, *fakeModels, *testPhotoSources, *plannerMemory, *plannerQuality) {
	t.Helper()
	planned, ports, _, models, memory, quality := plannerFixture(t)
	sources := &testPhotoSources{source: observationTestSource(photos)}
	factory := NewWritingTestFactory(planned.service, WritingTestFactoryDeps{Sources: sources, Models: ports, Profiles: ports, Templates: ports, Guidelines: ports, Candidates: ports})
	return factory, ports, models, sources, memory, quality
}

func observationTestRequest(t *testing.T, ports *plannerPorts, models *fakeModels, sources *testPhotoSources, count int) WritingTestSnapshotRequest {
	t.Helper()
	material := WritingTestMaterialRequest{Output: WritingTestOutputObservation, TargetLanguage: LanguageKorean}
	for _, attachment := range sources.source.Attachments {
		material.PhotoIDs = append(material.PhotoIDs, attachment.ID)
	}
	raw, err := EncodeWritingTestMaterialRequest(material)
	if err != nil {
		t.Fatal(err)
	}
	request := WritingTestSnapshotRequest{UserID: "alice", Factor: "model", ModelStage: "observe", Output: WritingTestOutputObservation, Count: count, CommonMaterial: raw}
	for index := range count {
		ref := llm.ModelRef{ProviderID: "p", ModelID: fmt.Sprintf("observer-%d", index)}
		ports.models[ref] = llm.ModelInfo{Ref: ref, Stages: []string{"observe"}, Vision: true, StructuredOutput: true, ContextTokens: 131072}
		models.infos[ref] = ports.models[ref]
		variant, _ := EncodeWritingTestReference(WritingTestReference{SourceKind: "model", Model: ref})
		request.Variants = append(request.Variants, variant)
	}
	return request
}

func TestObservationTestFreezesPhotosAndObserversButNoWriterOrWritingMaterial(t *testing.T) {
	factory, ports, models, sources, memory, quality := observationTestFixture(t, 5)
	request := observationTestRequest(t, ports, models, sources, 2)
	snapshot, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	common, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if common.Output != WritingTestOutputObservation || !strings.Contains(string(snapshot.Common), `"output":"observation"`) || len(common.Attachments) != 5 || common.Post.Template != nil || len(common.Post.Guidelines) != 0 || len(common.Post.DefaultGuidelines) != 0 || len(common.Post.Memories) != 0 || common.Post.Voice != (VoiceRef{}) || !common.Profile.NoVoice || common.Post.TagCount != 0 {
		t.Fatalf("observation common froze writing material: %+v", common)
	}
	if ports.voiceReads+ports.templateReads+ports.guidelineReads+ports.candidateReads+memory.calls+quality.calls != 0 {
		t.Fatalf("observation freeze read writing settings: %+v memory=%d quality=%d", ports, memory.calls, quality.calls)
	}
	for _, check := range ports.modelChecks {
		if !strings.Contains(check, ":observe:") {
			t.Fatalf("observation freeze prepared a non-observer model: %v", ports.modelChecks)
		}
	}
	for index, variant := range variants {
		if variant.WriteModel != (llm.ModelRef{}) || variant.WriteCompletionTokens != 0 || variant.ObserveModel.ModelID != fmt.Sprintf("observer-%d", index) {
			t.Fatalf("variant %d froze a writer: %+v", index, variant)
		}
	}
	// Two observers each observe five photos in batches of four: two calls each, no write.
	calls, err := factory.PlanWritingTest(snapshot)
	if err != nil || snapshot.ObserveCalls != 4 || len(calls) != 2 {
		t.Fatalf("plan=%+v observe=%d err=%v", calls, snapshot.ObserveCalls, err)
	}
	for _, call := range calls {
		if call.Stage != "observe" || call.Count != 2 {
			t.Fatalf("observation plan priced something other than observation: %+v", calls)
		}
	}
	// The hash binds the photo keys and dimensions.
	sources.source.Post.Images[0].Width = 800
	sources.source.Attachments[0].Image.Width = 800
	changed, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil || changed.Hash == snapshot.Hash {
		t.Fatalf("photo dimensions did not change the hash: %v", err)
	}
}

func TestObservationTestRefusesMismatchedOutputsWritingMaterialAndVideos(t *testing.T) {
	factory, ports, models, sources, _, _ := observationTestFixture(t, 2)
	request := observationTestRequest(t, ports, models, sources, 2)
	writer := request
	writer.ModelStage = "write"
	if _, err := factory.FreezeWritingTest(context.Background(), writer); !errors.Is(err, ErrWritingTestFactor) {
		t.Fatalf("write-stage observation test: %v", err)
	}
	post := request
	post.Output = ""
	if _, err := factory.FreezeWritingTest(context.Background(), post); !errors.Is(err, ErrWritingTestFactor) {
		t.Fatalf("request and material outputs disagree: %v", err)
	}
	voiced := request
	material := WritingTestMaterialRequest{Output: WritingTestOutputObservation, PhotoIDs: []string{"photo-0", "photo-1"}, TargetLanguage: LanguageKorean, VoiceID: "base-voice"}
	voiced.CommonMaterial, _ = EncodeWritingTestMaterialRequest(material)
	if _, err := factory.FreezeWritingTest(context.Background(), voiced); !errors.Is(err, ErrWritingTestMaterial) {
		t.Fatalf("observation test accepted a voice: %v", err)
	}
	video := Image{ID: "photo-1", Filename: "clip.mp4", Key: "tests/alice/clip.mp4", Kind: AttachmentVideo, ContentType: "video/mp4", DurationMs: 3000, Width: 1024, Height: 768}
	sources.source.Post.Images[1], sources.source.Attachments[1].Image = video, video
	if _, err := factory.FreezeWritingTest(context.Background(), request); !errors.Is(err, ErrWritingTestMaterial) {
		t.Fatalf("observation test accepted a video: %v", err)
	}
}

func TestObservationTestEntrantsObserveIndependentlyResumeAndRetryWithoutWriting(t *testing.T) {
	factory, ports, models, sources, _, _ := observationTestFixture(t, 5)
	snapshot, err := factory.FreezeWritingTest(context.Background(), observationTestRequest(t, ports, models, sources, 2))
	if err != nil {
		t.Fatal(err)
	}
	stored, save := checkpointStore()
	var saves []WritingTestCheckpoint
	recording := func(ctx context.Context, checkpoint WritingTestCheckpoint) error {
		saves = append(saves, cloneWritingTestCheckpoint(checkpoint))
		return save(ctx, checkpoint)
	}
	fail := true
	models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.Stage != "observe" {
			t.Fatalf("observation test issued a %s call", request.Stage)
		}
		if ref.ModelID == "observer-0" && fail && len(models.calls) == 2 {
			return llm.Response{Usage: llm.Usage{PromptTokens: 9}}, llm.ErrRateLimited
		}
		return observationAnswer(request), nil
	}
	prepared, err := factory.PrepareWritingTestInput(context.Background(), snapshot, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil || !prepared.Checkpoint.Prepared || len(models.calls) != 0 {
		t.Fatalf("shared preparation of an observation test must issue nothing: %+v %v", prepared, err)
	}

	// Entrant 0 fails its second batch after checkpointing the first.
	failed, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: recording})
	if !errors.Is(err, llm.ErrRateLimited) || failed.Checkpoint.CompletedObserveCalls != 1 || failed.Checkpoint.FailedStage != "observe" || !failed.Checkpoint.ObservationOnly {
		t.Fatalf("failed=%+v err=%v", failed.Checkpoint, err)
	}
	settledBatches := 0
	for _, checkpoint := range saves {
		if checkpoint.InFlightStage == "" && checkpoint.CompletedObserveCalls == 1 && checkpoint.FailedStage == "" {
			settledBatches++
		}
	}
	if settledBatches != 1 {
		t.Fatalf("the first batch was not checkpointed on its own: %+v", saves)
	}
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &failed.Checkpoint, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestRetryRequired) || len(models.calls) != 2 {
		t.Fatalf("implicit retry: %v", err)
	}
	calls, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0})
	if err != nil || len(calls) != 1 || calls[0].Stage != "observe" || calls[0].Count != 1 || calls[0].Ref.ModelID != "observer-0" {
		t.Fatalf("failed-only retry plan=%+v err=%v", calls, err)
	}
	fail = false
	finished, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &failed.Checkpoint, RetryFailed: true, SaveCheckpoint: save})
	if err != nil || len(models.calls) != 3 || !finished.Checkpoint.Settled || len(finished.Observations) != 5 || finished.Answer.Content.Title != "" {
		t.Fatalf("finished=%+v err=%v", finished, err)
	}
	for _, observation := range finished.Observations {
		if observation.Model != "p/observer-0" {
			t.Fatalf("observation names another observer: %+v", observation)
		}
	}

	// Entrant 1 is interrupted after its first batch and resumes without repeating it.
	ctx, cancel := context.WithCancel(context.Background())
	models.complete = func(ref llm.ModelRef, request llm.Request) (llm.Response, error) {
		if ref.ModelID != "observer-1" || request.Stage != "observe" {
			t.Fatalf("entrant 1 called %s %s", ref, request.Stage)
		}
		cancel()
		return observationAnswer(request), nil
	}
	interrupted, err := factory.RunWritingTestCandidate(ctx, snapshot, 1, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if !errors.Is(err, context.Canceled) || interrupted.Checkpoint.CompletedObserveCalls != 1 || len(models.calls) != 4 {
		t.Fatalf("interrupted=%+v err=%v", interrupted.Checkpoint, err)
	}
	resumed, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 1, nil, WritingTestRunOptions{Checkpoint: &interrupted.Checkpoint, SaveCheckpoint: save})
	if err != nil || len(models.calls) != 5 || !resumed.Checkpoint.Settled || len(resumed.Observations) != 5 {
		t.Fatalf("resumed=%+v err=%v", resumed, err)
	}

	// A settled checkpoint replays and plans nothing more.
	replayed, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &finished.Checkpoint, SaveCheckpoint: save})
	if err != nil || !replayed.Replayed || len(replayed.Observations) != 5 || replayed.Usage != (CandidateUsage{}) || len(models.calls) != 5 {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	if calls, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0, 1}); err != nil || len(calls) != 0 {
		t.Fatalf("settled observations must plan no call: %+v %v", calls, err)
	}
	// A settled flag never belongs to a writing entrant's checkpoint.
	forged := finished.Checkpoint
	forged.ObservationOnly = false
	if _, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &forged, SaveCheckpoint: save}); !errors.Is(err, ErrWritingTestCheckpointInvalid) {
		t.Fatalf("forged checkpoint: %v", err)
	}
}

// MODEL-100: a reasoning contestant freezes the observation headroom, another the batch budget,
// and the plan prices each frozen budget.
func TestObservationTestFreezesReasoningHeadroomPerObserver(t *testing.T) {
	factory, ports, models, sources, _, _ := observationTestFixture(t, 4)
	request := observationTestRequest(t, ports, models, sources, 2)
	reasoning := llm.ModelRef{ProviderID: "p", ModelID: "observer-0"}
	info := ports.models[reasoning]
	info.Reasons = true
	ports.models[reasoning], models.infos[reasoning] = info, info
	snapshot, err := factory.FreezeWritingTest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	_, variants, err := decodeWritingTestSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if variants[0].ObserveCompletionTokens != testBudget.Observation(true) || variants[1].ObserveCompletionTokens != testBudget.Observation(false) {
		t.Fatalf("frozen observe budgets = %d, %d; want %d, %d", variants[0].ObserveCompletionTokens, variants[1].ObserveCompletionTokens, testBudget.Observation(true), testBudget.Observation(false))
	}
	calls, err := factory.PlanWritingTest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	priced := map[string]int{}
	for _, call := range calls {
		priced[call.Ref.ModelID] = call.CompletionTokens
	}
	if priced["observer-0"] != testBudget.Observation(true) || priced["observer-1"] != testBudget.Observation(false) {
		t.Fatalf("plan priced %v, want the frozen budgets", priced)
	}
}
