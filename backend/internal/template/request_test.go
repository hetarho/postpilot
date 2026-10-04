package template

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// fakeModels answers each Complete with the next scripted response, and records every request
// so a test can read what the model was taught and given.
type fakeModels struct {
	info      llm.ModelInfo
	known     bool
	responses []llm.Response
	errs      []error
	requests  []llm.Request
}

func (f *fakeModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) { return f.info, f.known }

func (f *fakeModels) Complete(_ context.Context, _ llm.ModelRef, req llm.Request) (llm.Response, error) {
	f.requests = append(f.requests, req)
	i := len(f.requests) - 1
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	if i < len(f.responses) {
		return f.responses[i], err
	}
	return llm.Response{}, err
}

type fakeSamples struct {
	sample Sample
	err    error
	asked  string
}

func (f *fakeSamples) RequestSample(_ context.Context, _, slug string) (Sample, error) {
	f.asked = slug
	return f.sample, f.err
}

type fakeRequestJobs struct {
	enqueued  []RequestJob
	enqueue   error
	saved     map[string][]byte
	payload   []byte
	payloadEr error
	cancelled []string
	cancel    error
}

func (f *fakeRequestJobs) EnqueueRequest(_ context.Context, job RequestJob) (string, error) {
	f.enqueued = append(f.enqueued, job)
	if f.enqueue != nil {
		return "", f.enqueue
	}
	return "job-1", nil
}

func (f *fakeRequestJobs) CancelRequest(_ context.Context, _, jobID string) error {
	f.cancelled = append(f.cancelled, jobID)
	return f.cancel
}

func (f *fakeRequestJobs) SaveRequestResult(_ context.Context, jobID string, payload []byte) error {
	if f.saved == nil {
		f.saved = map[string][]byte{}
	}
	f.saved[jobID] = payload
	return nil
}

func (f *fakeRequestJobs) RequestPayload(context.Context, string, string) ([]byte, error) {
	return f.payload, f.payloadEr
}

// fakeBudget is the completion cap policy in the shape internal/platform/config serves it at the
// default floor: 8,192, doubled for a native-effort model. It lives here rather than importing
// config: the template context receives a policy and holds no number of its own.
type fakeBudget struct{}

func (fakeBudget) Short(nativeEffort bool) int {
	if nativeEffort {
		return 16384
	}
	return 8192
}

func requestLimits() RequestLimits {
	return RequestLimits{MaxChars: 50, CorrectionsMax: 3, WishesMax: 2, WishMaxChars: 10}
}

func writerInfo() llm.ModelInfo {
	return llm.ModelInfo{Stages: []string{llm.StageNameWrite}, StructuredOutput: true}
}

type requestHarness struct {
	service *Service
	store   *fakeStore
	models  *fakeModels
	samples *fakeSamples
	jobs    *fakeRequestJobs
}

func newRequestHarness(t *testing.T) requestHarness {
	t.Helper()
	store := newFakeStore()
	service := NewService(store, testLimits())
	h := requestHarness{
		service: service, store: store,
		models:  &fakeModels{info: writerInfo(), known: true},
		samples: &fakeSamples{sample: Sample{Title: "성수 카페", Text: "들어가자마자 향이 좋았다\n\n[사진]"}},
		jobs:    &fakeRequestJobs{},
	}
	service.ConfigureRequests(h.models, h.samples, h.jobs, fakeBudget{}, requestLimits())
	return h
}

func validStart() StartRequest {
	return StartRequest{WriteModel: "openrouter/writer", Language: LanguageKorean, Text: "맛집 리뷰 템플릿"}
}

