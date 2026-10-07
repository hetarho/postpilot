package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
)

type inspectionProvider struct {
	mu            sync.Mutex
	requests      []llm.Request
	failure       error
	freePathReads int
}

func (*inspectionProvider) Name() string { return "fixture" }
func (p *inspectionProvider) HasFreePath(context.Context, string, llm.FreePath) (bool, error) {
	p.mu.Lock()
	p.freePathReads++
	p.mu.Unlock()
	return true, nil
}
func (p *inspectionProvider) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	p.mu.Lock()
	p.requests = append(p.requests, request)
	failure := p.failure
	p.mu.Unlock()
	answer := `{"title":"검증 글","summary":"입력 확인","tags":[],"blocks":[{"type":"TEXT","content":"소유자가 알려준 방문"}],"nouns":[]}`
	switch request.Composition.Mode {
	case "storyline-create", "storyline-rewrite":
		answer = `{"storyline":[{"text":"소유자가 알려준 방문","files":[]}]}`
	case "direct":
		answer = `{"storyline":[{"text":"소유자가 알려준 방문","files":[]}],` + answer[1:]
	case "revision":
		answer = strings.ReplaceAll(answer, "검증 글", "수정한 글")
	}
	return llm.Response{Text: answer, FinishReason: "stop", Usage: llm.Usage{PromptTokens: 17, CompletionTokens: 9, ReasoningTokens: 3, CostMicrousd: 987654321, CostReported: true}}, failure
}

type inspectionModels struct{}

func (inspectionModels) Models() []llm.SourceModel {
	return []llm.SourceModel{{ModelID: "writer", Label: "Fixture writer", Vision: true, StructuredOutput: true, ContextTokens: 131072, InputUSDPerMillion: "0", OutputUSDPerMillion: "0", PricingCheckedAt: "2026-10-08T00:00:00Z", Stages: []string{"write", "observe"}, Levels: map[string]string{"write": "free", "observe": "free"}, Reasoning: map[string]llm.ReasoningEffort{"write": llm.ReasoningHigh}}}
}
func (s inspectionModels) Lookup(id string) (llm.SourceModel, bool) {
	for _, info := range s.Models() {
		if info.ModelID == id {
			return info, true
		}
	}
	return llm.SourceModel{}, false
}

type postInspectionHarness struct {
	app      *contexts
	provider *inspectionProvider
	client   postpilotv1connect.WritingInspectionServiceClient
	ref      llm.ModelRef
}

