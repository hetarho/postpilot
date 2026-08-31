package rpc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/experiment"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestExperimentErrorsHaveStableReasonsCodesAndAllowlistedParams(t *testing.T) {
	active := &experiment.JobAlreadyInProgressError{ActiveID: "job-active"}
	tests := map[string]struct {
		err    error
		code   connect.Code
		reason string
		params map[string]string
	}{
		"not found":               {experiment.ErrNotFound, connect.CodeNotFound, "EXPERIMENT_NOT_FOUND", nil},
		"candidate not found":     {experiment.ErrCandidateNotFound, connect.CodeNotFound, "EXPERIMENT_CANDIDATE_NOT_FOUND", nil},
		"forbidden":               {experiment.ErrForbidden, connect.CodePermissionDenied, "EXPERIMENT_FORBIDDEN", nil},
		"stage":                   {experiment.ErrInvalidStage, connect.CodeInvalidArgument, "EXPERIMENT_STAGE_INVALID", nil},
		"duplicate candidates":    {experiment.ErrDuplicateCandidates, connect.CodeInvalidArgument, "EXPERIMENT_CANDIDATES_DUPLICATE", nil},
		"target length":           {experiment.ErrInvalidTargetLength, connect.CodeInvalidArgument, "EXPERIMENT_TARGET_LENGTH_INVALID", nil},
		"voice required":          {experiment.ErrVoiceRequired, connect.CodeInvalidArgument, "EXPERIMENT_VOICE_REQUIRED", nil},
		"models required":         {experiment.ErrModelRequired, connect.CodeFailedPrecondition, "EXPERIMENT_MODELS_REQUIRED", nil},
		"target language":         {experiment.ErrLanguageRequired, connect.CodeFailedPrecondition, "POST_TARGET_LANGUAGE_REQUIRED", nil},
		"state":                   {experiment.ErrInvalidState, connect.CodeFailedPrecondition, "EXPERIMENT_STATE_INVALID", nil},
		"confirmation":            {experiment.ErrConfirmationRequired, connect.CodeFailedPrecondition, "EXPERIMENT_CONFIRMATION_REQUIRED", nil},
		"snapshot":                {experiment.ErrSnapshotUnavailable, connect.CodeFailedPrecondition, "EXPERIMENT_SNAPSHOT_UNAVAILABLE", nil},
		"retry model":             {experiment.ErrRetryModelUnavailable, connect.CodeFailedPrecondition, "EXPERIMENT_RETRY_MODEL_UNAVAILABLE", nil},
		"voice unavailable":       {experiment.ErrVoiceUnavailable, connect.CodeFailedPrecondition, "EXPERIMENT_VOICE_UNAVAILABLE", nil},
		"already running wrapped": {errors.Join(errors.New("private queue detail"), active), connect.CodeFailedPrecondition, "EXPERIMENT_ALREADY_RUNNING", map[string]string{"active_job_id": "job-active"}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mapped := toConnectError("experiment operation", test.err)
			if got := connect.CodeOf(mapped); got != test.code {
				t.Fatalf("code = %v, want %v", got, test.code)
			}
			detail := experimentAppErrorDetail(t, mapped)
			if detail.GetReason() != test.reason || !reflect.DeepEqual(detail.GetParams(), test.params) {
				t.Fatalf("detail = %#v, want reason=%q params=%#v", detail, test.reason, test.params)
			}
			if strings.Contains(mapped.Error(), "private queue detail") {
				t.Fatalf("private wrapped detail leaked: %v", mapped)
			}
		})
	}
}

func TestExperimentUnknownAndAuthenticationFailuresAreTypedAndPrivate(t *testing.T) {
	unknown := toConnectError("get experiment", errors.New("sql password=<secret> prompt=<private>"))
	if connect.CodeOf(unknown) != connect.CodeInternal || strings.Contains(unknown.Error(), "<secret>") || strings.Contains(unknown.Error(), "<private>") {
		t.Fatalf("unknown error leaked: %v", unknown)
	}
	if detail := experimentAppErrorDetail(t, unknown); detail.GetReason() != "UNKNOWN_FAILURE" || len(detail.GetParams()) != 0 {
		t.Fatalf("unknown detail = %#v", detail)
	}

	_, authErr := actingUser(context.Background())
	if connect.CodeOf(authErr) != connect.CodeUnauthenticated {
		t.Fatalf("authentication code = %v", connect.CodeOf(authErr))
	}
	if detail := experimentAppErrorDetail(t, authErr); detail.GetReason() != "AUTH_REQUIRED" || len(detail.GetParams()) != 0 {
		t.Fatalf("authentication detail = %#v", detail)
	}
}

