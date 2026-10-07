package main

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/auth"
	authoringrpc "github.com/postpilot/backend/internal/authoring/rpc"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/generation"
)

func TestWritingTestShapeRefusesBeforeAnyAdmission(t *testing.T) {
	valid := experiment.TestStart{UserID: "alice", RequestKey: "request", Factor: experiment.FactorModel, ModelStage: experiment.StageWrite, Count: 2, Entrants: []experiment.TestEntrantRef{{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "registry", ModelID: "one"}}, {SourceKind: "model", Model: experiment.ModelRef{ProviderID: "registry", ModelID: "two"}}}}
	if err := experiment.ValidateTestShape(valid); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 1, 3, 5, 7, 9, 17} {
		r := valid
		r.Count = count
		if !errors.Is(experiment.ValidateTestShape(r), experiment.ErrTestCount) {
			t.Fatalf("count %d was admitted", count)
		}
	}
	for _, count := range []int{2, 4, 8, 16} {
		r := valid
		r.Count = count
		r.Entrants = nil
		for i := range count {
			r.Entrants = append(r.Entrants, experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "registry", ModelID: fmt.Sprint(i)}})
		}
		if err := experiment.ValidateTestShape(r); err != nil {
			t.Fatalf("count %d: %v", count, err)
		}
	}
	r := valid
	r.Entrants = append([]experiment.TestEntrantRef(nil), valid.Entrants...)
	r.Entrants[1] = r.Entrants[0]
	if !errors.Is(experiment.ValidateTestShape(r), experiment.ErrTestDuplicate) {
		t.Fatal("duplicate entrant admitted")
	}
	r = valid
	r.Factor = experiment.FactorTemplate
	if !errors.Is(experiment.ValidateTestShape(r), experiment.ErrTestFactor) {
		t.Fatal("model stage on a setting factor admitted")
	}
	r = valid
	r.RequestKey = ""
	if !errors.Is(experiment.ValidateTestShape(r), experiment.ErrTestOperation) {
		t.Fatal("empty operation key admitted")
	}
	r = valid
	r.Entrants = append([]experiment.TestEntrantRef(nil), valid.Entrants...)
	r.Entrants[0].SettingID = "smuggled"
	if !errors.Is(experiment.ValidateTestShape(r), experiment.ErrTestEntrant) {
		t.Fatal("mixed references admitted")
	}
}
func TestFrozenWritingTestEnumMirrorsCoverGeneratedValues(t *testing.T) {
	for n, name := range v1.WritingTestFactor_name {
		f := experiment.TestFactor(strings.ToLower(strings.TrimPrefix(name, "WRITING_TEST_FACTOR_")))
		if f.Valid() != (n != 0) {
			t.Fatalf("factor mapping missing: %s", name)
		}
	}
	for n, name := range v1.WritingTestStatus_name {
		s := experiment.TestStatus(strings.ToLower(strings.TrimPrefix(name, "WRITING_TEST_STATUS_")))
		if s.Valid() != (n != 0) {
			t.Fatalf("status mapping missing: %s", name)
		}
	}
	for n, name := range v1.WritingTestCandidateStatus_name {
		status := experiment.TestCandidateStatus(strings.ToLower(strings.TrimPrefix(name, "WRITING_TEST_CANDIDATE_STATUS_")))
		if status.Valid() != (n != 0) {
			t.Fatalf("candidate status mapping missing: %s", name)
		}
	}
	for n, name := range v1.WritingTestPublicationAction_name {
		action := experiment.TestPublicationAction(strings.ToLower(strings.TrimPrefix(name, "WRITING_TEST_PUBLICATION_ACTION_")))
		if action.Valid() != (n != 0) {
			t.Fatalf("publication action mapping missing: %s", name)
		}
	}
	for n, name := range v1.WritingTestPublicationStatus_name {
		status := experiment.TestPublicationStatus(strings.ToLower(strings.TrimPrefix(name, "WRITING_TEST_PUBLICATION_STATUS_")))
		if status.Valid() != (n != 0) {
			t.Fatalf("publication status mapping missing: %s", name)
		}
	}
	for n, name := range v1.WritingTestStage_name {
		_, err := experiment.ParseStage(strings.ToLower(strings.TrimPrefix(name, "WRITING_TEST_STAGE_")))
		if (err == nil) != (n != 0) {
			t.Fatalf("stage mapping missing: %s", name)
		}
	}

	for n, name := range v1.AuthoringDraftState_name {
		state := authoring.DraftState(strings.ToLower(strings.TrimPrefix(name, "AUTHORING_DRAFT_STATE_")))
		valid := state == authoring.DraftValid || state == authoring.DraftIncomplete || state == authoring.DraftInvalid
		if valid != (n != 0) {
			t.Fatalf("draft mapping missing: %s", name)
		}
	}
}
func TestOrdinaryAuthoringCountDefaultIsBackwardCompatible(t *testing.T) {
	// Preparation may fill any exact number of missing slots. Its omitted count
	// retains the ordinary default; tournament formats stay independently bounded.
	for n := 0; n <= 16; n++ {
		result, err := authoring.NormalizeCandidateCount(n)
		if err != nil || (n == 0 && result != 8) {
			t.Fatalf("count %d: %d %v", n, result, err)
		}
	}
	for _, n := range []int{-1, 17} {
		if _, err := authoring.NormalizeCandidateCount(n); err == nil {
			t.Fatalf("count %d accepted", n)
		}
	}
}
func TestBlindWritingTestProjectionCannotExposePrivateCandidateData(t *testing.T) {
	c := experiment.TestCandidate{ID: "opaque", UserID: "alice", SourceRevision: "private", SeedPosition: 7, Ref: experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ModelID: "secret"}}, FrozenVariant: []byte("private prose"), Accounting: []byte("supplier cost"), Usage: &experiment.Usage{PromptTokens: 120, CompletionTokens: 45, LatencyMS: 678, CostMicrousd: 999, CostSource: experiment.CostReported}, Output: []byte("readable post"), Identity: &experiment.TestCandidateIdentity{Label: "secret model"}, Failure: &experiment.Failure{Reason: "UNKNOWN_FAILURE", Params: map[string]string{"model": "secret"}, TechnicalDetail: "provider secret"}}
	blind := c.Project(false, "A")
	if blind.Identity != nil || blind.Usage != nil || len(blind.Failure.Params) != 0 || blind.Failure.TechnicalDetail != "" {
		t.Fatal("blind result exposed identity or usage")
	}
	blind.Output[0] = 'x'
	if string(c.Output) != "readable post" {
		t.Fatal("projection shares mutable output")
	}
	if c.Project(true, "A").Identity.Label != "secret model" {
		t.Fatal("completed identity unavailable")
	}
}

