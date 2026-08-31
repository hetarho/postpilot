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
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/purpose"
	purposestore "github.com/postpilot/backend/internal/purpose/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type noBlobs struct{}

func (noBlobs) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (noBlobs) PresignGet(context.Context, string, time.Duration) (string, error) { return "", nil }
func (noBlobs) Head(context.Context, string) (int64, error)                       { return 0, post.ErrObjectNotFound }
func (noBlobs) Delete(context.Context, string) error                              { return nil }
func (noBlobs) List(context.Context, string) ([]post.Object, error)               { return nil, nil }

type trackingVoiceModels struct{ calls int }

func (m *trackingVoiceModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	m.calls++
	return llm.ModelRef{}, false, nil
}
func (m *trackingVoiceModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	m.calls++
	return llm.ModelInfo{}, false
}
func (m *trackingVoiceModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	m.calls++
	return llm.Response{}, nil
}
func (m *trackingVoiceModels) ModelEnabled(llm.ModelRef) bool {
	m.calls++
	return false
}

type trackingVoiceJobs struct{ calls int }

func (j *trackingVoiceJobs) Enqueue(context.Context, voice.AnalysisJobRequest) (string, error) {
	j.calls++
	return "", nil
}
func (j *trackingVoiceJobs) ActiveForVoiceKind(context.Context, string, string) (*voice.ActiveJob, error) {
	j.calls++
	return nil, nil
}
func (j *trackingVoiceJobs) HasActiveForVoice(context.Context, string) (bool, error) {
	j.calls++
	return false, nil
}
func (j *trackingVoiceJobs) EnqueuePersonalization(context.Context, voice.PersonalizationJobRequest) (string, error) {
	j.calls++
	return "", nil
}
func (j *trackingVoiceJobs) IsPersonalizationJobActive(context.Context, string, string) (bool, error) {
	j.calls++
	return false, nil
}
func (j *trackingVoiceJobs) FailQueuedPersonalization(context.Context, string, string, voice.Failure) (bool, error) {
	j.calls++
	return false, nil
}

// Plan 10 A2: a new account cannot create a post until the adduser bootstrap has given it
// an active default voice, and rerunning the bootstrap never duplicates that voice.
func TestAccountBootstrapPrecedesPostCreation(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "bootstrap.db"))
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
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})

	if _, err := voiceSvc.DefaultVoice(ctx, "alice"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("default before bootstrap = %v", err)
	}
	guess := "any"
	language := post.LanguageKorean
	if _, err := postSvc.SaveDraft(ctx, "alice", "", "first", "", &guess, nil, &language); !errors.Is(err, post.ErrVoiceNotFound) {
		t.Fatalf("post before bootstrap = %v", err)
	}
	for range 2 {
		if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
			t.Fatal(err)
		}
	}
	voices, err := voiceSvc.ListVoices(ctx, "alice")
	if err != nil || len(voices) != 1 || !voices[0].IsDefault || voices[0].Name != voice.DefaultVoiceName {
		t.Fatalf("voices after two bootstraps = %+v err=%v", voices, err)
	}
	created, err := postSvc.SaveDraft(ctx, "alice", "", "first", "", &voices[0].ID, nil, &language)
	if err != nil || created.VoiceID != voices[0].ID || created.Voice.Name != voice.DefaultVoiceName {
		t.Fatalf("post after bootstrap = %+v err=%v", created, err)
	}
}

