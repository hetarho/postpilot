package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/quality"
	qualitystore "github.com/postpilot/backend/internal/quality/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// The adapter stores the payload generation encoded byte for byte, and the worker's mapping hands
// those same bytes back: nothing in between decodes, retypes or drops a frozen member.
func TestTheGenerateRowCarriesStartsPayloadVerbatim(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "freeze.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, statement := range []string{
		"INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)",
		"INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-alice','alice','기본',1,?,?)",
		"INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('alice-post','alice','voice-alice',?,?)",
	} {
		args := []any{now}
		if strings.Count(statement, "?") == 2 {
			args = append(args, now)
		}
		if _, err := d.Writer.Exec(statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	queue := job.New(jobstore.New(d.Writer, d.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	queue.Admit(&stubAdmitter{})
	jobs := generationJobs{queue: queue, budget: testCompletionBudget()}

	raw := []byte(`{"target_language":"ko","quality_rules":["x"],"write_native_effort":true}`)
	id, err := jobs.EnqueueGeneration(ctx, generation.StartRequest{
		UserID: "alice", PostSlug: "alice-post", VoiceID: "voice-alice", WriteModel: "p/writer", TargetLanguage: generation.LanguageKorean,
	}, raw)
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err := d.Reader.QueryRow("SELECT payload FROM generation_jobs WHERE id = ?", id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, raw) {
		t.Fatalf("the row stored %s, want %s", stored, raw)
	}
	subjects, _ := postVoiceWork(job.KindGenerate, "alice", "alice-post", "voice-alice")
	run, err := generateJob(job.Job{ID: id, Kind: job.KindGenerate, UserID: "alice", Subjects: subjects, WriteModel: "p/writer", Payload: stored})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(run.Payload, raw) || run.PostSlug != "alice-post" || run.VoiceID != "voice-alice" || run.WriteModel != "p/writer" {
		t.Fatalf("the worker's run = %+v", run)
	}
	if _, err := generateJob(job.Job{Kind: job.KindGenerate, UserID: "alice"}); !errors.Is(err, job.ErrInvalidTarget) {
		t.Fatalf("a job without a post subject mapped: %v", err)
	}
}

// ARCH-3: the three copies of the tick ids — post's mirror, quality's list and the order both
// read them in — are one list.
func TestPostTickIdsAreTheQualityMetrics(t *testing.T) {
	metrics := quality.Metrics()
	reversed := make([]string, 0, len(metrics))
	for i := len(metrics) - 1; i >= 0; i-- {
		reversed = append(reversed, string(metrics[i]))
	}
	normalized, err := post.NormalizeQualityRules(reversed)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, len(metrics))
	for i, m := range metrics {
		want[i] = string(m)
	}
	if !reflect.DeepEqual(normalized, want) {
		t.Fatalf("post normalizes to %q, quality lists %q", normalized, want)
	}
	for mirror, metric := range map[string]quality.Metric{
		post.QualityRuleTitleSaturation:  quality.MetricTitleSaturation,
		post.QualityRuleCrossPostPhrases: quality.MetricCrossPostPhrases,
		post.QualityRuleInPostRepetition: quality.MetricInPostRepetition,
		post.QualityRuleComposition:      quality.MetricComposition,
	} {
		if mirror != string(metric) {
			t.Errorf("post's %q is quality's %q", mirror, metric)
		}
	}
}

// Generation's other collaborators, each answering "nothing here".
type (
	freezeProfiles    struct{}
	freezeRules       struct{}
	freezeImages      struct{}
	freezeExperiments struct{}
	freezeMemories    struct{}
	freezeCandidates  struct{}
	freezeSamples     struct{}
	freezeLinker      struct{}
)

func (freezeProfiles) ProfileForPrompt(context.Context, string, string, generation.Language) (generation.Profile, error) {
	return generation.Profile{}, nil
}
func (freezeRules) AppendRule(context.Context, string, string, string) error { return nil }
func (freezeImages) Read(context.Context, string) ([]byte, error) {
	return nil, errors.New("no images in this test")
}
func (freezeExperiments) PendingForPost(context.Context, string, string) (string, error) {
	return "", nil
}
func (freezeMemories) ForPost(context.Context, string, []string) ([]string, error) { return nil, nil }
func (freezeCandidates) Record(context.Context, string, string, string) error      { return nil }
func (freezeSamples) RecordVersionSample(context.Context, string, string, generation.PostContent) error {
	return nil
}
func (freezeLinker) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

// recordingModels answers every call with one content and keeps every request, from the worker's
// goroutine.
type recordingModels struct {
	mu       sync.Mutex
	requests []llm.Request
	// nativeEffort is what every model resolves with as ReasoningNativeEffort.
	nativeEffort bool
}

func (m *recordingModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Ref: ref, StructuredOutput: true, Stages: []string{llm.StageNameWrite}, ReasoningNativeEffort: m.nativeEffort}, true
}

func (m *recordingModels) Complete(_ context.Context, _ llm.ModelRef, request llm.Request) (llm.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, request)
	return llm.Response{Text: `{"title":"을지로 노포","summary":"요약","tags":["을지로","노포","맛집","식당"],"blocks":[{"type":"TEXT","content":"을지로 골목의 노포에 다녀왔다."}]}`}, nil
}