func TestWritingTestRevealedUsagePreservesFailedCallEvidenceWithoutSupplierCosts(t *testing.T) {
	publicFields := reflect.TypeFor[experiment.TestCandidateUsage]()
	var names []string
	for i := range publicFields.NumField() {
		names = append(names, publicFields.Field(i).Name)
	}
	if !reflect.DeepEqual(names, []string{"PromptTokens", "CompletionTokens", "LatencyMS"}) {
		t.Fatalf("public usage includes unexpected fields: %v", names)
	}
	for _, status := range []experiment.TestCandidateStatus{experiment.TestCandidateSucceeded, experiment.TestCandidateFailed, experiment.TestCandidateCancelled} {
		t.Run(string(status), func(t *testing.T) {
			c := experiment.TestCandidate{Status: string(status), Accounting: []byte("private supplier cost"), Usage: &experiment.Usage{PromptTokens: 120, CompletionTokens: 45, LatencyMS: 678, CostMicrousd: 999, CostSource: experiment.CostReported}}
			if c.Project(false, "A").Usage != nil {
				t.Fatal("blind projection exposed usage")
			}
			// Usage remains readable after abandonment even if the historical identity is absent.
			revealed := c.Project(true, "A")
			want := experiment.TestCandidateUsage{PromptTokens: 120, CompletionTokens: 45, LatencyMS: 678}
			if revealed.Usage == nil || *revealed.Usage != want {
				t.Fatalf("revealed call evidence lost: %#v", revealed.Usage)
			}
			revealed.Usage.PromptTokens = 0
			if c.Project(true, "A").Usage.PromptTokens != 120 {
				t.Fatal("projection shares mutable accounting")
			}
		})
	}
	if (experiment.TestCandidate{}).Project(true, "A").Usage != nil {
		t.Fatal("reveal fabricated usage for an unissued call")
	}
}

func TestAuthoringCountsRejectMissingStartBeforeProviderWork(t *testing.T) {
	// T625 activates 2/4/8/16. Unsupported counts and missing explicit start
	// fields still refuse before service/provider work; a nil service pins that fence.
	handler := authoringrpc.NewHandler(nil)
	for _, count := range []int32{1, 3, 5, 17, 2, 4, 8, 16} {
		_, err := handler.StartAuthoringOperation(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: count}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("count %d admitted an incomplete request: %v", count, err)
		}
	}
	_, err := handler.StartAuthoringOperation(context.Background(), connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: 3}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("count validation bypassed authentication")
	}
}

