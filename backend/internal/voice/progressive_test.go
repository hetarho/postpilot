package voice_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

func TestPromptProfileProjectionKeepsSourceSpecificEvidenceOutOfCrossLanguagePrompts(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	voiceID := h.voice("alice")
	h.svc.ConfigurePersonalization(voice.PersonalizationThresholds())
	measured := func(value string) voice.VoiceValue {
		return voice.VoiceValue{Value: value, Source: voice.SourceMeasured}
	}
	axis := func(value int) *int { return &value }
	structured := voice.StructuredProfile{
		Lexical: voice.LexicalProfile{
			Description: measured("LEXICAL_SECRET"),
			BannedWords: []voice.BannedItem{{Value: "BANNED_WORD_SECRET", Reason: "private lexical rule"}},
		},
		Endings: voice.EndingsProfile{
			BaseRegister: measured("ENDING_REGISTER_SECRET"), Distribution: []voice.EndingRatio{{Ending: "ENDING_SECRET", Ratio: 1}},
			BannedEndings: []string{"BANNED_ENDING_SECRET"},
		},
		Syntax: voice.SyntaxProfile{AverageSentenceChars: 37, ConnectiveStyle: measured("SYNTAX_SECRET")},
		Structure: voice.StructureProfile{
			IntroPattern: measured("PORTABLE_INTRO"), ClosingPattern: measured("PORTABLE_CLOSE"),
			ParagraphSentencesMin: 2, ParagraphSentencesMax: 4, HeadingHabit: measured("PORTABLE_HEADINGS"),
			ListHabit: measured("PORTABLE_LISTS"), EmojiUse: measured("PORTABLE_EMOJIS"),
		},
		Axes: voice.AxesProfile{Involvement: axis(1), Narrativity: axis(2), PersuasionOvertness: axis(3), Abstractness: axis(4), AddresseeFocus: axis(5), Humor: axis(6)},
	}
	if _, err := h.store.PublishProfileVersion(ctx, "alice", voiceID, structured, "analysis", 0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	h.addSample(t, "alice", voiceID, "portable-sample", "sample", strings.Repeat("EXCERPT_SECRET ", 80), time.Now().UTC())

	full, err := h.svc.PromptProfileForLanguage(ctx, "alice", voiceID, voice.LanguageKorean)
	if err != nil {
		t.Fatal(err)
	}
	if full.Portable || full.SourceLanguage != voice.LanguageKorean || full.TargetLanguage != voice.LanguageKorean {
		t.Fatalf("full tags = %+v", full)
	}
	for _, required := range []string{"LEXICAL_SECRET", "ENDING_REGISTER_SECRET", "SYNTAX_SECRET"} {
		if !strings.Contains(full.Styleguide, required) {
			t.Errorf("full projection missing %q: %+v", required, full)
		}
	}
	if len(full.Excerpts) == 0 || !strings.Contains(full.Excerpts[0], "EXCERPT_SECRET") {
		t.Fatalf("full excerpts = %q", full.Excerpts)
	}

	portable, err := h.svc.PromptProfileForLanguage(ctx, "alice", voiceID, voice.LanguageEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if !portable.Portable || portable.SourceLanguage != voice.LanguageKorean || portable.TargetLanguage != voice.LanguageEnglish || len(portable.Excerpts) != 0 {
		t.Fatalf("portable tags/evidence = %+v", portable)
	}
	for _, required := range []string{"PORTABLE_INTRO", "PORTABLE_CLOSE", "PORTABLE_HEADINGS", "PORTABLE_LISTS", "PORTABLE_EMOJIS", "paragraph sentences: 2-4", "involvement=1", "humor=6"} {
		if !strings.Contains(portable.Styleguide, required) {
			t.Errorf("portable projection missing %q: %s", required, portable.Styleguide)
		}
	}
	for _, forbidden := range []string{"LEXICAL_SECRET", "BANNED_WORD_SECRET", "ENDING_REGISTER_SECRET", "ENDING_SECRET", "BANNED_ENDING_SECRET", "SYNTAX_SECRET", "EXCERPT_SECRET"} {
		if strings.Contains(portable.Styleguide, forbidden) {
			t.Errorf("portable projection leaked %q: %s", forbidden, portable.Styleguide)
		}
	}
}

func TestManualOverrideClearAndRestorePublishImmutableWholeVersions(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	base := voice.StructuredProfile{Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: "분석값", Source: voice.SourceAnalyzed}}}
	if _, err := h.store.PublishProfileVersion(context.Background(), "alice", alice, base, "analysis", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	manual := "직접 정한 값"
	profile, err := h.svc.UpdateOverride(context.Background(), "alice", alice, voice.LayerLexical, "description", &manual)
	if err != nil || profile.Structured.Version != 2 || profile.Structured.Lexical.Description.Value != manual || profile.Structured.Lexical.Description.Source != voice.SourceManual {
		t.Fatalf("manual profile=%+v err=%v", profile.Structured, err)
	}
	profile, err = h.svc.UpdateOverride(context.Background(), "alice", alice, voice.LayerLexical, "description", nil)
	if err != nil || profile.Structured.Version != 3 || profile.Structured.Lexical.Description.Value != "분석값" {
		t.Fatalf("cleared profile=%+v err=%v", profile.Structured, err)
	}
	profile, err = h.svc.RestoreVersion(context.Background(), "alice", alice, 2)
	if err != nil || profile.Structured.Version != 4 || profile.Structured.Lexical.Description.Value != manual {
		t.Fatalf("restored profile=%+v err=%v", profile.Structured, err)
	}
	versions, err := h.svc.ListVersions(context.Background(), "alice", alice)
	if err != nil || len(versions) != 4 || versions[0].Origin != "restore" || versions[0].RestoredFromVersion != 2 || versions[3].Profile.Lexical.Description.Value != "분석값" {
		t.Fatalf("versions=%+v err=%v", versions, err)
	}
	// Version numbers count per voice: a sibling voice starts at v1 and sees none of these.
	other, _, _ := h.svc.CreateVoice(context.Background(), "alice", "다른 말투", voice.LanguageKorean, nil)
	if otherVersions, err := h.svc.ListVersions(context.Background(), "alice", other.ID); err != nil || len(otherVersions) != 0 {
		t.Fatalf("other voice versions=%+v err=%v", otherVersions, err)
	}
	if _, err := h.svc.RestoreVersion(context.Background(), "alice", other.ID, 2); !errors.Is(err, voice.ErrLearningNotFound) {
		t.Fatalf("cross-voice restore = %v", err)
	}
}

// VOICE-28: an analysis replays the overrides into the snapshot it publishes, and clearing one
// afterwards still returns its field to what the analysis said — the baked override is stripped
// before the remaining ones are replayed, so 직접 설정 해제 is never a no-op.
func TestClearingAnOverrideAfterAnAnalysisRestoresTheAnalyzedValue(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	manual := "직접 정한 값"
	if _, err := h.svc.UpdateOverride(context.Background(), "alice", alice, voice.LayerLexical, "description", &manual); err != nil {
		t.Fatal(err)
	}
	guide := "## 1. 종결어미 분포\n해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"
	h.models.response = analysisAnswer(guide)
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	analyzed, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || analyzed.Structured.Lexical.Description.Value != manual || analyzed.Structured.Lexical.Description.Source != voice.SourceManual {
		t.Fatalf("the analysis did not replay the override: %+v err=%v", analyzed.Structured.Lexical.Description, err)
	}
	cleared, err := h.svc.UpdateOverride(context.Background(), "alice", alice, voice.LayerLexical, "description", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := cleared.Structured.Lexical.Description; got.Value != guide || got.Source != voice.SourceAnalyzed {
		t.Fatalf("cleared description = %+v, want the analysis's own value", got)
	}
}

// A response that never mentions the axes must publish them as unknown, not as six neutral
// zeros — the fixture above hand-writes the exact key shape, which is how a prompt that never
// named that shape passed CI while a live account showed 0 everywhere.
func TestAnalysisPublishesUnansweredAxesAsUnknownAndRejectsOutOfRange(t *testing.T) {
	analyzeWith := func(t *testing.T, response string, structured bool) (voice.Profile, llm.Request, error) {
		t.Helper()
		h := newVoiceHarness(t)
		alice := h.voice("alice")
		h.addSample(t, "alice", alice, "axes", "축", longSample("걸"), time.Now())
		h.models.response = response
		h.models.structured = structured
		err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {})
		profile, getErr := h.svc.Get(context.Background(), "alice", alice)
		if getErr != nil {
			t.Fatal(getErr)
		}
		return profile, h.models.request, err
	}
	strings8 := `"lexical_description":"1. 종결어미 분포: 해요\n8. 절대 사용하지 않는 표현 (never uses): 과장","base_register":"해요","connective_style":"","intro_pattern":"","closing_pattern":"","heading_habit":"","list_habit":"","emoji_use":""`

	t.Run("omitted axes publish as unknown", func(t *testing.T) {
		profile, request, err := analyzeWith(t, `{`+strings8+`}`, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, axis := range profile.Structured.Axes.AxisValues() {
			if axis.Value != nil {
				t.Fatalf("axis %s published %d for a response that never answered it", axis.Key, *axis.Value)
			}
		}
		if request.JSONSchema != nil {
			t.Fatal("schema attached to a model without structured output")
		}
		for _, key := range []string{`"axes"`, "involvement", "narrativity", "persuasion_overtness", "abstractness", "addressee_focus", "humor"} {
			if !strings.Contains(request.System, key) {
				t.Fatalf("prompt does not name %s", key)
			}
		}
	})
	t.Run("partially answered axes keep the answered values", func(t *testing.T) {
		profile, _, err := analyzeWith(t, `{`+strings8+`,"axes":{"involvement":2,"humor":-1}}`, false)
		if err != nil {
			t.Fatal(err)
		}
		axes := profile.Structured.Axes
		if axes.Involvement == nil || *axes.Involvement != 2 || axes.Humor == nil || *axes.Humor != -1 || axes.Narrativity != nil {
			t.Fatalf("axes = %+v", axes)
		}
	})
	t.Run("out-of-range axis still fails the job", func(t *testing.T) {
		profile, _, err := analyzeWith(t, `{`+strings8+`,"axes":{"involvement":4}}`, false)
		if err == nil || !strings.Contains(err.Error(), "-3..3") {
			t.Fatalf("expected range error, got %v", err)
		}
		if profile.Structured.Version != 0 {
			t.Fatalf("a rejected analysis published version %d", profile.Structured.Version)
		}
	})
	t.Run("structured-output model receives the schema", func(t *testing.T) {
		_, request, err := analyzeWith(t, `{`+strings8+`,"axes":{"involvement":0,"narrativity":0,"persuasion_overtness":0,"abstractness":0,"addressee_focus":0,"humor":0}}`, true)
		if err != nil {
			t.Fatal(err)
		}
		if string(request.JSONSchema) != string(voice.VoiceAnalysisSchema()) {
			t.Fatal("structured-output model did not receive the voice analysis schema")
		}
		var schema struct {
			Required   []string `json:"required"`
			Properties struct {
				Axes struct {
					Required []string `json:"required"`
				} `json:"axes"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(request.JSONSchema, &schema); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(schema.Required, "axes") || len(schema.Properties.Axes.Required) != 6 {
			t.Fatalf("schema does not require axes and its six keys: %+v", schema)
		}
	})
}

// VOICE-27, review F95: analyze_voice is the analysis call. It attaches the schema when the
// model declares structured output, so a voice made from pasted samples gets its axes and
// structure habits, with the nine-section guide as its lexical description.
func TestAnImportOnlyAnalysisCarriesTheTypedDescriptors(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	h.models.structured = true
	h.models.response = `{"lexical_description":"1. 종결어미 분포: 해요\n8. 절대 사용하지 않는 표현 (never uses): 과장","base_register":"해요","connective_style":"그래서","intro_pattern":"바로 시작","closing_pattern":"질문으로 마침","heading_habit":"소제목 없음","list_habit":"목록 드묾","emoji_use":"안 씀","axes":{"involvement":2,"narrativity":1,"persuasion_overtness":0,"abstractness":-1,"addressee_focus":1,"humor":0}}`
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if h.models.request.JSONSchema == nil {
		t.Fatal("the analysis call sent no schema to a structured-output model")
	}
	profile, err := h.svc.Get(context.Background(), "alice", alice)
	if err != nil {
		t.Fatal(err)
	}
	p := profile.Structured
	if p.Axes.Involvement == nil || *p.Axes.Involvement != 2 || p.Structure.IntroPattern.Value != "바로 시작" || p.Structure.ClosingPattern.Value != "질문으로 마침" ||
		p.Structure.HeadingHabit.Value != "소제목 없음" || p.Structure.ListHabit.Value != "목록 드묾" || p.Structure.EmojiUse.Value != "안 씀" {
		t.Fatalf("typed descriptors = axes %+v structure %+v", p.Axes, p.Structure)
	}
	if !strings.HasPrefix(p.Lexical.Description.Value, "1. 종결어미 분포") {
		t.Fatalf("lexical description = %q, want the nine-section guide", p.Lexical.Description.Value)
	}
}

func insertPost(t *testing.T, h *voiceHarness, slug, user, voiceID, title, now string) {
	t.Helper()
	if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES(?,?,?,?,'','review',?,?)", slug, user, voiceID, title, now, now); err != nil {
		t.Fatal(err)
	}
}

// VOICE-46: the projection's excerpts come from the pasted samples alone, newest first by
// (created_at, id) and at most FewShotMax of them; nothing a finished post held reaches it.
func TestProjectionExcerptsComeFromPastedSamplesNewestFirst(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	h.svc.ConfigurePersonalization(voice.PersonalizationThresholds())
	base := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	for i, char := range []string{"가", "나", "다", "라"} {
		h.addSample(t, "alice", alice, "sample-"+char, char, longSample(char), base.Add(time.Duration(i)*time.Hour))
	}
	profile, err := h.svc.PromptProfileForLanguage(ctx, "alice", alice, voice.LanguageKorean)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Empty {
		t.Fatal("a voice with samples projected as empty")
	}
	want := []string{longSample("라"), longSample("다"), longSample("나")}
	if !slices.Equal(profile.Excerpts, want) {
		t.Fatalf("excerpts = %q, want the three newest samples, newest first", profile.Excerpts)
	}
	other, _, err := h.svc.CreateVoice(ctx, "alice", "빈 말투", voice.LanguageKorean, nil)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := h.svc.PromptProfileForLanguage(ctx, "alice", other.ID, voice.LanguageKorean)
	if err != nil || !empty.Empty || len(empty.Excerpts) != 0 {
		t.Fatalf("a voice with no sample and no version projected %+v err=%v", empty, err)
	}
}
