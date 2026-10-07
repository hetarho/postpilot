package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/usage"
)

// The provider is deterministic; the boot graph, authentication, Connect
// transport, SQLite, queue, generation stages and ledger are production code.
type writingIntegrationProvider struct {
	mu           sync.Mutex
	db           *db.DB
	requests     []llm.Request
	active, peak int
	failures     map[string]int
}

func (*writingIntegrationProvider) Name() string { return "fixture" }
func (*writingIntegrationProvider) HasFreePath(context.Context, string, llm.FreePath) (bool, error) {
	return true, nil
}
func (p *writingIntegrationProvider) Complete(ctx context.Context, request llm.Request) (llm.Response, error) {
	work, ok := usage.WorkFromContext(ctx)
	if !ok {
		return llm.Response{}, fmt.Errorf("provider called without admitted work")
	}
	var status string
	if err := p.db.Reader.QueryRowContext(ctx, `SELECT status FROM writing_tests WHERE user_id=? AND job_id=?`, work.UserID, work.JobID).Scan(&status); err != nil || status != "running" {
		return llm.Response{}, fmt.Errorf("provider preceded durable test/job binding: %s, %v", status, err)
	}
	p.mu.Lock()
	p.requests = append(p.requests, request)
	p.active++
	if p.active > p.peak {
		p.peak = p.active
	}
	sequence := len(p.requests)
	fail := p.failures[request.Model] > 0
	if fail {
		p.failures[request.Model]--
	}
	p.mu.Unlock()
	defer func() { p.mu.Lock(); p.active--; p.mu.Unlock() }()
	select {
	case <-ctx.Done():
		return llm.Response{}, ctx.Err()
	case <-time.After(5 * time.Millisecond):
	}
	if fail {
		return llm.Response{Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 7, CostReported: true}}, llm.ErrRateLimited
	}
	if request.Stage == "observe" {
		return llm.Response{Text: `{"observations":[{"file":"owned.jpg","scene":"A supplied photo scene","mood":"quiet","visible_text":"","objects":["path"],"people_present":false}]}`, FinishReason: "stop", Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 7, CostReported: true}}, nil
	}
	return llm.Response{Text: fmt.Sprintf(`{"storyline":[{"text":"Plan for the complete post","files":[]}],"title":"Complete post %d","summary":"Whole summary","tags":["fixture"],"blocks":[{"type":"TEXT","content":"A complete bounded post from explicitly supplied fictional material."}],"nouns":["post"]}`, sequence), FinishReason: "stop", Usage: llm.Usage{PromptTokens: 11, CompletionTokens: 7, CostReported: true}}, nil
}

type writingIntegrationModels struct{}

func (writingIntegrationModels) Models() []llm.SourceModel {
	var models []llm.SourceModel
	for index := 0; index < 16; index++ {
		models = append(models, llm.SourceModel{ModelID: "writer-" + strconv.Itoa(index), Label: "Registered writer " + strconv.Itoa(index), Vision: true, StructuredOutput: true, ContextTokens: 131072, InputUSDPerMillion: "0", OutputUSDPerMillion: "0", PricingCheckedAt: time.Now().UTC().Format(time.RFC3339Nano), Stages: []string{"write", "observe", "analyze"}, Levels: map[string]string{"write": "free", "observe": "free", "analyze": "free"}})
	}
	return models
}
func (s writingIntegrationModels) Lookup(id string) (llm.SourceModel, bool) {
	for _, model := range s.Models() {
		if model.ModelID == id {
			return model, true
		}
	}
	return llm.SourceModel{}, false
}

type writingIntegrationHarness struct {
	app        *contexts
	platform   *platform
	provider   *writingIntegrationProvider
	client     postpilotv1connect.WritingTestServiceClient
	inspection postpilotv1connect.WritingInspectionServiceClient
	cookie     string
}

