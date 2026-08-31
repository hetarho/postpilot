package voice_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/voice"
)

func TestPostLearningLanguageMismatchHasNoDurableQueueOrProviderSideEffects(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	voiceID := h.voice("alice")
	now := time.Now().UTC()
	insertPost(t, h, "english-post", "alice", voiceID, "English", now.Format(time.RFC3339Nano))
	raw := `{"title":"English","summary":"","tags":[],"blocks":[{"type":"TEXT","content":"A final English post."}]}`
	snapshot := voice.FinalizationInput{
		PostSlug: "english-post", UserID: "alice", VoiceID: voiceID, BaselineVoiceID: voiceID,
		BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1,
		ContentLanguage: voice.LanguageEnglish, VoiceSourceLanguage: voice.LanguageKorean,
	}
	h.svc.ConfigurePersonalization(learningPosts{snapshot: snapshot}, personalizationConfig())
	before := voiceStateCounts(t, h)
	if _, _, _, err := h.svc.LearnFromFinalizedPost(ctx, "alice", "english-post", analyzeRef); !errors.Is(err, voice.ErrContentLanguageMismatch) {
		t.Fatalf("finalize mismatch = %v", err)
	}
	if _, err := h.svc.GiveFeedback(ctx, "alice", "english-post", "sentence-1", "ending", "A final English post.", false); !errors.Is(err, voice.ErrContentLanguageMismatch) {
		t.Fatalf("feedback mismatch = %v", err)
	}
	assertVoiceStateUnchanged(t, h, before)

	// A durable retry created by an older binary is revalidated before its status,
	// job link, queue, or provider can change.
	event := voice.LearningEvent{
		ID: "mismatched-event", UserID: "alice", VoiceID: voiceID, PostSlug: "english-post",
		BaselineRevision: 1, InputHash: "mismatched", BaselineJSON: raw, FinalJSON: raw,
		ModelRef: analyzeRef.String(), Status: "retryable", JobID: "old-job", CreatedAt: now,
		ContentLanguage: voice.LanguageEnglish, SourceLanguage: voice.LanguageKorean,
	}
	if err := h.store.InsertLearningEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	before = voiceStateCounts(t, h)
	if _, _, err := h.svc.RetryLearning(ctx, "alice", event.ID, analyzeRef); !errors.Is(err, voice.ErrContentLanguageMismatch) {
		t.Fatalf("retry mismatch = %v", err)
	}
	if err := h.svc.Learn(ctx, voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); !errors.Is(err, voice.ErrContentLanguageMismatch) {
		t.Fatalf("worker mismatch = %v", err)
	}
	assertVoiceStateUnchanged(t, h, before)
	stored, err := h.svc.GetLearningEvent(ctx, "alice", event.ID)
	if err != nil || stored.Status != "retryable" || stored.JobID != "old-job" {
		t.Fatalf("mismatched event mutated = %#v, err=%v", stored, err)
	}
}

func TestPostLearningLanguageMismatchIsSymmetricAndSideEffectFree(t *testing.T) {
	for _, test := range []struct {
		name    string
		content voice.Language
		source  voice.Language
	}{
		{name: "English content Korean voice", content: voice.LanguageEnglish, source: voice.LanguageKorean},
		{name: "Korean content English voice", content: voice.LanguageKorean, source: voice.LanguageEnglish},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newVoiceHarness(t)
			ctx := context.Background()
			voiceID := h.voice("alice")
			if test.source == voice.LanguageEnglish {
				created, err := h.svc.CreateVoice(ctx, "alice", "English", voice.LanguageEnglish)
				if err != nil {
					t.Fatal(err)
				}
				voiceID = created.ID
			}
			now := time.Now().UTC()
			insertPost(t, h, "symmetric", "alice", voiceID, "Mismatch", now.Format(time.RFC3339Nano))
			raw := `{"title":"Mismatch","summary":"","tags":[],"blocks":[{"type":"TEXT","content":"Language mismatch."}]}`
			h.svc.ConfigurePersonalization(learningPosts{snapshot: voice.FinalizationInput{
				PostSlug: "symmetric", UserID: "alice", VoiceID: voiceID, BaselineVoiceID: voiceID,
				BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1,
				ContentLanguage: test.content, VoiceSourceLanguage: test.source,
			}}, personalizationConfig())
			before := voiceStateCounts(t, h)
			if _, _, _, err := h.svc.LearnFromFinalizedPost(ctx, "alice", "symmetric", analyzeRef); !errors.Is(err, voice.ErrContentLanguageMismatch) {
				t.Fatalf("finalize mismatch = %v", err)
			}
			if _, err := h.svc.GiveFeedback(ctx, "alice", "symmetric", "s1", "ending", "Language mismatch.", false); !errors.Is(err, voice.ErrContentLanguageMismatch) {
				t.Fatalf("feedback mismatch = %v", err)
			}
			assertVoiceStateUnchanged(t, h, before)
		})
	}
}