func TestStartRequestEnqueuesTheFrozenInputWithEveryCorrectionPriced(t *testing.T) {
	h := newRequestHarness(t)
	start := validStart()
	start.Draft = Draft{Name: "초안", Body: "<write>인트로"}
	start.SamplePostSlug = "post-1"
	id, err := h.service.StartRequest(context.Background(), "alice", start)
	if err != nil || id != "job-1" {
		t.Fatalf("StartRequest = %q, %v", id, err)
	}
	if len(h.jobs.enqueued) != 1 {
		t.Fatalf("enqueued %d jobs", len(h.jobs.enqueued))
	}
	job := h.jobs.enqueued[0]
	if job.UserID != "alice" || job.WriteModel != "openrouter/writer" || job.Calls != 4 || job.CompletionTokens != 8192 {
		t.Fatalf("job = %+v", job)
	}
	var input requestInput
	if err := json.Unmarshal(job.Payload, &input); err != nil {
		t.Fatal(err)
	}
	if input.CompletionTokens != job.CompletionTokens {
		t.Fatalf("the input froze %d tokens, the hold priced %d", input.CompletionTokens, job.CompletionTokens)
	}
	// An unparsable draft body is accepted: the request may be "fix this".
	if input.Text != "맛집 리뷰 템플릿" || input.Draft.Body != "<write>인트로" || input.Sample == nil || input.Sample.Title != "성수 카페" {
		t.Fatalf("frozen input = %+v", input)
	}
	if h.samples.asked != "post-1" {
		t.Fatalf("sample asked for %q", h.samples.asked)
	}
}

func TestStartRequestRefusesBeforeAnyHold(t *testing.T) {
	cases := []struct {
		name  string
		setup func(h *requestHarness, start *StartRequest)
		want  func(error) bool
	}{
		{"box over its ceiling", func(_ *requestHarness, s *StartRequest) { s.Text = strings.Repeat("가", 51) }, func(err error) bool {
			var tooLong *FieldTooLongError
			return errors.As(err, &tooLong) && tooLong.Field == "request" && tooLong.Max == 50
		}},
		{"blank text without a sample", func(_ *requestHarness, s *StartRequest) { s.Text = "   " }, func(err error) bool { return errors.Is(err, ErrRequestEmpty) }},
		{"draft field over its ceiling", func(_ *requestHarness, s *StartRequest) { s.Draft.Name = strings.Repeat("가", 41) }, func(err error) bool {
			var tooLong *FieldTooLongError
			return errors.As(err, &tooLong) && tooLong.Field == "name"
		}},
		{"foreign template", func(_ *requestHarness, s *StartRequest) { s.TemplateID = "missing" }, func(err error) bool { return errors.Is(err, ErrNotFound) }},
		{"new template at the cap", func(h *requestHarness, _ *StartRequest) {
			for _, id := range []string{"a", "b", "c"} {
				h.store.rows[id] = Template{ID: id, UserID: "alice", Name: id}
			}
		}, func(err error) bool { return errors.Is(err, ErrTooMany) }},
		{"unreadable sample", func(h *requestHarness, s *StartRequest) {
			s.SamplePostSlug = "gone"
			h.samples.err = ErrSampleUnavailable
		}, func(err error) bool { return errors.Is(err, ErrSampleUnavailable) }},
		{"missing write ref", func(_ *requestHarness, s *StartRequest) { s.WriteModel = "" }, func(err error) bool { return errors.Is(err, ErrWriteModelRequired) }},
		{"unknown write model", func(h *requestHarness, _ *StartRequest) { h.models.known = false }, func(err error) bool { return errors.Is(err, ErrWriteModelRequired) }},
		{"disabled write model", func(h *requestHarness, _ *StartRequest) { h.models.info.Disabled = true }, func(err error) bool { return errors.Is(err, ErrWriteModelRequired) }},
		{"not a writer", func(h *requestHarness, _ *StartRequest) { h.models.info.Stages = []string{llm.StageNameObserve} }, func(err error) bool { return errors.Is(err, ErrWriteModelRequired) }},
		{"unsupported language", func(_ *requestHarness, s *StartRequest) { s.Language = "ja" }, func(err error) bool { return errors.Is(err, ErrUnsupportedLanguage) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newRequestHarness(t)
			start := validStart()
			tc.setup(&h, &start)
			_, err := h.service.StartRequest(context.Background(), "alice", start)
			if !tc.want(err) {
				t.Fatalf("err = %v", err)
			}
			if len(h.jobs.enqueued) != 0 {
				t.Fatalf("a refused request was enqueued")
			}
		})
	}
}

