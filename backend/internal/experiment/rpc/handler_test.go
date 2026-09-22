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
		"not found":                {experiment.ErrNotFound, connect.CodeNotFound, "EXPERIMENT_NOT_FOUND", nil},
		"candidate not found":      {experiment.ErrCandidateNotFound, connect.CodeNotFound, "EXPERIMENT_CANDIDATE_NOT_FOUND", nil},
		"forbidden":                {experiment.ErrForbidden, connect.CodePermissionDenied, "EXPERIMENT_FORBIDDEN", nil},
		"stage":                    {experiment.ErrInvalidStage, connect.CodeInvalidArgument, "EXPERIMENT_STAGE_INVALID", nil},
		"duplicate candidates":     {experiment.ErrDuplicateCandidates, connect.CodeInvalidArgument, "EXPERIMENT_CANDIDATES_DUPLICATE", nil},
		"target length":            {experiment.ErrInvalidTargetLength, connect.CodeInvalidArgument, "EXPERIMENT_TARGET_LENGTH_INVALID", nil},
		"voice required":           {experiment.ErrVoiceRequired, connect.CodeInvalidArgument, "EXPERIMENT_VOICE_REQUIRED", nil},
		"models required":          {experiment.ErrModelRequired, connect.CodeFailedPrecondition, "EXPERIMENT_MODELS_REQUIRED", nil},
		"signed video unsupported": {&experiment.VideoUnsupportedError{Model: "provider/video"}, connect.CodeFailedPrecondition, "MODEL_VIDEO_UNSUPPORTED", map[string]string{"model": "provider/video"}},
		"target language":          {experiment.ErrLanguageRequired, connect.CodeFailedPrecondition, "POST_TARGET_LANGUAGE_REQUIRED", nil},
		"state":                    {experiment.ErrInvalidState, connect.CodeFailedPrecondition, "EXPERIMENT_STATE_INVALID", nil},
		"confirmation":             {experiment.ErrConfirmationRequired, connect.CodeFailedPrecondition, "EXPERIMENT_CONFIRMATION_REQUIRED", nil},
		"snapshot":                 {experiment.ErrSnapshotUnavailable, connect.CodeFailedPrecondition, "EXPERIMENT_SNAPSHOT_UNAVAILABLE", nil},
		"retry model":              {experiment.ErrRetryModelUnavailable, connect.CodeFailedPrecondition, "EXPERIMENT_RETRY_MODEL_UNAVAILABLE", nil},
		"voice unavailable":        {experiment.ErrVoiceUnavailable, connect.CodeFailedPrecondition, "EXPERIMENT_VOICE_UNAVAILABLE", nil},
		"post finalized":           {experiment.ErrPostFinalized, connect.CodeFailedPrecondition, "EXPERIMENT_POST_FINALIZED", nil},
		"badges invalid":           {experiment.ErrBadgesInvalid, connect.CodeInvalidArgument, "EXPERIMENT_BADGES_INVALID", nil},
		"already running wrapped":  {errors.Join(errors.New("private queue detail"), active), connect.CodeFailedPrecondition, "EXPERIMENT_ALREADY_RUNNING", map[string]string{"active_job_id": "job-active"}},
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

