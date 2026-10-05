package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// VOICE-62: every block a post can hold either reaches the fingerprint as its own prose type or,
// for IMAGE and VIDEO, is skipped with its caption.
func TestFingerprintBlocksKeepProseAndSkipMedia(t *testing.T) {
	kept := map[string]string{}
	for value, name := range postpilotv1.BlockType_name {
		if value == int32(postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED) {
			continue
		}
		block := post.Block{Type: post.BlockType(name), Content: "본문", Caption: "캡션", Items: []string{"하나", "둘"}}
		for _, got := range fingerprintBlocks([]post.Block{block}) {
			kept[name] = got.Type
		}
	}
	want := map[string]string{"TEXT": voice.BlockText, "HEADING": voice.BlockHeading, "LIST": voice.BlockList, "QUOTE": voice.BlockQuote}
	if len(kept) != len(want) {
		t.Fatalf("kept %v, want %v", kept, want)
	}
	for name, kind := range want {
		if kept[name] != kind {
			t.Fatalf("%s reached the fingerprint as %q", name, kept[name])
		}
	}
	list := fingerprintBlocks([]post.Block{{Type: post.BlockList, Items: []string{"하나", "둘"}}})
	if len(list) != 1 || len(list[0].Items) != 2 || list[0].Content != "" {
		t.Fatalf("a list reached the fingerprint as %+v", list)
	}
}