// A stored template's request is not bounded by the cap: it creates nothing (TMPL-62).
func TestStartRequestOnAStoredTemplateIgnoresTheCap(t *testing.T) {
	h := newRequestHarness(t)
	for _, id := range []string{"a", "b", "c"} {
		h.store.rows[id] = Template{ID: id, UserID: "alice", Name: id}
	}
	start := validStart()
	start.TemplateID = "a"
	if _, err := h.service.StartRequest(context.Background(), "alice", start); err != nil {
		t.Fatal(err)
	}
}

func TestStartRequestMapsARunningRequest(t *testing.T) {
	h := newRequestHarness(t)
	h.jobs.enqueue = ErrRequestRunning
	if _, err := h.service.StartRequest(context.Background(), "alice", validStart()); !errors.Is(err, ErrRequestRunning) {
		t.Fatalf("err = %v", err)
	}
}

// A sample alone is a request: the box may be blank when a post is attached (TMPL-58).
func TestStartRequestTakesASampleWithNoText(t *testing.T) {
	h := newRequestHarness(t)
	start := validStart()
	start.Text, start.SamplePostSlug = "", "post-1"
	if _, err := h.service.StartRequest(context.Background(), "alice", start); err != nil {
		t.Fatal(err)
	}
}

func answer(t *testing.T, fields map[string]any) llm.Response {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return llm.Response{Text: string(raw), FinishReason: "stop"}
}

func goodAnswer(t *testing.T) llm.Response {
	return answer(t, map[string]any{
		"name": "  맛집 리뷰  ", "description": "맛집 방문기", "title_area": "<write>가게 이름</write>",
		"body": "<write>방문 이유</write>\n<slot kind=\"photo\" count=\"2\"/>", "wishes": []string{"  친근하게  ", "", "가격은 빼고 써 주세요 꼭", "짧게"},
	})
}

func runPayload(t *testing.T, input requestInput) []byte {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func runRequest(t *testing.T, h requestHarness, input requestInput) error {
	t.Helper()
	return h.service.RunRequest(context.Background(), RequestRun{
		ID: "job-1", UserID: "alice", WriteModel: "openrouter/writer", Payload: runPayload(t, input),
	}, func(string, int, int) {})
}

func savedResult(t *testing.T, h requestHarness) RequestResult {
	t.Helper()
	var result RequestResult
	if err := json.Unmarshal(h.jobs.saved["job-1"], &result); err != nil {
		t.Fatalf("saved result: %v", err)
	}
	return result
}

func TestRunRequestSavesACheckedAnswer(t *testing.T) {
	h := newRequestHarness(t)
	h.models.responses = []llm.Response{goodAnswer(t)}
	if err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"}); err != nil {
		t.Fatal(err)
	}
	result := savedResult(t, h)
	if result.Name != "맛집 리뷰" || result.TitleArea != "<write>가게 이름</write>" {
		t.Fatalf("result = %+v", result)
	}
	// Trimmed, blanks dropped, the first two kept, each cut to ten characters (TMPL-61).
	if want := []string{"친근하게", "가격은 빼고 써 주"}; strings.Join(result.Wishes, "|") != strings.Join(want, "|") {
		t.Fatalf("wishes = %q", result.Wishes)
	}
	if len(h.models.requests) != 1 {
		t.Fatalf("calls = %d", len(h.models.requests))
	}
	req := h.models.requests[0]
	// A payload frozen without a cap is an ordinary model's: the floor.
	if req.Stage != llm.StageNameWrite || req.Reasoning != llm.ReasoningLow || req.MaxTokens != 8192 || req.JSONSchema == nil {
		t.Fatalf("request = %+v", req)
	}
}

