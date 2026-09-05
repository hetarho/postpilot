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
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
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
func (m *trackingVoiceModels) ModelEnabled(llm.ModelRef, string) bool {
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
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20, 30)
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

// Plan 11 A4/A7: the composition root is the ONLY place the post's template id crosses into
// generation, and every prompt hangs off it. This walks the real adapters end to end — a post
// saved with a 템플릿, read back through generationPosts, resolved through generationTemplates —
// because both contexts' own tests inject the id into a fake and so cannot see it dropped here.
func TestGenerationAdapterCarriesThePostTemplateThroughToTheFrozenBrief(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "template.db"))
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
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20, 30)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	templateSvc := template.NewService(
		templatestore.New(handle.Writer, handle.Reader),
		template.Limits{
			NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000,
			MaxPerAccount: 50, MaxRepeatExpansion: 40,
		},
	)
	postSvc.SetTemplateDirectory(postTemplates{service: templateSvc})

	created, err := templateSvc.Create(ctx, "alice", "정보성 식당 리뷰", "협찬 방문 리뷰",
		"<write>인트로</write>\n<slot kind=\"place\" label=\"네이버 지도\"/>\n<repeat each=\"photo\">\n<slot kind=\"photo\"/>\n<write>사진 설명</write>\n</repeat>")
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
	if input.TemplateID != created.ID {
		t.Fatalf("the adapter dropped the template: TemplateID=%q, want %q", input.TemplateID, created.ID)
	}

	// The render is where the two contexts actually meet: generation hands over the frozen
	// attachment order and receives prompt text plus the slots that text declared.
	brief, ok, err := (generationTemplates{service: templateSvc}).RenderedFor(ctx, "alice", input.TemplateID, []string{"IMG_1.jpg", "IMG_2.jpg"})
	if err != nil || !ok {
		t.Fatalf("render: ok=%v err=%v", ok, err)
	}
	if brief.Name != "정보성 식당 리뷰" {
		t.Fatalf("brief = %+v", brief)
	}
	if len(brief.Slots) != 1 || brief.Slots[0].Kind != "place" || brief.Slots[0].Label != "네이버 지도" {
		t.Fatalf("slots = %+v", brief.Slots)
	}
	// Two photos, so the repeat expanded twice and each iteration is bound to its own file.
	if !strings.Contains(brief.Body, "{{photo:IMG_1.jpg}}") || !strings.Contains(brief.Body, "{{photo:IMG_2.jpg}}") {
		t.Fatalf("the repeat did not expand per attachment:\n%s", brief.Body)
	}
	if got := strings.Count(brief.Body, "<write>사진 설명</write>"); got != 2 {
		t.Fatalf("per-photo write rendered %d times, want 2", got)
	}
	// And the prompt that render produces actually carries it.
	system, _ := generation.BuildWritePrompt(generation.Profile{}, nil, "", "", nil, nil, &brief, nil)
	if !strings.Contains(system, "[글 템플릿: 정보성 식당 리뷰]") {
		t.Fatalf("the frozen template did not reach the prompt:\n%s", system)
	}

	// A post left on 없음 resolves to no brief, so the prompt is the pre-template one.
	plain, err := postSvc.SaveDraft(ctx, "alice", "", "템플릿 없는 글", "", &defaultVoice.ID, nil, &language)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", plain.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if bare.TemplateID != "" {
		t.Fatalf("a post with no template reported %q", bare.TemplateID)
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
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20, 30)
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

