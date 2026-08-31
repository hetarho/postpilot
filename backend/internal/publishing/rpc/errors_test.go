package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/publishing"
)

func TestRevokedAgentBusinessStateDoesNotInvalidateHumanSession(t *testing.T) {
	err := toConnectError(publishing.ErrAgentRevoked)
	if code := connect.CodeOf(err); code != connect.CodeFailedPrecondition {
		t.Fatalf("revoked agent business-state code=%v", code)
	}
	if reason := publishingErrorReason(t, err); reason != "PUBLISH_AGENT_REVOKED" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestPublishingErrorsHaveDistinctStableReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
	}{
		"not found":          {publishing.ErrNotFound, connect.CodeNotFound, "PUBLISH_NOT_FOUND"},
		"forbidden":          {publishing.ErrForbidden, connect.CodePermissionDenied, "PUBLISH_FORBIDDEN"},
		"invalid":            {publishing.ErrInvalid, connect.CodeInvalidArgument, "PUBLISH_REQUEST_INVALID"},
		"invalid pairing":    {publishing.ErrPairingInvalid, connect.CodeInvalidArgument, "PUBLISH_PAIRING_INVALID"},
		"pairing limit":      {publishing.ErrPairingLimit, connect.CodeAlreadyExists, "PUBLISH_PAIRING_LIMIT"},
		"already publishing": {publishing.ErrAlreadyPublishing, connect.CodeAlreadyExists, "PUBLISH_ALREADY_EXISTS"},
		"not ready":          {publishing.ErrAgentNotReady, connect.CodeFailedPrecondition, "PUBLISH_AGENT_NOT_READY"},
		"category":           {publishing.ErrCategoryNotFound, connect.CodeFailedPrecondition, "PUBLISH_CATEGORY_NOT_FOUND"},
		"not finalized":      {publishing.ErrPostNotFinalized, connect.CodeFailedPrecondition, "PUBLISH_POST_NOT_FINALIZED"},
		"stale":              {publishing.ErrStaleRevision, connect.CodeAborted, "PUBLISH_STALE_REVISION"},
		"lease":              {publishing.ErrLeaseInvalid, connect.CodeAborted, "PUBLISH_LEASE_INVALID"},
		"transition":         {publishing.ErrTransition, connect.CodeAborted, "PUBLISH_TRANSITION_INVALID"},
		"commit fence":       {publishing.ErrCommitFence, connect.CodeFailedPrecondition, "PUBLISH_COMMIT_FENCE"},
		"url":                {publishing.ErrPublishedURLInvalid, connect.CodeFailedPrecondition, "PUBLISH_URL_INVALID"},
	} {
		t.Run(name, func(t *testing.T) {
			err := toConnectError(tc.err)
			if got := connect.CodeOf(err); got != tc.code {
				t.Fatalf("code = %v, want %v", got, tc.code)
			}
			if got := publishingErrorReason(t, err); got != tc.reason {
				t.Fatalf("reason = %q, want %q", got, tc.reason)
			}
		})
	}
}

func publishingErrorReason(t *testing.T, err error) string {
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
	return detail.GetReason()
}