// GEN-22, QUOTA-67: a native-effort model's request freezes the doubled cap at start, the hold
// prices it, and the first call and every correction send it — even after the catalog flag
// flips between the enqueue and the run.
func TestARequestSendsTheCapItsStartFrozeOnEveryCall(t *testing.T) {
	h := newRequestHarness(t)
	h.models.info.ReasoningNativeEffort = true
	if _, err := h.service.StartRequest(context.Background(), "alice", validStart()); err != nil {
		t.Fatal(err)
	}
	job := h.jobs.enqueued[0]
	var input requestInput
	if err := json.Unmarshal(job.Payload, &input); err != nil {
		t.Fatal(err)
	}
	if job.CompletionTokens != 16384 || input.CompletionTokens != 16384 {
		t.Fatalf("the hold priced %d and the input froze %d, want both 16384", job.CompletionTokens, input.CompletionTokens)
	}
	h.models.info.ReasoningNativeEffort = false
	notJSON := llm.Response{Text: "템플릿을 만들었어요!", FinishReason: "stop"}
	h.models.responses = []llm.Response{notJSON, notJSON, notJSON, goodAnswer(t)}
	if err := h.service.RunRequest(context.Background(), RequestRun{
		ID: "job-1", UserID: "alice", WriteModel: job.WriteModel, Payload: job.Payload,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(h.models.requests) != 4 {
		t.Fatalf("calls = %d, want the first and three corrections", len(h.models.requests))
	}
	for i, req := range h.models.requests {
		if req.MaxTokens != 16384 {
			t.Errorf("call %d sent %d tokens, want the frozen 16384", i, req.MaxTokens)
		}
	}
}

// The model is taught the very guide the copy button gives, then the request rules, and given
// the request, the draft and the sample — nothing else (TMPL-59).
func TestRunRequestTeachesTheSharedGuideAndGivesOnlyTheRequest(t *testing.T) {
	h := newRequestHarness(t)
	h.models.responses = []llm.Response{goodAnswer(t)}
	input := requestInput{
		Language: LanguageKorean, Text: "사진 줄을 2장으로 바꿔줘",
		Draft:  Draft{Name: "기존 이름", Body: "<write>인트로</write>"},
		Sample: &Sample{Title: "성수 카페", Text: "[사진]"},
	}
	if err := runRequest(t, h, input); err != nil {
		t.Fatal(err)
	}
	req := h.models.requests[0]
	guide, err := FormatGuide(LanguageKorean, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(req.System, guide) || !strings.Contains(req.System, "[이번 요청에서 지켜야 할 것]") {
		t.Fatalf("system does not start with the shared guide and carry the rules")
	}
	if !strings.Contains(req.System, `<ask label="직접 겪은 일" required="true">방문 중 직접 겪고 확인한 일은 무엇인가요?</ask>`) {
		t.Fatalf("request rules do not teach required fields: %s", req.System)
	}
	if strings.Contains(req.System, "{nameMax}") || !strings.Contains(req.System, "40자까지") {
		t.Fatalf("rules did not state the configured ceilings")
	}
	user := req.Messages[0].Parts[0].Text
	for _, want := range []string{"[요청]\n사진 줄을 2장으로 바꿔줘", "이름: 기존 이름", "설명: (비어 있음)", "본문:\n<write>인트로</write>", "[참고 글]\n제목: 성수 카페\n[사진]"} {
		if !strings.Contains(user, want) {
			t.Errorf("user message lacks %q:\n%s", want, user)
		}
	}
	if len(req.Messages) != 1 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
}

func TestRequestRulesAllowConcreteExperienceQuestions(t *testing.T) {
	svc, _ := newService(t)
	for language, phrases := range map[Language][]string{
		LanguageKorean:  {`<ask label="직접 겪은 일" required="true">방문 중 직접 겪고 확인한 일은 무엇인가요?</ask>`, "실제 경험·확인한 사실·불확실한 점을 묻는 구체적인 질문", "말투·길이·서식·생략·반복을 지시하지"},
		LanguageEnglish: {`<ask label="firsthand experience" required="true">What did you personally experience or verify?</ask>`, "firsthand experience, verified facts, or uncertainty", "tone, length, formatting, omissions, or repetition"},
	} {
		system, err := svc.requestSystem(language)
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range phrases {
			if !strings.Contains(system, phrase) {
				t.Errorf("%s request rules do not say %q", language, phrase)
			}
		}
	}
}

func TestRunRequestCorrectsAnAnswerThatDoesNotParse(t *testing.T) {
	h := newRequestHarness(t)
	broken := answer(t, map[string]any{"name": "리뷰", "description": "", "title_area": "", "body": "<write>닫히지 않음", "wishes": []string{}})
	h.models.responses = []llm.Response{broken, goodAnswer(t)}
	if err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"}); err != nil {
		t.Fatal(err)
	}
	if len(h.models.requests) != 2 {
		t.Fatalf("calls = %d", len(h.models.requests))
	}
	second := h.models.requests[1].Messages
	if len(second) != 3 || second[1].Role != llm.RoleAssistant || second[1].Parts[0].Text != broken.Text {
		t.Fatalf("the correction does not carry the previous answer back: %+v", second)
	}
	if correction := second[2].Parts[0].Text; !strings.Contains(correction, "body 1번째 줄") || !strings.Contains(correction, "unclosed_tag") {
		t.Fatalf("correction = %q", correction)
	}
	if savedResult(t, h).Name != "맛집 리뷰" {
		t.Fatalf("the corrected answer was not saved")
	}
}

// Each correction is the request, the last answer and what it broke: an earlier wrong answer is
// not sent again, so a run's prompt grows by one answer per call, not by every answer so far.
func TestACorrectionCarriesOnlyTheAnswerItCorrects(t *testing.T) {
	h := newRequestHarness(t)
	unclosed := answer(t, map[string]any{"name": "리뷰", "description": "", "title_area": "", "body": "<write>닫히지 않음", "wishes": []string{}})
	tooLong := answer(t, map[string]any{"name": strings.Repeat("가", 41), "description": "", "title_area": "", "body": "<write>인트로</write>", "wishes": []string{}})
	h.models.responses = []llm.Response{unclosed, tooLong, goodAnswer(t)}
	if err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"}); err != nil {
		t.Fatal(err)
	}
	if len(h.models.requests) != 3 {
		t.Fatalf("calls = %d, want the first and two corrections", len(h.models.requests))
	}
	ask := h.models.requests[0].Messages[0].Parts[0].Text
	for i, corrected := range []llm.Response{unclosed, tooLong} {
		messages := h.models.requests[i+1].Messages
		if len(messages) != 3 {
			t.Fatalf("call %d carried %d messages, want the request, the last answer and the correction", i+2, len(messages))
		}
		if messages[0].Role != llm.RoleUser || messages[0].Parts[0].Text != ask {
			t.Errorf("call %d does not open with the request: %+v", i+2, messages[0])
		}
		if messages[1].Role != llm.RoleAssistant || messages[1].Parts[0].Text != corrected.Text {
			t.Errorf("call %d carries answer %q, want the one it corrects", i+2, messages[1].Parts[0].Text)
		}
		if messages[2].Role != llm.RoleUser {
			t.Errorf("call %d does not end with the correction: %+v", i+2, messages[2])
		}
	}
	if correction := h.models.requests[2].Messages[2].Parts[0].Text; !strings.Contains(correction, "name가 41자로") || strings.Contains(correction, "unclosed_tag") {
		t.Fatalf("the third call corrects %q, want only the second answer's broken rule", correction)
	}
	if savedResult(t, h).Name != "맛집 리뷰" {
		t.Fatalf("the corrected answer was not saved")
	}
}

func TestRunRequestFailsAfterTheLastCorrection(t *testing.T) {
	h := newRequestHarness(t)
	notJSON := llm.Response{Text: "템플릿을 만들었어요!", FinishReason: "stop"}
	h.models.responses = []llm.Response{notJSON, notJSON, notJSON, notJSON, goodAnswer(t)}
	err := runRequest(t, h, requestInput{Language: LanguageEnglish, Text: "a review"})
	var invalid *RequestAnswerInvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v", err)
	}
	if invalid.Failure().Reason != "TEMPLATE_REQUEST_ANSWER_INVALID" {
		t.Fatalf("reason = %q", invalid.Failure().Reason)
	}
	// The first call and three corrections, every one of them billed by the metered registry.
	if len(h.models.requests) != 4 {
		t.Fatalf("calls = %d", len(h.models.requests))
	}
	if h.jobs.saved != nil {
		t.Fatalf("an invalid answer was saved")
	}
	if correction := h.models.requests[1].Messages[2].Parts[0].Text; !strings.Contains(correction, "not the requested JSON object") {
		t.Fatalf("english correction = %q", correction)
	}
}