// Plan 11 A4/A7: the composition root is the ONLY place the post's purpose id crosses into
// generation, and every prompt hangs off it. This walks the real adapters end to end — a post
// saved with a 용도, read back through generationPosts, resolved through generationPurposes —
// because both contexts' own tests inject the id into a fake and so cannot see it dropped here.
func TestGenerationAdapterCarriesThePostPurposeThroughToTheFrozenBrief(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "purpose.db"))
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
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}

	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	purposeSvc := purpose.NewService(
		purposestore.New(handle.Writer, handle.Reader),
		purpose.Limits{NameMaxChars: 40, DescriptionMaxChars: 200, InstructionsMaxChars: 2000},
	)
	postSvc.SetPurposeDirectory(postPurposes{service: purposeSvc})

	created, err := purposeSvc.Create(ctx, "alice", "정보성 식당 리뷰", "협찬 방문 리뷰", "사진마다 설명하세요")
	if err != nil {
		t.Fatal(err)
	}
	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	language := post.LanguageKorean
	saved, err := postSvc.SaveDraft(ctx, "alice", "", "제주", "", &defaultVoice.ID, &created.ID, &language)
	if err != nil {
		t.Fatal(err)
	}

	input, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if input.PurposeID != created.ID {
		t.Fatalf("the adapter dropped the purpose: PurposeID=%q, want %q", input.PurposeID, created.ID)
	}

	brief, ok, err := (generationPurposes{service: purposeSvc}).BriefFor(ctx, "alice", input.PurposeID)
	if err != nil || !ok {
		t.Fatalf("brief lookup: ok=%v err=%v", ok, err)
	}
	want := generation.PurposeBrief{Name: "정보성 식당 리뷰", Description: "협찬 방문 리뷰", Instructions: "사진마다 설명하세요"}
	if brief != want {
		t.Fatalf("brief = %+v, want %+v", brief, want)
	}
	// And the prompt that brief produces actually carries it.
	system, _ := generation.BuildWritePrompt(generation.Profile{}, nil, "", "", nil, nil, &brief, nil)
	if !strings.Contains(system, "[글의 용도: 정보성 식당 리뷰]") {
		t.Fatalf("the frozen brief did not reach the prompt:\n%s", system)
	}

	// A post left on 없음 resolves to no brief, so the prompt is the pre-purpose one.
	plain, err := postSvc.SaveDraft(ctx, "alice", "", "용도 없는 글", "", &defaultVoice.ID, nil, &language)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", plain.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if bare.PurposeID != "" {
		t.Fatalf("a post with no purpose reported %q", bare.PurposeID)
	}
}

// Plan 13 A13: the composition root must enrich the post-owned content provenance with
// the voice-owned source language before handing a finalized snapshot to voice. Context
// tests exercise each side with fakes; this regression walks the real stores and adapter
// so neither language can be silently dropped at the seam.
func TestVoiceLearningAdapterCarriesBothLanguagesBeforeTheEqualityGate(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-language.db"))
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
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}

	models := &trackingVoiceModels{}
	jobs := &trackingVoiceJobs{}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models, jobs)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})

	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	target := post.LanguageEnglish
	created, err := postSvc.SaveDraft(ctx, "alice", "", "English final", "", &defaultVoice.ID, nil, &target)
	if err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{
		Title:  "An English post",
		Blocks: []post.Block{{Type: post.BlockText, Content: "English content under a Korean-source voice."}},
	}
	if err := postSvc.SetGeneratedContent(ctx, "alice", created.Slug, content, post.LanguageEnglish); err != nil {
		t.Fatal(err)
	}
	if _, err := postSvc.Finalize(ctx, "alice", created.Slug, 1); err != nil {
		t.Fatal(err)
	}

	adapter := voicePosts{service: postSvc}
	snapshot, err := adapter.LearningSnapshot(ctx, "alice", created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ContentLanguage != voice.LanguageEnglish || snapshot.VoiceSourceLanguage != voice.LanguageKorean {
		t.Fatalf("language hand-off = content %q, source %q; want en/ko", snapshot.ContentLanguage, snapshot.VoiceSourceLanguage)
	}
	if snapshot.VoiceID != defaultVoice.ID || snapshot.BaselineVoiceID != defaultVoice.ID {
		t.Fatalf("voice hand-off = current %q, baseline %q; want %q", snapshot.VoiceID, snapshot.BaselineVoiceID, defaultVoice.ID)
	}

	voiceSvc.ConfigurePersonalization(adapter, voice.PersonalizationConfig{
		FewShotTargetCount: 2, FewShotMax: 3, FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800,
		EmbeddingSwitchPosts: 50, DiffMaxRules: 3, DiffMinPatternEdits: 2, RuleActivationEvidence: 3,
		RuleRetireAfter: 180 * 24 * time.Hour, ValidationPostCount: voice.DefaultValidationPostCount, EndingMaxConsecutive: 2,
	})
	requested := llm.ModelRef{ProviderID: "test", ModelID: "analyze"}
	if _, _, _, err := voiceSvc.LearnFromFinalizedPost(ctx, "alice", created.Slug, requested); !errors.Is(err, voice.ErrContentLanguageMismatch) {
		t.Fatalf("cross-language learning = %v, want ErrContentLanguageMismatch", err)
	}
	if models.calls != 0 || jobs.calls != 0 {
		t.Fatalf("language mismatch crossed an external boundary: model calls=%d job calls=%d", models.calls, jobs.calls)
	}
	var events int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM voice_learning_events WHERE user_id = ?", "alice").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("language mismatch inserted %d learning events", events)
	}
}