// A comparison's origin crosses the wire in both directions and is not an identity: it says
// which verdict form the review offers, and it is stated on the experiment rather than
// derived per stage, so the browser never has to guess.
func TestExperimentOriginMapsBothWays(t *testing.T) {
	t.Run("into the domain", func(t *testing.T) {
		cases := map[postpilotv1.ExperimentOrigin]experiment.Origin{
			postpilotv1.ExperimentOrigin_EXPERIMENT_ORIGIN_LAB:         experiment.OriginLab,
			postpilotv1.ExperimentOrigin_EXPERIMENT_ORIGIN_EDITOR:      experiment.OriginEditor,
			postpilotv1.ExperimentOrigin_EXPERIMENT_ORIGIN_UNSPECIFIED: experiment.OriginEditor,
		}
		for wire, want := range cases {
			if got := fromProtoOrigin(wire); got != want {
				t.Errorf("fromProtoOrigin(%v) = %q, want %q", wire, got, want)
			}
		}
	})

	t.Run("onto the wire", func(t *testing.T) {
		cases := []struct {
			origin experiment.Origin
			want   postpilotv1.ExperimentOrigin
		}{
			{experiment.OriginLab, postpilotv1.ExperimentOrigin_EXPERIMENT_ORIGIN_LAB},
			{experiment.OriginEditor, postpilotv1.ExperimentOrigin_EXPERIMENT_ORIGIN_EDITOR},
		}
		for _, sample := range cases {
			found := experiment.Experiment{ID: "exp", Stage: experiment.StageWrite, Origin: sample.origin, Status: experiment.StatusReview}
			if got := toProtoExperiment(found).GetOrigin(); got != sample.want {
				t.Errorf("origin %q maps to %v, want %v", sample.origin, got, sample.want)
			}
		}
	})

	t.Run("a blind comparison still names no model", func(t *testing.T) {
		found := experiment.Experiment{
			ID: "exp", Stage: experiment.StageWrite, Origin: experiment.OriginLab, Status: experiment.StatusReview,
			Candidates: []experiment.Candidate{{
				ID: "left", ExperimentID: "exp", DisplaySide: experiment.SideLeft, Status: experiment.CandidateSucceeded,
				Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, ModelLabel: "A",
			}},
		}
		mapped := toProtoExperiment(found)
		if mapped.GetOrigin() != postpilotv1.ExperimentOrigin_EXPERIMENT_ORIGIN_LAB {
			t.Fatalf("origin = %v", mapped.GetOrigin())
		}
		if mapped.GetCandidates()[0].GetModel() != nil || mapped.GetCandidates()[0].GetModelLabel() != "" {
			t.Fatalf("origin mapping revealed an identity: %+v", mapped.GetCandidates()[0])
		}
	})
}

// A board's window and scope resolve here so that a client stating nothing and one stating
// the defaults get the same board, and nothing on the wire can ask for an all-time board.
func TestLeaderboardRequestResolvesItsDefaults(t *testing.T) {
	t.Run("window", func(t *testing.T) {
		cases := map[postpilotv1.LeaderboardWindow]experiment.Window{
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_DAY:         experiment.WindowDay,
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_WEEK:        experiment.WindowWeek,
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_MONTH:       experiment.WindowMonth,
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_UNSPECIFIED: experiment.WindowWeek,
		}
		for wire, want := range cases {
			if got := fromProtoWindow(wire); got != want {
				t.Errorf("fromProtoWindow(%v) = %q, want %q", wire, got, want)
			}
		}
	})

	t.Run("scope", func(t *testing.T) {
		cases := map[postpilotv1.LeaderboardScope]experiment.Scope{
			postpilotv1.LeaderboardScope_LEADERBOARD_SCOPE_ALL:         experiment.ScopeAll,
			postpilotv1.LeaderboardScope_LEADERBOARD_SCOPE_ME:          experiment.ScopeMe,
			postpilotv1.LeaderboardScope_LEADERBOARD_SCOPE_UNSPECIFIED: experiment.ScopeMe,
		}
		for wire, want := range cases {
			if got := fromProtoScope(wire); got != want {
				t.Errorf("fromProtoScope(%v) = %q, want %q", wire, got, want)
			}
		}
	})

	t.Run("every window the wire names is a bounded one", func(t *testing.T) {
		for _, wire := range []postpilotv1.LeaderboardWindow{
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_DAY,
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_WEEK,
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_MONTH,
			postpilotv1.LeaderboardWindow_LEADERBOARD_WINDOW_UNSPECIFIED,
		} {
			if length := fromProtoWindow(wire).Length(); length <= 0 || length > experiment.LeaderboardWindowMonth {
				t.Errorf("%v resolves to %v, which is not one of the three bounded windows", wire, length)
			}
		}
	})

	t.Run("a window or scope outside the vocabulary is an invalid argument", func(t *testing.T) {
		for _, err := range []error{experiment.ErrInvalidWindow, experiment.ErrInvalidScope} {
			mapped := toConnectError("get leaderboard", err)
			if got := connect.CodeOf(mapped); got != connect.CodeInvalidArgument {
				t.Errorf("%v mapped to %v, want InvalidArgument", err, got)
			}
			if detail := experimentAppErrorDetail(t, mapped); detail.GetReason() != "EXPERIMENT_STAGE_INVALID" {
				t.Errorf("%v reason = %q", err, detail.GetReason())
			}
		}
	})
}

