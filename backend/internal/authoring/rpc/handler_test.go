package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func errorDetail(t *testing.T, err error) *v1.AppErrorDetail {
	t.Helper()
	var failure *connect.Error
	if !errors.As(err, &failure) || len(failure.Details()) != 1 {
		t.Fatalf("error missing one product detail: %v", err)
	}
	value, e := failure.Details()[0].Value()
	if e != nil {
		t.Fatal(e)
	}
	valueDetail, ok := value.(*v1.AppErrorDetail)
	if !ok {
		t.Fatalf("unexpected detail %T", value)
	}
	return valueDetail
}

func TestAllProceduresRequireAuthenticatedOwnerBeforeServiceAccess(t *testing.T) {
	h := NewHandler(nil)
	ctx := context.Background()
	calls := []func() error{
		func() error {
			_, e := h.CreateAuthoringSession(ctx, connect.NewRequest(&v1.CreateAuthoringSessionRequest{}))
			return e
		},
		func() error {
			_, e := h.GetAuthoringSession(ctx, connect.NewRequest(&v1.GetAuthoringSessionRequest{}))
			return e
		},
		func() error {
			_, e := h.GetLatestAuthoringSession(ctx, connect.NewRequest(&v1.GetLatestAuthoringSessionRequest{}))
			return e
		},
		func() error {
			_, e := h.EstimateAuthoringOperation(ctx, connect.NewRequest(&v1.EstimateAuthoringOperationRequest{}))
			return e
		},
		func() error {
			_, e := h.StartAuthoringOperation(ctx, connect.NewRequest(&v1.StartAuthoringOperationRequest{}))
			return e
		},
		func() error {
			_, e := h.SelectAuthoringCandidate(ctx, connect.NewRequest(&v1.SelectAuthoringCandidateRequest{}))
			return e
		},
		func() error {
			_, e := h.CancelAuthoringOperation(ctx, connect.NewRequest(&v1.CancelAuthoringOperationRequest{}))
			return e
		},
		func() error {
			_, e := h.SaveAuthoringSession(ctx, connect.NewRequest(&v1.SaveAuthoringSessionRequest{}))
			return e
		},
	}
	for i, call := range calls {
		err := call()
		if connect.CodeOf(err) != connect.CodeUnauthenticated || errorDetail(t, err).Reason != "AUTH_REQUIRED" {
			t.Fatalf("procedure %d exposed unauthenticated service: %v", i, err)
		}
	}
}

func TestAuthoringRefusalsHaveStableReasonsAndNoPrivateText(t *testing.T) {
	cases := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{authoring.ErrNotFound, connect.CodeNotFound, "AUTHORING_SESSION_NOT_FOUND"},
		{authoring.ErrStale, connect.CodeAborted, "AUTHORING_REVISION_CONFLICT"},
		{authoring.ErrBusy, connect.CodeFailedPrecondition, "AUTHORING_RUNNING"},
		{authoring.ErrOutput, connect.CodeFailedPrecondition, "AUTHORING_OUTPUT_INVALID"},
		{authoring.ErrNoSelection, connect.CodeFailedPrecondition, "AUTHORING_NOT_READY"},
		{authoring.ErrInvalid, connect.CodeInvalidArgument, "AUTHORING_MESSAGE_INVALID"},
		{authoring.ErrFeatureUnavailable, connect.CodeUnimplemented, "AUTHORING_FEATURE_UNAVAILABLE"},
		{authoring.ErrHistoryFull, connect.CodeFailedPrecondition, "AUTHORING_HISTORY_FULL"},
		{authoring.ErrTargetConflict, connect.CodeAborted, "AUTHORING_SAVE_CONFLICT"},
		{authoring.ErrInvalidKind, connect.CodeInvalidArgument, "AUTHORING_KIND_INVALID"},
		{authoring.ErrModel, connect.CodeFailedPrecondition, "AUTHORING_MODEL_REQUIRED"},
		{authoring.ErrPublication, connect.CodeFailedPrecondition, "AUTHORING_NOT_READY"},
		{errors.New("private SQL DSN/provider payload"), connect.CodeInternal, "UNKNOWN_FAILURE"},
	}
	for _, tc := range cases {
		mapped := toError(fmt.Errorf("private diagnostic: %w", tc.err))
		detail := errorDetail(t, mapped)
		if connect.CodeOf(mapped) != tc.code || detail.Reason != tc.reason || len(detail.Params) != 0 || strings.Contains(mapped.Error(), "private") || strings.Contains(mapped.Error(), "payload") {
			t.Fatalf("refusal %v: %v %+v", tc.err, mapped, detail)
		}
	}
}

func TestSessionProjectionExcludesPrivatePublicationAndSourceContext(t *testing.T) {
	value := sessionFixture()
	wire := session(&value)
	if wire.Id != value.ID || wire.Selected.Id != value.Selected.ID || wire.ActiveJobId != value.ActiveJobID || wire.PendingRequest != value.PendingRequest || wire.Saved.Id != value.Saved.ID {
		t.Fatal("public durable state was lost")
	}
	if strings.Contains(wire.String(), "private") {
		t.Fatal("private source/publication metadata was exposed")
	}
}

type targetLimitFailure struct{}

func (targetLimitFailure) Error() string             { return "target account limit reached" }
func (targetLimitFailure) Reason() string            { return "TEMPLATE_LIMIT_REACHED" }
func (targetLimitFailure) Params() map[string]string { return nil }

func TestTargetDomainRefusalKeepsOwnedStableReason(t *testing.T) {
	wrapped := fmt.Errorf("private context: %w", targetLimitFailure{})
	mapped := toError(wrapped)
	if connect.CodeOf(mapped) != connect.CodeFailedPrecondition || errorDetail(t, mapped).Reason != "TEMPLATE_LIMIT_REACHED" || strings.Contains(mapped.Error(), "private") {
		t.Fatal("target domain refusal became unknown or leaked adapter text", mapped)
	}
}

func sessionFixture() authoring.Session {
	return authoring.Session{ID: "session", Kind: authoring.WritingVoice, Revision: 7, Phase: "saved", Selected: &authoring.Artifact{ID: "candidate", Name: "말투", Body: "예시"}, ActiveJobID: "job", PendingRequest: "수정 요청", Saved: &authoring.SavedRef{Kind: authoring.WritingVoice, ID: "saved"}, UserID: "private-owner", SourceContext: "private-analysis", Publication: &authoring.Publication{Key: "private-key"}}
}