// Plan 16 A4/A8: the composition root is the ONLY place a post's template id reaches the
// guideline context, and the whole prompt section hangs off it. This walks the real adapters
// end to end — a template and two guidelines created through their own services, a post saved
// with that 템플릿, read back through generationPosts, resolved through generationGuidelines,
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
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20, 30)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	templateSvc := template.NewService(
		templatestore.New(handle.Writer, handle.Reader),
		template.Limits{
			NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000,
			MaxPerAccount: 50, MaxRepeatExpansion: 40,
		},
	)
	postSvc.SetTemplateDirectory(postTemplates{service: templateSvc})
	guidelineSvc := guideline.NewService(
		guidelinestore.New(handle.Writer, handle.Reader),
		guideline.Limits{TextMaxChars: 300, MaxPerAccount: 100},
		50,
	)
	guidelineSvc.SetTemplateDirectory(guidelineTemplates{service: templateSvc})

	review, err := templateSvc.Create(ctx, "alice", "무인가게 리뷰", "", "사진마다 설명하세요")
	if err != nil {
		t.Fatal(err)
	}
	other, err := templateSvc.Create(ctx, "alice", "협찬 리뷰", "", "협찬을 밝히세요")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := guidelineSvc.Create(ctx, "alice", "없는 사실을 쓰지 않기", guideline.ScopeGlobal, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := guidelineSvc.Create(ctx, "alice", "CCTV를 언급하지 않기", guideline.ScopeTemplates, []string{review.ID}, ""); err != nil {
		t.Fatal(err)
	}
	// Scoped to the OTHER template, so it must never reach this post's prompt.
	if _, err := guidelineSvc.Create(ctx, "alice", "협찬 표기를 빠뜨리지 않기", guideline.ScopeTemplates, []string{other.ID}, ""); err != nil {
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
	texts, err := adapter.ForPrompt(ctx, "alice", &input.TemplateID)
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
		t.Fatalf("a guideline scoped to another template reached the prompt:\n%s", system)
	}

	// A post left on 없음 receives the global group alone.
	plain, err := postSvc.SaveDraft(ctx, "alice", "", "템플릿 없는 글", "", &defaultVoice.ID, nil, &language)
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
	if bare.TemplateID != "" || len(global) != 1 || global[0] != "없는 사실을 쓰지 않기" {
		t.Fatalf("a post with no template resolved %v (template id %q)", global, bare.TemplateID)
	}
}

