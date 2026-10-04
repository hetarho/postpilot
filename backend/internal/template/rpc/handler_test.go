package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/template"
)

// A store the test drives: every method answers what the case under test needs, and records
// what the handler asked it for, so the mapping is what is being checked — not the SQL.
type fakeStore struct {
	templates []template.Template
	inserted  template.Template
	patch     template.Patch
	deleted   string
	detached  int
	err       error
}

func (f *fakeStore) Insert(_ context.Context, t template.Template, _ int) error {
	f.inserted = t
	return f.err
}
func (f *fakeStore) List(context.Context, string) ([]template.Template, error) {
	return f.templates, f.err
}
func (f *fakeStore) Get(context.Context, string, string) (template.Template, error) {
	if len(f.templates) == 0 {
		return template.Template{}, template.ErrNotFound
	}
	return f.templates[0], f.err
}
func (f *fakeStore) Update(_ context.Context, _, _ string, patch template.Patch, _ time.Time, check func(template.Template) error) (template.Template, error) {
	f.patch = patch
	if f.err != nil {
		return template.Template{}, f.err
	}
	if check != nil {
		if err := check(f.templates[0]); err != nil {
			return template.Template{}, err
		}
	}
	return f.templates[0], nil
}
func (f *fakeStore) Delete(_ context.Context, _, id string) (int, error) {
	f.deleted = id
	return f.detached, f.err
}

func handler(store *fakeStore) *Handler {
	return NewHandler(template.NewService(store, template.NewLimits(template.Ceilings{
		NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200,
		MaxPerAccount: 3, PhotoRowMax: 4,
		AskLabelMaxChars: 40, AskMaxPerBody: 8,
	}, template.NumberBounds{TargetLengthMin: 100, TargetLengthMax: 10_000, TagCountMin: 1, TagCountMax: 10})))
}

func signedIn(t *testing.T) context.Context {
	t.Helper()
	return auth.WithUser(context.Background(), "alice")
}

func detail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("not a connect error: %v", err)
	}
	for _, d := range connectErr.Details() {
		value, decodeErr := d.Value()
		if decodeErr != nil {
			continue
		}
		if app, ok := value.(*postpilotv1.AppErrorDetail); ok {
			return app
		}
	}
	t.Fatalf("no app error detail on %v", err)
	return nil
}

func length(value int) *int { return &value }

