package voice_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

const seededStyleguide = "## 1. 종결어미 분포\n농담조 해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"

func describedVoice(description string) *voice.VoiceSeed {
	return &voice.VoiceSeed{Description: description, AnalyzeModel: analyzeRef}
}

// --- creation (change 14 A6, A7, A12, A13) ---

func TestCreateVoiceWithoutADescriptionStartsNoWork(t *testing.T) {
	h := newVoiceHarness(t)
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, jobID, err := h.svc.CreateVoice(context.Background(), "alice", "요리", voice.LanguageKorean, nil)
	if err != nil || created.ID == "" || jobID != "" {
		t.Fatalf("plain creation = %+v job=%q err=%v", created, jobID, err)
	}
	if len(h.jobs.personalizationCalls) != 0 || h.models.completeCalls != 0 {
		t.Fatalf("plain creation reached the queue or a provider: %+v", h.jobs.personalizationCalls)
	}
}

func TestCreateVoiceWithADescriptionEnqueuesOneSeedJob(t *testing.T) {
	h := newVoiceHarness(t)
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, jobID, err := h.svc.CreateVoice(
		context.Background(), "alice", "요리", voice.LanguageKorean, describedVoice("  단순하고 농담조인 요리 말투  "),
	)
	if err != nil || jobID != "job-new" {
		t.Fatalf("described creation = %+v job=%q err=%v", created, jobID, err)
	}
	if len(h.jobs.personalizationCalls) != 1 {
		t.Fatalf("seed enqueues = %+v", h.jobs.personalizationCalls)
	}
	enqueued := h.jobs.personalizationCalls[0]
	if enqueued.Kind != voice.SeedJobKind || enqueued.VoiceID != created.ID || enqueued.Model != analyzeRef.String() {
		t.Fatalf("seed job = %+v", enqueued)
	}
	// Trimmed on the way in, so the prompt never carries the field's stray whitespace.
	if enqueued.Payload != "단순하고 농담조인 요리 말투" {
		t.Fatalf("seed payload = %q", enqueued.Payload)
	}
	// Enqueue only: the RPC must not block on a provider ([I5]).
	if h.models.completeCalls != 0 {
		t.Fatal("creation called a provider")
	}
}

func TestCreateVoiceRefusesABadDescriptionBeforeInsertingAnything(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	before, _ := h.svc.ListVoices(ctx, "alice")

	var tooLong *voice.VoiceDescriptionTooLongError
	over := strings.Repeat("가", voice.VoiceDescriptionMaxChars+1)
	if _, _, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice(over)); !errors.As(err, &tooLong) ||
		tooLong.Chars != voice.VoiceDescriptionMaxChars+1 {
		t.Fatalf("over-long description = %v", err)
	}

	// No analyze selection for this account: the description cannot be honored, so the whole
	// creation is refused rather than producing a silently unseeded voice.
	h.models.selected = map[string]llm.ModelRef{}
	if _, _, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice("농담조")); !errors.Is(err, voice.ErrAnalyzeModelRequired) {
		t.Fatalf("description without an analyze model = %v", err)
	}

	after, _ := h.svc.ListVoices(ctx, "alice")
	if len(after) != len(before) {
		t.Fatalf("a refused creation left a voice behind: %d -> %d", len(before), len(after))
	}
	if len(h.jobs.personalizationCalls) != 0 {
		t.Fatalf("a refused creation reached the queue: %+v", h.jobs.personalizationCalls)
	}
}

// --- the seeding run (change 14 A8, A9, A10, A11, A14) ---