func TestRunRequestCorrectsAFieldRule(t *testing.T) {
	h := newRequestHarness(t)
	long := answer(t, map[string]any{"name": strings.Repeat("가", 41), "description": "", "title_area": "", "body": "<write>인트로</write>", "wishes": []string{}})
	h.models.responses = []llm.Response{long, goodAnswer(t)}
	if err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"}); err != nil {
		t.Fatal(err)
	}
	if correction := h.models.requests[1].Messages[2].Parts[0].Text; !strings.Contains(correction, "name가 41자로, 40자까지만") {
		t.Fatalf("correction = %q", correction)
	}
}

// A truncation is not the model's to fix by being told: it fails at once (TMPL-60).
func TestRunRequestDoesNotCorrectATruncation(t *testing.T) {
	h := newRequestHarness(t)
	h.models.responses = []llm.Response{{Text: `{"name": "리뷰", "body": "<write>`, FinishReason: "length"}}
	err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"})
	if !errors.Is(err, llm.ErrOutputTruncated) {
		t.Fatalf("err = %v", err)
	}
	if len(h.models.requests) != 1 {
		t.Fatalf("calls = %d", len(h.models.requests))
	}
}

func TestRunRequestDoesNotCorrectAProviderFailure(t *testing.T) {
	h := newRequestHarness(t)
	h.models.errs = []error{llm.ErrRateLimited}
	if err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"}); !errors.Is(err, llm.ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
	if len(h.models.requests) != 1 {
		t.Fatalf("calls = %d", len(h.models.requests))
	}
}