func newPostInspectionHarness(t *testing.T) *postInspectionHarness {
	t.Helper()
	t.Setenv("MAIL_DRIVER", "log")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := wiringPlatform(t, cfg)
	modelProvider := &inspectionProvider{}
	p.registry, err = llm.Parse([]byte("providers:\n  - id: fixture\n    adapter: fixture\n    base_url: https://PRIVATE_ENDPOINT.invalid\n    api_key_env: PRIVATE_KEY\n"), func(string) string { return "PRIVATE_CREDENTIAL" }, map[string]llm.AdapterFactory{"fixture": func(llm.AdapterConfig) (llm.Provider, error) { return modelProvider, nil }}, inspectionModels{}, llm.Options{Timeout: time.Minute, MaxTokens: 32768})
	if err != nil {
		t.Fatal(err)
	}
	p.registry.WithModelGrades()
	now := time.Now().UTC()
	for _, user := range []string{"alice", "bob"} {
		if _, err := p.db.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?,?,?)`, user, "fixture", now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(user + "-inspection-session"))
		if _, err := p.db.Writer.Exec(`INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES(?,?,?,?)`, hex.EncodeToString(hash[:]), user, now.Add(time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	app, err := buildContexts(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	registerJobs(app)
	server := rpcserver.New(cfg, "inspection-fixture", rpcserver.Options{Handlers: handlers(app), Interceptors: []connect.Interceptor{authrpc.NewInterceptor(app.auth, app.throttle, cfg.ClientIPHeader)}})
	httpServer := httptest.NewServer(server.Handler)
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan struct{})
	go func() { defer close(finished); app.jobs.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-finished })
	return &postInspectionHarness{app: app, provider: modelProvider, client: postpilotv1connect.NewWritingInspectionServiceClient(httpServer.Client(), httpServer.URL), ref: llm.ModelRef{ProviderID: "fixture", ModelID: "writer"}}
}

func (h *postInspectionHarness) read(t *testing.T, user, slug, stage string, status v1.InspectionStatus) (*v1.GetPostRequestInspectionResponse, error) {
	t.Helper()
	request := connect.NewRequest(&v1.GetPostRequestInspectionRequest{PostSlug: slug, Stage: stage, Status: status})
	request.Header().Set("Cookie", auth.SessionCookieName+"="+user+"-inspection-session")
	response, err := h.client.GetPostRequestInspection(t.Context(), request)
	if err != nil {
		return nil, err
	}
	return response.Msg, nil
}

func (h *postInspectionHarness) wait(t *testing.T, id string, expected string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		found, err := h.app.jobs.Get(t.Context(), id, "alice")
		if err != nil {
			t.Fatal(err)
		}
		if job.Terminal(found.Status) {
			if found.Status != expected {
				t.Fatalf("job status %s, want %s: %+v", found.Status, expected, found.Failure)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPostRequestInspectionProductionTransportSharesEffectiveRegistryWithoutExecution(t *testing.T) {
	h := newPostInspectionHarness(t)
	language := post.LanguageKorean
	draft, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Title: "검증", Memo: "노포에 갔다. OWNER_PRIVATE_MATERIAL", TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	missing, err := h.read(t, "alice", draft.Slug, "write", v1.InspectionStatus_INSPECTION_STATUS_PREPARED)
	if err != nil || missing.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE || missing.Inspection.UnavailableReason == "" {
		t.Fatalf("absent selection was silently initialized: %+v %v", missing, err)
	}
	if _, err := h.app.provider.SaveSelection(t.Context(), "alice", provider.StageWrite, h.ref); err != nil {
		t.Fatal(err)
	}
	h.provider.mu.Lock()
	endpointReadsBefore := h.provider.freePathReads
	h.provider.mu.Unlock()
	prepared, err := h.read(t, "alice", draft.Slug, "write", v1.InspectionStatus_INSPECTION_STATUS_PREPARED)
	if err != nil || prepared.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_PREPARED || prepared.Inspection.IssuedAt != nil || prepared.Inspection.Conditions.GetReasoningEffort() != "high" {
		t.Fatalf("effective preview unavailable: %+v %v", prepared, err)
	}
	current, err := h.read(t, "alice", draft.Slug, "write", v1.InspectionStatus_INSPECTION_STATUS_CURRENT)
	if err != nil || current.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CURRENT {
		t.Fatalf("current configuration status lost: %+v %v", current, err)
	}
	before, _ := h.app.post.Get(t.Context(), "alice", draft.Slug)
	for _, stage := range []string{"observe", "plan", "revise", "write"} {
		if _, err := h.read(t, "alice", draft.Slug, stage, v1.InspectionStatus_INSPECTION_STATUS_CAPTURED); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := h.app.post.Get(t.Context(), "alice", draft.Slug)
	h.provider.mu.Lock()
	calls := len(h.provider.requests)
	endpointReadsAfter := h.provider.freePathReads
	h.provider.mu.Unlock()
	var jobs, holds int
	if err := h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM generation_jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions`).Scan(&holds); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || endpointReadsBefore != endpointReadsAfter || jobs != 0 || holds != 0 || !reflect.DeepEqual(before, after) {
		t.Fatalf("inspection executed or mutated: calls=%d jobs=%d holds=%d", calls, jobs, holds)
	}
	id, err := h.app.generation.Start(t.Context(), generation.StartRequest{UserID: "alice", PostSlug: draft.Slug, WriteModel: h.ref.String()})
	if err != nil {
		t.Fatal(err)
	}
	h.wait(t, id, job.StatusDone)
	captured, err := h.read(t, "alice", draft.Slug, "write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || captured.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || captured.Inspection.IssuedAt == nil || captured.Inspection.CallId == "" || len(captured.Inspections) != 1 {
		t.Fatalf("actual current issued call not captured: %+v %v", captured, err)
	}
	if !reflect.DeepEqual(captured.Inspection.Conditions, prepared.Inspection.Conditions) || captured.Inspection.Measures.GetProviderPromptTokens() != 17 || captured.Inspection.Measures.GetProviderCompletionTokens() != 9 || captured.Inspection.Measures.GetProviderReasoningTokens() != 3 {
		t.Fatalf("actual effective conditions/usage diverged: %+v %+v", captured.Inspection.Conditions, prepared.Inspection.Conditions)
	}
	h.provider.mu.Lock()
	actual := h.provider.requests[0]
	h.provider.mu.Unlock()
	var system, user strings.Builder
	var actualUser strings.Builder
	for _, message := range actual.Messages {
		for _, part := range message.Parts {
			if message.Role == llm.RoleUser {
				actualUser.WriteString(part.Text)
			}
		}
	}
	for _, fragment := range captured.Inspection.Fragments {
		if fragment.Role == v1.InspectionRole_INSPECTION_ROLE_SYSTEM {
			system.WriteString(fragment.Text)
		} else if fragment.Role == v1.InspectionRole_INSPECTION_ROLE_USER {
			user.WriteString(fragment.Text)
		}
	}
	if system.String() != actual.System || user.String() != actualUser.String() {
		t.Fatal("captured text was rebuilt from current rows instead of actual request")
	}
	raw, _ := json.Marshal(captured)
	for _, secret := range []string{"PRIVATE_ENDPOINT", "PRIVATE_CREDENTIAL", "987654321", "CostMicrousd", "base_url"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("private transport/supplier fields leaked: %s", secret)
		}
	}
	for _, test := range []struct {
		user, slug string
		code       connect.Code
	}{{"bob", draft.Slug, connect.CodePermissionDenied}, {"alice", "unknown-post", connect.CodeNotFound}} {
		if _, err := h.read(t, test.user, test.slug, "write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED); connect.CodeOf(err) != test.code {
			t.Fatalf("owner fence for %s/%s = %v", test.user, test.slug, err)
		}
	}
	if _, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Slug: draft.Slug, Title: draft.Title, Memo: "Changed owner source"}); err != nil {
		t.Fatal(err)
	}
	stale, err := h.read(t, "alice", draft.Slug, "write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || stale.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_UNAVAILABLE || len(stale.Inspections) != 0 {
		t.Fatalf("source edit retargeted capture: %+v %v", stale, err)
	}
	if err := poststore.New(h.app.platform.db.Writer, h.app.platform.db.Reader).PurgePostRequestCaptures(t.Context(), "alice", draft.Slug); err != nil {
		t.Fatal(err)
	}
	h.provider.mu.Lock()
	calls = len(h.provider.requests)
	h.provider.mu.Unlock()
	if calls != 1 {
		t.Fatalf("inspection/purge replayed paid work: %d", calls)
	}
	photoDraft, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Title: "PRIVATE_TITLE_NOT_FOR_OBSERVE", Memo: "PRIVATE_MEMO_NOT_FOR_OBSERVE", TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 6; index++ {
		if _, err := h.app.platform.db.Writer.Exec(`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES(?,?,?,?,120,180,4,?)`, fmt.Sprintf("opaque-photo-%d", index), photoDraft.Slug, fmt.Sprintf("photo-%d.jpg", index), fmt.Sprintf("PRIVATE_STORAGE_KEY/%d", index), time.Now().UTC().Add(time.Duration(index)*time.Second).Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.app.provider.SaveSelection(t.Context(), "alice", provider.StageObserve, h.ref); err != nil {
		t.Fatal(err)
	}
	observation, err := h.read(t, "alice", photoDraft.Slug, "observe", v1.InspectionStatus_INSPECTION_STATUS_PREPARED)
	if err != nil || observation.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_PREPARED || len(observation.Inspection.Attachments) != h.app.platform.cfg.ObserveBatchSize || len(observation.Inspection.Omissions) == 0 {
		t.Fatalf("bounded safe observation preview unavailable: %+v %v", observation, err)
	}
	raw, _ = json.Marshal(observation)
	for _, secret := range []string{"PRIVATE_TITLE_NOT_FOR_OBSERVE", "PRIVATE_MEMO_NOT_FOR_OBSERVE", "PRIVATE_STORAGE_KEY", "media-placeholder"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("observation preview read contextual/transport media: %s", secret)
		}
	}
	h.provider.mu.Lock()
	calls = len(h.provider.requests)
	h.provider.mu.Unlock()
	if calls != 1 {
		t.Fatalf("media preview read storage or executed: %d", calls)
	}
}

