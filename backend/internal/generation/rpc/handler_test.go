package rpc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/llm"
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
		"post not found":           {"start generation", generation.ErrNotFound, connect.CodeNotFound, "POST_NOT_FOUND", nil},
		"post forbidden":           {"start revision", generation.ErrForbidden, connect.CodePermissionDenied, "POST_FORBIDDEN", nil},
		"job not found":            {"get generation", generation.ErrNotFound, connect.CodeNotFound, "JOB_NOT_FOUND", nil},
		"job forbidden":            {"get generation", generation.ErrForbidden, connect.CodePermissionDenied, "JOB_FORBIDDEN", nil},
		"write model":              {"start generation", generation.ErrWriteModelRequired, connect.CodeFailedPrecondition, "GENERATION_WRITE_MODEL_REQUIRED", nil},
		"observe model":            {"start generation", generation.ErrObserveModelRequired, connect.CodeFailedPrecondition, "GENERATION_OBSERVE_MODEL_REQUIRED", nil},
		"target language":          {"start generation", generation.ErrLanguageRequired, connect.CodeFailedPrecondition, "POST_TARGET_LANGUAGE_REQUIRED", nil},
		"content language":         {"start revision", generation.ErrContentLanguageRequired, connect.CodeFailedPrecondition, "CONTENT_LANGUAGE_REQUIRED", nil},
		"revision content":         {"start revision", generation.ErrRevisionContentRequired, connect.CodeFailedPrecondition, "REVISION_CONTENT_REQUIRED", nil},
		"storyline missing":        {"start storyline revision", generation.ErrStorylineMissing, connect.CodeFailedPrecondition, "POST_STORYLINE_MISSING", nil},
		"required template answer": {"start generation", errors.Join(errors.New("private answer detail"), &generation.RequiredTemplateAnswerError{Label: "직접 겪은 일"}), connect.CodeFailedPrecondition, "TEMPLATE_REQUIRED_ANSWER_MISSING", map[string]string{"label": "직접 겪은 일"}},
		"voice not made":           {"start generation", generation.ErrVoiceNotMade, connect.CodeFailedPrecondition, "VOICE_NOT_MADE", nil},
		"voice deleted":            {"start generation", generation.ErrVoiceDeleted, connect.CodeFailedPrecondition, "VOICE_DELETED", nil},
		"voice changed":            {"start generation", generation.ErrVoiceMismatch, connect.CodeFailedPrecondition, "GENERATION_VOICE_MISMATCH", nil},
		"post published":           {"start revision", generation.ErrPostPublished, connect.CodeFailedPrecondition, "POST_PUBLISHED_LOCKED", nil},
		"instruction required":     {"start revision", generation.ErrRevisionInstructionRequired, connect.CodeInvalidArgument, "REVISION_INSTRUCTION_REQUIRED", nil},
		"instruction too long":     {"start revision", generation.ErrRevisionInstructionTooLong, connect.CodeInvalidArgument, "REVISION_INSTRUCTION_TOO_LONG", map[string]string{"max": "500"}},
		"target length":            {"start generation", generation.ErrInvalidTargetLength, connect.CodeInvalidArgument, "GENERATION_TARGET_LENGTH_INVALID", nil},
		"already running wrapped":  {"start generation", errors.Join(errors.New("private queue detail"), active), connect.CodeFailedPrecondition, "GENERATION_ALREADY_RUNNING", map[string]string{"active_job_id": "job-active"}},
		// A comparison holding the post is named as one, never as an active job (GEN-23).
		"comparison holds the post": {"start revision", errors.Join(errors.New("private queue detail"), &generation.ExperimentPendingError{ExperimentID: "experiment-1"}), connect.CodeFailedPrecondition, "EXPERIMENT_ALREADY_RUNNING", map[string]string{"experiment_id": "experiment-1"}},
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

