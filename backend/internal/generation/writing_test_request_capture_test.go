package generation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestWritingTestCapturesPrivateSharedAndCandidateCallsBeforeCanonicalParsing(t *testing.T) {
	models := newFakeModels()
	posts := &fakePosts{}
	factory := runTestFactory(models, posts)
	postCapture := &inspectionCaptureFixture{}
	factory.service.inspection = &RequestInspectionDependencies{Captures: postCapture}
	images := []Image{{ID: "photo-one", Filename: "one.jpg", Key: "private-key"}, {ID: "photo-two", Filename: "two.jpg", Key: "private-key-two"}, {ID: "photo-three", Filename: "three.jpg", Key: "private-key-three"}}
	snapshot := runTestSnapshot(t, "template", "", 2, images)
	stored, save := checkpointStore()
	models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
		if request.Stage == "observe" {
			return fixtureIssuedResponse(t, request, observationAnswer(request)), nil
		}
		// A malformed answer still has honest execution evidence.
		return fixtureIssuedResponse(t, request, llm.Response{Text: "malformed canonical answer"}), nil
	}
	prepared, err := factory.PrepareWritingTestInput(t.Context(), snapshot, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil || len(prepared.Checkpoint.RequestInspections) != 2 || len(stored[-1].RequestInspections) != 2 {
		t.Fatalf("shared capture unavailable: %+v %v", prepared.Checkpoint, err)
	}
	result, err := factory.RunWritingTestCandidate(t.Context(), snapshot, 0, &prepared.Checkpoint, WritingTestRunOptions{SaveCheckpoint: save})
	if err == nil || len(result.Checkpoint.RequestInspections) != 1 || result.Checkpoint.Answer != nil || result.Checkpoint.FailedStage != "write" {
		t.Fatalf("parse failure lost actual capture: %+v %v", result.Checkpoint, err)
	}
	first := prepared.Checkpoint.RequestInspections[0]
	if first.Status != llm.InspectionCaptured || len(first.Attachments) != 2 || first.Attachments[0].ID != "photo-one" || first.CallID == result.Checkpoint.RequestInspections[0].CallID {
		t.Fatal("batch or candidate identities were retargeted")
	}
	for _, inspection := range append(prepared.Checkpoint.RequestInspections, result.Checkpoint.RequestInspections...) {
		for _, fragment := range inspection.Fragments {
			if strings.Contains(fragment.Text, "private-key") {
				t.Fatal("object capability entered private inspection")
			}
		}
	}
	prepared.Checkpoint.RequestInspections[0].Fragments[0].Text = "changed returned copy"
	if stored[-1].RequestInspections[0].Fragments[0].Text == "changed returned copy" || len(postCapture.calls) != 0 || len(postCapture.finished) != 0 || posts.reads != 0 {
		t.Fatal("private test capture shared memory or touched the source post")
	}
}

func TestWritingTestRequestCaptureFencesPurgeAndRetainsFailuresWithoutGuessing(t *testing.T) {
	for _, variant := range []string{"provider failure", "missing witness", "purged callback"} {
		t.Run(variant, func(t *testing.T) {
			models := newFakeModels()
			factory := runTestFactory(models, &fakePosts{})
			snapshot := runTestSnapshot(t, "template", "", 2, nil)
			_, save := checkpointStore()
			fenced := errors.New("private payload purged")
			models.complete = func(_ llm.ModelRef, request llm.Request) (llm.Response, error) {
				if variant == "missing witness" {
					return runTestAnswer(), nil
				}
				response := fixtureIssuedResponse(t, request, runTestAnswer())
				if variant == "provider failure" {
					return response, llm.WithRequestInspectionError(llm.ErrRateLimited, *response.Inspection)
				}
				return response, nil
			}
			wrappedSave := func(ctx context.Context, cp WritingTestCheckpoint) error {
				if variant == "purged callback" && len(cp.RequestInspections) > 0 {
					return fenced
				}
				return save(ctx, cp)
			}
			result, err := factory.RunWritingTestCandidate(t.Context(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: wrappedSave})
			if len(models.calls) != 1 {
				t.Fatal("capture retried the provider")
			}
			if variant == "purged callback" {
				if !errors.Is(err, fenced) || result.Checkpoint.Answer != nil {
					t.Fatal("purged callback published the answer", err)
				}
				return
			}
			if len(result.Checkpoint.RequestInspections) != 1 {
				t.Fatal("attempt evidence missing")
			}
			inspection := result.Checkpoint.RequestInspections[0]
			if variant == "missing witness" {
				if err != nil || inspection.Status != llm.InspectionUnavailable || len(inspection.Fragments) != 0 || inspection.Conditions != nil {
					t.Fatal("missing historical capture was reconstructed", err)
				}
			} else if !errors.Is(err, llm.ErrRateLimited) || inspection.Status != llm.InspectionCaptured {
				t.Fatal("failed issued call lost captured evidence", err)
			}
		})
	}
}