// Plan 16 A4/A8: the composition root is the ONLY place a post's purpose id reaches the
// guideline context, and the whole prompt section hangs off it. This walks the real adapters
// end to end — a purpose and two guidelines created through their own services, a post saved
// with that 용도, read back through generationPosts, resolved through generationGuidelines,
// rendered by the real prompt builder. The job 22 review caught this seam silently dropping a
// field, and only a real-wiring test prevents a repeat.
func TestGuidelineAdapterCarriesScopeThroughToTheFrozenPromptSection(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "guideline.db"))
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
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}

	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	purposeSvc := purpose.NewService(
		purposestore.New(handle.Writer, handle.Reader),
		purpose.Limits{NameMaxChars: 40, DescriptionMaxChars: 200, InstructionsMaxChars: 2000},
	)
	postSvc.SetPurposeDirectory(postPurposes{service: purposeSvc})
	guidelineSvc := guideline.NewService(
		guidelinestore.New(handle.Writer, handle.Reader),
		guideline.Limits{TextMaxChars: 300, MaxPerAccount: 100},
	)
	guidelineSvc.SetPurposeDirectory(guidelinePurposes{service: purposeSvc})

	review, err := purposeSvc.Create(ctx, "alice", "무인가게 리뷰", "", "사진마다 설명하세요")
	if err != nil {
		t.Fatal(err)
	}
	other, err := purposeSvc.Create(ctx, "alice", "협찬 리뷰", "", "협찬을 밝히세요")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guidelineSvc.Create(ctx, "alice", "없는 사실을 쓰지 않기", guideline.ScopeGlobal, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := guidelineSvc.Create(ctx, "alice", "CCTV를 언급하지 않기", guideline.ScopePurposes, []string{review.ID}); err != nil {
		t.Fatal(err)
	}
	// Scoped to the OTHER purpose, so it must never reach this post's prompt.
	if _, err := guidelineSvc.Create(ctx, "alice", "협찬 표기를 빠뜨리지 않기", guideline.ScopePurposes, []string{other.ID}); err != nil {
		t.Fatal(err)
	}

	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	language := post.LanguageKorean
	saved, err := postSvc.SaveDraft(ctx, "alice", "", "무인 떡집", "", &defaultVoice.ID, &review.ID, &language)
	if err != nil {
		t.Fatal(err)
	}
	input, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}

	adapter := generationGuidelines{service: guidelineSvc}
	texts, err := adapter.ForPrompt(ctx, "alice", &input.PurposeID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"없는 사실을 쓰지 않기", "CCTV를 언급하지 않기"}
	if len(texts) != len(want) {
		t.Fatalf("resolved %v, want %v", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Fatalf("resolved %v, want %v", texts, want)
		}
	}
	system, _ := generation.BuildWritePrompt(generation.Profile{}, nil, "", "", nil, nil, nil, texts)
	if !strings.Contains(system, "[작문 지침]\n- 없는 사실을 쓰지 않기\n- CCTV를 언급하지 않기") {
		t.Fatalf("the frozen guidelines did not reach the prompt:\n%s", system)
	}
	if strings.Contains(system, "협찬 표기") {
		t.Fatalf("a guideline scoped to another purpose reached the prompt:\n%s", system)
	}

	// A post left on 없음 receives the global group alone.
	plain, err := postSvc.SaveDraft(ctx, "alice", "", "용도 없는 글", "", &defaultVoice.ID, nil, &language)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", plain.Slug)
	if err != nil {
		t.Fatal(err)
	}
	global, err := adapter.ForPrompt(ctx, "alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if bare.PurposeID != "" || len(global) != 1 || global[0] != "없는 사실을 쓰지 않기" {
		t.Fatalf("a post with no purpose resolved %v (purpose id %q)", global, bare.PurposeID)
	}
}