func TestGenerationJobCarriesTheQueueCancellationAnswer(t *testing.T) {
	// A clip run polls this projection; without these fields its cancel control reads the
	// job as legacy and offers no cancellation (CLIP-78).
	requested := time.Date(2026, 10, 29, 8, 0, 0, 5, time.UTC)
	expires := requested.Add(30 * time.Minute)
	mapped := toProtoJob(&generation.JobSummary{
		Kind: "generate_clip", CanCancel: true, CancellationPolicyVersion: 1,
		CancelRequestedAt: &requested, WaitExpiresAt: &expires,
	})
	if !mapped.GetCanCancel() || mapped.GetCancellationPolicyVersion() != 1 ||
		mapped.GetCancelRequestedAt() != "2026-10-29T08:00:00.000000005Z" ||
		mapped.GetWaitExpiresAt() != "2026-10-29T08:30:00.000000005Z" {
		t.Fatalf("mapped cancellation = %+v", mapped)
	}
	if idle := toProtoJob(&generation.JobSummary{}); idle.GetCancelRequestedAt() != "" || idle.GetWaitExpiresAt() != "" || idle.GetCanCancel() {
		t.Fatalf("absent cancellation = %+v", idle)
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

// startHarness is a generation service over in-memory ports, enough for the start RPCs to reach
// the queue: one post with two photos and one stored observation.
type startHarness struct {
	post generation.PostInput
	jobs startJobs
}

type startJobs struct {
	err          error
	observations []generation.StartObservationRequest
	generations  []generation.StartRequest
	payloads     [][]byte
}

func (j *startJobs) EnqueueGeneration(_ context.Context, request generation.StartRequest, payload []byte) (string, error) {
	j.generations = append(j.generations, request)
	j.payloads = append(j.payloads, payload)
	return "generate-job", j.err
}
func (j *startJobs) EnqueueRevision(context.Context, generation.StartRevisionRequest, []byte) (string, error) {
	return "", errors.New("unexpected revision")
}
func (j *startJobs) EnqueueStoryline(context.Context, generation.StartStorylineRequest, []byte) (string, error) {
	return "", errors.New("unexpected storyline")
}
func (j *startJobs) EnqueueStorylineRevision(context.Context, generation.StartStorylineRevisionRequest, []byte) (string, error) {
	return "", errors.New("unexpected storyline revision")
}
func (j *startJobs) EnqueueObservation(_ context.Context, request generation.StartObservationRequest, payload []byte) (string, error) {
	j.observations = append(j.observations, request)
	j.payloads = append(j.payloads, payload)
	return "observe-job", j.err
}
func (j *startJobs) GetGeneration(context.Context, string, string) (*generation.JobSummary, error) {
	return nil, generation.ErrNotFound
}

type startPosts struct{ harness *startHarness }

func (p startPosts) AttachedImages(context.Context, string, string) (generation.PostInput, error) {
	return p.harness.post, nil
}
func (startPosts) SetObservations(context.Context, string, string, []generation.Observation) error {
	return errors.New("a start wrote observations")
}
func (startPosts) SetGeneratedContent(context.Context, string, string, generation.PostContent, generation.Language, *generation.WriteAnnotations) error {
	return errors.New("a start wrote content")
}
func (startPosts) SetStoryline(context.Context, string, string, generation.Storyline) error {
	return errors.New("a start wrote a storyline")
}

type startModels struct{}

func (startModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Stages: []string{llm.StageNameObserve, llm.StageNameWrite}}, ref.String() != ""
}
func (startModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	return llm.Response{}, errors.New("a start called a model")
}

type startProfiles struct{}

func (startProfiles) ProfileForPrompt(context.Context, string, string, generation.Language) (generation.Profile, error) {
	return generation.Profile{}, nil
}

type startBudget struct{}

func (startBudget) Write(*int, bool) int       { return 1000 }
func (startBudget) Revise(int, *int, bool) int { return 1000 }
func (startBudget) Storyline(bool) int         { return 1000 }
func (startBudget) Observation() int           { return 1000 }

type startBriefs struct{}

func (startBriefs) RenderedFor(context.Context, string, string, bool, []generation.TemplateAnswer) (generation.TemplateBrief, bool, error) {
	return generation.TemplateBrief{}, false, nil
}
func (startBriefs) RenderedForNewWrite(context.Context, string, string, bool, []generation.TemplateAnswer) (generation.TemplateBrief, bool, error) {
	return generation.TemplateBrief{}, false, nil
}

type startGuidelines struct{}

func (startGuidelines) ForPrompt(context.Context, string, *string, *string, generation.Language, bool) (generation.FrozenGuidelines, error) {
	return generation.FrozenGuidelines{}, nil
}

type startMemories struct{}

func (startMemories) ForPost(context.Context, string, []string) ([]string, error) { return nil, nil }

type startCandidates struct{}

func (startCandidates) Record(context.Context, string, string, string) error { return nil }

type startLinker struct{}

func (startLinker) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", errors.New("no video in this test")
}

type startQuality struct{}

func (startQuality) RulesFor(context.Context, string, string, []string, generation.Language) ([]string, error) {
	return nil, nil
}

func newStartHarness() (*startHarness, *Handler) {
	harness := &startHarness{post: generation.PostInput{
		Slug: "post", UserID: "alice", TargetLanguage: generation.LanguageKorean,
		Images:       []generation.Image{{Filename: "IMG_1.jpg", Key: "k1"}, {Filename: "IMG_2.jpg", Key: "k2"}},
		Observations: []generation.Observation{{File: "IMG_1.jpg", Scene: "stored", Model: "p/old"}},
	}}
	service := generation.NewService(startPosts{harness}, startProfiles{}, startModels{}, nil, &harness.jobs, 4,
		generation.DefaultReasoningPolicy(), startBudget{}, generation.Deps{
			Templates: startBriefs{}, Guidelines: startGuidelines{}, Memories: startMemories{}, Candidates: startCandidates{},
			Videos: startLinker{}, VideoURLTTL: time.Minute, QualityRules: startQuality{},
		})
	return harness, NewHandler(service)
}

