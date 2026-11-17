package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestWritingTestTransportReadsOutputAndPhotos(t *testing.T) {
	post := rpcTestPlan(v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL, 2)
	decoded, err := writingStartFromProto("alice", post)
	if err != nil || decoded.Output != experiment.OutputPost {
		t.Fatalf("an unspecified output reads as a post: %q %v", decoded.Output, err)
	}
	post.Context.PhotoIds = []string{"photo"}
	if _, err := writingStartFromProto("alice", post); !errors.Is(err, experiment.ErrTestMaterialInvalid) {
		t.Fatalf("a post test refuses photo ids: %v", err)
	}
	observe := rpcTestPlan(v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL, 2)
	observe.Output = v1.WritingTestOutput_WRITING_TEST_OUTPUT_OBSERVATION
	observe.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_OBSERVE
	observe.Context.Material, observe.Context.TagCount = nil, 0
	observe.Context.PhotoIds = []string{"one", "two"}
	decoded, err = writingStartFromProto("alice", observe)
	if err != nil || decoded.Output != experiment.OutputObservation || len(decoded.Input.PhotoIDs) != 2 {
		t.Fatalf("observation decoded=%+v err=%v", decoded, err)
	}
	observe.Output = v1.WritingTestOutput(9)
	if _, err := writingStartFromProto("alice", observe); !errors.Is(err, experiment.ErrTestFactor) {
		t.Fatalf("an unknown output is refused: %v", err)
	}
}

func TestObservationTestWireListsPhotosAndBlindObservations(t *testing.T) {
	raw, err := experiment.EncodeTestOutput(experiment.TestOutput{Observations: []experiment.TestOutputObservation{{File: "one.jpg", Scene: "국수", Objects: []string{"그릇"}, Model: "p/private-observer"}}})
	if err != nil {
		t.Fatal(err)
	}
	found := rpcWritingTest()
	found.Output, found.ModelStage, found.Status = experiment.OutputObservation, experiment.StageObserve, experiment.TestReview
	found.Photos = []experiment.TestPhoto{{ID: "one", Filename: "one.jpg", Key: "tests/alice/one", Width: 1024, Height: 768}}
	found.Candidates = []experiment.TestCandidate{{ID: "opaque", Status: "succeeded", Output: raw, Identity: &experiment.TestCandidateIdentity{Label: "observer", Ref: experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "p", ModelID: "observer"}}}}}
	signed := 0
	view := func(_ context.Context, key string) (string, time.Time, error) {
		signed++
		return "https://signed/" + key, time.Unix(0, 0), nil
	}
	wire, err := writingTestToProto(context.Background(), found, view)
	if err != nil {
		t.Fatal(err)
	}
	if wire.GetOutput() != v1.WritingTestOutput_WRITING_TEST_OUTPUT_OBSERVATION || len(wire.Photos) != 1 || wire.Photos[0].GetViewUrl() != "https://signed/tests/alice/one" || wire.Photos[0].GetWidth() != 1024 {
		t.Fatalf("photos %+v", wire.Photos)
	}
	candidate := wire.Candidates[0]
	if candidate.Output != nil || len(candidate.Observations) != 1 || candidate.Observations[0].GetScene() != "국수" || candidate.Observations[0].GetModel() != "" {
		t.Fatalf("blind observation %+v", candidate)
	}
	found.Status = experiment.TestCompleted
	wire, err = writingTestToProto(context.Background(), found, view)
	if err != nil || wire.Candidates[0].Observations[0].GetModel() != "p/observer" {
		t.Fatalf("revealed observation names its observer: %v %+v", err, wire.Candidates)
	}
	found.PurgeFence, found.Candidates = 1, nil
	before := signed
	wire, err = writingTestToProto(context.Background(), found, view)
	if err != nil || signed != before || wire.Photos[0].GetViewUrl() != "" {
		t.Fatalf("a purged payload signs no photo: %v %+v", err, wire.Photos)
	}
	post, err := writingTestToProto(context.Background(), rpcWritingTest(), view)
	if err != nil || post.GetOutput() != v1.WritingTestOutput_WRITING_TEST_OUTPUT_POST || len(post.Photos) != 0 {
		t.Fatalf("a post test: %v %+v", err, post)
	}
}