// Badges cross the wire only inside the reveal. Before a verdict they would say which
// candidate the owner had already judged, which is the one thing the blind pair hides.
func TestBadgesCrossTheWireOnlyWithTheIdentity(t *testing.T) {
	candidates := []experiment.Candidate{{
		ID: "left", ExperimentID: "exp", DisplaySide: experiment.SideLeft, Status: experiment.CandidateSucceeded,
		Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, ModelLabel: "A",
		Badges: []experiment.Badge{experiment.BadgeFast, experiment.BadgeOther}, OtherNote: "설명",
	}}

	blind := toProtoExperiment(experiment.Experiment{
		ID: "exp", Stage: experiment.StageWrite, Origin: experiment.OriginLab,
		Status: experiment.StatusReview, Candidates: candidates,
	})
	if len(blind.GetCandidates()[0].GetBadges()) != 0 || blind.GetCandidates()[0].GetOtherNote() != "" {
		t.Fatalf("badges leaked before the verdict: %+v", blind.GetCandidates()[0])
	}

	revealed := toProtoExperiment(experiment.Experiment{
		ID: "exp", Stage: experiment.StageWrite, Origin: experiment.OriginLab,
		Status: experiment.StatusDecided, WinnerCandidateID: "left", Candidates: candidates,
	})
	got := revealed.GetCandidates()[0]
	if len(got.GetBadges()) != 2 || got.GetOtherNote() != "설명" {
		t.Fatalf("revealed candidate = %+v", got)
	}
	if got.GetBadges()[0] != postpilotv1.VerdictBadge_VERDICT_BADGE_FAST ||
		got.GetBadges()[1] != postpilotv1.VerdictBadge_VERDICT_BADGE_OTHER {
		t.Fatalf("badge mapping = %v", got.GetBadges())
	}
}

// The catalog is one list on both sides. Every value the wire names maps to a domain badge
// and back, so a badge added to one end without the other fails here rather than silently
// dropping on a verdict.
func TestEveryWireBadgeMapsBothWays(t *testing.T) {
	for wire, name := range postpilotv1.VerdictBadge_name {
		badge := postpilotv1.VerdictBadge(wire)
		if badge == postpilotv1.VerdictBadge_VERDICT_BADGE_UNSPECIFIED {
			continue
		}
		mapped := fromProtoBadges([]*postpilotv1.CandidateBadges{{CandidateId: "c", Badges: []postpilotv1.VerdictBadge{badge}}})
		if len(mapped) != 1 || len(mapped[0].Badges) != 1 {
			t.Fatalf("%s does not map into the domain", name)
		}
		if back := toProtoBadge(mapped[0].Badges[0]); back != badge {
			t.Fatalf("%s mapped back as %v", name, back)
		}
	}
	if toProtoBadge(experiment.Badge("vibes")) != postpilotv1.VerdictBadge_VERDICT_BADGE_UNSPECIFIED {
		t.Fatal("a badge outside the catalog produced a wire value")
	}
}

// A tally crossing the wire names a model's badge and how often it was earned, and nothing
// else: no account, no experiment, no note (MODEL-41).
func TestBadgeTalliesCrossTheWireAsCountsAlone(t *testing.T) {
	mapped := toProtoTallies([]experiment.BadgeTally{
		{Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, Badge: experiment.BadgeFast, Count: 4},
		{Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, Badge: experiment.BadgeOther, Count: 1},
	})
	if len(mapped) != 2 {
		t.Fatalf("tallies = %+v", mapped)
	}
	if mapped[0].GetBadge() != postpilotv1.VerdictBadge_VERDICT_BADGE_FAST || mapped[0].GetCount() != 4 {
		t.Fatalf("first tally = %+v", mapped[0])
	}
	// `other` is counted like any other badge; the note it came with is not part of a tally.
	if mapped[1].GetBadge() != postpilotv1.VerdictBadge_VERDICT_BADGE_OTHER || mapped[1].GetCount() != 1 {
		t.Fatalf("other tally = %+v", mapped[1])
	}
	if fields := mapped[0].ProtoReflect().Descriptor().Fields(); fields.Len() != 2 {
		t.Fatalf("a tally carries %d fields, want only the badge and the count", fields.Len())
	}
	if len(toProtoTallies(nil)) != 0 {
		t.Fatal("an empty tally list produced a row")
	}
}
