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
	for _, count := range []int32{1, 3, 17} {
		_, err := h.StartAuthoringOperation(ctx, connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: count}))
		if errorDetail(t, err).Reason != "AUTHORING_CANDIDATE_COUNT_INVALID" {
			t.Fatal("invalid count admitted")
		}
	}
	for _, count := range []int32{2, 4, 8, 16} {
		_, err := h.StartAuthoringOperation(ctx, connect.NewRequest(&v1.StartAuthoringOperationRequest{CandidateCount: count}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatal("missing explicit request admitted", err)
		}
	}
}
