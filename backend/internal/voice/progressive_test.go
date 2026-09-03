package voice_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

// learningPosts stands in for the post context: the snapshot names the post's current voice
// and the voice its machine baseline was written under, exactly as the real port does.
type learningPosts struct{ snapshot voice.FinalizationInput }

func (p learningPosts) LearningSnapshot(context.Context, string, string) (voice.FinalizationInput, error) {
	return p.snapshot, nil
}

func personalizationConfig() voice.PersonalizationConfig {
	return voice.PersonalizationConfig{FewShotTargetCount: 2, FewShotMax: 3, FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800, EmbeddingSwitchPosts: 50, DiffMaxRules: 3, DiffMinPatternEdits: 2, RuleActivationEvidence: 3, RuleRetireAfter: 180 * 24 * time.Hour, ValidationPostCount: 3, EndingMaxConsecutive: 2}
}

func TestPromptProfileProjectionKeepsSourceSpecificEvidenceOutOfCrossLanguagePrompts(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	voiceID := h.voice("alice")
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
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
	if err := h.svc.AppendRule(ctx, "alice", voiceID, "MANUAL_RULE_SECRET"); err != nil {
		t.Fatal(err)
	}
	h.addSample(t, "alice", voiceID, "portable-sample", "sample", strings.Repeat("EXCERPT_SECRET ", 80), time.Now().UTC())
	now := time.Now().UTC()
	if _, err := h.db.Writer.Exec(
		"INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)",
		"portable-rule", "alice", voiceID, "ACTIVE_RULE_SECRET", "portable-rule", string(voice.LayerLexical), 3, string(voice.RuleActive), "manual", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}

	full, err := h.svc.PromptProfileForLanguage(ctx, "alice", voiceID, voice.LanguageKorean)
	if err != nil {
		t.Fatal(err)
	}
	if full.Portable || full.SourceLanguage != voice.LanguageKorean || full.TargetLanguage != voice.LanguageKorean {
		t.Fatalf("full tags = %+v", full)
	}
	for _, required := range []string{"LEXICAL_SECRET", "ENDING_REGISTER_SECRET", "SYNTAX_SECRET", "MANUAL_RULE_SECRET", "ACTIVE_RULE_SECRET"} {
		if !strings.Contains(full.Styleguide+full.ManualRules+full.ActiveRules, required) {
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
	if !portable.Portable || portable.SourceLanguage != voice.LanguageKorean || portable.TargetLanguage != voice.LanguageEnglish || len(portable.Excerpts) != 0 || portable.ManualRules != "" || portable.ActiveRules != "" {
		t.Fatalf("portable tags/evidence = %+v", portable)
	}
	for _, required := range []string{"PORTABLE_INTRO", "PORTABLE_CLOSE", "PORTABLE_HEADINGS", "PORTABLE_LISTS", "PORTABLE_EMOJIS", "paragraph sentences: 2-4", "involvement=1", "humor=6"} {
		if !strings.Contains(portable.Styleguide, required) {
			t.Errorf("portable projection missing %q: %s", required, portable.Styleguide)
		}
	}
	for _, forbidden := range []string{"LEXICAL_SECRET", "BANNED_WORD_SECRET", "ENDING_REGISTER_SECRET", "ENDING_SECRET", "BANNED_ENDING_SECRET", "SYNTAX_SECRET", "MANUAL_RULE_SECRET", "ACTIVE_RULE_SECRET", "EXCERPT_SECRET"} {
		if strings.Contains(portable.Styleguide, forbidden) {
			t.Errorf("portable projection leaked %q: %s", forbidden, portable.Styleguide)
		}
	}
}

func insertPost(t *testing.T, h *voiceHarness, slug, user, voiceID, title, now string) {
	t.Helper()
	if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES(?,?,?,?,'','review',?,?)", slug, user, voiceID, title, now, now); err != nil {
		t.Fatal(err)
	}
}

type scriptedPersonalizationModels struct {
	mu        sync.Mutex
	responses []string
	calls     int
}

func (m *scriptedPersonalizationModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return analyzeRef, true, nil
}
func (m *scriptedPersonalizationModels) ModelEnabled(ref llm.ModelRef, _ string) bool {
	return ref.ProviderID != "" && ref.ModelID != ""
}
func (m *scriptedPersonalizationModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{}, false
}
func (m *scriptedPersonalizationModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.responses) == 0 {
		return llm.Response{}, nil
	}
	response := m.responses[0]
	m.responses = m.responses[1:]
	m.calls++
	return llm.Response{Text: response}, nil
}