// POST-102: the adapter hands the voice context an owned post's voice, revision and prose blocks
// through post.CurrentContent, "" for 말투 없음 and no blocks before the first write; another
// account's post and an unknown one become the voice context's own sentinels.
func TestVoicePostsReadsAnOwnedPostsBlocks(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-posts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(ctx, auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := createTestVoice(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, testPostLimits(), testPostDeps(voiceSvc))
	aliceVoice, err := firstTestVoice(ctx, voiceSvc, "alice")
	if err != nil {
		t.Fatal(err)
	}
	makeVoice(t, handle, "alice", aliceVoice.ID)
	language := post.LanguageKorean
	withVoice, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "국숫집", VoiceID: &aliceVoice.ID, TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	noVoice, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "말투 없이", TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	adapter := voicePosts{service: postSvc}

	voiceID, revision, blocks, err := adapter.PostForFingerprint(ctx, "alice", withVoice.Slug)
	if err != nil || voiceID != aliceVoice.ID || revision != 0 || blocks != nil {
		t.Fatalf("before the first write = %q %d %v err=%v", voiceID, revision, blocks, err)
	}
	content := post.PostContent{Title: "국숫집", Blocks: []post.Block{
		{Type: post.BlockHeading, Content: "첫 방문", Level: 2},
		{Type: post.BlockText, Content: "국물이 진했다."},
		{Type: post.BlockQuote, Content: "또 올게요."},
		{Type: post.BlockList, Items: []string{"가격 9000원", "주차 가능"}},
	}}
	if err := postSvc.SetGeneratedContent(ctx, "alice", withVoice.Slug, content, post.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	voiceID, revision, blocks, err = adapter.PostForFingerprint(ctx, "alice", withVoice.Slug)
	if err != nil || voiceID != aliceVoice.ID || revision != 1 || len(blocks) != 4 || blocks[3].Type != voice.BlockList || len(blocks[3].Items) != 2 {
		t.Fatalf("after a write = %q %d %+v err=%v", voiceID, revision, blocks, err)
	}
	if voiceID, _, _, err := adapter.PostForFingerprint(ctx, "alice", noVoice.Slug); err != nil || voiceID != "" {
		t.Fatalf("말투 없음 = %q err=%v", voiceID, err)
	}
	if _, _, _, err := adapter.PostForFingerprint(ctx, "bob", withVoice.Slug); !errors.Is(err, voice.ErrPostForbidden) {
		t.Fatalf("a foreign post = %v", err)
	}
	if _, _, _, err := adapter.PostForFingerprint(ctx, "alice", "nope"); !errors.Is(err, voice.ErrPostNotFound) {
		t.Fatalf("an unknown post = %v", err)
	}
}

type capturedAdmission struct {
	starts []job.Start
	refuse error
}

func (a *capturedAdmission) Hold(_ context.Context, start job.Start) error {
	a.starts = append(a.starts, start)
	return a.refuse
}
func (a *capturedAdmission) Release(context.Context, string)             {}
func (a *capturedAdmission) Settle(context.Context, string, string)      {}
func (a *capturedAdmission) OpenHolds(context.Context) ([]string, error) { return nil, nil }

// QUOTA-13, VOICE-43: a 검증 passes the shared admission as one write call at the write stage's
// floor, one runs per voice at a time, and a refused admission leaves no job.
func TestAVoiceCheckIsAdmittedAsOneWriteCall(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-check-jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := createTestVoice(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	found, err := firstTestVoice(ctx, voiceSvc, "alice")
	if err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	admission := &capturedAdmission{}
	queue.Admit(admission)
	budget := config.LLMCompletionBudget{WriteFloor: 8192, Ceiling: 32768}
	adapter := voiceJobs{queue: queue, budget: budget}
	request := voice.CheckJobRequest{UserID: "alice", VoiceID: found.ID, CheckID: "check-1", WriteModel: "stub/write"}

	id, err := adapter.EnqueueCheck(ctx, request)
	if err != nil || id == "" {
		t.Fatalf("enqueue = %q %v", id, err)
	}
	if len(admission.starts) != 1 {
		t.Fatalf("admissions = %+v", admission.starts)
	}
	start := admission.starts[0]
	if start.Kind != job.KindCheckVoice || len(start.Calls) != 1 || start.Calls[0] != (job.PlannedCall{Ref: "stub/write", Stage: "write", Count: 1, CompletionTokens: 8192}) {
		t.Fatalf("admitted %+v", start)
	}
	var kind, payload string
	if err := handle.Reader.QueryRow(`SELECT kind, payload FROM generation_jobs WHERE id = ? AND voice_id = ?`, id, found.ID).Scan(&kind, &payload); err != nil || kind != job.KindCheckVoice || payload != "check-1" {
		t.Fatalf("the job = %s %q err=%v", kind, payload, err)
	}
	var active *voice.JobAlreadyInProgressError
	if _, err := adapter.EnqueueCheck(ctx, voice.CheckJobRequest{UserID: "alice", VoiceID: found.ID, CheckID: "check-2", WriteModel: "stub/write"}); !errors.As(err, &active) || active.ActiveID != id {
		t.Fatalf("a second 검증 of one voice = %v", err)
	}

	refused := errors.New("no credits")
	admission.refuse = refused
	other, err := voiceSvc.CreateVoice(ctx, "alice", "일상")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.EnqueueCheck(ctx, voice.CheckJobRequest{UserID: "alice", VoiceID: other.ID, CheckID: "check-3", WriteModel: "stub/write"}); !errors.Is(err, refused) {
		t.Fatalf("a refused admission = %v", err)
	}
	var jobs int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM generation_jobs WHERE voice_id = ?`, other.ID).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("a refused admission left %d jobs: %v", jobs, err)
	}
}

// analyzeModels serves the analyze stage and counts provider calls; no test here reaches one.
type analyzeModels struct{ completes int }

func (m *analyzeModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Stages: []string{llm.StageNameAnalyze}}, true
}

func (m *analyzeModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	m.completes++
	return llm.Response{}, errors.New("no provider call is expected")
}

var analyzeTestRef = llm.ModelRef{ProviderID: "stub", ModelID: "analyze"}

// analysisHarness is one free account whose voice service enqueues through voiceJobs onto a
// queue the test admits on.
func analysisHarness(t *testing.T) (*db.DB, *job.Queue, *voice.Service, *analyzeModels) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-analysis-jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	models := &analyzeModels{}
	budget := config.LLMCompletionBudget{WriteFloor: 8192, Ceiling: 32768}
	return handle, queue, voice.NewService(voicestore.New(handle.Writer, handle.Reader), models, voiceJobs{queue: queue, budget: budget}), models
}

// analysisVoice is a new voice of alice's holding one 학습 글 of `lines` copies of one sentence
// (16 characters and a line break), which reaches 100% at voice.ReadySentences.
func analysisVoice(t *testing.T, handle *db.DB, voices *voice.Service, name, sampleID string, lines int) string {
	t.Helper()
	created, err := voices.CreateVoice(context.Background(), "alice", name)
	if err != nil {
		t.Fatal(err)
	}
	if err := voicestore.New(handle.Writer, handle.Reader).InsertSample(context.Background(), voice.Sample{
		ID: sampleID, UserID: "alice", VoiceID: created.ID, Kind: voice.SampleKindPost, Label: name,
		Body: strings.Repeat("오늘도 정말 맛있게 먹었어요.\n", lines), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

// QUOTA-14, VOICE-22: an analysis is admitted as one analyze call at the write floor, declaring
// the prompt its voice context sized, and its job row freezes the 학습 글 it will read.
func TestAVoiceAnalysisIsAdmittedAtItsDeclaredPrompt(t *testing.T) {
	handle, queue, voices, _ := analysisHarness(t)
	admission := &capturedAdmission{}
	queue.Admit(admission)
	voiceID := analysisVoice(t, handle, voices, "기본 말투", "sample", voice.ReadySentences)
	id, err := voices.AnalyzeVoice(context.Background(), "alice", voiceID, analyzeTestRef)
	if err != nil {
		t.Fatal(err)
	}
	if len(admission.starts) != 1 || len(admission.starts[0].Calls) != 1 {
		t.Fatalf("admissions = %+v", admission.starts)
	}
	call := admission.starts[0].Calls[0]
	if call.Ref != "stub/analyze" || call.Stage != llm.StageNameAnalyze || call.Count != 1 || call.CompletionTokens != 8192 ||
		call.PromptTokens < voice.ReadySentences*16 || call.PromptTokens >= 30_000 {
		t.Fatalf("admitted call = %+v", call)
	}
	var payload []byte
	if err := handle.Reader.QueryRow(`SELECT payload FROM generation_jobs WHERE id = ?`, id).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if frozen, err := voice.DecodeAnalysisSnapshot(payload); err != nil || strings.Join(frozen, ",") != "sample" {
		t.Fatalf("the job froze %v err=%v", frozen, err)
	}
}

// QUOTA-14, QUOTA-18: on the real ledger a small corpus holds the 30 000-token default and a
// 50 000-character one holds its own size, so a balance that covers the default but not the
// larger hold refuses the large start with the insufficient-credit failure: no job, no call.
func TestAVoiceAnalysisOverTheBalanceIsRefusedAtTheStart(t *testing.T) {
	handle, queue, voices, models := analysisHarness(t)
	ctx := context.Background()
	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	store := usagestore.New(handle.Writer, handle.Reader)
	// One USD per million prompt tokens and nothing for completion, at 1 400 KRW per USD: the
	// 30 000-token default holds 42 credits and 50 000 tokens hold 70.
	ledger := usage.NewService(store, lookupModels{analyzeTestRef: {Ref: analyzeTestRef, InputUSDPerMillion: "1", OutputUSDPerMillion: "0"}},
		8192, usageAnchors{auth: authSvc}, usage.NewFixedRateSelector(14_000_000))
	queue.Admit(jobAdmission{ledger: ledger, plans: authSvc})
	if err := ledger.Grant(ctx, "alice", 60, nil); err != nil {
		t.Fatal(err)
	}

	large := analysisVoice(t, handle, voices, "긴 말투", "long", 3_000)
	_, err := voices.AnalyzeVoice(ctx, "alice", large, analyzeTestRef)
	var refusal *plan.InsufficientCreditsError
	if !errors.As(err, &refusal) || refusal.Required < 70 || refusal.Balance != 60 {
		t.Fatalf("a 50 000-character analysis on 60 credits = %v", err)
	}
	var jobs int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM generation_jobs WHERE voice_id = ?`, large).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("a refused start left %d jobs: %v", jobs, err)
	}

	small := analysisVoice(t, handle, voices, "짧은 말투", "short", voice.ReadySentences)
	id, err := voices.AnalyzeVoice(ctx, "alice", small, analyzeTestRef)
	if err != nil {
		t.Fatal(err)
	}
	if admission, found, err := ledger.AdmissionForJob(ctx, id); err != nil || !found || admission.HoldCredits != 42 {
		t.Fatalf("a small corpus held %+v found=%v err=%v, want the default's 42", admission, found, err)
	}
	if models.completes != 0 {
		t.Fatal("starting an analysis called a provider")
	}
}