func TestPostRequestInspectionProductionPlanRevisionAndFailedInvokeRemainPrivate(t *testing.T) {
	h := newPostInspectionHarness(t)
	language := post.LanguageKorean
	draft, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Title: "현재 입력", Memo: "소유자가 알려준 방문", TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.provider.SaveSelection(t.Context(), "alice", provider.StageWrite, h.ref); err != nil {
		t.Fatal(err)
	}
	id, err := h.app.generation.StartStoryline(t.Context(), generation.StartStorylineRequest{UserID: "alice", PostSlug: draft.Slug, WriteModel: h.ref.String()})
	if err != nil {
		t.Fatal(err)
	}
	h.wait(t, id, job.StatusDone)
	plan, err := h.read(t, "alice", draft.Slug, "plan", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || plan.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || plan.Inspection.Mode != "storyline-create" {
		t.Fatalf("plan call capture lost: %+v %v", plan, err)
	}
	manualPlan := post.StorylineEdit{Paragraphs: []post.StorylineParagraph{{Text: "Owner-edited arrangement"}}}
	if _, err := h.app.post.SaveDraft(t.Context(), "alice", post.DraftSave{Slug: draft.Slug, Title: draft.Title, Memo: draft.Memo, Storyline: &manualPlan}); err != nil {
		t.Fatal(err)
	}
	id, err = h.app.generation.Start(t.Context(), generation.StartRequest{UserID: "alice", PostSlug: draft.Slug, WriteModel: h.ref.String(), FromStoryline: true})
	if err != nil {
		t.Fatal(err)
	}
	h.wait(t, id, job.StatusDone)
	followed, err := h.read(t, "alice", draft.Slug, "write", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || followed.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || followed.Inspection.Mode != "frozen-storyline" {
		t.Fatalf("legitimate frozen owner-edited plan rejected capture: %+v %v", followed, err)
	}
	id, err = h.app.generation.StartRevision(t.Context(), generation.StartRevisionRequest{UserID: "alice", PostSlug: draft.Slug, WriteModel: h.ref.String(), Instruction: "제목만 수정"})
	if err != nil {
		t.Fatal(err)
	}
	h.wait(t, id, job.StatusDone)
	revised, err := h.read(t, "alice", draft.Slug, "revise", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || revised.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || revised.Inspection.Mode != "revision" {
		t.Fatalf("revision actual call capture lost: %+v %v", revised, err)
	}
	before, _ := h.app.post.Get(t.Context(), "alice", draft.Slug)
	if before.Storyline == nil || !before.Storyline.EditedByHand || before.Storyline.Paragraphs[0].Text != "Owner-edited arrangement" {
		t.Fatal("prose revision lost the existing owner-edited plan")
	}
	h.provider.mu.Lock()
	h.provider.failure = fmt.Errorf("private provider failed: %w", llm.ErrRateLimited)
	h.provider.mu.Unlock()
	id, err = h.app.generation.StartRevision(t.Context(), generation.StartRevisionRequest{UserID: "alice", PostSlug: draft.Slug, WriteModel: h.ref.String(), Instruction: "본문만 수정"})
	if err != nil {
		t.Fatal(err)
	}
	h.wait(t, id, job.StatusFailed)
	failed, err := h.read(t, "alice", draft.Slug, "revise", v1.InspectionStatus_INSPECTION_STATUS_CAPTURED)
	if err != nil || failed.Inspection.Status != v1.InspectionStatus_INSPECTION_STATUS_CAPTURED || failed.Inspection.IssuedAt == nil || failed.Inspection.Measures.GetProviderPromptTokens() != 17 {
		t.Fatalf("failed actual invocation confused with missing capture: %+v %v", failed, err)
	}
	after, _ := h.app.post.Get(t.Context(), "alice", draft.Slug)
	if before.ContentRevision != after.ContentRevision || !reflect.DeepEqual(before.Content, after.Content) {
		t.Fatal("failed request capture changed canonical result")
	}
	if err := h.app.post.DeletePost(t.Context(), "alice", draft.Slug); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := h.app.platform.db.Reader.QueryRow(`SELECT COUNT(*) FROM post_request_captures WHERE post_slug=?`, draft.Slug).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("post deletion retained private requests: %d %v", remaining, err)
	}
	h.provider.mu.Lock()
	calls := len(h.provider.requests)
	h.provider.mu.Unlock()
	if calls != 4 {
		t.Fatalf("inspection altered four admitted calls: %d", calls)
	}
}
