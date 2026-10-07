package main

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/auth"
	authoringrpc "github.com/postpilot/backend/internal/authoring/rpc"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/authoring"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
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
	for _, n := range []int{0, 2, 4, 8, 16} {
		result, err := authoring.NormalizeCandidateCount(n)
		if err != nil || (n == 0 && result != 8) {
			t.Fatalf("count %d: %d %v", n, result, err)
		}
	}
	for _, n := range []int{-1, 1, 3, 5, 17} {
		if _, err := authoring.NormalizeCandidateCount(n); err == nil {
			t.Fatalf("count %d accepted", n)
		}
	}
}
func TestBlindWritingTestProjectionCannotExposePrivateCandidateData(t *testing.T) {
	c := experiment.TestCandidate{ID: "opaque", UserID: "alice", SourceRevision: "private", SeedPosition: 7, Ref: experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ModelID: "secret"}}, FrozenVariant: []byte("private prose"), Accounting: []byte("supplier cost"), Output: []byte("readable post"), Identity: &experiment.TestCandidateIdentity{Label: "secret model"}, Failure: &experiment.Failure{Reason: "UNKNOWN_FAILURE", Params: map[string]string{"model": "secret"}, TechnicalDetail: "provider secret"}}
	blind := c.Project(false, "A")
	if blind.Identity != nil || len(blind.Failure.Params) != 0 || blind.Failure.TechnicalDetail != "" {
		t.Fatal("blind result exposed identity")
	}
	blind.Output[0] = 'x'
	if string(c.Output) != "readable post" {
		t.Fatal("projection shares mutable output")
	}
	if c.Project(true, "A").Identity.Label != "secret model" {
		t.Fatal("completed identity unavailable")
	}
}

func TestUnintegratedCountsNeverReachBaselineProviderWork(t *testing.T) {
	// A nil service would panic if these count gates admitted existing eight-output execution.
	handler := authoringrpc.NewHandler(nil)
	for _, count := range []int32{1, 3, 5, 17, 2, 4, 16} {
		_, err := handler.StartAuthoringOperation(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: count}))
		if err == nil {
			t.Fatalf("count %d started baseline work", count)
		}
		if count == 2 || count == 4 || count == 16 {
			if connect.CodeOf(err) != connect.CodeUnimplemented {
				t.Fatalf("count %d: %v", count, err)
			}
		} else if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("count %d: %v", count, err)
		}
	}
	_, err := handler.StartAuthoringOperation(context.Background(), connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: 3}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("count gate bypassed authentication")
	}
}
