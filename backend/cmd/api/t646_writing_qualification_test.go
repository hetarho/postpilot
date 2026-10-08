package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	"github.com/postpilot/backend/internal/authoring"
	authoringstore "github.com/postpilot/backend/internal/authoring/store"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
)

const qualificationProse = "It tasted good. A blue cup. A fragrant note🙂."
const qualificationOrigins = `[{"field":{"kind":"block_content","block_index":0},"quote":"It tasted good","category":"owner_input","source_refs":["current.memo"]},{"field":{"kind":"block_content","block_index":0},"quote":"A blue cup","category":"photo_interpretation","source_refs":["current.visual.0.0"]},{"field":{"kind":"block_content","block_index":0},"quote":"A fragrant note🙂","category":"ai_added","source_refs":[]}]`
const qualificationPlanOrigins = `[{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"It tasted good","category":"owner_input","source_refs":["current.memo"]},{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"A blue cup","category":"photo_interpretation","source_refs":["current.visual.0.0"]},{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"A fragrant note🙂","category":"ai_added","source_refs":[]}]`

// Only the provider is synthetic. Registry resolution, registered Connect
// handlers, owner sessions, SQLite, admission, queue and every callback are real.
type writingQualificationProvider struct {
	mu               sync.Mutex
	requests         []llm.Request
	qualifications   int
	originTail       *string
	entered, release chan struct{}
	gateOnce         sync.Once
}

func (*writingQualificationProvider) Name() string { return "fixture" }
func (p *writingQualificationProvider) HasFreePath(context.Context, string, llm.FreePath) (bool, error) {
	p.mu.Lock()
	p.qualifications++
	p.mu.Unlock()
	return true, nil
}
func (p *writingQualificationProvider) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	p.mu.Lock()
	p.requests = append(p.requests, request)
	tail := p.originTail
	p.mu.Unlock()
	if p.entered != nil {
		p.gateOnce.Do(func() { close(p.entered); <-p.release })
	}
	plan := `"storyline":[{"text":"` + qualificationProse + `","files":["owned.jpg"]}]`
	body := `"title":"Qualified title","summary":"Owner taste and visual cup","tags":[],"blocks":[{"type":"TEXT","content":"` + qualificationProse + `"}],"nouns":[]`
	answer := "{" + body + `,"origins":` + qualificationOrigins + "}"
	switch request.Composition.Mode {
	case "observation":
		answer = `{"observations":[{"file":"owned.jpg","scene":"A blue cup","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":0}],"origins":[{"file":"owned.jpg","field":{"kind":"observation_scene"},"quote":"A blue cup","category":"photo_interpretation","source_refs":["media.0"]}]}`
	case "storyline-create", "storyline-rewrite":
		answer = "{" + plan + `,"origins":` + qualificationPlanOrigins + "}"
	case "direct", "full-test-direct":
		origins := qualificationOrigins[:len(qualificationOrigins)-1] + "," + qualificationPlanOrigins[1:]
		answer = "{" + plan + "," + body + `,"origins":` + origins + "}"
	case "revision":
		// Missing new candidates cannot relabel unchanged, proved prior meanings.
		answer = "{" + strings.ReplaceAll(body, "Qualified title", "Revised title") + "}"
	case "post_guideline/recommend":
		answer = `{"candidates":[{"name":"Grounded writing","body":"Preserve supplied facts and distinguish sensory proposals."}]}`
	}
	if request.Stage == "observe" {
		answer = `{"observations":[{"file":"owned.jpg","scene":"A blue cup","mood":"","visible_text":"","objects":[],"people_present":false,"rotation":0}],"origins":[{"file":"owned.jpg","field":{"kind":"observation_scene"},"quote":"A blue cup","category":"photo_interpretation","source_refs":["media.0"]}]}`
	}
	if tail != nil && request.Stage == "write" {
		answer = "{" + strings.ReplaceAll(plan, `"files":["owned.jpg"]`, `"files":[]`) + "," + body + *tail + "}"
	}
	return llm.Response{Text: answer, FinishReason: "stop", Usage: llm.Usage{PromptTokens: 17, CompletionTokens: 9, ReasoningTokens: 3, CostMicrousd: 987654321, CostReported: true}}, nil
}

type writingQualificationHarness struct {
	*writingIntegrationHarness
	witness    *writingQualificationProvider
	posts      postpilotv1connect.PostServiceClient
	generation postpilotv1connect.GenerationServiceClient
	stop       func()
}