// Change 26 through the real adapters and the real store: recording rides the generation
// context's port, review reads the guideline context, and approval is the create. This is the
// only place the two contexts meet, so it is where the seam is worth proving.
func TestGuidelineCandidateAdaptersRecordReviewAndApproveAcrossTheSeam(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "guideline-candidate.db"))
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
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20, 30)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	templateSvc := template.NewService(
		templatestore.New(handle.Writer, handle.Reader),
		template.Limits{
			NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000,
			MaxPerAccount: 50, MaxRepeatExpansion: 40,
		},
	)
	postSvc.SetTemplateDirectory(postTemplates{service: templateSvc})
	// Two pending candidates allowed, so the bound is reachable in a test without 50 rows.
	guidelineSvc := guideline.NewService(
		guidelinestore.New(handle.Writer, handle.Reader),
		guideline.Limits{TextMaxChars: 300, MaxPerAccount: 100},
		2,
	)
	guidelineSvc.SetTemplateDirectory(guidelineTemplates{service: templateSvc})
	// The delete path's two required hooks, stubbed: this test is about the candidate link.
	postSvc.SetExperimentContentPurger(noPurge{})
	postSvc.SetLivePublishFinder(noLivePublish{})
	postSvc.SetGuidelineCandidateDetacher(postCandidateLinks{service: guidelineSvc})

	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	language := post.LanguageKorean
	saved, err := postSvc.SaveDraft(ctx, "alice", "", "무인 떡집", "", &defaultVoice.ID, nil, &language)
	if err != nil {
		t.Fatal(err)
	}

	// A1: recording through the generation context's port, wired exactly as main wires it.
	recorder := generationCandidates{service: guidelineSvc}
	const instruction = "여기  너무 광고 같아!! (특히 마지막 문단)"
	if err := recorder.Record(ctx, "alice", saved.Slug, "  "+instruction+"  "); err != nil {
		t.Fatal(err)
	}
	candidates, full, err := guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || full {
		t.Fatalf("candidates=%d full=%v", len(candidates), full)
	}
	if candidates[0].Text != instruction || candidates[0].PostSlug != saved.Slug {
		t.Fatalf("recorded %+v, want the instruction verbatim against the post", candidates[0])
	}

	// A2: a repeat counts up rather than creating a second row.
	if err := recorder.Record(ctx, "alice", saved.Slug, instruction); err != nil {
		t.Fatal(err)
	}
	candidates, _, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Occurrences != 2 {
		t.Fatalf("after a repeat: %+v", candidates)
	}

	// A9: the queue stops rather than evicting, and the screen is told why.
	if err := recorder.Record(ctx, "alice", saved.Slug, "존댓말로 써줘"); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(ctx, "alice", saved.Slug, "문단을 짧게"); err != nil {
		t.Fatal(err)
	}
	candidates, full, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || !full {
		t.Fatalf("at the bound: candidates=%d full=%v", len(candidates), full)
	}

	// A5: approval is the standard create, with the scope chosen here and nowhere earlier.
	review, err := templateSvc.Create(ctx, "alice", "무인가게 리뷰", "", "사진마다 설명하세요")
	if err != nil {
		t.Fatal(err)
	}
	created, err := guidelineSvc.Create(ctx, "alice", "광고처럼 읽히는 문장을 쓰지 않기", guideline.ScopeTemplates, []string{review.ID}, candidates[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if created.Scope != guideline.ScopeTemplates || len(created.TemplateIDs) != 1 {
		t.Fatalf("approved with scope %+v", created)
	}
	candidates, full, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || full {
		t.Fatalf("after approval: candidates=%d full=%v", len(candidates), full)
	}

	// A3: the approved instruction is never recorded again, and neither is a saved guideline's
	// text — recording resumed (the queue has room), so this proves the dedupe and not the cap.
	if err := recorder.Record(ctx, "alice", saved.Slug, instruction); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(ctx, "alice", saved.Slug, "광고처럼 읽히는 문장을 쓰지 않기"); err != nil {
		t.Fatal(err)
	}
	candidates, _, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Text != "존댓말로 써줘" {
		t.Fatalf("a ruled-on instruction came back: %+v", candidates)
	}

	// A4: with candidates recorded and only the approved guideline saved, the prompt carries the
	// guideline and nothing else — no candidate text reaches it.
	texts, err := generationGuidelines{service: guidelineSvc}.ForPrompt(ctx, "alice", &review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0] != "광고처럼 읽히는 문장을 쓰지 않기" {
		t.Fatalf("prompt guidelines = %v", texts)
	}
	system, _ := generation.BuildWritePrompt(generation.Profile{}, nil, "", "", nil, nil, nil, texts)
	for _, candidateText := range []string{instruction, "존댓말로 써줘", "문단을 짧게"} {
		if strings.Contains(system, candidateText) {
			t.Fatalf("candidate text %q reached the prompt:\n%s", candidateText, system)
		}
	}

	// A11: deleting the source post leaves the candidate with its text and no link.
	if err := postSvc.DeletePost(ctx, "alice", saved.Slug); err != nil {
		t.Fatal(err)
	}
	candidates, _, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Text != "존댓말로 써줘" || candidates[0].PostSlug != "" {
		t.Fatalf("after the post delete: %+v", candidates)
	}

	// A11 second half: deleting the guideline neither revives nor resets its approved candidate.
	if err := guidelineSvc.Delete(ctx, "alice", created.ID); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(ctx, "alice", saved.Slug, instruction); err != nil {
		t.Fatal(err)
	}
	candidates, _, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Text != "존댓말로 써줘" {
		t.Fatalf("deleting the guideline revived its candidate: %+v", candidates)
	}

	// A6: 무시 marks the row, and a later revision does not bring it back.
	if err := guidelineSvc.DismissCandidate(ctx, "alice", candidates[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Record(ctx, "alice", saved.Slug, "존댓말로 써줘"); err != nil {
		t.Fatal(err)
	}
	candidates, _, err = guidelineSvc.ListCandidates(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("a dismissed instruction reappeared: %+v", candidates)
	}
}

type noPurge struct{}

func (noPurge) PurgePost(context.Context, string, string) error { return nil }

type noLivePublish struct{}

func (noLivePublish) LiveForPost(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}