func TestListTemplatesCarriesEveryFieldAcrossTheWireEdge(t *testing.T) {
	created := time.Date(2026, 9, 20, 1, 2, 3, 0, time.UTC)
	store := &fakeStore{templates: []template.Template{{
		ID: "t1", Name: "여행", Description: "여행 글", Body: "# 제목", TitleArea: "[여행] <write>여행지</write>", PostCount: 3,
		TargetLength: length(1200), CreatedAt: created, UpdatedAt: created,
	}, {ID: "t2", Name: "리뷰"}}}
	res, err := handler(store).ListTemplates(signedIn(t), connect.NewRequest(&postpilotv1.ListTemplatesRequest{}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	out := res.Msg.GetTemplates()
	if len(out) != 2 || out[0].GetId() != "t1" || out[0].GetName() != "여행" {
		t.Fatalf("templates = %+v", out)
	}
	if out[0].GetPostCount() != 3 || out[0].GetTargetLength() != 1200 {
		t.Fatalf("counts = %+v", out[0])
	}
	if out[0].GetTitleArea() != "[여행] <write>여행지</write>" || out[1].GetTitleArea() != "" {
		t.Fatalf("title areas = %q, %q", out[0].GetTitleArea(), out[1].GetTitleArea())
	}
	// "No opinion" survives the edge as an absent field rather than as a zero.
	if out[1].TargetLength != nil || out[1].TagCount != nil {
		t.Fatalf("an unset number became a value: %+v", out[1])
	}
	if out[0].GetCreatedAt() != "2026-09-20T01:02:03.000000000Z" {
		t.Fatalf("created at = %q", out[0].GetCreatedAt())
	}
}

func TestCreateAndUpdateCarryPresenceRatherThanZeroes(t *testing.T) {
	store := &fakeStore{templates: []template.Template{{ID: "t1", Name: "여행", Body: "# 제목"}}}
	h := handler(store)
	length := int32(900)
	_, err := h.CreateTemplate(signedIn(t), connect.NewRequest(&postpilotv1.CreateTemplateRequest{
		Name: "여행", Description: "설명", Body: "# 제목", TitleArea: "[여행] <write>여행지</write>", TargetLength: &length,
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if store.inserted.Name != "여행" || store.inserted.TargetLength == nil || *store.inserted.TargetLength != 900 {
		t.Fatalf("inserted = %+v", store.inserted)
	}
	if store.inserted.TitleArea != "[여행] <write>여행지</write>" {
		t.Fatalf("inserted title area = %q", store.inserted.TitleArea)
	}
	if store.inserted.TagCount != nil {
		t.Fatalf("an absent tag count became a value: %+v", store.inserted.TagCount)
	}

	name := "여행 2"
	if _, err := h.UpdateTemplate(signedIn(t), connect.NewRequest(&postpilotv1.UpdateTemplateRequest{
		Id: "t1", Name: &name,
	})); err != nil {
		t.Fatalf("update: %v", err)
	}
	// Presence is the edit unit: the fields the request did not carry are not in the patch,
	// while the two generation numbers always travel as one pair (TMPL-8).
	if store.patch.Name == nil || *store.patch.Name != "여행 2" {
		t.Fatalf("patch name = %+v", store.patch.Name)
	}
	if store.patch.Description != nil || store.patch.Body != nil || store.patch.TitleArea != nil {
		t.Fatalf("an untouched field reached the patch: %+v", store.patch)
	}
	if store.patch.Numbers == nil || store.patch.Numbers.TargetLength != nil {
		t.Fatalf("numbers = %+v", store.patch.Numbers)
	}
	// A present empty title area is an edit that clears it, not an absent field.
	empty := ""
	if _, err := h.UpdateTemplate(signedIn(t), connect.NewRequest(&postpilotv1.UpdateTemplateRequest{
		Id: "t1", TitleArea: &empty,
	})); err != nil {
		t.Fatalf("update: %v", err)
	}
	if store.patch.TitleArea == nil || *store.patch.TitleArea != "" || store.patch.Name != nil {
		t.Fatalf("patch = %+v", store.patch)
	}
}

func TestDeleteReportsHowManyPostsWereDetached(t *testing.T) {
	store := &fakeStore{detached: 4, templates: []template.Template{{ID: "t1", Name: "여행"}}}
	res, err := handler(store).DeleteTemplate(signedIn(t), connect.NewRequest(&postpilotv1.DeleteTemplateRequest{Id: "t1"}))
	if err != nil || res.Msg.GetDetachedPosts() != 4 || store.deleted != "t1" {
		t.Fatalf("delete = %+v, %v (store saw %q)", res.Msg, err, store.deleted)
	}
}

func TestEveryDomainRefusalHasItsOwnCodeAndReason(t *testing.T) {
	cases := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{template.ErrNameRequired, connect.CodeInvalidArgument, "TEMPLATE_NAME_REQUIRED"},
		{template.ErrBodyRequired, connect.CodeInvalidArgument, "TEMPLATE_BODY_REQUIRED"},
		{template.ErrDuplicateName, connect.CodeAlreadyExists, "TEMPLATE_NAME_TAKEN"},
		{template.ErrTooMany, connect.CodeFailedPrecondition, "TEMPLATE_LIMIT_REACHED"},
		{template.ErrNotFound, connect.CodeNotFound, "TEMPLATE_NOT_FOUND"},
		{&template.FieldTooLongError{Field: "name", Max: 60, Chars: 61}, connect.CodeInvalidArgument, "TEMPLATE_FIELD_TOO_LONG"},
		{&template.NumberOutOfRangeError{Field: "tagCount", Min: 1, Max: 10, Value: 20}, connect.CodeInvalidArgument, "TEMPLATE_NUMBER_OUT_OF_RANGE"},
		{&template.ParseError{Line: 4, Reason: "unknown_directive"}, connect.CodeInvalidArgument, "TEMPLATE_PARSE_FAILED"},
		{errors.New("disk on fire"), connect.CodeInternal, "UNKNOWN_FAILURE"},
	}
	for _, test := range cases {
		err := toConnectError("list templates", test.err)
		if connect.CodeOf(err) != test.code || detail(t, err).GetReason() != test.reason {
			t.Errorf("%v → code=%s reason=%s", test.err, connect.CodeOf(err), detail(t, err).GetReason())
		}
	}

	// The editor points at the offending line in its area, so every parse failure carries the
	// line, the reason and the area as params (TMPL-20).
	parse := detail(t, toConnectError("create template", &template.ParseError{Line: 4, Reason: "unknown_directive", Area: template.AreaBody}))
	if len(parse.GetParams()) != 3 || parse.GetParams()["line"] != "4" || parse.GetParams()["reason"] != "unknown_directive" || parse.GetParams()["area"] != "body" {
		t.Fatalf("body parse params = %+v", parse.GetParams())
	}
	titleErr := toConnectError("update template", &template.ParseError{Line: 1, Reason: template.ReasonNotInTitle, Area: template.AreaTitle})
	title := detail(t, titleErr)
	if len(title.GetParams()) != 3 || title.GetParams()["area"] != "title_area" || title.GetParams()["line"] != "1" || title.GetParams()["reason"] != "not_in_title" {
		t.Fatalf("title parse params = %+v", title.GetParams())
	}
	var connectErr *connect.Error
	if !errors.As(titleErr, &connectErr) || connectErr.Message() != "template does not parse" {
		t.Fatalf("parse message = %v", titleErr)
	}
	// A number with no product ceiling claims none.
	open := detail(t, toConnectError("create template", &template.NumberOutOfRangeError{Field: "targetLength", Min: 1, Value: 0}))
	if _, claimed := open.GetParams()["max"]; claimed {
		t.Fatalf("an open-ended number claimed a ceiling: %+v", open.GetParams())
	}
}

func TestEveryRpcRefusesAnAnonymousCaller(t *testing.T) {
	h := handler(&fakeStore{})
	calls := map[string]error{}
	_, calls["list"] = h.ListTemplates(context.Background(), connect.NewRequest(&postpilotv1.ListTemplatesRequest{}))
	_, calls["create"] = h.CreateTemplate(context.Background(), connect.NewRequest(&postpilotv1.CreateTemplateRequest{}))
	_, calls["update"] = h.UpdateTemplate(context.Background(), connect.NewRequest(&postpilotv1.UpdateTemplateRequest{}))
	_, calls["delete"] = h.DeleteTemplate(context.Background(), connect.NewRequest(&postpilotv1.DeleteTemplateRequest{}))
	for name, err := range calls {
		if connect.CodeOf(err) != connect.CodeUnauthenticated || detail(t, err).GetReason() != "AUTH_REQUIRED" {
			t.Errorf("%s without a session = %v", name, err)
		}
	}
}

// TMPL-41: the guide comes from the backend in the reader's language, with this process's own
// ceilings, and an unnamed language is refused rather than guessed.
func TestGetFormatGuide(t *testing.T) {
	h := handler(&fakeStore{})
	for language, phrase := range map[postpilotv1.ContentLanguage]string{
		postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:  "본문만 보내 주세요",
		postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH: "Send the body only",
	} {
		resp, err := h.GetFormatGuide(signedIn(t), connect.NewRequest(&postpilotv1.GetFormatGuideRequest{Language: language}))
		if err != nil {
			t.Fatalf("%v: %v", language, err)
		}
		text := resp.Msg.GetText()
		if !strings.Contains(text, phrase) || !strings.Contains(text, template.GuideExampleBody) {
			t.Errorf("%v guide = %q", language, text)
		}
		// AskMaxPerBody is 8 in this handler's limits: the number stated is the server's own.
		if !strings.Contains(text, "8") {
			t.Errorf("%v guide does not state the configured ask ceiling", language)
		}
	}

	_, err := h.GetFormatGuide(signedIn(t), connect.NewRequest(&postpilotv1.GetFormatGuideRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || detail(t, err).GetReason() != postpilotv1.FailureReason_CONTENT_LANGUAGE_REQUIRED.String() {
		t.Fatalf("unspecified language: err = %v", err)
	}

	_, err = h.GetFormatGuide(context.Background(), connect.NewRequest(&postpilotv1.GetFormatGuideRequest{Language: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("no session: err = %v", err)
	}
}

type rpcModels struct{ info llm.ModelInfo }

func (m rpcModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) { return m.info, true }
func (rpcModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	return llm.Response{}, errors.New("not called")
}

type rpcSamples struct{ err error }

func (s rpcSamples) RequestSample(context.Context, string, string) (template.Sample, error) {
	return template.Sample{Title: "글", Text: "[사진]"}, s.err
}

type rpcJobs struct {
	enqueue error
	payload []byte
	read    error
}

func (j rpcJobs) EnqueueRequest(context.Context, template.RequestJob) (string, error) {
	return "job-1", j.enqueue
}
func (rpcJobs) SaveRequestResult(context.Context, string, []byte) error { return nil }
func (j rpcJobs) CancelRequest(context.Context, string, string) error   { return j.read }
func (j rpcJobs) RequestPayload(context.Context, string, string) ([]byte, error) {
	return j.payload, j.read
}

// rpcBudget is the completion cap policy at the platform's default floor.
type rpcBudget struct{}

func (rpcBudget) Short(bool) int { return 8192 }

func requestHandler(models rpcModels, samples rpcSamples, jobs rpcJobs) *Handler {
	h := handler(&fakeStore{})
	h.service.ConfigureRequests(models, samples, jobs, rpcBudget{}, template.RequestLimits{MaxChars: 100, CorrectionsMax: 3, WishesMax: 5, WishMaxChars: 200})
	return h
}

func writer() rpcModels {
	return rpcModels{info: llm.ModelInfo{Stages: []string{llm.StageNameWrite}}}
}

func startRequest(text string) *connect.Request[postpilotv1.StartTemplateRequestRequest] {
	return connect.NewRequest(&postpilotv1.StartTemplateRequestRequest{
		WriteModel: &postpilotv1.ModelRef{ProviderId: "openrouter", ModelId: "writer"},
		Language:   postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN, Text: text,
		Draft: &postpilotv1.TemplateDraft{},
	})
}

func TestStartTemplateRequest(t *testing.T) {
	resp, err := requestHandler(writer(), rpcSamples{}, rpcJobs{}).StartTemplateRequest(signedIn(t), startRequest("맛집 리뷰"))
	if err != nil || resp.Msg.GetJobId() != "job-1" {
		t.Fatalf("start = %v, %v", resp, err)
	}
}

// Every refusal the request can state reaches the wire as the reason the browser renders.
func TestStartTemplateRequestRefusals(t *testing.T) {
	cases := []struct {
		name    string
		handler *Handler
		req     *connect.Request[postpilotv1.StartTemplateRequestRequest]
		code    connect.Code
		reason  postpilotv1.FailureReason
	}{
		{"empty", requestHandler(writer(), rpcSamples{}, rpcJobs{}), startRequest(" "), connect.CodeInvalidArgument, postpilotv1.FailureReason_TEMPLATE_REQUEST_EMPTY},
		{"too long", requestHandler(writer(), rpcSamples{}, rpcJobs{}), startRequest(strings.Repeat("가", 101)), connect.CodeInvalidArgument, postpilotv1.FailureReason_TEMPLATE_FIELD_TOO_LONG},
		{"running", requestHandler(writer(), rpcSamples{}, rpcJobs{enqueue: template.ErrRequestRunning}), startRequest("맛집"), connect.CodeFailedPrecondition, postpilotv1.FailureReason_TEMPLATE_REQUEST_RUNNING},
		{"no writer", requestHandler(rpcModels{}, rpcSamples{}, rpcJobs{}), startRequest("맛집"), connect.CodeFailedPrecondition, postpilotv1.FailureReason_GENERATION_WRITE_MODEL_REQUIRED},
		{"language", requestHandler(writer(), rpcSamples{}, rpcJobs{}), connect.NewRequest(&postpilotv1.StartTemplateRequestRequest{Text: "맛집"}), connect.CodeInvalidArgument, postpilotv1.FailureReason_CONTENT_LANGUAGE_REQUIRED},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.handler.StartTemplateRequest(signedIn(t), tc.req)
			if connect.CodeOf(err) != tc.code || detail(t, err).GetReason() != tc.reason.String() {
				t.Fatalf("err = %v", err)
			}
		})
	}

	sample := startRequest("")
	slug := "gone"
	sample.Msg.SamplePostSlug = &slug
	_, err := requestHandler(writer(), rpcSamples{err: template.ErrSampleUnavailable}, rpcJobs{}).StartTemplateRequest(signedIn(t), sample)
	if detail(t, err).GetReason() != postpilotv1.FailureReason_TEMPLATE_SAMPLE_UNAVAILABLE.String() {
		t.Fatalf("sample: err = %v", err)
	}
}

func TestGetTemplateRequestResult(t *testing.T) {
	jobs := rpcJobs{payload: []byte(`{"name":"리뷰","description":"방문기","title_area":"","body":"<write>인트로</write>","wishes":["친근하게"]}`)}
	resp, err := requestHandler(writer(), rpcSamples{}, jobs).GetTemplateRequestResult(signedIn(t), connect.NewRequest(&postpilotv1.GetTemplateRequestResultRequest{JobId: "job-1"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetDraft().GetName() != "리뷰" || resp.Msg.GetDraft().GetBody() != "<write>인트로</write>" || len(resp.Msg.GetWishes()) != 1 {
		t.Fatalf("result = %v", resp.Msg)
	}

	for read, reason := range map[error]postpilotv1.FailureReason{
		template.ErrRequestNotReady: postpilotv1.FailureReason_TEMPLATE_REQUEST_NOT_READY,
		template.ErrNotFound:        postpilotv1.FailureReason_TEMPLATE_NOT_FOUND,
	} {
		_, err := requestHandler(writer(), rpcSamples{}, rpcJobs{read: read}).GetTemplateRequestResult(signedIn(t), connect.NewRequest(&postpilotv1.GetTemplateRequestResultRequest{JobId: "job-1"}))
		if detail(t, err).GetReason() != reason.String() {
			t.Errorf("%v: err = %v", read, err)
		}
	}
}

type rpcEstimator struct {
	credits int
	ok      bool
}

func (e rpcEstimator) CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool) {
	return e.credits, e.ok
}

// The figure travels as credits only, and a model with none answers with neither field.
func TestEstimateTemplateRequest(t *testing.T) {
	estimate := func(level string, estimator rpcEstimator) *postpilotv1.EstimateTemplateRequestResponse {
		t.Helper()
		models := writer()
		models.info.Levels = map[string]string{llm.StageNameWrite: level}
		h := requestHandler(models, rpcSamples{}, rpcJobs{})
		h.service.ConfigureEstimate(estimator)
		resp, err := h.EstimateTemplateRequest(signedIn(t), connect.NewRequest(&postpilotv1.EstimateTemplateRequestRequest{
			WriteModel: &postpilotv1.ModelRef{ProviderId: "openrouter", ModelId: "writer"},
		}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg
	}
	if got := estimate("balanced", rpcEstimator{credits: 2, ok: true}); got.GetFree() || got.Credits == nil || got.GetCredits() != 2 {
		t.Fatalf("priced = %v", got)
	}
	if got := estimate("free", rpcEstimator{credits: 2, ok: true}); !got.GetFree() || got.Credits != nil {
		t.Fatalf("free = %v", got)
	}
	if got := estimate("balanced", rpcEstimator{}); got.GetFree() || got.Credits != nil {
		t.Fatalf("unpriced = %v", got)
	}
}