func TestZeroHistoryFinalizeLearnsOneSourceOnlyAfterExplicitJob(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	insertPost(t, h, "first", "alice", alice, "첫 글", now)
	raw := `{"title":"첫 글","summary":"","tags":["산책"],"blocks":[{"type":"TEXT","content":"오늘은 천천히 걸어요. 바람이 참 좋아요."}]}`
	targetLength := 900
	snapshot := voice.FinalizationInput{PostSlug: "first", UserID: "alice", VoiceID: alice, BaselineVoiceID: alice, BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1, TargetLength: &targetLength, ContentLanguage: voice.LanguageKorean, VoiceSourceLanguage: voice.LanguageKorean}
	h.svc.ConfigurePersonalization(learningPosts{snapshot: snapshot}, personalizationConfig())
	h.models.response = `{"lexical_description":"담백한 어휘","base_register":"해요","connective_style":"짧은 연결","intro_pattern":"바로 시작","closing_pattern":"짧게 마침","heading_habit":"","list_habit":"","emoji_use":"","axes":{"involvement":1,"narrativity":1,"persuasion_overtness":0,"abstractness":0,"addressee_focus":0,"humor":0}}`

	event, jobID, reused, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "first", analyzeRef)
	if err != nil || reused || jobID != "job-new" || h.models.completeCalls != 0 || event.VoiceID != alice {
		t.Fatalf("finalize event=%+v job=%q reused=%v calls=%d err=%v", event, jobID, reused, h.models.completeCalls, err)
	}
	if calls := h.jobs.personalizationCalls; len(calls) != 1 || calls[0].VoiceID != alice || calls[0].Kind != voice.LearnJobKind {
		t.Fatalf("learn job did not freeze the voice: %+v", calls)
	}
	// A boot sweep fails the durable queued job but deliberately does not call a
	// provider. The next explicit Finalize action must recover that same immutable
	// learning event with a fresh job.
	h.jobs.personalizationActive[jobID] = false
	h.jobs.enqueueID = "job-after-restart"
	recovered, recoveredJobID, reused, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "first", analyzeRef)
	if err != nil || !reused || recovered.ID != event.ID || recoveredJobID != "job-after-restart" || h.models.completeCalls != 0 {
		t.Fatalf("restart recovery event=%+v job=%q reused=%v calls=%d err=%v", recovered, recoveredJobID, reused, h.models.completeCalls, err)
	}
	if err = h.svc.Learn(context.Background(), voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	profile, err := h.svc.Get(context.Background(), "alice", alice)
	if err != nil || profile.Structured.Version != 1 || profile.SourceCount != 1 || profile.Structured.Empty {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	_, excerpts, _, empty, err := h.svc.ProfileForPromptForTopic(context.Background(), "alice", alice, "다른 주제", nil)
	if err != nil || empty || len(excerpts) != 1 || !strings.Contains(excerpts[0], "천천히") {
		t.Fatalf("prompt excerpts=%v empty=%v err=%v", excerpts, empty, err)
	}
	// The source, version and excerpt belong to the learned voice alone.
	other, _, _ := h.svc.CreateVoice(context.Background(), "alice", "다른 말투", voice.LanguageKorean, nil)
	if otherProfile, err := h.svc.Get(context.Background(), "alice", other.ID); err != nil || otherProfile.SourceCount != 0 || otherProfile.Structured.Version != 0 {
		t.Fatalf("learning leaked into another voice: %+v err=%v", otherProfile, err)
	}
	calls := h.models.completeCalls
	if err = h.svc.Learn(context.Background(), voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil || h.models.completeCalls != calls {
		t.Fatalf("completed retry called provider: calls=%d→%d err=%v", calls, h.models.completeCalls, err)
	}
}

func TestContradictoryEvidenceLeavesProfileHeadAndActiveRuleUnchanged(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	nowTime := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	now := nowTime.Format(time.RFC3339Nano)
	insertPost(t, h, "second", "alice", alice, "둘째", now)
	rule := voice.ContrastRule{ID: "active-rule", UserID: "alice", VoiceID: alice, Statement: "LLM does formal endings, but I do polite endings", CanonicalKey: "active", Layer: voice.LayerEndings, EvidenceCount: 3, Status: voice.RuleActive, Origin: "diff", CreatedAt: nowTime, LastEvidenceAt: nowTime}
	baseProfile := voice.StructuredProfile{Rules: []voice.ContrastRule{rule}, SourceCount: 1}
	if _, err := h.store.PublishProfileVersion(context.Background(), "alice", alice, baseProfile, "analysis", 0, nowTime); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", rule.ID, rule.UserID, alice, rule.Statement, rule.CanonicalKey, string(rule.Layer), rule.EvidenceCount, string(rule.Status), rule.Origin, now, now); err != nil {
		t.Fatal(err)
	}
	event := voice.LearningEvent{ID: "event-contradiction", UserID: "alice", VoiceID: alice, PostSlug: "second", BaselineRevision: 1, InputHash: "hash", BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(), Status: "running", CreatedAt: nowTime, ContentLanguage: voice.LanguageKorean, SourceLanguage: voice.LanguageKorean}
	if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	result := voice.LearningResult{
		Source:  voice.AuthoredSource{ID: "source-contradiction", UserID: "alice", VoiceID: alice, PostSlug: "second", LearningEventID: event.ID, Title: "둘째", Body: "새 근거", Excerpt: "새 근거", CreatedAt: nowTime, SourceLanguage: voice.LanguageKorean},
		Profile: voice.StructuredProfile{SourceCount: 2},
		Rules:   []voice.ExtractedRule{{Statement: "LLM does polite endings, but I do formal endings", Layer: voice.LayerEndings, ContradictsRuleID: rule.ID}},
	}
	if err := h.store.ApplyLearningResult(context.Background(), event, result, voice.PersonalizationConfig{RuleActivationEvidence: 3}, nowTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || profile.Structured.Version != 1 {
		t.Fatalf("contradiction changed profile head: version=%d err=%v", profile.Structured.Version, err)
	}
	storedRule, err := h.store.GetRule(context.Background(), "alice", rule.ID)
	if err != nil || storedRule.Statement != rule.Statement || storedRule.Status != voice.RuleActive || storedRule.EvidenceCount != 3 || storedRule.VoiceID != alice {
		t.Fatalf("active rule changed: %+v err=%v", storedRule, err)
	}
	confirmations, err := h.store.ListConfirmations(context.Background(), "alice", alice)
	if err != nil || len(confirmations) != 1 || confirmations[0].Status != "pending" {
		t.Fatalf("confirmations=%+v err=%v", confirmations, err)
	}
	replaced, err := h.svc.ResolveConfirmation(context.Background(), "alice", confirmations[0].ID, true)
	if err != nil || len(replaced.Structured.Rules) != 1 {
		t.Fatalf("replacement profile=%+v err=%v", replaced.Structured, err)
	}
	replacedRule := replaced.Structured.Rules[0]
	if replacedRule.Statement != confirmations[0].ProposedStatement || replacedRule.Status != voice.RuleCandidate || replacedRule.EvidenceCount != 1 {
		t.Fatalf("replacement rule=%+v", replacedRule)
	}
	versions, err := h.store.ListProfileVersions(context.Background(), "alice", alice)
	if err != nil || len(versions) != 2 || len(versions[0].Profile.Rules) != 1 || versions[0].Profile.Rules[0] != replacedRule {
		t.Fatalf("replacement snapshot=%+v err=%v", versions, err)
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

func TestIndependentLearningEventsPromoteRuleAtConfiguredEvidenceCount(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	statement := "LLM does formal endings, but I do polite endings"
	for i := 1; i <= 3; i++ {
		nowTime := time.Date(2026, 8, 29, 12, i, 0, 0, time.UTC)
		now := nowTime.Format(time.RFC3339Nano)
		slug := "rule-post-" + string(rune('0'+i))
		insertPost(t, h, slug, "alice", alice, "규칙", now)
		event := voice.LearningEvent{ID: "rule-event-" + string(rune('0'+i)), UserID: "alice", VoiceID: alice, PostSlug: slug, BaselineRevision: 1, InputHash: "hash-" + slug, BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(), Status: "running", CreatedAt: nowTime, ContentLanguage: voice.LanguageKorean, SourceLanguage: voice.LanguageKorean}
		if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		result := voice.LearningResult{Source: voice.AuthoredSource{ID: "rule-source-" + string(rune('0'+i)), UserID: "alice", VoiceID: alice, PostSlug: slug, LearningEventID: event.ID, Title: slug, Body: slug, Excerpt: slug, CreatedAt: nowTime, SourceLanguage: voice.LanguageKorean}, Rules: []voice.ExtractedRule{{Statement: statement, Layer: voice.LayerEndings}}}
		if err := h.store.ApplyLearningResult(context.Background(), event, result, voice.PersonalizationConfig{RuleActivationEvidence: 3}, nowTime); err != nil {
			t.Fatal(err)
		}
		rules, err := h.store.ListRules(context.Background(), "alice", alice)
		if err != nil || len(rules) != 1 || rules[0].EvidenceCount != i {
			t.Fatalf("after event %d rules=%+v err=%v", i, rules, err)
		}
		want := voice.RuleCandidate
		if i == 3 {
			want = voice.RuleActive
		}
		if rules[0].Status != want {
			t.Fatalf("after event %d status=%s want=%s", i, rules[0].Status, want)
		}
	}
	_, _, projected, _, err := h.svc.ProfileForPrompt(context.Background(), "alice", alice)
	if err != nil || projected != statement {
		t.Fatalf("active projection=%q err=%v", projected, err)
	}
	// The same statement in a sibling voice is a different rule with its own evidence.
	other, _, _ := h.svc.CreateVoice(context.Background(), "alice", "다른 말투", voice.LanguageKorean, nil)
	if _, _, projectedOther, _, err := h.svc.ProfileForPrompt(context.Background(), "alice", other.ID); err != nil || projectedOther != "" {
		t.Fatalf("active rule leaked into another voice: %q err=%v", projectedOther, err)
	}
}

func TestOneLearningEventCountsAtMostOneEvidencePerMatchedRule(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	nowTime := time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC)
	now := nowTime.Format(time.RFC3339Nano)
	insertPost(t, h, "dedupe", "alice", alice, "중복", now)
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('shared','alice',?,'LLM does formal endings, but I do polite endings','shared','endings',1,'candidate','diff',?,?)", alice, now, now); err != nil {
		t.Fatal(err)
	}
	event := voice.LearningEvent{ID: "event-dedupe", UserID: "alice", VoiceID: alice, PostSlug: "dedupe", BaselineRevision: 1, InputHash: "dedupe-hash", BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(), Status: "running", CreatedAt: nowTime, ContentLanguage: voice.LanguageKorean, SourceLanguage: voice.LanguageKorean}
	if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	result := voice.LearningResult{
		Source: voice.AuthoredSource{ID: "source-dedupe", UserID: "alice", VoiceID: alice, PostSlug: "dedupe", LearningEventID: event.ID, Title: "중복", Body: "본문", Excerpt: "본문", CreatedAt: nowTime, SourceLanguage: voice.LanguageKorean},
		Rules: []voice.ExtractedRule{
			{Statement: "LLM does rigid endings, but I do gentle endings", Layer: voice.LayerEndings, MatchRuleID: "shared"},
			{Statement: "LLM does stiff endings, but I do conversational endings", Layer: voice.LayerEndings, MatchRuleID: "shared"},
		},
	}
	if err := h.store.ApplyLearningResult(context.Background(), event, result, voice.PersonalizationConfig{RuleActivationEvidence: 3}, nowTime); err != nil {
		t.Fatal(err)
	}
	rule, err := h.store.GetRule(context.Background(), "alice", "shared")
	if err != nil || rule.EvidenceCount != 2 || rule.Status != voice.RuleCandidate {
		t.Fatalf("deduplicated rule=%+v err=%v", rule, err)
	}
	var evidence int
	if err = h.db.Reader.QueryRow("SELECT count(*) FROM voice_rule_evidence WHERE rule_id='shared' AND event_id=? AND origin='diff'", event.ID).Scan(&evidence); err != nil || evidence != 1 {
		t.Fatalf("evidence count=%d err=%v", evidence, err)
	}
}

func TestExplicitRuleComparisonAndValidationStayOutsideModelExperiments(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	models := &scriptedPersonalizationModels{}
	svc := voice.NewService(h.store, models, h.jobs)
	svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	profile := voice.StructuredProfile{Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: "담백함", Source: voice.SourceAnalyzed}}}
	if _, err := h.store.PublishProfileVersion(context.Background(), "alice", alice, profile, "analysis", 0, nowTime); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('candidate','alice',?,'LLM does long sentences, but I do concise sentences','candidate','syntax',1,'candidate','diff',?,?)", alice, now, now); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"source-a", "source-b", "source-c"} {
		body := []string{"첫 번째 원문은 바닷가 기록입니다.", "두 번째 원문은 산책 기록입니다.", "세 번째 원문은 카페 기록입니다."}[i]
		if _, err := h.db.Writer.Exec("INSERT INTO voice_authored_sources(id,user_id,voice_id,title,tags,body,excerpt,created_at) VALUES(?,'alice',?,?,'[]',?,?,?)", id, alice, "주제 "+id, body, body, now); err != nil {
			t.Fatal(err)
		}
	}
	// A source from another voice of the same account is not comparison material for this rule.
	other, _, _ := svc.CreateVoice(context.Background(), "alice", "다른 말투", voice.LanguageKorean, nil)
	if _, err := h.db.Writer.Exec("INSERT INTO voice_authored_sources(id,user_id,voice_id,title,tags,body,excerpt,created_at) VALUES('source-other','alice',?,'다른','[]','다른 말투의 원문입니다.','다른',?)", other.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.StartRuleComparison(context.Background(), "alice", "candidate", "source-other", nil, analyzeRef); !errors.Is(err, voice.ErrLearningNotFound) {
		t.Fatalf("cross-voice source = %v", err)
	}

	targetLength := 900
	comparisonID, jobID, err := svc.StartRuleComparison(context.Background(), "alice", "candidate", "source-a", &targetLength, analyzeRef)
	if err != nil || comparisonID == "" || jobID == "" || models.calls != 0 {
		t.Fatalf("comparison start id=%q job=%q calls=%d err=%v", comparisonID, jobID, models.calls, err)
	}
	if calls := h.jobs.personalizationCalls; len(calls) == 0 || calls[len(calls)-1].VoiceID != alice {
		t.Fatalf("comparison job did not freeze the rule's voice: %+v", calls)
	}
	models.responses = []string{"비교 본문", "비교 본문"}
	if err = svc.CompareRule(context.Background(), "alice", comparisonID, analyzeRef.String(), func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	comparison, err := svc.GetRuleComparison(context.Background(), "alice", comparisonID)
	if err != nil || comparison.Status != "review" || len(comparison.Candidates) != 2 || comparison.VoiceID != alice {
		t.Fatalf("comparison=%+v err=%v", comparison, err)
	}
	if _, err = svc.DecideRuleComparison(context.Background(), "alice", comparisonID, comparison.RuleOnSide); err != nil {
		t.Fatal(err)
	}
	var experimentCount int
	if err = h.db.Reader.QueryRow("SELECT count(*) FROM model_experiments").Scan(&experimentCount); err != nil || experimentCount != 0 {
		t.Fatalf("rule comparison wrote model experiments: count=%d err=%v", experimentCount, err)
	}

	// Validation eligibility counts only the selected voice's sources: the sibling voice
	// with one source cannot start, the default with three can.
	if _, _, err := svc.StartValidation(context.Background(), "alice", other.ID, analyzeRef, analyzeRef, false); !errors.Is(err, voice.ErrInsufficientSources) {
		t.Fatalf("other voice validation = %v", err)
	}
	models.responses = []string{"중립 주제입니다.", "새로 작성한 글입니다.", "중립 주제입니다.", "새로 작성한 글입니다.", "중립 주제입니다.", "새로 작성한 글입니다."}
	validationID, validationJobID, err := svc.StartValidation(context.Background(), "alice", alice, analyzeRef, analyzeRef, false)
	if err != nil || validationID == "" || validationJobID == "" {
		t.Fatalf("validation start id=%q job=%q err=%v", validationID, validationJobID, err)
	}
	beforeValidationCalls := models.calls
	if err = svc.ValidateProfile(context.Background(), "alice", validationID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	validation, err := svc.GetValidation(context.Background(), "alice", validationID)
	if err != nil || validation.Status != "done" || validation.TotalCount != 0 || models.calls-beforeValidationCalls != 6 || validation.VoiceID != alice {
		t.Fatalf("validation=%+v calls=%d err=%v", validation, models.calls-beforeValidationCalls, err)
	}
	if listed, err := svc.ListValidations(context.Background(), "alice", other.ID); err != nil || len(listed) != 0 {
		t.Fatalf("validation listed under another voice: %+v err=%v", listed, err)
	}
	if _, err = svc.GetValidation(context.Background(), "bob", validationID); err == nil {
		t.Fatal("second account read the first account's validation")
	}
	judgeScores := `{"endings":true,"sentence_rhythm":true,"opening_closing":true,"vocabulary":true,"addressee":true}`
	models.responses = []string{"중립 주제입니다.", "새로 작성한 글입니다.", judgeScores, "중립 주제입니다.", "새로 작성한 글입니다.", judgeScores, "중립 주제입니다.", "새로 작성한 글입니다.", judgeScores}
	judgedID, _, err := svc.StartValidation(context.Background(), "alice", alice, analyzeRef, analyzeRef, true)
	if err != nil {
		t.Fatal(err)
	}
	beforeJudgeCalls := models.calls
	if err = svc.ValidateProfile(context.Background(), "alice", judgedID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	judged, err := svc.GetValidation(context.Background(), "alice", judgedID)
	if err != nil || judged.Status != "done" || judged.YCount != 15 || judged.TotalCount != 15 || models.calls-beforeJudgeCalls != 9 {
		t.Fatalf("judged validation=%+v calls=%d err=%v", judged, models.calls-beforeJudgeCalls, err)
	}
	current, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || current.Structured.Version != 2 {
		t.Fatalf("validation mutated profile head: version=%d err=%v", current.Structured.Version, err)
	}
}

// A response that never mentions the axes must publish them as unknown, not as six neutral
// zeros — the fixture above hand-writes the exact key shape, which is how a prompt that never
// named that shape passed CI while a live account showed 0 everywhere.
func TestLearnPublishesUnansweredAxesAsUnknownAndRejectsOutOfRange(t *testing.T) {
	learnWith := func(t *testing.T, response string, structured bool) (voice.Profile, llm.Request, error) {
		t.Helper()
		h := newVoiceHarness(t)
		alice := h.voice("alice")
		now := time.Now().UTC().Format(time.RFC3339Nano)
		insertPost(t, h, "axes", "alice", alice, "축", now)
		raw := `{"title":"축","summary":"","tags":[],"blocks":[{"type":"TEXT","content":"오늘은 천천히 걸어요. 바람이 참 좋아요."}]}`
		snapshot := voice.FinalizationInput{PostSlug: "axes", UserID: "alice", VoiceID: alice, BaselineVoiceID: alice, BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1, ContentLanguage: voice.LanguageKorean, VoiceSourceLanguage: voice.LanguageKorean}
		h.svc.ConfigurePersonalization(learningPosts{snapshot: snapshot}, personalizationConfig())
		h.models.response = response
		h.models.structured = structured
		event, _, _, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "axes", analyzeRef)
		if err != nil {
			t.Fatal(err)
		}
		err = h.svc.Learn(context.Background(), voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {})
		profile, getErr := h.svc.Get(context.Background(), "alice", alice)
		if getErr != nil {
			t.Fatal(getErr)
		}
		return profile, h.models.request, err
	}
	strings8 := `"lexical_description":"담백","base_register":"해요","connective_style":"","intro_pattern":"","closing_pattern":"","heading_habit":"","list_habit":"","emoji_use":""`

	t.Run("omitted axes publish as unknown", func(t *testing.T) {
		profile, request, err := learnWith(t, `{`+strings8+`}`, false)
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
		profile, _, err := learnWith(t, `{`+strings8+`,"axes":{"involvement":2,"humor":-1}}`, false)
		if err != nil {
			t.Fatal(err)
		}
		axes := profile.Structured.Axes
		if axes.Involvement == nil || *axes.Involvement != 2 || axes.Humor == nil || *axes.Humor != -1 || axes.Narrativity != nil {
			t.Fatalf("axes = %+v", axes)
		}
	})
	t.Run("out-of-range axis still fails the job", func(t *testing.T) {
		profile, _, err := learnWith(t, `{`+strings8+`,"axes":{"involvement":4}}`, false)
		if err == nil || !strings.Contains(err.Error(), "-3..3") {
			t.Fatalf("expected range error, got %v", err)
		}
		if profile.Structured.Version != 0 {
			t.Fatalf("a rejected analysis published version %d", profile.Structured.Version)
		}
	})
	t.Run("structured-output model receives the schema", func(t *testing.T) {
		_, request, err := learnWith(t, `{`+strings8+`,"axes":{"involvement":0,"narrativity":0,"persuasion_overtness":0,"abstractness":0,"addressee_focus":0,"humor":0}}`, true)
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

// Finalization learning is the boundary where a reassigned post must not leak evidence: the
// post's current voice and the voice its machine baseline was written under have to agree,
// and the voice must be alive. A retry follows the event's frozen voice, never the post.
func TestFinalizeLearnRequiresMatchingLiveVoiceAndRetryFollowsTheEvent(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	formal, _, _ := h.svc.CreateVoice(context.Background(), "alice", "격식", voice.LanguageKorean, nil)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	insertPost(t, h, "moved", "alice", formal.ID, "옮긴 글", now)
	raw := `{"title":"옮긴 글","summary":"","tags":[],"blocks":[{"type":"TEXT","content":"본문입니다."}]}`
	posts := &mutableLearningPosts{snapshot: voice.FinalizationInput{PostSlug: "moved", UserID: "alice", VoiceID: formal.ID, BaselineVoiceID: alice, BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1, ContentLanguage: voice.LanguageKorean, VoiceSourceLanguage: voice.LanguageKorean}}
	h.svc.ConfigurePersonalization(posts, personalizationConfig())

	// Reassigned after generation: baseline voice != current voice.
	if _, _, _, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "moved", analyzeRef); !errors.Is(err, voice.ErrBaselineVoiceMismatch) {
		t.Fatalf("mismatched baseline = %v", err)
	}
	if _, err := h.svc.GiveFeedback(context.Background(), "alice", "moved", "s1", "ending", "본문입니다.", false); !errors.Is(err, voice.ErrBaselineVoiceMismatch) {
		t.Fatalf("mismatched feedback = %v", err)
	}
	// A fresh machine result under the new voice makes the post learnable there.
	posts.snapshot.BaselineVoiceID = formal.ID
	event, jobID, _, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "moved", analyzeRef)
	if err != nil || event.VoiceID != formal.ID || jobID == "" {
		t.Fatalf("learn in new voice event=%+v job=%q err=%v", event, jobID, err)
	}
	// Reassigning the post again does not retarget the frozen event: the retry runs in the
	// event's voice and the post's new voice sees nothing of it.
	posts.snapshot.VoiceID, posts.snapshot.BaselineVoiceID = alice, alice
	h.jobs.personalizationActive[jobID] = false
	h.jobs.enqueueID = "retry-job"
	retried, retryJob, err := h.svc.RetryLearning(context.Background(), "alice", event.ID, analyzeRef)
	if err != nil || retried.VoiceID != formal.ID || retryJob != "retry-job" {
		t.Fatalf("retry event=%+v job=%q err=%v", retried, retryJob, err)
	}
	if calls := h.jobs.personalizationCalls; calls[len(calls)-1].VoiceID != formal.ID {
		t.Fatalf("retry did not follow the event's voice: %+v", calls[len(calls)-1])
	}
	// A deleted voice refuses both a new learn and a retry, and the queued job fails safely.
	if _, err := h.svc.DeleteVoice(context.Background(), "alice", formal.ID); err != nil {
		t.Fatal(err)
	}
	h.jobs.personalizationActive["retry-job"] = false
	if _, _, err := h.svc.RetryLearning(context.Background(), "alice", event.ID, analyzeRef); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("retry in deleted voice = %v", err)
	}
	if err := h.svc.Learn(context.Background(), voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); !errors.Is(err, voice.ErrVoiceDeleted) || h.models.completeCalls != 0 {
		t.Fatalf("learn job in deleted voice err=%v calls=%d", err, h.models.completeCalls)
	}
	if stored, _ := h.svc.GetLearningEvent(context.Background(), "alice", event.ID); stored.Status != "retryable" {
		t.Fatalf("event after refused job = %+v", stored)
	}
	posts.snapshot.VoiceID, posts.snapshot.BaselineVoiceID = formal.ID, formal.ID
	if _, _, _, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "moved", analyzeRef); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("learn in deleted voice = %v", err)
	}
}

