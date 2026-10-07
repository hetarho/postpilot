package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
)

type testInspectionStore struct {
	work  experiment.TestExecutionWork
	reads int
}

func (s *testInspectionStore) ReadTestInspectionWork(_ context.Context, user, test string) (experiment.TestExecutionWork, error) {
	s.reads++
	if s.work.Test.UserID != user || s.work.Test.ID != test {
		return experiment.TestExecutionWork{}, experiment.ErrTestNotFound
	}
	return s.work, nil
}

func testCapture(stage, id string) llm.RequestInspection {
	issued := time.Now().UTC()
	return llm.RequestInspection{Version: 1, Status: llm.InspectionCaptured, Stage: stage, Mode: "post-writing", PromptVersion: "v1", SchemaVersion: "v1", CallID: id, IssuedAt: &issued, Output: llm.OutputContractInspection{Name: "post", Version: "v1"}, Conditions: &llm.EffectiveRequestConditions{Model: &llm.ModelRef{ProviderID: "private-provider", ModelID: "private-model"}}, Fragments: []llm.RequestFragment{{ID: "private-model-fragment", Role: llm.InspectionRoleUser, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "private-setting", Text: "private instructions", SourceFiles: []string{"private-source"}}}, Omissions: []llm.RequestOmission{{ID: "private-model-setting", Reason: "private exclusion"}}}
}

func inspectionWork(t *testing.T) experiment.TestExecutionWork {
	t.Helper()
	shared, err := encodeTestCheckpoint(generation.WritingTestCheckpoint{SnapshotHash: "hash", Index: -1, RequestInspections: []llm.RequestInspection{testCapture("observe", "photo1"), testCapture("observe", "photo2")}})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := encodeTestCheckpoint(generation.WritingTestCheckpoint{SnapshotHash: "hash", Index: 0, RequestInspections: []llm.RequestInspection{testCapture("write", "writer")}})
	if err != nil {
		t.Fatal(err)
	}
	output, _ := experiment.EncodeTestOutput(experiment.TestOutput{ContentLanguage: "en", Content: experiment.TestOutputContent{Title: "complete"}, RequestInspections: []llm.RequestInspection{testCapture("write", "writer")}})
	return experiment.TestExecutionWork{Test: experiment.WritingTest{ID: "test", UserID: "owner", Status: experiment.TestCompleted, CommonHash: "hash", Candidates: []experiment.TestCandidate{{ID: "candidate", SnapshotIndex: 0, Output: output}}}, SharedCheckpoint: shared, CandidateCheckpoints: map[string][]byte{"candidate": candidate}}
}

func TestTestInspectionDeniesPrivateIdentityThroughEveryBlindStateAndView(t *testing.T) {
	for _, state := range []experiment.TestStatus{experiment.TestQueued, experiment.TestRunning, experiment.TestPartial, experiment.TestFailed, experiment.TestReview} {
		for _, status := range []llm.InspectionStatus{llm.InspectionCurrent, llm.InspectionPrepared, llm.InspectionCaptured} {
			store := &testInspectionStore{work: inspectionWork(t)}
			store.work.Test.Status = state
			reader := NewWritingTestInspection(store)
			values, err := reader.ReadTestRequestInspections(context.Background(), "owner", "test", "candidate", "write", status)
			if err != nil || len(values) != 1 || values[0].Status != llm.InspectionUnavailable || values[0].UnavailableReason != "blind_test_identity_hidden_until_reveal" {
				t.Fatalf("%s/%s: %v, %#v", state, status, err, values)
			}
			raw, _ := json.Marshal(values)
			for _, secret := range []string{"private", "model", "fragment", "source", "exclusion", "photo1", "writer"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("blind hidden wire leaked %s: %s", secret, raw)
				}
			}
		}
	}
}

func TestTestInspectionReturnsAllMatchingCallsAfterRevealAndFencesBadBindings(t *testing.T) {
	for _, state := range []experiment.TestStatus{experiment.TestCompleted, experiment.TestCancelled} {
		store := &testInspectionStore{work: inspectionWork(t)}
		store.work.Test.Status = state
		reader := NewWritingTestInspection(store)
		observations, err := reader.ReadTestRequestInspections(context.Background(), "owner", "test", "candidate", "post-observation", llm.InspectionCaptured)
		if err != nil || len(observations) != 2 || observations[0].CallID != "photo1" || observations[1].CallID != "photo2" {
			t.Fatalf("shared captures: %v, %#v", err, observations)
		}
		writing, err := reader.ReadTestRequestInspections(context.Background(), "owner", "test", "candidate", "write", llm.InspectionCaptured)
		if err != nil || len(writing) != 1 || writing[0].CallID != "writer" || writing[0].Conditions.Model.ModelID != "private-model" {
			t.Fatalf("deduplicated captures: %v, %#v", err, writing)
		}
		writing[0].Fragments[0].Text = "mutated"
		again, _ := reader.ReadTestRequestInspection(context.Background(), "owner", "test", "candidate", "write", llm.InspectionCaptured)
		if again.Fragments[0].Text != "private instructions" {
			t.Fatal("capture returned private mutable alias")
		}
		store.work.SharedCheckpoint, _ = encodeTestCheckpoint(generation.WritingTestCheckpoint{SnapshotHash: "foreign", Index: -1, RequestInspections: []llm.RequestInspection{testCapture("observe", "foreign")}})
		absent, err := reader.ReadTestRequestInspection(context.Background(), "owner", "test", "candidate", "observe", llm.InspectionCaptured)
		if err != nil || absent.Status != llm.InspectionUnavailable {
			t.Fatalf("stale snapshot leaked: %v %#v", err, absent)
		}
		for _, status := range []llm.InspectionStatus{llm.InspectionCurrent, llm.InspectionPrepared} {
			absent, err := reader.ReadTestRequestInspection(context.Background(), "owner", "test", "candidate", "write", status)
			if err != nil || absent.Status != llm.InspectionUnavailable || absent.IssuedAt != nil {
				t.Fatalf("capture mislabeled preview: %v %#v", err, absent)
			}
		}
	}
}

func TestTestInspectionOwnerCandidateExpiryAndPurgeFences(t *testing.T) {
	store := &testInspectionStore{work: inspectionWork(t)}
	reader := NewWritingTestInspection(store)
	for _, scope := range []struct{ user, test, candidate string }{{"foreign", "test", "candidate"}, {"owner", "unknown", "candidate"}, {"owner", "test", "foreign-candidate"}} {
		_, err := reader.ReadTestRequestInspection(context.Background(), scope.user, scope.test, scope.candidate, "write", llm.InspectionCaptured)
		if !errors.Is(err, experiment.ErrTestNotFound) {
			t.Fatalf("scope disclosed: %v", err)
		}
	}
	for _, fence := range []string{"expiry", "purge"} {
		store.work = inspectionWork(t)
		if fence == "expiry" {
			at := time.Now().Add(-time.Second)
			store.work.Test.ContentExpiresAt = &at
		} else {
			store.work.Test.PurgeFence = 1
		}
		value, err := reader.ReadTestRequestInspection(context.Background(), "owner", "test", "candidate", "write", llm.InspectionCaptured)
		if err != nil || value.Status != llm.InspectionUnavailable || len(value.Fragments) != 0 {
			t.Fatalf("%s resurrected: %v %#v", fence, err, value)
		}
	}
}
