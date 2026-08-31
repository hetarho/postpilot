package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
)

func TestProviderErrorsHaveStableReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
	}{
		"stage":          {provider.ErrUnknownStage, connect.CodeInvalidArgument, "MODEL_STAGE_INVALID"},
		"missing model":  {provider.ErrModelNotRegistered, connect.CodeNotFound, "MODEL_NOT_REGISTERED"},
		"recommendation": {provider.ErrRecommendationNotFound, connect.CodeNotFound, "MODEL_RECOMMENDATION_NOT_FOUND"},
		"disabled":       {provider.ErrModelDisabled, connect.CodeFailedPrecondition, "MODEL_DISABLED"},
		"unsuitable":     {provider.ErrModelUnsuitable, connect.CodeFailedPrecondition, "MODEL_UNSUITABLE"},
		"duplicates":     {provider.ErrDuplicateCandidates, connect.CodeFailedPrecondition, "MODEL_CANDIDATES_DUPLICATE"},
	} {
		t.Run(name, func(t *testing.T) {
			err := toConnectError("provider operation", tc.err)
			if got := connect.CodeOf(err); got != tc.code {
				t.Fatalf("code = %v, want %v", got, tc.code)
			}
			if got := providerErrorDetail(t, err).GetReason(); got != tc.reason {
				t.Fatalf("reason = %q, want %q", got, tc.reason)
			}
		})
	}
}

func TestProviderUnknownErrorDoesNotLeakDetail(t *testing.T) {
	err := toConnectError("list models", errors.New("database password=<secret>"))
	if connect.CodeOf(err) != connect.CodeInternal || strings.Contains(err.Error(), "<secret>") {
		t.Fatalf("unexpected error = %v", err)
	}
	detail := providerErrorDetail(t, err)
	if detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestSaveSelectionDistinguishesMissingAndUnknownStage(t *testing.T) {
	handler := NewHandler(nil)
	ctx := auth.WithActor(context.Background(), auth.Actor{UserID: "alice", Plan: plan.Free})
	for name, tc := range map[string]struct {
		stage  postpilotv1.Stage
		reason string
	}{
		"missing": {postpilotv1.Stage_STAGE_UNSPECIFIED, "MODEL_STAGE_REQUIRED"},
		"unknown": {postpilotv1.Stage(999), "MODEL_STAGE_INVALID"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := handler.SaveSelection(ctx, connect.NewRequest(&postpilotv1.SaveSelectionRequest{Stage: tc.stage}))
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("code = %v", connect.CodeOf(err))
			}
			if got := providerErrorDetail(t, err).GetReason(); got != tc.reason {
				t.Fatalf("reason = %q, want %q", got, tc.reason)
			}
		})
	}
}

func providerErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T", err)
	}
	if len(connectErr.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(connectErr.Details()))
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T", value)
	}
	if len(detail.GetParams()) != 0 {
		t.Fatalf("unexpected params = %#v", detail.GetParams())
	}
	return detail
}