// A model without structured output still answers: the rules state the same shape.
func TestRunRequestAttachesTheSchemaOnlyForStructuredOutput(t *testing.T) {
	h := newRequestHarness(t)
	h.models.info.StructuredOutput = false
	h.models.responses = []llm.Response{{Text: "```json\n" + goodAnswer(t).Text + "\n```", FinishReason: "stop"}}
	if err := runRequest(t, h, requestInput{Language: LanguageKorean, Text: "맛집"}); err != nil {
		t.Fatal(err)
	}
	if h.models.requests[0].JSONSchema != nil {
		t.Fatalf("a schema was attached to a model without structured output")
	}
}

func TestRequestResultReadsTheAnswer(t *testing.T) {
	h := newRequestHarness(t)
	h.jobs.payload = []byte(`{"name":"리뷰","description":"","title_area":"","body":"<write>인트로</write>"}`)
	result, err := h.service.RequestResult(context.Background(), "alice", "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "리뷰" || result.Wishes == nil || len(result.Wishes) != 0 {
		t.Fatalf("result = %+v", result)
	}
	h.jobs.payloadEr = ErrRequestNotReady
	if _, err := h.service.RequestResult(context.Background(), "alice", "job-1"); !errors.Is(err, ErrRequestNotReady) {
		t.Fatalf("err = %v", err)
	}
}