func TestInvalidStyleCountsRefuseBeforeProviderWork(t *testing.T) {
	handler := voicerpc.NewCandidateHandler(nil)
	for _, count := range []int32{1, 3, 5, 17} {
		_, err := handler.StartWritingVoiceCandidates(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.StartWritingVoiceCandidatesRequest{CandidateCount: count}))
		if err == nil {
			t.Fatalf("count %d started baseline work", count)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("count %d: %v", count, err)
		}
	}
}

func TestFrozenRefusalContractsUseRegisteredParameterFreeReasons(t *testing.T) {
	refusals := []interface {
		error
		Reason() string
		Params() map[string]string
	}{authoring.ErrDraftInvalid, voice.ErrSampleRevisionConflict, voice.ErrSampleUpdateInvalid, voice.ErrCheckRetired, experiment.ErrTestCount, experiment.ErrTestFactor, experiment.ErrTestEntrant, experiment.ErrTestDuplicate, experiment.ErrTestOperation, experiment.ErrTestNotFound, experiment.ErrTestRevisionConflict, experiment.ErrTestMatchInvalid, experiment.ErrTestDecisionConflict, experiment.ErrTestStateInvalid, experiment.ErrTestPublicationConflict, experiment.ErrTestOutputIncompatible, experiment.ErrTestQuoteRequired, experiment.ErrTestRunning, experiment.ErrTestMaterialInvalid, experiment.ErrTestLegacyReadOnly}
	for _, refusal := range refusals {
		if _, ok := v1.FailureReason_value[refusal.Reason()]; !ok || len(refusal.Params()) != 0 || refusal.Error() == "" {
			t.Fatalf("invalid frozen refusal: %v", refusal)
		}
	}
}

// Exercise the published generation port as its experiment consumer sees it.
type testSnapshotWriter struct{ answer generation.WriteAnswer }

func (w testSnapshotWriter) FreezeWritingTest(context.Context, generation.WritingTestSnapshotRequest) (generation.WritingTestSnapshot, error) {
	return generation.WritingTestSnapshot{}, nil
}
func (w testSnapshotWriter) WriteTestEntrant(context.Context, generation.WritingTestSnapshot, int, *generation.WritingTestCheckpoint, generation.WritingTestRunOptions) (generation.WriteAnswer, error) {
	return w.answer, nil
}
func TestFrozenGenerationResultCarriesStorylineAndNounsToItsConsumer(t *testing.T) {
	var producer generation.WritingTestSnapshots = testSnapshotWriter{answer: generation.WriteAnswer{Content: generation.PostContent{Title: "complete candidate"}, Nouns: []string{"owned scene"}, Storyline: &generation.Storyline{Paragraphs: []generation.StorylineParagraph{{Text: "candidate-specific plan", Files: []string{"owned.jpg"}}}, MadeWith: []string{"owned.jpg"}}}}
	answer, err := producer.WriteTestEntrant(context.Background(), generation.WritingTestSnapshot{}, 0, nil, generation.WritingTestRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	annotations := answer.Annotations()
	if answer.Content.Title != "complete candidate" || annotations.Storyline == nil || annotations.Storyline.Paragraphs[0].Text != "candidate-specific plan" || annotations.Storyline.MadeWith[0] != "owned.jpg" || annotations.Nouns[0] != "owned scene" {
		t.Fatalf("generation consumer lost annotations: %#v", answer)
	}
}
func TestFailedRetryQuoteShapeDoesNotChangeTheOriginalTestFormat(t *testing.T) {
	request := experiment.TestRetryQuoteRequest{UserID: "alice", TestID: "four-entry-test", ExpectedRevision: 3, CandidateIDs: []string{"failed-one"}}
	if err := experiment.ValidateRetryQuoteShape(request); err != nil {
		t.Fatal(err)
	}
	request.CandidateIDs = []string{"failed-one", "failed-one"}
	if !errors.Is(experiment.ValidateRetryQuoteShape(request), experiment.ErrTestDuplicate) {
		t.Fatal("duplicate retry admitted")
	}
	request.CandidateIDs = nil
	if !errors.Is(experiment.ValidateRetryQuoteShape(request), experiment.ErrTestOperation) {
		t.Fatal("empty retry admitted")
	}
	request.CandidateIDs = []string{"failed-one"}
	request.UserID = ""
	if !errors.Is(experiment.ValidateRetryQuoteShape(request), experiment.ErrTestOperation) {
		t.Fatal("unowned retry admitted")
	}
}
