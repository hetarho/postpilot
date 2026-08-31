package rpc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/generation"
)

func TestGenerationErrorsHaveStableReasonsCodesAndAllowlistedParams(t *testing.T) {
	active := &generation.JobAlreadyInProgressError{ActiveID: "job-active"}
	tests := map[string]struct {
		op     string
		err    error
		code   connect.Code
		reason string
		params map[string]string
	}{
		"post not found":          {"start generation", generation.ErrNotFound, connect.CodeNotFound, "POST_NOT_FOUND", nil},
		"post forbidden":          {"start revision", generation.ErrForbidden, connect.CodePermissionDenied, "POST_FORBIDDEN", nil},
		"job not found":           {"get generation", generation.ErrNotFound, connect.CodeNotFound, "JOB_NOT_FOUND", nil},
		"job forbidden":           {"get generation", generation.ErrForbidden, connect.CodePermissionDenied, "JOB_FORBIDDEN", nil},
		"write model":             {"start generation", generation.ErrWriteModelRequired, connect.CodeFailedPrecondition, "GENERATION_WRITE_MODEL_REQUIRED", nil},
		"observe model":           {"start generation", generation.ErrObserveModelRequired, connect.CodeFailedPrecondition, "GENERATION_OBSERVE_MODEL_REQUIRED", nil},
		"target language":         {"start generation", generation.ErrLanguageRequired, connect.CodeFailedPrecondition, "POST_TARGET_LANGUAGE_REQUIRED", nil},
		"content language":        {"start revision", generation.ErrContentLanguageRequired, connect.CodeFailedPrecondition, "CONTENT_LANGUAGE_REQUIRED", nil},
		"revision content":        {"start revision", generation.ErrRevisionContentRequired, connect.CodeFailedPrecondition, "REVISION_CONTENT_REQUIRED", nil},
		"voice required":          {"start generation", generation.ErrVoiceRequired, connect.CodeFailedPrecondition, "VOICE_REQUIRED", nil},
		"voice deleted":           {"start generation", generation.ErrVoiceDeleted, connect.CodeFailedPrecondition, "VOICE_DELETED", nil},
		"voice changed":           {"start generation", generation.ErrVoiceMismatch, connect.CodeFailedPrecondition, "GENERATION_VOICE_MISMATCH", nil},
		"voice language mismatch": {"start revision", generation.ErrVoiceContentLanguageMismatch, connect.CodeFailedPrecondition, "VOICE_CONTENT_LANGUAGE_MISMATCH", nil},
		"instruction required":    {"start revision", generation.ErrRevisionInstructionRequired, connect.CodeInvalidArgument, "REVISION_INSTRUCTION_REQUIRED", nil},
		"instruction too long":    {"start revision", generation.ErrRevisionInstructionTooLong, connect.CodeInvalidArgument, "REVISION_INSTRUCTION_TOO_LONG", map[string]string{"max": "500"}},
		"target length":           {"start generation", generation.ErrInvalidTargetLength, connect.CodeInvalidArgument, "GENERATION_TARGET_LENGTH_INVALID", nil},
		"already running wrapped": {"start generation", errors.Join(errors.New("private queue detail"), active), connect.CodeFailedPrecondition, "GENERATION_ALREADY_RUNNING", map[string]string{"active_job_id": "job-active"}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mapped := toConnectError(test.op, test.err)
			if got := connect.CodeOf(mapped); got != test.code {
				t.Fatalf("code = %v, want %v", got, test.code)
			}
			detail := generationAppErrorDetail(t, mapped)
			if detail.GetReason() != test.reason || !reflect.DeepEqual(detail.GetParams(), test.params) {
				t.Fatalf("detail = %#v, want reason=%q params=%#v", detail, test.reason, test.params)
			}
			if strings.Contains(mapped.Error(), "private queue detail") {
				t.Fatalf("private wrapped detail leaked: %v", mapped)
			}
		})
	}
}

func TestGenerationUnknownAndAuthenticationFailuresAreTypedAndPrivate(t *testing.T) {
	unknown := toConnectError("start generation", errors.New("sql password=<secret> post body=<private>"))
	if connect.CodeOf(unknown) != connect.CodeInternal || strings.Contains(unknown.Error(), "<secret>") || strings.Contains(unknown.Error(), "<private>") {
		t.Fatalf("unknown error leaked: %v", unknown)
	}
	if detail := generationAppErrorDetail(t, unknown); detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Fatalf("unknown detail = %#v", detail)
	}

	_, authErr := actingUser(context.Background())
	if connect.CodeOf(authErr) != connect.CodeUnauthenticated {
		t.Fatalf("authentication code = %v", connect.CodeOf(authErr))
	}
	if detail := generationAppErrorDetail(t, authErr); detail.GetReason() != "AUTH_REQUIRED" || len(detail.GetParams()) != 0 {
		t.Fatalf("authentication detail = %#v", detail)
	}
}

func TestGenerationActiveJobParamRejectsNonOpaqueValues(t *testing.T) {
	mapped := toConnectError("start generation", &generation.JobAlreadyInProgressError{ActiveID: `<script>private</script>`})
	detail := generationAppErrorDetail(t, mapped)
	if detail.GetReason() != "GENERATION_ALREADY_RUNNING" || len(detail.GetParams()) != 0 || strings.Contains(mapped.Error(), "private") {
		t.Fatalf("unsafe active job detail = %#v / %v", detail, mapped)
	}
}

func TestGenerationJobMapsFrozenTargetLanguage(t *testing.T) {
	for name, test := range map[string]struct {
		language generation.Language
		want     postpilotv1.ContentLanguage
	}{
		"Korean":  {generation.LanguageKorean, postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN},
		"English": {generation.LanguageEnglish, postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH},
		"invalid": {generation.Language("fr"), postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
	} {
		t.Run(name, func(t *testing.T) {
			got := toProtoJob(&generation.JobSummary{TargetLanguage: test.language}).GetTargetLanguage()
			if got != test.want {
				t.Fatalf("target = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGenerationJobProjectsStructuredFailureWithoutDeprecatedRawError(t *testing.T) {
	params := map[string]string{"actual": "20", "min": "200"}
	mapped := toProtoJob(&generation.JobSummary{Failure: &generation.Failure{
		Reason: "VOICE_SAMPLE_TOO_SHORT", Params: params, TechnicalDetail: "provider detail",
	}})
	if mapped.GetFailure().GetReason() != "VOICE_SAMPLE_TOO_SHORT" ||
		mapped.GetFailure().GetParams()["actual"] != "20" ||
		mapped.GetFailure().GetTechnicalDetail() != "provider detail" || mapped.GetError() != "" {
		t.Fatalf("mapped failure = %+v", mapped)
	}
	params["actual"] = "mutated"
	if mapped.GetFailure().GetParams()["actual"] != "20" {
		t.Fatalf("proto params alias domain map: %#v", mapped.GetFailure().GetParams())
	}
}

func generationAppErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
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