func TestExperimentActiveJobParamRejectsNonOpaqueValues(t *testing.T) {
	mapped := toConnectError("start experiment", &experiment.JobAlreadyInProgressError{ActiveID: `<script>private</script>`})
	detail := experimentAppErrorDetail(t, mapped)
	if detail.GetReason() != "EXPERIMENT_ALREADY_RUNNING" || len(detail.GetParams()) != 0 || strings.Contains(mapped.Error(), "private") {
		t.Fatalf("unsafe active job detail = %#v / %v", detail, mapped)
	}
}

func TestExperimentMapsOnlyFrozenWriteTargetLanguage(t *testing.T) {
	english := experiment.LanguageEnglish
	if got := toProtoExperiment(experiment.Experiment{TargetLanguage: &english}).GetTargetLanguage(); got != postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH {
		t.Fatalf("English target = %v", got)
	}
	if got := toProtoExperiment(experiment.Experiment{}).GetTargetLanguage(); got != postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED {
		t.Fatalf("absent target = %v", got)
	}
}

func experimentAppErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
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

func TestCandidateMappingIsBlindUntilVerdict(t *testing.T) {
	candidate := experiment.Candidate{
		ID: "opaque", Model: experiment.ModelRef{ProviderID: "secret-provider", ModelID: "secret-model"},
		ModelLabel: "Secret label", DisplaySide: experiment.SideLeft, Status: experiment.CandidateFailed,
		Output: []byte(`"style guide"`), Failure: &experiment.Failure{Reason: "MODEL_RATE_LIMITED", TechnicalDetail: "upstream secret error"},
		Usage: experiment.Usage{PromptTokens: 12, CompletionTokens: 3, CostMicrousd: 8, CostSource: experiment.CostReported, LatencyMS: 99},
	}
	blind := toProtoCandidate(experiment.Experiment{Stage: experiment.StageAnalyze, Status: experiment.StatusPartial}, candidate)
	if blind.GetModel() != nil || blind.GetModelLabel() != "" || blind.GetUsage() != nil || blind.GetFailure() != nil || blind.GetError() != "" {
		t.Fatalf("pre-verdict response leaked identity/accounting: %+v", blind)
	}
	if blind.GetStyleguide() != "style guide" || blind.GetId() != "opaque" {
		t.Fatalf("blind output/id missing: %+v", blind)
	}

	revealed := toProtoCandidate(experiment.Experiment{Stage: experiment.StageAnalyze, Status: experiment.StatusDismissed}, candidate)
	if revealed.GetModel().GetModelId() != "secret-model" || revealed.GetModelLabel() != "Secret label" ||
		revealed.GetUsage().GetCostMicrousd() != 8 || revealed.GetFailure().GetReason() != "MODEL_RATE_LIMITED" ||
		revealed.GetFailure().GetTechnicalDetail() != "upstream secret error" || revealed.GetError() != "" {
		t.Fatalf("terminal response did not reveal snapshot: %+v", revealed)
	}
}

func TestExperimentMappingProjectsStructuredAggregateFailuresOnly(t *testing.T) {
	found := experiment.Experiment{
		ApplyFailure:    &experiment.Failure{Reason: "UNKNOWN_FAILURE", Params: map[string]string{"safe": "value"}},
		AdoptionFailure: &experiment.Failure{Reason: "MODEL_UNAVAILABLE", TechnicalDetail: "provider detail"},
	}
	mapped := toProtoExperiment(found)
	if mapped.GetApplyFailure().GetReason() != "UNKNOWN_FAILURE" || mapped.GetApplyFailure().GetParams()["safe"] != "value" {
		t.Fatalf("apply failure = %#v", mapped.GetApplyFailure())
	}
	if mapped.GetAdoptionFailure().GetReason() != "MODEL_UNAVAILABLE" || mapped.GetAdoptionFailure().GetTechnicalDetail() != "provider detail" {
		t.Fatalf("adoption failure = %#v", mapped.GetAdoptionFailure())
	}
	if mapped.GetApplyError() != "" || mapped.GetAdoptionError() != "" {
		t.Fatalf("deprecated raw failures populated: %+v", mapped)
	}
}