func newWritingIntegrationHarness(t *testing.T, configure ...func(*platform)) *writingIntegrationHarness {
	t.Helper()
	t.Setenv("MAIL_DRIVER", "log")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := wiringPlatform(t, cfg)
	for _, configurePlatform := range configure {
		configurePlatform(p)
	}
	modelProvider := &writingIntegrationProvider{db: p.db}
	path := filepath.Join(t.TempDir(), "providers.yaml")
	if err := os.WriteFile(path, []byte("providers:\n  - id: fixture\n    adapter: fixture\n    base_url: http://fixture.invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p.registry, err = llm.Load(path, func(string) string { return "" }, map[string]llm.AdapterFactory{"fixture": func(llm.AdapterConfig) (llm.Provider, error) { return modelProvider, nil }}, writingIntegrationModels{}, llm.Options{Timeout: time.Minute, MaxTokens: 32768})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, user := range []string{"alice", "bob"} {
		if _, err := p.db.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?,?,?)`, user, "fixture", now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		raw := user + "-authenticated-session"
		hash := sha256.Sum256([]byte(raw))
		if _, err := p.db.Writer.Exec(`INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES(?,?,?,?)`, hex.EncodeToString(hash[:]), user, now.Add(time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	app, err := buildContexts(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	registerJobs(app)
	server := rpcserver.New(cfg, "writing-test-fixture", rpcserver.Options{Handlers: handlers(app), Interceptors: []connect.Interceptor{authrpc.NewInterceptor(app.auth, app.throttle, cfg.ClientIPHeader)}})
	httpServer := httptest.NewServer(server.Handler)
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan struct{})
	go func() { defer close(finished); app.jobs.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-finished })
	return &writingIntegrationHarness{app: app, platform: p, provider: modelProvider, client: postpilotv1connect.NewWritingTestServiceClient(httpServer.Client(), httpServer.URL), inspection: postpilotv1connect.NewWritingInspectionServiceClient(httpServer.Client(), httpServer.URL), cookie: auth.SessionCookieName + "=alice-authenticated-session"}
}

func writingRPCRequest[T any](h *writingIntegrationHarness, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Cookie", h.cookie)
	return req
}

func (h *writingIntegrationHarness) plan(count int) *v1.WritingTestPlan {
	p := &v1.WritingTestPlan{Factor: v1.WritingTestFactor_WRITING_TEST_FACTOR_MODEL, ModelStage: v1.WritingTestStage_WRITING_TEST_STAGE_WRITE, Count: int32(count), Context: &v1.WritingTestContext{TargetLanguage: v1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH, TargetLength: 1600, TagCount: 3, Material: &v1.WritingTestMaterial{Text: "A fictional account of a quiet morning walk, using only the supplied scenario.", Fictional: true}}}
	for index := 0; index < count; index++ {
		p.Entrants = append(p.Entrants, &v1.WritingTestEntrant{Source: &v1.WritingTestEntrant_Model{Model: &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-" + strconv.Itoa(index)}}})
	}
	return p
}

func (h *writingIntegrationHarness) get(t *testing.T, id string) *v1.WritingTest {
	t.Helper()
	res, err := h.client.GetWritingTest(t.Context(), writingRPCRequest(h, &v1.GetWritingTestRequest{TestId: id}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.Test
}

func TestWritingTestsProductionTransportGeneratesSixteenPostsAndFifteenFreeDecisions(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	plan := h.plan(16)
	quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h, &v1.EstimateWritingTestRequest{Plan: plan}))
	if err != nil {
		t.Fatal(err)
	}
	if !quote.Msg.Free || quote.Msg.GetCredits() != 0 {
		t.Fatalf("wrong bounded free quote: %+v", quote.Msg)
	}
	started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, &v1.StartWritingTestRequest{Plan: plan, RequestKey: "start-sixteen", QuoteKey: quote.Msg.QuoteKey}))
	if err != nil {
		t.Fatal(err)
	}
	current := started.Msg.Test
	for deadline := time.Now().Add(30 * time.Second); current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		current = h.get(t, current.Id)
		if current.Status == v1.WritingTestStatus_WRITING_TEST_STATUS_FAILED || current.Status == v1.WritingTestStatus_WRITING_TEST_STATUS_PARTIAL {
			t.Fatalf("generation failed: %+v", current.Failure)
		}
	}
	if current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW || len(current.Candidates) != 16 || current.Revealed {
		t.Fatalf("all-success blind barrier absent: %+v", current)
	}
	current = h.settledTest(t, current.Id, v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW)
	for _, candidate := range current.Candidates {
		if candidate.Identity != nil || candidate.Usage != nil || candidate.Output == nil || len(candidate.Output.Blocks) != 1 || candidate.Storyline == nil || len(candidate.Storyline.Paragraphs) != 1 {
			t.Fatalf("incomplete or unblinded output: %+v", candidate)
		}
	}
	for index := 0; index < 15; index++ {
		var match *v1.WritingTestMatch
		for _, m := range current.Matches {
			if m.WinnerCandidateId == "" {
				match = m
				break
			}
		}
		if match == nil {
			t.Fatalf("missing binary decision %d", index)
		}
		decision := &v1.DecideTestMatchRequest{TestId: current.Id, ExpectedRevision: current.Revision, RequestKey: fmt.Sprintf("vote-%d", index), MatchId: match.Id, WinnerCandidateId: match.LeftCandidateId}
		picked, err := h.client.DecideTestMatch(t.Context(), writingRPCRequest(h, decision))
		if err != nil {
			t.Fatal(err)
		}
		current = picked.Msg.Test
		replayed, err := h.client.DecideTestMatch(t.Context(), writingRPCRequest(h, decision))
		if err != nil || replayed.Msg.Test.WinnerCandidateId != current.WinnerCandidateId {
			t.Fatalf("vote replay: %v", err)
		}
	}
	if current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_COMPLETED || !current.Revealed || len(current.Matches) != 15 {
		t.Fatalf("wrong champion: %+v", current)
	}
	h.provider.mu.Lock()
	calls, peak := len(h.provider.requests), h.provider.peak
	h.provider.mu.Unlock()
	if calls != 16 || peak > 5 {
		t.Fatalf("calls=%d peak=%d", calls, peak)
	}
	var admissions, events, posts int
	for table, out := range map[string]*int{"usage_admissions": &admissions, "usage_events": &events, "posts": &posts} {
		if err := h.platform.db.Reader.QueryRow(`SELECT count(*) FROM ` + table).Scan(out); err != nil {
			t.Fatal(err)
		}
	}
	if admissions != 1 || events != 16 || posts != 0 {
		t.Fatalf("admissions=%d calls=%d canonicalposts=%d", admissions, events, posts)
	}
	foreign := writingRPCRequest(h, &v1.GetWritingTestRequest{TestId: current.Id})
	foreign.Header().Set("Cookie", auth.SessionCookieName+"=bob-authenticated-session")
	if _, err := h.client.GetWritingTest(t.Context(), foreign); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("foreign history disclosed: %v", err)
	}
	publication := &v1.SaveWritingTestWinnerRequest{TestId: current.Id, ExpectedRevision: current.Revision, RequestKey: "adopt-champion", WinnerCandidateId: current.WinnerCandidateId, Action: v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_ADOPT_MODEL}
	saved, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, publication))
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, publication))
	if err != nil || repeated.Msg.Publication.TargetId != saved.Msg.Publication.TargetId {
		t.Fatalf("publication replay: %v", err)
	}
	h.provider.mu.Lock()
	defer h.provider.mu.Unlock()
	if len(h.provider.requests) != 16 {
		t.Fatal("publication issued provider work")
	}
}
