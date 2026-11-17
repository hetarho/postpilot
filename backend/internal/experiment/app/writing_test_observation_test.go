package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

// testPhotoSpy is the test photo rows as ListReadyTestPhotos answers: ready photos of the
// owner among the ids, oldest first, anything else absent.
type testPhotoSpy struct{ photos []experiment.TestPhoto }

func (s testPhotoSpy) ListReadyTestPhotos(_ context.Context, user string, ids []string) ([]experiment.TestPhoto, error) {
	var out []experiment.TestPhoto
	for _, photo := range s.photos {
		if photo.UserID == user && photo.Status == experiment.TestPhotoReady && slices.Contains(ids, photo.ID) {
			out = append(out, photo)
		}
	}
	return out, nil
}

func readyTestPhoto(user, id, filename string) experiment.TestPhoto {
	return experiment.TestPhoto{ID: id, UserID: user, Filename: filename, Key: "tests/" + user + "/" + id + ".jpg", Status: experiment.TestPhotoReady, Bytes: 2048, Width: 1024, Height: 768}
}

func observationMaterial(ids ...string) generation.WritingTestMaterialRequest {
	return generation.WritingTestMaterialRequest{Output: generation.WritingTestOutputObservation, PhotoIDs: ids, TargetLanguage: generation.LanguageKorean}
}

func TestResolveTestPhotosFreezesOwnedReadyPhotosAndRefusesOthers(t *testing.T) {
	spy := testPhotoSpy{photos: []experiment.TestPhoto{
		readyTestPhoto("alice", "a", "menu.jpg"), readyTestPhoto("alice", "b", "menu.jpg"), readyTestPhoto("alice", "c", "menu.jpg"),
		readyTestPhoto("bob", "foreign", "x.jpg"),
	}}
	held := readyTestPhoto("alice", "held", "held.jpg")
	held.TestID = "another-test"
	pending := readyTestPhoto("alice", "pending", "p.jpg")
	pending.Status = experiment.TestPhotoPending
	spy.photos = append(spy.photos, held, pending)
	resolver := &WritingTestResolvers{dependencies: ResolverDependencies{Photos: spy}}

	source, err := resolver.ResolveWritingTestSource(context.Background(), "alice", "", 0, 0, observationMaterial("c", "a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for index, image := range source.Post.Images {
		names = append(names, image.Filename)
		attachment := source.Attachments[index]
		if image.Kind != generation.AttachmentPhoto || image.ContentType != "image/jpeg" || image.Width != 1024 || image.Height != 768 || !strings.HasPrefix(image.Key, "tests/alice/") || attachment.Image != image || attachment.Fingerprint == "" {
			t.Fatalf("frozen image %d: %+v %+v", index, image, attachment)
		}
	}
	if strings.Join(names, "|") != "menu.jpg|menu (2).jpg|menu (3).jpg" || source.Post.Images[0].ID != "c" || source.Post.UserID != "alice" || source.Post.Memo != "" || source.Revision == "" {
		t.Fatalf("source=%+v", source)
	}
	for _, ids := range [][]string{{"a", "foreign"}, {"a", "held"}, {"a", "pending"}, {"a", "missing"}} {
		if _, err := resolver.ResolveWritingTestSource(context.Background(), "alice", "", 0, 0, observationMaterial(ids...)); !errors.Is(err, experiment.ErrTestMaterialInvalid) {
			t.Fatalf("%v must refuse alike: %v", ids, err)
		}
	}
	if _, err := resolver.ResolveWritingTestSource(context.Background(), "alice", "source", 1, 1, observationMaterial("a")); !errors.Is(err, experiment.ErrTestMaterialInvalid) {
		t.Fatalf("an observation test names no source post: %v", err)
	}
	post := observationMaterial("a")
	post.Output = ""
	if _, err := resolver.ResolveWritingTestSource(context.Background(), "alice", "", 0, 0, post); !errors.Is(err, experiment.ErrTestMaterialInvalid) {
		t.Fatalf("a post test carries no test photos: %v", err)
	}
}

type observationAdapterImages struct{}

func (observationAdapterImages) Read(context.Context, string) ([]byte, error) {
	return []byte{0xff, 0xd8, 0xff, 0xe0}, nil
}

// observationAdapterLLM answers every observe call with one entry per requested file and
// refuses any other stage.
type observationAdapterLLM struct{ calls []adapterLLMCall }

func (*observationAdapterLLM) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return adapterModel(ref), ref.ProviderID == "approved" && ref.ModelID != ""
}
func (m *observationAdapterLLM) Complete(_ context.Context, ref llm.ModelRef, r llm.Request) (llm.Response, error) {
	m.calls = append(m.calls, adapterLLMCall{ref, r})
	if r.Stage != llm.StageNameObserve {
		return llm.Response{}, fmt.Errorf("observation test issued a %s call", r.Stage)
	}
	var files []string
	for _, part := range r.Messages[0].Parts {
		if value, ok := strings.CutPrefix(part.Text, "files: "); ok {
			files = strings.Split(value, ", ")
		}
	}
	items := make([]string, len(files))
	for i, file := range files {
		items[i] = fmt.Sprintf(`{"file":%q,"scene":"seen by %s","mood":"","visible_text":"","objects":["cup"],"people_present":false}`, file, ref.ModelID)
	}
	return llm.Response{Text: `{"observations":[` + strings.Join(items, ",") + `]}`, Usage: llm.Usage{PromptTokens: 5, CompletionTokens: 3}}, nil
}