func TestSeedPublishesAFirstProfileWithNoMeasurements(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, _, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice("농담조 요리 말투"))
	if err != nil {
		t.Fatal(err)
	}
	h.models.response = seededStyleguide

	var stages [][3]any
	if err := h.svc.Seed(ctx, voice.SeedJob{
		UserID: "alice", VoiceID: created.ID, Description: "농담조 요리 말투", WriteModel: analyzeRef.String(),
	}, func(stage string, done, total int) { stages = append(stages, [3]any{stage, done, total}) }); err != nil {
		t.Fatal(err)
	}
	if h.models.completeCalls != 1 {
		t.Fatalf("provider calls = %d", h.models.completeCalls)
	}
	// The description is the whole user message, and it is a request rather than prose, so the
	// prompt that frames it must be the seeding one.
	if got := h.models.request.Messages[0].Parts[0].Text; got != "농담조 요리 말투" {
		t.Fatalf("seed input = %q", got)
	}
	if !strings.Contains(h.models.request.System, "문체 설계자") {
		t.Fatalf("seed system prompt = %q", h.models.request.System)
	}
	if len(stages) == 0 || stages[len(stages)-1] != [3]any{"seed", 1, 1} {
		t.Fatalf("seed progress = %+v", stages)
	}

	profile, err := h.store.GetProfile(ctx, "alice", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Styleguide != seededStyleguide {
		t.Fatalf("styleguide = %q", profile.Styleguide)
	}
	versions, err := h.store.ListProfileVersions(ctx, "alice", created.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions = %+v err=%v", versions, err)
	}
	head := versions[0]
	if head.Version != 1 || head.Origin != "seed" {
		t.Fatalf("head = %+v", head)
	}
	if head.Profile.Lexical.Description.Source != voice.SourceAnalyzed || head.Profile.Lexical.Description.Value != seededStyleguide {
		t.Fatalf("seeded description = %+v", head.Profile.Lexical.Description)
	}
	// A description is an instruction, not a corpus: nothing may be reported as measured.
	if head.Profile.Syntax.AverageSentenceChars != 0 || len(head.Profile.Endings.Distribution) != 0 {
		t.Fatalf("seed reported measurements: %+v %+v", head.Profile.Syntax, head.Profile.Endings)
	}
	if head.Profile.Axes.Involvement != nil || head.Profile.Axes.Humor != nil {
		t.Fatalf("seed reported axes: %+v", head.Profile.Axes)
	}
	if head.Profile.SourceCount != 0 {
		t.Fatalf("seed counted a source: %d", head.Profile.SourceCount)
	}

	// Nothing evidence-shaped was created, so the voice is still empty for every later path.
	live, err := h.svc.Get(ctx, "alice", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(live.Samples) != 0 || live.SourceCount != 0 || live.CanValidate {
		t.Fatalf("seeding produced evidence: samples=%d sources=%d canValidate=%v", len(live.Samples), live.SourceCount, live.CanValidate)
	}
}

func TestSeededProfileNeverStatesAMeasurementItDoesNotHave(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, _, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice("농담조"))
	if err != nil {
		t.Fatal(err)
	}
	h.models.response = seededStyleguide
	if err := h.svc.Seed(ctx, voice.SeedJob{
		UserID: "alice", VoiceID: created.ID, Description: "농담조", WriteModel: analyzeRef.String(),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}

	// The published version is real (v1, not empty), so the projection renders it. A zero it
	// never measured must read as unknown there, not as a fact the write prompt will obey.
	projection, err := h.svc.PromptProfileForLanguage(ctx, "alice", created.ID, voice.LanguageKorean)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(projection.Styleguide, "average sentence: 0.00") ||
		strings.Contains(projection.Styleguide, "paragraph sentences: 0-0") {
		t.Fatalf("seeded projection states unmeasured numbers:\n%s", projection.Styleguide)
	}
	if !strings.Contains(projection.Styleguide, "average sentence: unknown") {
		t.Fatalf("seeded projection = %s", projection.Styleguide)
	}
	// Portable (cross-language) projection has its own allowlist and the same rule.
	portable, err := h.svc.PromptProfileForLanguage(ctx, "alice", created.ID, voice.LanguageEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(portable.Styleguide, "paragraph sentences: 0-0") {
		t.Fatalf("portable seeded projection states unmeasured numbers:\n%s", portable.Styleguide)
	}
}

func TestSeedStandsDownWhenRealEvidenceAlreadyPublished(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, _, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice("농담조"))
	if err != nil {
		t.Fatal(err)
	}
	analyzed := voice.StructuredProfile{
		Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: "real analysis", Source: voice.SourceAnalyzed}},
		Syntax:  voice.SyntaxProfile{AverageSentenceChars: 42},
	}
	if _, err := h.store.PublishProfileVersion(ctx, "alice", created.ID, analyzed, "analysis", 0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	h.models.response = seededStyleguide

	if err := h.svc.Seed(ctx, voice.SeedJob{
		UserID: "alice", VoiceID: created.ID, Description: "농담조", WriteModel: analyzeRef.String(),
	}, func(string, int, int) {}); err != nil {
		t.Fatalf("a late seed must finish quietly, not fail: %v", err)
	}
	if h.models.completeCalls != 0 {
		t.Fatal("a late seed spent a provider call")
	}
	versions, _ := h.store.ListProfileVersions(ctx, "alice", created.ID)
	if len(versions) != 1 || versions[0].Origin != "analysis" || versions[0].Profile.Syntax.AverageSentenceChars != 42 {
		t.Fatalf("a late seed overwrote real work: %+v", versions)
	}
}

func TestSeedRefusesADeletedVoiceAndKeepsFailuresOffTheProfile(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, _, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice("농담조"))
	if err != nil {
		t.Fatal(err)
	}

	// A provider that answers with the wrong shape must leave the voice exactly as created.
	h.models.response = "설명만 있고 섹션이 없어요"
	if err := h.svc.Seed(ctx, voice.SeedJob{
		UserID: "alice", VoiceID: created.ID, Description: "농담조", WriteModel: analyzeRef.String(),
	}, func(string, int, int) {}); err == nil || !strings.Contains(err.Error(), "종결어미") {
		t.Fatalf("bad shape = %v", err)
	}
	profile, _ := h.store.GetProfile(ctx, "alice", created.ID)
	if profile.Styleguide != "" || profile.Structured.Version != 0 {
		t.Fatalf("a failed seed wrote a partial profile: %+v", profile)
	}

	if _, err := h.svc.DeleteVoice(ctx, "alice", created.ID); err != nil {
		t.Fatal(err)
	}
	h.models.response = seededStyleguide
	before := h.models.completeCalls
	if err := h.svc.Seed(ctx, voice.SeedJob{
		UserID: "alice", VoiceID: created.ID, Description: "농담조", WriteModel: analyzeRef.String(),
	}, func(string, int, int) {}); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("seed for a deleted voice = %v", err)
	}
	if h.models.completeCalls != before {
		t.Fatal("a deleted voice reached a provider")
	}
}

func TestSeedingWorkIsVoiceOwnedForTheDeleteGuard(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	created, jobID, err := h.svc.CreateVoice(ctx, "alice", "요리", voice.LanguageKorean, describedVoice("농담조"))
	if err != nil || jobID == "" {
		t.Fatalf("described creation = %v job=%q", err, jobID)
	}
	// The queue reports the seed as this voice's active work, which is what DeleteVoice asks.
	h.jobs.active[created.ID] = &voice.ActiveJob{ID: jobID}
	if _, err := h.svc.DeleteVoice(ctx, "alice", created.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete while seeding = %v", err)
	}
	// The same run is what the profile screen shows progress for.
	profile, err := h.svc.Get(ctx, "alice", created.ID)
	if err != nil || profile.ActiveJobID != jobID {
		t.Fatalf("active job on the profile = %q err=%v", profile.ActiveJobID, err)
	}
}
