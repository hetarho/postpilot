package rpc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

// Every domain error the service can raise must have a code. An unmapped one becomes
// Internal, which tells the client to retry something that will never succeed.
func TestToConnectErrorMapsEveryDomainError(t *testing.T) {
	cases := []struct {
		name   string
		op     string
		err    error
		code   connect.Code
		reason string
	}{
		{"post missing", "get post", post.ErrNotFound, connect.CodeNotFound, "POST_NOT_FOUND"},
		{"upload missing", "confirm upload", post.ErrNotFound, connect.CodeNotFound, "UPLOAD_NOT_FOUND"},
		{"forbidden", "get post", post.ErrForbidden, connect.CodePermissionDenied, "POST_FORBIDDEN"},
		{"filename", "create upload", post.ErrDuplicateFilename, connect.CodeAlreadyExists, "POST_FILENAME_TAKEN"},
		{"object", "confirm upload", post.ErrObjectMissing, connect.CodeFailedPrecondition, "UPLOAD_OBJECT_MISSING"},
		{"image", "confirm upload", post.ErrInvalidImage, connect.CodeInvalidArgument, "UPLOAD_INVALID"},
		{"busy", "save draft", post.ErrPostBusy, connect.CodeFailedPrecondition, "POST_BUSY"},
		{"publishing", "delete post", post.ErrPostPublishing, connect.CodeFailedPrecondition, "POST_PUBLISHING"},
		{"stale", "save post content", post.ErrStaleContentRevision, connect.CodeAborted, "POST_CONTENT_STALE"},
		{"baseline", "finalize post", post.ErrNoMachineBaseline, connect.CodeFailedPrecondition, "POST_MACHINE_BASELINE_REQUIRED"},
		{"not finalized", "publish", post.ErrPostNotFinalized, connect.CodeFailedPrecondition, "POST_NOT_FINALIZED"},
		{"invalid content", "save post content", &post.InvalidContentError{Reason: "private authored content"}, connect.CodeInvalidArgument, "POST_CONTENT_INVALID"},
		{"voice required", "save draft", post.ErrVoiceRequired, connect.CodeInvalidArgument, "VOICE_REQUIRED"},
		{"voice missing", "save draft", post.ErrVoiceNotFound, connect.CodeNotFound, "VOICE_NOT_FOUND"},
		{"template missing", "save draft", post.ErrTemplateNotFound, connect.CodeNotFound, "PURPOSE_NOT_FOUND"},
		{"voice deleted", "save draft", post.ErrVoiceDeleted, connect.CodeFailedPrecondition, "VOICE_DELETED"},
		{"language", "save draft", post.ErrLanguageRequired, connect.CodeInvalidArgument, "POST_TARGET_LANGUAGE_REQUIRED"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			// Wrapped, as the service actually returns them.
			got := toConnectError(test.op, errors.Join(errors.New("private context"), test.err))
			if connect.CodeOf(got) != test.code {
				t.Errorf("code = %v, want %v", connect.CodeOf(got), test.code)
			}
			detail := postAppErrorDetail(t, got)
			if detail.GetReason() != test.reason || !reflect.DeepEqual(detail.GetParams(), map[string]string(nil)) {
				t.Errorf("detail = %#v, want reason %q", detail, test.reason)
			}
			if strings.Contains(got.Error(), "private") {
				t.Errorf("private detail leaked: %v", got)
			}
		})
	}
}

// An unexpected failure must not put a SQL string or a bucket name on the wire.
func TestToConnectErrorHidesUnexpectedDetail(t *testing.T) {
	got := toConnectError("save draft", errors.New("no such table: posts (file /data/postpilot.db)"))

	if connect.CodeOf(got) != connect.CodeInternal {
		t.Errorf("code = %v, want internal", connect.CodeOf(got))
	}
	if got.Error() != "internal: save draft failed" {
		t.Errorf("message = %q, want it to carry no detail", got.Error())
	}
	if detail := postAppErrorDetail(t, got); detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Errorf("detail = %#v", detail)
	}
}

func TestDraftLanguageAndContentDecodeFailuresAreStable(t *testing.T) {
	handler := NewHandler(nil)
	ctx := auth.WithUser(context.Background(), "alice")
	for name, test := range map[string]struct {
		language postpilotv1.ContentLanguage
		reason   string
	}{
		"missing": {postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED, "POST_TARGET_LANGUAGE_REQUIRED"},
		"unknown": {postpilotv1.ContentLanguage(999), "POST_TARGET_LANGUAGE_UNSUPPORTED"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := handler.SavePostDraft(ctx, connect.NewRequest(&postpilotv1.SavePostDraftRequest{TargetLanguage: &test.language}))
			if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != test.reason {
				t.Fatalf("error = %v, detail = %#v", err, postAppErrorDetail(t, err))
			}
		})
	}

	_, err := handler.SavePostContent(ctx, connect.NewRequest(&postpilotv1.SavePostContentRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != "POST_CONTENT_INVALID" {
		t.Fatalf("content error = %v", err)
	}
}

func TestActiveJobProjectsStructuredFailureWithoutDeprecatedRawError(t *testing.T) {
	params := map[string]string{"safe": "value"}
	mapped := toProtoActiveJob(&post.ActiveJob{Failure: &post.Failure{
		Reason: "MODEL_UNAVAILABLE", Params: params, TechnicalDetail: "provider detail",
	}})
	if mapped.GetFailure().GetReason() != "MODEL_UNAVAILABLE" ||
		mapped.GetFailure().GetParams()["safe"] != "value" ||
		mapped.GetFailure().GetTechnicalDetail() != "provider detail" || mapped.GetError() != "" {
		t.Fatalf("mapped failure = %+v", mapped)
	}
	params["safe"] = "mutated"
	if mapped.GetFailure().GetParams()["safe"] != "value" {
		t.Fatalf("proto params alias domain map: %#v", mapped.GetFailure().GetParams())
	}
}

func postAppErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
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
	return detail
}
