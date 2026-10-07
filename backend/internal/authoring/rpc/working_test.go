package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"testing"
)

func TestWorkingSourceProceduresFenceAuthenticationBeforeServiceAccess(t *testing.T) {
	h := NewHandler(nil)
	ctx := context.Background()
	calls := []func() error{
		func() error {
			_, err := h.PatchAuthoringDraft(ctx, connect.NewRequest(&v1.PatchAuthoringDraftRequest{}))
			return err
		},
		func() error {
			_, err := h.ResetAuthoringChat(ctx, connect.NewRequest(&v1.ResetAuthoringChatRequest{}))
			return err
		},
		func() error {
			_, err := h.ResetAuthoringBaseline(ctx, connect.NewRequest(&v1.ResetAuthoringBaselineRequest{}))
			return err
		},
		func() error {
			_, err := h.ListAuthoringSummaries(ctx, connect.NewRequest(&v1.ListAuthoringSummariesRequest{}))
			return err
		},
	}
	for _, call := range calls {
		err := call()
		if connect.CodeOf(err) != connect.CodeUnauthenticated || errorDetail(t, err).Reason != "AUTH_REQUIRED" {
			t.Fatal("working-source procedure bypassed authentication", err)
		}
	}
}
func TestDraftStateMappingPinsEveryFrozenEnumAndSessionFacts(t *testing.T) {
	expected := map[authoring.DraftState]v1.AuthoringDraftState{"": v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_UNSPECIFIED, authoring.DraftValid: v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_VALID, authoring.DraftInvalid: v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_INVALID, authoring.DraftIncomplete: v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_INCOMPLETE}
	if len(expected) != len(v1.AuthoringDraftState_name) {
		t.Fatal("draft enum changed without mapper review")
	}
	for state, wire := range expected {
		if draftState(state) != wire {
			t.Fatal("draft state drift")
		}
	}
	baseline := authoring.Artifact{ID: "draft", Name: "saved", Body: "valid", Revision: 1}
	source := baseline
	source.Body = ""
	result := session(&authoring.Session{ID: "session", Kind: authoring.PostTemplate, Revision: 4, Phase: "editing", Selected: &baseline, SavedBaseline: &baseline, WorkingSource: &source, DraftState: authoring.DraftIncomplete, HasUnpublishedChanges: true, SavedAvailable: true, RequestedCandidateCount: 16})
	if result.WorkingSource == nil || result.WorkingSource.Body != "" || result.Selected.Body != "valid" || result.SavedBaseline.Body != "valid" || !result.SavedAvailable || !result.HasUnpublishedChanges || result.CandidateCount != 16 {
		t.Fatal("session collapsed working source/preview/saved availability")
	}
}
func TestInvalidCountsAndMissingExplicitStartNeverReachService(t *testing.T) {
	h := NewHandler(nil)
	ctx := auth.WithUser(context.Background(), "alice")
	for _, count := range []int32{-1, 17} {
		_, err := h.StartAuthoringOperation(ctx, connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: count}))
		if errorDetail(t, err).Reason != "AUTHORING_CANDIDATE_COUNT_INVALID" {
			t.Fatal("invalid count admitted")
		}
		_, err = h.EstimateAuthoringOperation(ctx, connect.NewRequest(&v1.EstimateAuthoringOperationRequest{CandidateCount: count}))
		if errorDetail(t, err).Reason != "AUTHORING_CANDIDATE_COUNT_INVALID" {
			t.Fatal("invalid count reached estimation")
		}
	}
	for count := int32(0); count <= authoring.MaxCandidateCount; count++ {
		_, err := h.StartAuthoringOperation(ctx, connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: count}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatal("missing explicit request admitted", err)
		}
	}
}

// A baseline-only adapter may serve older durable reads but cannot accept the
// working-draft contract. This is server capability, not an invalid owner input.
type baselineOnlyStore struct{ authoring.Store }

type unavailableBoundaryPorts struct {
	authoring.Models
	authoring.Jobs
	authoring.Targets
	authoring.Budget
	authoring.Estimator
}

func TestUnavailableDurableDraftBoundaryPreservesAuthenticationAndDoesNoWork(t *testing.T) {
	ports := unavailableBoundaryPorts{}
	h := NewHandler(authoring.NewService(baselineOnlyStore{}, ports, ports, ports, ports, ports))
	request := connect.NewRequest(&v1.ListAuthoringSummariesRequest{Kind: v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE})
	_, err := h.ListAuthoringSummaries(context.Background(), request)
	if connect.CodeOf(err) != connect.CodeUnauthenticated || errorDetail(t, err).Reason != "AUTH_REQUIRED" {
		t.Fatal("capability refusal preceded authentication", err)
	}
	_, err = h.ListAuthoringSummaries(auth.WithUser(context.Background(), "alice"), request)
	if connect.CodeOf(err) != connect.CodeUnimplemented || errorDetail(t, err).Reason != "AUTHORING_FEATURE_UNAVAILABLE" {
		t.Fatal("missing server capability blamed user input", err)
	}
}