func newWritingQualificationHarness(t *testing.T, witness *writingQualificationProvider) *writingQualificationHarness {
	t.Helper()
	t.Setenv("MAIL_DRIVER", "log")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := wiringPlatform(t, cfg)
	p.bucket = writingPhotoBucket(t)
	p.registry, err = llm.Parse([]byte("providers:\n  - id: fixture\n    adapter: fixture\n    base_url: https://PRIVATE_ENDPOINT.invalid\n    api_key_env: PRIVATE_KEY\n"), func(string) string { return "PRIVATE_CREDENTIAL" }, map[string]llm.AdapterFactory{"fixture": func(llm.AdapterConfig) (llm.Provider, error) { return witness, nil }}, writingIntegrationModels{}, llm.Options{Timeout: time.Minute, MaxTokens: 32768})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, user := range []string{"alice", "bob"} {
		if _, err := p.db.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?,?,?)`, user, "fixture", now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(user + "-authenticated-session"))
		if _, err := p.db.Writer.Exec(`INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES(?,?,?,?)`, hex.EncodeToString(hash[:]), user, now.Add(time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	app, err := buildContexts(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	registerJobs(app)
	server := rpcserver.New(cfg, "writing-qualification", rpcserver.Options{Handlers: handlers(app), Interceptors: []connect.Interceptor{authrpc.NewInterceptor(app.auth, app.throttle, cfg.ClientIPHeader)}})
	httpServer := httptest.NewServer(server.Handler)
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan struct{})
	go func() { defer close(finished); app.jobs.Run(ctx) }()
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { cancel(); <-finished }) }
	t.Cleanup(stop)
	return &writingQualificationHarness{writingIntegrationHarness: &writingIntegrationHarness{app: app, platform: p, client: postpilotv1connect.NewWritingTestServiceClient(httpServer.Client(), httpServer.URL), inspection: postpilotv1connect.NewWritingInspectionServiceClient(httpServer.Client(), httpServer.URL), cookie: auth.SessionCookieName + "=alice-authenticated-session"}, witness: witness, posts: postpilotv1connect.NewPostServiceClient(httpServer.Client(), httpServer.URL), generation: postpilotv1connect.NewGenerationServiceClient(httpServer.Client(), httpServer.URL), stop: stop}
}

func (h *writingQualificationHarness) waitJob(t *testing.T, id string, expected string) {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); ; {
		found, err := h.app.jobs.Get(t.Context(), id, "alice")
		if err != nil {
			t.Fatal(err)
		}
		if job.Terminal(found.Status) {
			if found.Status != expected {
				t.Fatalf("job %s = %s, expected %s: %+v", id, found.Status, expected, found.Failure)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("qualification job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func qualificationScan(t *testing.T, value proto.Message, additional ...string) {
	t.Helper()
	raw, err := protojson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range append([]string{"PRIVATE_ENDPOINT", "PRIVATE_CREDENTIAL", "PRIVATE_STORAGE_KEY", "BOB_PRIVATE_MATERIAL", "987654321", "costMicrousd", "CostMicrousd", "supplierCost", "api_key", "X-Amz-", "data:image", "base_url"}, additional...) {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("nested private inspection leaked %q", forbidden)
		}
	}
}

func qualificationCategories(t *testing.T, review *v1.OriginReview) {
	t.Helper()
	if review == nil || len(review.Spans) != 3 {
		t.Fatalf("expected three independently grounded phrases: %+v", review)
	}
	want := map[string]v1.SemanticOriginCategory{"It tasted good": v1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_OWNER_INPUT, "A blue cup": v1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_PHOTO_INTERPRETATION, "A fragrant note🙂": v1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_AI_ADDED}
	for _, span := range review.Spans {
		if category, ok := want[span.Quote]; !ok || span.Category != category {
			t.Fatalf("meaning was promoted or retargeted: %+v", span)
		}
		delete(want, span.Quote)
	}
	if len(want) != 0 {
		t.Fatal("missing origin meaning", want)
	}
}

func TestT646RegisteredWritingLineageThroughManualRevisionChampionAndExpiry(t *testing.T) {
	h := newWritingQualificationHarness(t, &writingQualificationProvider{})
	ref := &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-0"}
	for _, stage := range []provider.Stage{provider.StageWrite, provider.StageObserve} {
		if _, err := h.app.provider.SaveSelection(t.Context(), "alice", stage, llm.ModelRef{ProviderID: ref.ProviderId, ModelID: ref.ModelId}); err != nil {
			t.Fatal(err)
		}
	}
	language := v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
	bobLanguage := post.LanguageEnglish
	if _, err := h.app.post.SaveDraft(t.Context(), "bob", post.DraftSave{Title: "BOB_PRIVATE_MATERIAL", Memo: "BOB_PRIVATE_MATERIAL", TargetLanguage: &bobLanguage}); err != nil {
		t.Fatal(err)
	}
	created, err := h.posts.SavePostDraft(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.SavePostDraftRequest{Title: "Owner source", Memo: "It tasted good. OWNER_PRIVATE_MATERIAL", TargetLanguage: &language}))
	if err != nil {
		t.Fatal(err)
	}
	slug := created.Msg.Post.Slug
	length := 1600
	if _, err := h.app.post.SaveGenerationOptions(t.Context(), "alice", slug, post.GenerationOptionsSet{TargetLength: &length, TagCount: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.platform.db.Writer.Exec(`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('owned-photo',?,'owned.jpg','PRIVATE_STORAGE_KEY/photo.jpg',100,100,4,?)`, slug, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	readInspection := func(stage string, status v1.InspectionStatus) *v1.GetPostRequestInspectionResponse {
		t.Helper()
		response, err := h.inspection.GetPostRequestInspection(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequestInspectionRequest{PostSlug: slug, Stage: stage, Status: status}))
		if err != nil {
			t.Fatal(err)
		}
		qualificationScan(t, response.Msg)
		return response.Msg
	}
	h.witness.mu.Lock()
	qualificationsBefore := h.witness.qualifications
	h.witness.mu.Unlock()
	for _, status := range []v1.InspectionStatus{v1.InspectionStatus_INSPECTION_STATUS_CURRENT, v1.InspectionStatus_INSPECTION_STATUS_PREPARED} {
		view := readInspection("observe", status)
		if view.Inspection.Status != status || view.Inspection.IssuedAt != nil {
			t.Fatal("preview confused with execution", view)
		}
	}
	if absent := readInspection("write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED); absent.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE || len(absent.Inspections) != 0 {
		t.Fatal("missing history was reconstructed")
	}
	h.witness.mu.Lock()
	if len(h.witness.requests) != 0 || h.witness.qualifications != qualificationsBefore {
		t.Fatal("preview executed provider")
	}
	h.witness.mu.Unlock()
	var previewJobs, previewHolds int
	if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM generation_jobs`).Scan(&previewJobs); err != nil {
		t.Fatal(err)
	}
	if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions`).Scan(&previewHolds); err != nil {
		t.Fatal(err)
	}
	if previewJobs != 0 || previewHolds != 0 {
		t.Fatal("inspection admitted work")
	}
	planRun, err := h.generation.StartStoryline(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartStorylineRequest{PostSlug: slug, ObserveModel: ref, WriteModel: ref}))
	if err != nil {
		t.Fatal(err)
	}
	h.waitJob(t, planRun.Msg.JobId, job.StatusDone)
	planned, err := h.posts.GetPost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequest{Slug: slug}))
	if err != nil || planned.Msg.Post.Storyline == nil {
		t.Fatal("observed plan lost mixed origins", planned, err)
	}
	storedPlan, err := h.app.post.Get(t.Context(), "alice", slug)
	if err != nil || storedPlan.Storyline == nil || storedPlan.Storyline.Origins == nil || len(storedPlan.Storyline.Origins.Spans) != 3 {
		t.Fatal("stored plan origin lineage missing", err)
	}
	for _, stage := range []string{"observe", "plan"} {
		view := readInspection(stage, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
		if view.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || view.Inspection.CallId == "" {
			t.Fatal("issued plan/observation witness lost", stage)
		}
	}
	written, err := h.generation.StartGeneration(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartGenerationRequest{PostSlug: slug, ObserveModel: ref, WriteModel: ref, FromStoryline: true}))
	if err != nil {
		t.Fatal(err)
	}
	h.waitJob(t, written.Msg.JobId, job.StatusDone)
	current, err := h.posts.GetPost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequest{Slug: slug}))
	if err != nil {
		t.Fatal(err)
	}
	qualificationCategories(t, current.Msg.Post.ContentOrigins)
	if current.Msg.Post.Status != "review" || len(current.Msg.Post.Content.Tags) != 0 {
		t.Fatal("generation implicitly finalized or filled tag cap")
	}
	checkBaseline := func(revision int64) {
		t.Helper()
		var canonical, baseline string
		var storedRevision int64
		if err := h.platform.db.Reader.QueryRow(`SELECT content,machine_baseline,machine_baseline_revision FROM posts WHERE slug=?`, slug).Scan(&canonical, &baseline, &storedRevision); err != nil || canonical != baseline || storedRevision != revision || strings.Contains(canonical, "origins") || strings.Contains(canonical, "current.memo") {
			t.Fatal("canonical/baseline equality or isolation lost", err)
		}
	}
	checkBaseline(current.Msg.Post.ContentRevision)
	captured := readInspection("write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	h.witness.mu.Lock()
	actual := h.witness.requests[len(h.witness.requests)-1]
	h.witness.mu.Unlock()
	var actualText, capturedText strings.Builder
	for _, message := range actual.Messages {
		for _, part := range message.Parts {
			if message.Role == llm.RoleUser {
				actualText.WriteString(part.Text)
			}
		}
	}
	for _, fragment := range captured.Inspection.Fragments {
		if fragment.Role == v1.InspectionRole_INSPECTION_ROLE_USER {
			capturedText.WriteString(fragment.Text)
		}
	}
	if actualText.String() != capturedText.String() || captured.Inspection.Measures.GetProviderPromptTokens() != 17 {
		t.Fatal("captured request was reconstructed")
	}
	manual := proto.Clone(current.Msg.Post.Content).(*v1.PostContent)
	manual.Title = "Manual title"
	save := &v1.SavePostContentRequest{Slug: slug, Content: manual, ExpectedRevision: current.Msg.Post.ContentRevision}
	edited, err := h.posts.SavePostContent(t.Context(), writingRPCRequest(h.writingIntegrationHarness, save))
	if err != nil {
		t.Fatal(err)
	}
	qualificationCategories(t, edited.Msg.Post.ContentOrigins)
	if edited.Msg.Post.MachineBaselineRevision != current.Msg.Post.MachineBaselineRevision {
		t.Fatal("manual edit changed machine baseline")
	}
	if _, err := h.posts.SavePostContent(t.Context(), writingRPCRequest(h.writingIntegrationHarness, save)); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatal("stale manual CAS accepted", err)
	}
	save.ExpectedRevision = edited.Msg.Post.ContentRevision
	noop, err := h.posts.SavePostContent(t.Context(), writingRPCRequest(h.writingIntegrationHarness, save))
	if err != nil || noop.Msg.Post.ContentRevision != edited.Msg.Post.ContentRevision {
		t.Fatal("identical manual save advanced identity", err)
	}
	if stale := readInspection("write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED); stale.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE {
		t.Fatal("old actual capture retargeted manual result")
	}
	revision, err := h.generation.StartRevision(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartRevisionRequest{PostSlug: slug, Instruction: "Change only the title to Revised title; retain the body and tags.", WriteModel: ref}))
	if err != nil {
		t.Fatal(err)
	}
	h.waitJob(t, revision.Msg.JobId, job.StatusDone)
	revised, err := h.posts.GetPost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequest{Slug: slug}))
	if err != nil {
		t.Fatal(err)
	}
	qualificationCategories(t, revised.Msg.Post.ContentOrigins)
	if revised.Msg.Post.Content.Blocks[0].Content != qualificationProse || len(revised.Msg.Post.Content.Tags) != 0 || revised.Msg.Post.Status != "review" {
		t.Fatal("requested-only revision changed untouched meaning/tags or finalized")
	}
	checkBaseline(revised.Msg.Post.ContentRevision)
	readInspection("revise", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	foreign := connect.NewRequest(&v1.GetPostOriginReviewRequest{PostSlug: slug})
	foreign.Header().Set("Cookie", auth.SessionCookieName+"=bob-authenticated-session")
	if _, err := h.inspection.GetPostOriginReview(t.Context(), foreign); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatal("foreign origin review exposed", err)
	}
	source, err := h.app.post.Get(t.Context(), "alice", slug)
	if err != nil {
		t.Fatal(err)
	}
	plan := h.plan(2)
	plan.Context = &v1.WritingTestContext{SourcePostSlug: slug, ExpectedInputRevision: source.InputRevision, ExpectedContentRevision: source.ContentRevision, Material: &v1.WritingTestMaterial{Text: source.Memo, AttachmentIds: []string{"owned-photo"}}, ObserveModel: ref, WriteModel: ref, TargetLanguage: language, TargetLength: int32(*source.TargetLength), TagCount: int32(source.TagCount)}
	quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.EstimateWritingTestRequest{Plan: plan}))
	if err != nil {
		t.Fatal(err)
	}
	started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartWritingTestRequest{Plan: plan, QuoteKey: quote.Msg.QuoteKey, RequestKey: "qualification-champion"}))
	if err != nil {
		t.Fatal(err)
	}
	blind := h.settledTest(t, started.Msg.Test.Id, v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW)
	qualificationScan(t, blind, "writer-", "Registered writer", "origins", "requestInspections", "OWNER_PRIVATE_MATERIAL")
	for _, candidate := range blind.Candidates {
		for _, status := range []v1.InspectionStatus{v1.InspectionStatus_INSPECTION_STATUS_CURRENT, v1.InspectionStatus_INSPECTION_STATUS_PREPARED, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED} {
			view, err := h.inspection.GetWritingTestRequestInspection(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetWritingTestRequestInspectionRequest{TestId: blind.Id, CandidateId: candidate.Id, Stage: "write", Status: status}))
			if err != nil || view.Msg.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE {
				t.Fatal("blind identity boundary lost", err)
			}
			qualificationScan(t, view.Msg, "writer-", "Registered writer", "OWNER_PRIVATE_MATERIAL", "fragments", "conditions", "sourceFiles")
		}
	}
	match := blind.Matches[0]
	chosen, err := h.client.DecideTestMatch(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.DecideTestMatchRequest{TestId: blind.Id, ExpectedRevision: blind.Revision, RequestKey: "qualification-vote", MatchId: match.Id, WinnerCandidateId: match.LeftCandidateId}))
	if err != nil {
		t.Fatal(err)
	}
	champion := chosen.Msg.Test
	retained, err := h.app.experimentStore.GetTest(t.Context(), "alice", champion.Id)
	if err != nil {
		t.Fatal(err)
	}
	var winner experiment.TestOutput
	for _, candidate := range retained.Candidates {
		if candidate.ID == champion.WinnerCandidateId {
			winner, err = experiment.DecodeTestOutput(candidate.Output)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if winner.Origins == nil || len(winner.Origins.Spans) != 3 || winner.Storyline == nil || winner.Storyline.Origins == nil || len(winner.Storyline.Origins.Spans) != 3 || len(winner.RequestInspections) != 1 {
		t.Fatal("private champion lost lineage or actual witness", winner)
	}
	view, err := h.inspection.GetWritingTestRequestInspection(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetWritingTestRequestInspectionRequest{TestId: champion.Id, CandidateId: champion.WinnerCandidateId, Stage: "write", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
	if err != nil || view.Msg.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED {
		t.Fatal("revealed actual witness missing", err)
	}
	qualificationScan(t, view.Msg)
	beforeApply, err := h.app.post.Get(t.Context(), "alice", slug)
	if err != nil || !reflect.DeepEqual(beforeApply.Content, source.Content) || beforeApply.ContentRevision != source.ContentRevision {
		t.Fatal("champion reveal implicitly changed source", err)
	}
	apply := &v1.ApplyWritingTestOutputRequest{TestId: champion.Id, WinnerCandidateId: champion.WinnerCandidateId, ExpectedRevision: champion.Revision, RequestKey: "qualification-apply", ExpectedInputRevision: source.InputRevision, ExpectedContentRevision: source.ContentRevision - 1}
	if _, err := h.client.ApplyWritingTestOutput(t.Context(), writingRPCRequest(h.writingIntegrationHarness, apply)); connect.CodeOf(err) != connect.CodeAborted && connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("stale champion CAS accepted", err)
	}
	apply.ExpectedContentRevision = source.ContentRevision
	published, err := h.client.ApplyWritingTestOutput(t.Context(), writingRPCRequest(h.writingIntegrationHarness, apply))
	if err != nil {
		t.Fatal(err)
	}
	applied, err := h.posts.GetPost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequest{Slug: slug}))
	if err != nil {
		t.Fatal(err)
	}
	qualificationCategories(t, applied.Msg.Post.ContentOrigins)
	if applied.Msg.Post.Status != "review" || applied.Msg.Post.ContentRevision != source.ContentRevision+1 || applied.Msg.Post.Storyline == nil {
		t.Fatal("explicit compatible apply lost plan or implicitly finalized")
	}
	checkBaseline(applied.Msg.Post.ContentRevision)
	// Expiry denies reads before the sweep, then erases every private test payload.
	if _, err := h.platform.db.Writer.Exec(`UPDATE writing_tests SET content_expires_at=? WHERE id=?`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), champion.Id); err != nil {
		t.Fatal(err)
	}
	expired, err := h.inspection.GetWritingTestRequestInspection(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetWritingTestRequestInspectionRequest{TestId: champion.Id, CandidateId: champion.WinnerCandidateId, Stage: "write", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
	if err != nil || expired.Msg.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE {
		t.Fatal("unswept expiry returned private evidence", err)
	}
	qualificationScan(t, expired.Msg, "writer-", "OWNER_PRIVATE_MATERIAL", "fragments", "conditions")
	if count, err := h.app.writingTestLifecycle.PurgeExpired(t.Context(), time.Now()); err != nil || count != 1 {
		t.Fatal("actual expiry lifecycle did not purge", count, err)
	}
	work, err := h.app.experimentStore.ReadTestInspectionWork(t.Context(), "alice", champion.Id)
	if err != nil || len(work.Test.CommonSnapshot) != 0 || len(work.SharedCheckpoint) != 0 {
		t.Fatal("private inputs/shared captures survived sweep", err)
	}
	for _, checkpoint := range work.CandidateCheckpoints {
		if len(checkpoint) != 0 {
			t.Fatal("candidate checkpoint retained private payload")
		}
	}
	for _, candidate := range work.Test.Candidates {
		if len(candidate.Output) != 0 || len(candidate.FrozenVariant) != 0 {
			t.Fatal("candidate origins/requests/output survived sweep")
		}
	}
	later := proto.Clone(applied.Msg.Post.Content).(*v1.PostContent)
	later.Title = "Later owner title"
	ownerEdit, err := h.posts.SavePostContent(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.SavePostContentRequest{Slug: slug, Content: later, ExpectedRevision: applied.Msg.Post.ContentRevision}))
	if err != nil {
		t.Fatal(err)
	}
	apply.RequestKey = "qualification-lost-response"
	replay, err := h.client.ApplyWritingTestOutput(t.Context(), writingRPCRequest(h.writingIntegrationHarness, apply))
	if err != nil || replay.Msg.Publication.TargetId != published.Msg.Publication.TargetId {
		t.Fatal("purged receipt recovery failed", err)
	}
	final, err := h.posts.GetPost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequest{Slug: slug}))
	if err != nil || !proto.Equal(final.Msg.Post.Content, ownerEdit.Msg.Post.Content) || final.Msg.Post.ContentRevision != ownerEdit.Msg.Post.ContentRevision {
		t.Fatal("receipt recovery reversed later owner edit", err)
	}
	if _, err := h.posts.DeleteImage(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.DeleteImageRequest{ImageId: "owned-photo"})); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := h.inspection.GetPostOriginReview(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostOriginReviewRequest{PostSlug: slug}))
	if err != nil {
		t.Fatal(err)
	}
	qualificationScan(t, withdrawn.Msg)
	var unavailableVisual bool
	for _, source := range withdrawn.Msg.Review.Sources {
		if source.AttachmentId == "owned-photo" {
			unavailableVisual = !source.Available && source.Text == "A blue cup"
		}
	}
	if !unavailableVisual {
		t.Fatal("withdrawn visual evidence was retargeted or disappeared")
	}
	h.witness.mu.Lock()
	calls := len(h.witness.requests)
	h.witness.mu.Unlock()
	var admissions, receipts int
	if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions WHERE user_id='alice'`).Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM post_test_publications WHERE test_id=?`, champion.Id).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if calls != 7 || admissions != 4 || receipts != 1 {
		t.Fatalf("inspection/origin/apply added calls or receipts: calls=%d admissions=%d receipts=%d", calls, admissions, receipts)
	}
}

func TestT646RegisteredCanonicalBodyWithInvalidOrLegacyOriginsRemainsEditableAndFinalizable(t *testing.T) {
	for _, tc := range []struct{ name, tail string }{{"legacy", ""}, {"invalid", `,"origins":"invalid"`}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newWritingQualificationHarness(t, &writingQualificationProvider{originTail: &tc.tail})
			language := v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH
			created, err := h.posts.SavePostDraft(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.SavePostDraftRequest{Title: "Owner source", Memo: "It tasted good.", TargetLanguage: &language}))
			if err != nil {
				t.Fatal(err)
			}
			run, err := h.generation.StartGeneration(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartGenerationRequest{PostSlug: created.Msg.Post.Slug, WriteModel: &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-0"}}))
			if err != nil {
				t.Fatal(err)
			}
			h.waitJob(t, run.Msg.JobId, job.StatusDone)
			current, err := h.posts.GetPost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequest{Slug: created.Msg.Post.Slug}))
			if err != nil || current.Msg.Post.Content.Blocks[0].Content != qualificationProse || current.Msg.Post.ContentOrigins == nil || len(current.Msg.Post.ContentOrigins.Spans) != 0 {
				t.Fatal("usable unconfirmed body discarded/guessed", err)
			}
			body := proto.Clone(current.Msg.Post.Content).(*v1.PostContent)
			body.Title = "Owner edited title"
			edited, err := h.posts.SavePostContent(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.SavePostContentRequest{Slug: created.Msg.Post.Slug, Content: body, ExpectedRevision: current.Msg.Post.ContentRevision}))
			if err != nil {
				t.Fatal(err)
			}
			finalized, err := h.posts.FinalizePost(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.FinalizePostRequest{Slug: created.Msg.Post.Slug, ExpectedRevision: edited.Msg.Post.ContentRevision}))
			if err != nil || finalized.Msg.Post.Status != "finalized" || !proto.Equal(finalized.Msg.Post.Content, body) || len(finalized.Msg.Post.ContentOrigins.Spans) != 0 {
				t.Fatal("unconfirmed content gained extra finalize gate or invented origins", err)
			}
			h.witness.mu.Lock()
			calls := len(h.witness.requests)
			h.witness.mu.Unlock()
			if calls != 1 {
				t.Fatal("legacy/invalid origins triggered repair call")
			}
		})
	}
}

func TestT646RegisteredIssuedCallbacksCannotRestorePurgedPrivateEvidence(t *testing.T) {
	for _, kind := range []string{"post", "post-account", "authoring", "test"} {
		t.Run(kind, func(t *testing.T) {
			witness := &writingQualificationProvider{entered: make(chan struct{}), release: make(chan struct{})}
			h := newWritingQualificationHarness(t, witness)
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(witness.release) }) }
			t.Cleanup(release)
			ref := llm.ModelRef{ProviderID: "fixture", ModelID: "writer-0"}
			if _, err := h.app.provider.SaveSelection(t.Context(), "alice", provider.StageWrite, ref); err != nil {
				t.Fatal(err)
			}
			var jobID, slug, sessionID, testID string
			// One issued call per entrant, and never a retry. A writing test admits its
			// entrants concurrently (experiment.MaxConcurrentTestCandidates) and
			// experiment.ValidTestCount forbids a single-entrant plan, so every entrant is
			// already in flight when the owner purges; only a replay would add more.
			issued := 1
			if kind == "authoring" {
				state, err := h.app.authoring.Create(t.Context(), "alice", authoring.PostGuideline, "", "qualification-private")
				if err != nil {
					t.Fatal(err)
				}
				sessionID = state.ID
				jobID, _, err = h.app.authoring.Start(t.Context(), "alice", authoring.Start{SessionID: state.ID, ExpectedRevision: state.Revision, RequestID: "qualification-issue", Mode: authoring.Recommend, Prompt: "Ground the writing.", RequestedCandidateCount: 1, WriteModel: ref})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				language := post.LanguageEnglish
				source, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Title: "Owner source", Memo: "It tasted good.", TargetLanguage: &language})
				if err != nil {
					t.Fatal(err)
				}
				slug = source.Slug
				if kind == "post" || kind == "post-account" {
					run, err := h.generation.StartGeneration(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartGenerationRequest{PostSlug: slug, WriteModel: &v1.ModelRef{ProviderId: ref.ProviderID, ModelId: ref.ModelID}}))
					if err != nil {
						t.Fatal(err)
					}
					jobID = run.Msg.JobId
				} else {
					plan := h.plan(2)
					issued = len(plan.Entrants)
					plan.Context.SourcePostSlug = slug
					plan.Context.ExpectedInputRevision = source.InputRevision
					plan.Context.ExpectedContentRevision = source.ContentRevision
					quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.EstimateWritingTestRequest{Plan: plan}))
					if err != nil {
						t.Fatal(err)
					}
					started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.StartWritingTestRequest{Plan: plan, QuoteKey: quote.Msg.QuoteKey, RequestKey: "qualification-private"}))
					if err != nil {
						t.Fatal(err)
					}
					testID = started.Msg.Test.Id
					jobID = started.Msg.Test.JobId
				}
			}
			select {
			case <-witness.entered:
			case <-time.After(10 * time.Second):
				t.Fatal("actual issued provider callback did not enter")
			}
			// The gate holds only the first call; a concurrent entrant reaches the
			// provider on its own schedule, so wait until every one is in flight.
			for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
				witness.mu.Lock()
				inFlight := len(witness.requests)
				witness.mu.Unlock()
				if inFlight >= issued {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("issued entrants did not all reach the provider", inFlight, issued)
				}
			}
			switch kind {
			case "post-account":
				// The account owner erases its rows while the real issued callback
				// is blocked; the FK lifecycle includes post origins and captures.
				if _, err := h.platform.db.Writer.Exec(`DELETE FROM users WHERE id='alice'`); err != nil {
					t.Fatal(err)
				}
			case "post":
				if err := poststore.NewRequestCaptureStore(h.platform.db.Writer, h.platform.db.Reader, postRequestCaptureJobs{}).PurgePostRequestCaptures(t.Context(), "alice", slug); err != nil {
					t.Fatal(err)
				}
			case "authoring":
				if err := authoringstore.New(h.platform.db.Writer, h.platform.db.Reader).PurgeAuthoringRequestCaptures(t.Context(), "alice", sessionID); err != nil {
					t.Fatal(err)
				}
			case "test":
				if err := h.app.post.DeletePost(t.Context(), "alice", slug); err != nil {
					t.Fatal(err)
				}
			}
			release()
			if kind != "test" && kind != "post-account" {
				h.waitJob(t, jobID, job.StatusDone)
			}
			// Join the real queue worker, including capture/completion callbacks.
			h.stop()
			switch kind {
			case "post-account":
				for _, table := range []string{"posts", "post_request_captures", "post_request_capture_purges"} {
					var rows int
					if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE user_id='alice'`).Scan(&rows); err != nil || rows != 0 {
						t.Fatal("late result/origin/request callback restored deleted owner payload", table, rows, err)
					}
				}
			case "post":
				var captures int
				if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM post_request_captures WHERE post_slug=?`, slug).Scan(&captures); err != nil || captures != 0 {
					t.Fatal("late issued post capture restored private evidence", captures, err)
				}
				read, err := h.inspection.GetPostRequestInspection(t.Context(), writingRPCRequest(h.writingIntegrationHarness, &v1.GetPostRequestInspectionRequest{PostSlug: slug, Stage: "write", Status: v1.InspectionStatus_INSPECTION_STATUS_CAPTURED}))
				if err != nil || read.Msg.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE {
					t.Fatal("purged post inspection restored", err)
				}
			case "authoring":
				var captures int
				if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM configuration_authoring_operations WHERE session_id=? AND request_capture IS NOT NULL`, sessionID).Scan(&captures); err != nil || captures != 0 {
					t.Fatal("late authoring capture restored private evidence", captures, err)
				}
			case "test":
				work, err := h.app.experimentStore.ReadTestInspectionWork(t.Context(), "alice", testID)
				if err != nil || work.Test.PurgeFence != 1 || len(work.Test.CommonSnapshot) != 0 || len(work.SharedCheckpoint) != 0 {
					t.Fatal("late shared/entrant callback restored private test evidence", err)
				}
				for _, checkpoint := range work.CandidateCheckpoints {
					if len(checkpoint) != 0 {
						t.Fatal("candidate checkpoint retained private payload")
					}
				}
				for _, candidate := range work.Test.Candidates {
					if len(candidate.Output) != 0 || len(candidate.FrozenVariant) != 0 {
						t.Fatal("late output/origin/request callback restored private payload")
					}
				}
				var rows int
				if err := h.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM posts WHERE slug=?`, slug).Scan(&rows); err != nil || rows != 0 {
					t.Fatal("test callback restored deleted source", err)
				}
			}
			witness.mu.Lock()
			calls := len(witness.requests)
			witness.mu.Unlock()
			if calls != issued {
				t.Fatal("late-callback purge caused retry or additional entrant call", calls, issued)
			}
		})
	}
}