func (m *recordingModels) last() llm.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests[len(m.requests)-1]
}

// drainHarness is the real generate path end to end: real stores and services, the real
// generationJobs adapter, a running queue worker with the production generateJob mapping, and
// models that record every request.
type drainHarness struct {
	ctx        context.Context
	handle     *db.DB
	posts      *post.Service
	guidelines *guideline.Service
	generation *generation.Service
	quality    *qualitystore.Store
	voiceID    string
	waitDone   func(id string)
}

func newDrainHarness(t *testing.T, models *recordingModels) *drainHarness {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "phrases.db"))
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
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, testPostLimits(), testPostDeps(voiceSvc))
	templateSvc := template.NewService(templatestore.New(handle.Writer, handle.Reader), testTemplateLimits())
	postSvc.SetTemplateDirectory(postTemplates{service: templateSvc})
	guidelineSvc := guideline.NewService(guidelinestore.New(handle.Writer, handle.Reader), blogFields{}, guideline.Limits{TextMaxChars: 300, MaxPerAccount: 100}, 50)
	guidelineSvc.SetTemplateDirectory(guidelineTemplates{service: templateSvc})
	qualityStore := qualitystore.New(handle.Writer, handle.Reader)
	qualitySvc := quality.NewService(quality.Deps{Measurements: qualityStore, Phrases: qualityStore, Posts: qualityPosts{service: postSvc}, Now: time.Now})

	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	queue.Admit(&stubAdmitter{})
	generationSvc := generation.NewService(
		generationPosts{service: postSvc}, freezeProfiles{}, freezeRules{}, models, freezeImages{},
		generationJobs{queue: queue, budget: testCompletionBudget()}, 4, generation.DefaultReasoningPolicy(), testCompletionBudget(),
		generation.Deps{
			Experiments: freezeExperiments{}, Templates: generationTemplates{service: templateSvc},
			Guidelines: generationGuidelines{service: guidelineSvc}, Memories: freezeMemories{},
			Candidates: freezeCandidates{}, Samples: freezeSamples{}, Videos: freezeLinker{}, VideoURLTTL: time.Minute,
			QualityRules: generationQuality{service: qualitySvc}, FieldPhrases: generationFieldPhrases{service: qualitySvc},
		},
	)
	// The worker's own handlers, not a copy (review F17): a mapping change in registerJobs is what
	// this harness runs. Only the generate and revise kinds are ever enqueued here, so the other
	// contexts registerJobs captures can stay nil.
	registerJobs(&contexts{jobs: queue, generation: generationSvc})
	workerCtx, stop := context.WithCancel(ctx)
	t.Cleanup(stop)
	go queue.Run(workerCtx)
	waitDone := func(id string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			summary, err := queue.Get(ctx, id, "alice")
			if err != nil {
				t.Fatal(err)
			}
			if job.Terminal(summary.Status) {
				if summary.Status != job.StatusDone {
					t.Fatalf("job %s finished %s: %+v", id, summary.Status, summary.Failure)
				}
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("job %s did not finish", id)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	return &drainHarness{
		ctx: ctx, handle: handle, posts: postSvc, guidelines: guidelineSvc, generation: generationSvc,
		quality: qualityStore, voiceID: defaultVoice.ID, waitDone: waitDone,
	}
}

// draft saves a Korean draft on the default voice, optionally in a 분야.
func (h *drainHarness) draft(t *testing.T, field string) post.Post {
	t.Helper()
	language := post.LanguageKorean
	save := post.DraftSave{Title: "을지로", Memo: "노포에 갔다", VoiceID: &h.voiceID, TargetLanguage: &language}
	if field != "" {
		save.Field = &field
	}
	saved, err := h.posts.SaveDraft(h.ctx, "alice", save)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

// Review F1: a write model with native reasoning effort is held for its headroom at the enqueue,
// so the drained write call must ask for that same headroom — not the bare budget.
func TestANativeEffortStartDrainsWithReasoningHeadroom(t *testing.T) {
	models := &recordingModels{nativeEffort: true}
	h := newDrainHarness(t, models)
	saved := h.draft(t, "")
	writer := llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()
	id, err := h.generation.Start(h.ctx, generation.StartRequest{UserID: "alice", PostSlug: saved.Slug, WriteModel: writer})
	if err != nil {
		t.Fatal(err)
	}
	h.waitDone(id)
	if got, want := models.last().MaxTokens, testCompletionBudget().Write(nil, true); got != want {
		t.Fatalf("the drained write asked for %d tokens, want %d (the headroom the hold priced)", got, want)
	}
}

// GEN-48, GUIDE-30, GEN-57 end to end on the real stores and adapters: a post in 분야 restaurant,
// with a stored list and the preset on for restaurant, writes with the phrase section and the
// preset line; its revision carries neither.
func TestFieldPhrasesReachTheWritePromptAndNeverTheRevisePrompt(t *testing.T) {
	models := &recordingModels{}
	h := newDrainHarness(t, models)
	ctx, qualityStore, guidelineSvc, generationSvc, waitDone := h.ctx, h.quality, h.guidelines, h.generation, h.waitDone

	// Phrases the model's content never says, so the revise prompt, which quotes the content,
	// cannot carry them by accident.
	listed := []string{"웨이팅 필수 맛집", "줄 서는 식당"}
	refreshed := time.Now()
	if err := qualityStore.ReplacePhraseList(ctx, quality.PhraseList{Field: "restaurant", Phrases: listed, CorpusSize: 120, RefreshedAt: &refreshed, NextRefreshAt: refreshed.Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	on, restaurant := true, []string{"restaurant"}
	if _, err := guidelineSvc.UpdatePreset(ctx, "alice", guideline.PresetPatch{Enabled: &on, Fields: &restaurant}); err != nil {
		t.Fatal(err)
	}
	saved := h.draft(t, "restaurant")

	writer := llm.ModelRef{ProviderID: "p", ModelID: "writer"}.String()

	id, err := generationSvc.Start(ctx, generation.StartRequest{UserID: "alice", PostSlug: saved.Slug, WriteModel: writer})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(id)
	written := models.last()
	user := written.Messages[0].Parts[0].Text
	for _, want := range append([]string{generation.FieldPhrasesHeading}, listed...) {
		if !strings.Contains(user, want) {
			t.Errorf("the write's per-post half lacks %q", want)
		}
	}
	if !strings.Contains(written.System, guideline.PresetText) {
		t.Errorf("the write's system prompt lacks the preset line:\n%s", written.System)
	}

	revision, err := generationSvc.StartRevision(ctx, generation.StartRevisionRequest{UserID: "alice", PostSlug: saved.Slug, Instruction: "조금 더 짧게", WriteModel: writer})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(revision)
	revised := models.last()
	prompt := revised.System + "\n" + revised.Messages[0].Parts[0].Text
	for _, never := range append([]string{generation.FieldPhrasesHeading, guideline.PresetText}, listed...) {
		if strings.Contains(prompt, never) {
			t.Errorf("the revise prompt carries %q", never)
		}
	}
}

// The adapter renders the ticks in the run's language — the quality service's own texts, in
// Korean or English — and refuses a language it cannot name rather than guessing one.
func TestGenerationQualityRendersTheTicksInTheRunsLanguage(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "rules.db"))
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
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, testPostLimits(), testPostDeps(voiceSvc))
	qualityStore := qualitystore.New(handle.Writer, handle.Reader)
	qualitySvc := quality.NewService(quality.Deps{Measurements: qualityStore, Phrases: qualityStore, Posts: qualityPosts{service: postSvc}, Now: time.Now})
	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// Three published posts of two block types each: composition is over its band (QUAL-11).
	language := post.LanguageKorean
	var slug string
	for i := 0; i < 3; i++ {
		created, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "기록", VoiceID: &defaultVoice.ID, TargetLanguage: &language})
		if err != nil {
			t.Fatal(err)
		}
		content := post.PostContent{Title: "기록", Blocks: []post.Block{
			{Type: post.BlockHeading, Content: "첫날", Level: 2},
			{Type: post.BlockText, Content: "협재 해변은 물빛이 맑았다."},
		}}
		if err := postSvc.SetGeneratedContent(ctx, "alice", created.Slug, content, post.LanguageKorean, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := postSvc.Finalize(ctx, "alice", created.Slug, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := postSvc.SavePublishedURL(ctx, "alice", created.Slug, "https://blog.naver.com/alice/22300000000"+string(rune('1'+i))); err != nil {
			t.Fatal(err)
		}
		slug = created.Slug
	}
	ticked := []string{string(quality.MetricComposition)}
	adapter := generationQuality{service: qualitySvc}
	for generationLanguage, qualityLanguage := range map[generation.Language]quality.Language{
		generation.LanguageKorean:  quality.LanguageKorean,
		generation.LanguageEnglish: quality.LanguageEnglish,
	} {
		want, err := qualitySvc.RulesFor(ctx, "alice", slug, ticked, qualityLanguage)
		if err != nil || len(want) != 1 {
			t.Fatalf("%s: the service rendered %q (%v); the fixture is not over band", generationLanguage, want, err)
		}
		got, err := adapter.RulesFor(ctx, "alice", slug, ticked, generationLanguage)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: the adapter rendered %q (%v), want %q", generationLanguage, got, err, want)
		}
	}
	korean, _ := adapter.RulesFor(ctx, "alice", slug, ticked, generation.LanguageKorean)
	english, _ := adapter.RulesFor(ctx, "alice", slug, ticked, generation.LanguageEnglish)
	if reflect.DeepEqual(korean, english) {
		t.Fatalf("both languages rendered %q", korean)
	}
	if _, err := adapter.RulesFor(ctx, "alice", slug, ticked, generation.Language("fr")); err == nil {
		t.Fatal("an unknown language was rendered")
	}
}