func TestRuleComparisonAndValidationExcludeMixedLanguageSourcesBeforeWrites(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	voiceID := h.voice("alice")
	now := time.Now().UTC()
	if _, err := h.store.PublishProfileVersion(ctx, "alice", voiceID, voice.StructuredProfile{}, "analysis", 0, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(
		"INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('language-rule','alice',?,'I use concise sentences','language-rule','syntax',1,'candidate','manual',?,?)",
		voiceID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}
	insertAuthoredSourceForLanguage(t, h, voiceID, "ko-a", voice.LanguageKorean, now)
	insertAuthoredSourceForLanguage(t, h, voiceID, "ko-b", voice.LanguageKorean, now.Add(time.Second))
	insertAuthoredSourceForLanguage(t, h, voiceID, "en-mixed", voice.LanguageEnglish, now.Add(2*time.Second))
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())

	before := voiceStateCounts(t, h)
	if _, _, err := h.svc.StartRuleComparison(ctx, "alice", "language-rule", "source-en-mixed", nil, analyzeRef); !errors.Is(err, voice.ErrContentLanguageMismatch) {
		t.Fatalf("comparison mismatch = %v", err)
	}
	if _, _, err := h.svc.StartValidation(ctx, "alice", voiceID, analyzeRef, analyzeRef, false); !errors.Is(err, voice.ErrInsufficientSources) {
		t.Fatalf("mixed source counted toward validation = %v", err)
	}
	assertVoiceStateUnchanged(t, h, before)

	insertAuthoredSourceForLanguage(t, h, voiceID, "ko-c", voice.LanguageKorean, now.Add(3*time.Second))
	validationID, _, err := h.svc.StartValidation(ctx, "alice", voiceID, analyzeRef, analyzeRef, false)
	if err != nil {
		t.Fatal(err)
	}
	validation, err := h.svc.GetValidation(ctx, "alice", validationID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]bool, len(validation.Items))
	for _, item := range validation.Items {
		ids[item.SourceID] = true
	}
	if len(ids) != 3 || ids["source-en-mixed"] || !ids["source-ko-a"] || !ids["source-ko-b"] || !ids["source-ko-c"] {
		t.Fatalf("validation sources = %#v", ids)
	}
	if h.models.completeCalls != 0 {
		t.Fatalf("validation start called provider %d times", h.models.completeCalls)
	}
}