// GEN-89, GEN-9: StartObservation maps the slug, the observe model and the selection with its
// presence, and answers the job id.
func TestStartObservationMapsTheRequest(t *testing.T) {
	harness, handler := newStartHarness()
	ctx := auth.WithUser(context.Background(), "alice")
	for _, test := range []struct {
		name      string
		reobserve *postpilotv1.ReobserveSelection
		wantCalls int
	}{
		{name: "absent observes both photos", reobserve: nil, wantCalls: 1},
		{name: "empty observes only the missing one", reobserve: &postpilotv1.ReobserveSelection{}, wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness.jobs.observations = nil
			response, err := handler.StartObservation(ctx, connect.NewRequest(&postpilotv1.StartObservationRequest{
				PostSlug: "post", Reobserve: test.reobserve, ObserveModel: &postpilotv1.ModelRef{ProviderId: "p", ModelId: "eye"},
			}))
			if err != nil || response.Msg.GetJobId() != "observe-job" {
				t.Fatalf("response = %v, %v", response, err)
			}
			request := harness.jobs.observations[0]
			if request.UserID != "alice" || request.PostSlug != "post" || request.ObserveModel != "p/eye" || request.ObserveCalls != test.wantCalls {
				t.Fatalf("request = %+v", request)
			}
		})
	}
	payload := string(harness.jobs.payloads[len(harness.jobs.payloads)-1])
	if !strings.Contains(payload, `"observe_files":["IMG_2.jpg"]`) {
		t.Fatalf("empty selection froze %s, want only the photo lacking an observation", payload)
	}
	if _, err := handler.StartObservation(context.Background(), connect.NewRequest(&postpilotv1.StartObservationRequest{PostSlug: "post"})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unauthenticated = %v", err)
	}
}

// POST-74, GEN-23: a published post and a post holding an active job refuse the observation job
// with the shared reasons, the active job's id carried.
func TestStartObservationRefusesPublishedAndBusyPosts(t *testing.T) {
	harness, handler := newStartHarness()
	ctx := auth.WithUser(context.Background(), "alice")
	request := &postpilotv1.StartObservationRequest{PostSlug: "post", ObserveModel: &postpilotv1.ModelRef{ProviderId: "p", ModelId: "eye"}}

	harness.post.Published = true
	_, err := handler.StartObservation(ctx, connect.NewRequest(request))
	if detail := generationAppErrorDetail(t, err); connect.CodeOf(err) != connect.CodeFailedPrecondition || detail.GetReason() != "POST_PUBLISHED_LOCKED" {
		t.Fatalf("published = %v", err)
	}
	harness.post.Published = false
	harness.jobs.err = &generation.JobAlreadyInProgressError{ActiveID: "job-active"}
	_, err = handler.StartObservation(ctx, connect.NewRequest(request))
	detail := generationAppErrorDetail(t, err)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || detail.GetReason() != "GENERATION_ALREADY_RUNNING" || detail.GetParams()["active_job_id"] != "job-active" {
		t.Fatalf("busy = %v (%#v)", err, detail)
	}
}

// GEN-89: a writing start still accepts a selection from an older client and ignores it — the
// run freezes only the photo lacking an observation.
func TestStartGenerationAcceptsAndIgnoresASelection(t *testing.T) {
	harness, handler := newStartHarness()
	ctx := auth.WithUser(context.Background(), "alice")
	response, err := handler.StartGeneration(ctx, connect.NewRequest(&postpilotv1.StartGenerationRequest{
		PostSlug: "post", Reobserve: &postpilotv1.ReobserveSelection{Files: []string{"IMG_1.jpg"}},
		ObserveModel: &postpilotv1.ModelRef{ProviderId: "p", ModelId: "eye"},
		WriteModel:   &postpilotv1.ModelRef{ProviderId: "p", ModelId: "pen"},
	}))
	if err != nil || response.Msg.GetJobId() != "generate-job" {
		t.Fatalf("response = %v, %v", response, err)
	}
	if calls := harness.jobs.generations[0].ObserveCalls; calls != 1 {
		t.Fatalf("ObserveCalls = %d, want 1", calls)
	}
	if payload := string(harness.jobs.payloads[0]); !strings.Contains(payload, `"observe_files":["IMG_2.jpg"]`) {
		t.Fatalf("payload froze %s, want only IMG_2.jpg", payload)
	}
}