type mutableLearningPosts struct{ snapshot voice.FinalizationInput }

func (p *mutableLearningPosts) LearningSnapshot(context.Context, string, string) (voice.FinalizationInput, error) {
	return p.snapshot, nil
}

// Rule and confirmation operations derive their voice from the owned aggregate, so a rule of
// voice A can only ever move inside voice A, and its status change publishes a version there.
func TestRuleStatusChangesStayInsideTheRulesVoice(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	formal, _, _ := h.svc.CreateVoice(context.Background(), "alice", "격식", voice.LanguageKorean, nil)
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	for _, v := range []string{alice, formal.ID} {
		if _, err := h.store.PublishProfileVersion(context.Background(), "alice", v, voice.StructuredProfile{}, "analysis", 0, nowTime); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('formal-rule','alice',?,'LLM does casual endings, but I do formal endings','k','endings',2,'candidate','diff',?,?)", formal.ID, now, now); err != nil {
		t.Fatal(err)
	}
	h.svc.ConfigurePersonalization(learningPosts{}, personalizationConfig())
	profile, err := h.svc.ChangeRuleStatus(context.Background(), "alice", "formal-rule", voice.RuleActive)
	if err != nil || profile.VoiceID != formal.ID || len(profile.Structured.Rules) != 1 || profile.Structured.Rules[0].Status != voice.RuleActive || profile.Structured.Version != 2 {
		t.Fatalf("rule status change = %+v err=%v", profile, err)
	}
	defaultProfile, _ := h.svc.Get(context.Background(), "alice", alice)
	if defaultProfile.Structured.Version != 1 || len(defaultProfile.Structured.Rules) != 0 {
		t.Fatalf("rule change published into the wrong voice: %+v", defaultProfile.Structured)
	}
	if _, err := h.svc.ChangeRuleStatus(context.Background(), "bob", "formal-rule", voice.RuleRetired); !errors.Is(err, voice.ErrRuleNotFound) {
		t.Fatalf("foreign rule = %v", err)
	}
	if _, err := h.svc.DeleteVoice(context.Background(), "alice", formal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.ChangeRuleStatus(context.Background(), "alice", "formal-rule", voice.RuleRetired); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("rule change in deleted voice = %v", err)
	}
}