func TestEnglishLearningSelectsEnglishCorpusPromptSchemaAndMeasurements(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	english, err := h.svc.CreateVoice(ctx, "alice", "English", voice.LanguageEnglish)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	// Simulate a legacy mixed-language row under the voice. It must not enter the
	// provider corpus or the profile's source count.
	insertAuthoredSourceForLanguage(t, h, english.ID, "ko-secret", voice.LanguageKorean, now)
	insertPost(t, h, "english-learn", "alice", english.ID, "English", now.Add(time.Second).Format(time.RFC3339Nano))
	raw := `{"title":"English","summary":"","tags":[],"blocks":[{"type":"TEXT","content":"However, we can't ignore this decision. Are you ready?"}]}`
	h.svc.ConfigurePersonalization(learningPosts{snapshot: voice.FinalizationInput{
		PostSlug: "english-learn", UserID: "alice", VoiceID: english.ID, BaselineVoiceID: english.ID,
		BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1,
		ContentLanguage: voice.LanguageEnglish, VoiceSourceLanguage: voice.LanguageEnglish,
	}}, personalizationConfig())
	h.models.structured = true
	h.models.response = `{"lexical_description":"clear vocabulary","base_register":"conversational","connective_style":"explicit","intro_pattern":"direct","closing_pattern":"question","heading_habit":"absent","list_habit":"absent","emoji_use":"absent","axes":{"involvement":1,"narrativity":0,"persuasion_overtness":1,"abstractness":0,"addressee_focus":2,"humor":0}}`
	event, _, _, err := h.svc.LearnFromFinalizedPost(ctx, "alice", "english-learn", analyzeRef)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Learn(ctx, voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.models.request.System, "English authored corpus") || strings.Contains(h.models.request.System, "Korean authored corpus") {
		t.Fatalf("English learning system prompt = %q", h.models.request.System)
	}
	if !bytes.Equal(h.models.request.JSONSchema, voice.VoiceAnalysisSchemaForLanguage(voice.LanguageEnglish)) {
		t.Fatal("English learning did not select the English response schema")
	}
	corpus := h.models.request.Messages[0].Parts[0].Text
	if strings.Contains(corpus, "ko-secret") || strings.Contains(corpus, "body ko-secret") {
		t.Fatalf("mixed-language source leaked into English corpus: %q", corpus)
	}
	profile, err := h.svc.Get(ctx, "alice", english.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.SourceCount != 1 || profile.Structured.Syntax.AverageSentenceWords == nil || profile.Structured.Endings.BaseRegister.Value == "" {
		t.Fatalf("English profile = %#v", profile.Structured)
	}
	for _, cadence := range profile.Structured.Endings.Distribution {
		if cadence.Ending != "statement" && cadence.Ending != "question" && cadence.Ending != "exclamation" && cadence.Ending != "fragment" {
			t.Fatalf("non-English cadence category = %#v", cadence)
		}
	}
}

func insertAuthoredSourceForLanguage(t *testing.T, h *voiceHarness, voiceID, suffix string, language voice.Language, now time.Time) {
	t.Helper()
	slug, eventID, sourceID := "post-"+suffix, "event-"+suffix, "source-"+suffix
	insertPost(t, h, slug, "alice", voiceID, suffix, now.Format(time.RFC3339Nano))
	event := voice.LearningEvent{
		ID: eventID, UserID: "alice", VoiceID: voiceID, PostSlug: slug, BaselineRevision: 1,
		InputHash: "hash-" + suffix, BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(),
		Status: "done", CreatedAt: now, ProcessedAt: &now, ContentLanguage: language, SourceLanguage: language,
	}
	if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(
		"INSERT INTO voice_authored_sources(id,user_id,voice_id,post_slug,learning_event_id,title,tags,body,excerpt,created_at) VALUES(?,'alice',?,?,?,?, '[]',?,?,?)",
		sourceID, voiceID, slug, eventID, suffix, "body "+suffix, "excerpt "+suffix, now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}
}

func voiceStateCounts(t *testing.T, h *voiceHarness) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, table := range []string{"voice_learning_events", "voice_authored_sources", "voice_sentence_feedback", "voice_rule_evidence", "voice_rule_comparisons", "voice_rule_comparison_candidates", "voice_profile_validations", "voice_profile_validation_items", "voice_profile_versions"} {
		var count int
		if err := h.db.Reader.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	counts["queue"] = len(h.jobs.personalizationCalls)
	counts["provider"] = h.models.completeCalls
	return counts
}

func assertVoiceStateUnchanged(t *testing.T, h *voiceHarness, before map[string]int) {
	t.Helper()
	if after := voiceStateCounts(t, h); !reflect.DeepEqual(after, before) {
		t.Fatalf("state changed on language refusal\nbefore=%#v\nafter=%#v", before, after)
	}
}