func TestObservationTestAdapterPricesObservationOnlyAndStoresEachObserversObservations(t *testing.T) {
	ports := &adapterFactoryPorts{}
	models := &observationAdapterLLM{}
	unused := adapterUnusedReads{ImageReader: observationAdapterImages{}}
	service := generation.NewService(&adapterCanonicalPosts{}, unused, models, unused, unused, 2, generation.DefaultReasoningPolicy(), adapterBudget{}, generation.Deps{Templates: unused, Guidelines: unused, Memories: unused, Candidates: unused, QualityRules: unused, Videos: unused, VideoURLTTL: time.Minute})
	photos := testPhotoSpy{photos: []experiment.TestPhoto{readyTestPhoto("alice", "a", "a.jpg"), readyTestPhoto("alice", "b", "b.jpg"), readyTestPhoto("alice", "c", "c.jpg")}}
	sources := &WritingTestResolvers{dependencies: ResolverDependencies{Photos: photos}}
	factory := generation.NewWritingTestFactory(service, generation.WritingTestFactoryDeps{Sources: sources, Models: ports, Profiles: ports, Templates: unused, Guidelines: ports, Candidates: unused})
	store := &adapterWorkStore{}
	adapter := NewWritingTestGeneration(factory, store)
	start := experiment.TestStart{UserID: "alice", RequestKey: "start", Factor: experiment.FactorModel, ModelStage: experiment.StageObserve, Output: experiment.OutputObservation, Count: 2, Input: experiment.TestInput{PhotoIDs: []string{"a", "b", "c"}, TargetLanguage: "ko"}}
	var variants []experiment.FrozenTestVariant
	for i := range 2 {
		ref := experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "approved", ModelID: fmt.Sprintf("observer-%d", i)}}
		start.Entrants = append(start.Entrants, ref)
		variants = append(variants, experiment.FrozenTestVariant{Reference: ref})
	}
	plan, err := adapter.PrepareWritingTest(context.Background(), start, variants)
	if err != nil {
		t.Fatal(err)
	}
	// Three photos in batches of two: two observe calls per observer and nothing else.
	if len(plan.Calls) != 2 {
		t.Fatalf("calls=%+v", plan.Calls)
	}
	for _, call := range plan.Calls {
		if call.Stage != experiment.StageObserve || call.Count != 2 {
			t.Fatalf("the estimate priced more than observation: %+v", plan.Calls)
		}
	}
	test, err := experiment.BuildWritingTest(start, plan, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	work := experiment.TestExecutionWork{Test: test, Plan: plan, CandidateIndices: map[string]int{}, CandidateCheckpoints: map[string][]byte{}}
	for _, c := range test.Candidates {
		work.CandidateIDs = append(work.CandidateIDs, c.ID)
		work.CandidateIndices[c.ID] = c.SnapshotIndex
	}
	store.work = work
	shared := prepareAdapterInput(t, adapter, store)
	for _, candidate := range test.Candidates {
		result, err := adapter.RunTestCandidate(context.Background(), store.work, candidate.ID, shared, saveAdapterCheckpoint(store, candidate.ID), nil)
		if err != nil {
			t.Fatal(err)
		}
		output, err := experiment.DecodeTestOutput(result.Output)
		if err != nil {
			t.Fatal(err)
		}
		observer := candidate.Ref.Model.ModelID
		if len(output.Observations) != 3 || output.Content.Title != "" || output.Storyline != nil || len(result.Accounting) == 0 {
			t.Fatalf("candidate %s output=%+v", observer, output)
		}
		for _, o := range output.Observations {
			if o.Scene != "seen by "+observer || o.Model != "approved/"+observer {
				t.Fatalf("candidate %s stored another observer's observation: %+v", observer, o)
			}
		}
	}
	if len(models.calls) != 4 {
		t.Fatalf("calls=%d", len(models.calls))
	}
}
