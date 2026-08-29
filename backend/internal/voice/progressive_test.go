package voice_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

type learningPosts struct{ snapshot voice.FinalizationInput }

func (p learningPosts) LearningSnapshot(context.Context, string, string) (voice.FinalizationInput, error) {
	return p.snapshot, nil
}

type scriptedPersonalizationModels struct {
	mu        sync.Mutex
	responses []string
	calls     int
}

func (m *scriptedPersonalizationModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return analyzeRef, true, nil
}
func (m *scriptedPersonalizationModels) ModelEnabled(ref llm.ModelRef) bool {
	return ref.ProviderID != "" && ref.ModelID != ""
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
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('first','alice','첫 글','','review',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	raw := `{"title":"첫 글","summary":"","tags":["산책"],"blocks":[{"type":"TEXT","content":"오늘은 천천히 걸어요. 바람이 참 좋아요."}]}`
	targetLength := 900
	snapshot := voice.FinalizationInput{PostSlug: "first", UserID: "alice", BaselineJSON: raw, FinalJSON: raw, BaselineRevision: 1, ContentRevision: 1, TargetLength: &targetLength}
	h.svc.ConfigurePersonalization(learningPosts{snapshot: snapshot}, voice.PersonalizationConfig{FewShotTargetCount: 2, FewShotMax: 3, FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800, EmbeddingSwitchPosts: 50, DiffMaxRules: 3, DiffMinPatternEdits: 2, RuleActivationEvidence: 3, RuleRetireAfter: 180 * 24 * time.Hour, ValidationPostCount: 3, EndingMaxConsecutive: 2})
	h.models.response = `{"lexical_description":"담백한 어휘","base_register":"해요","connective_style":"짧은 연결","intro_pattern":"바로 시작","closing_pattern":"짧게 마침","heading_habit":"","list_habit":"","emoji_use":"","axes":{"involvement":1,"narrativity":1,"persuasion_overtness":0,"abstractness":0,"addressee_focus":0,"humor":0}}`

	event, jobID, reused, err := h.svc.LearnFromFinalizedPost(context.Background(), "alice", "first", analyzeRef)
	if err != nil || reused || jobID != "job-new" || h.models.completeCalls != 0 {
		t.Fatalf("finalize event=%+v job=%q reused=%v calls=%d err=%v", event, jobID, reused, h.models.completeCalls, err)
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
	profile, err := h.svc.Get(context.Background(), "alice")
	if err != nil || profile.Structured.Version != 1 || profile.SourceCount != 1 || profile.Structured.Empty {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	_, excerpts, _, empty, err := h.svc.ProfileForPromptForTopic(context.Background(), "alice", "다른 주제", nil)
	if err != nil || empty || len(excerpts) != 1 || !strings.Contains(excerpts[0], "천천히") {
		t.Fatalf("prompt excerpts=%v empty=%v err=%v", excerpts, empty, err)
	}
	calls := h.models.completeCalls
	if err = h.svc.Learn(context.Background(), voice.LearningJob{UserID: "alice", EventID: event.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil || h.models.completeCalls != calls {
		t.Fatalf("completed retry called provider: calls=%d→%d err=%v", calls, h.models.completeCalls, err)
	}
}

func TestContradictoryEvidenceLeavesProfileHeadAndActiveRuleUnchanged(t *testing.T) {
	h := newVoiceHarness(t)
	nowTime := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	now := nowTime.Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('second','alice','둘째','','review',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	rule := voice.ContrastRule{ID: "active-rule", UserID: "alice", Statement: "LLM does formal endings, but I do polite endings", CanonicalKey: "active", Layer: voice.LayerEndings, EvidenceCount: 3, Status: voice.RuleActive, Origin: "diff", CreatedAt: nowTime, LastEvidenceAt: nowTime}
	baseProfile := voice.StructuredProfile{Rules: []voice.ContrastRule{rule}, SourceCount: 1}
	if _, err := h.store.PublishProfileVersion(context.Background(), "alice", baseProfile, "analysis", 0, nowTime); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES(?,?,?,?,?,?,?,?,?,?)", rule.ID, rule.UserID, rule.Statement, rule.CanonicalKey, string(rule.Layer), rule.EvidenceCount, string(rule.Status), rule.Origin, now, now); err != nil {
		t.Fatal(err)
	}
	event := voice.LearningEvent{ID: "event-contradiction", UserID: "alice", PostSlug: "second", BaselineRevision: 1, InputHash: "hash", BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(), Status: "running", CreatedAt: nowTime}
	if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	result := voice.LearningResult{
		Source:  voice.AuthoredSource{ID: "source-contradiction", UserID: "alice", PostSlug: "second", LearningEventID: event.ID, Title: "둘째", Body: "새 근거", Excerpt: "새 근거", CreatedAt: nowTime},
		Profile: voice.StructuredProfile{SourceCount: 2},
		Rules:   []voice.ExtractedRule{{Statement: "LLM does polite endings, but I do formal endings", Layer: voice.LayerEndings, ContradictsRuleID: rule.ID}},
	}
	if err := h.store.ApplyLearningResult(context.Background(), event, result, voice.PersonalizationConfig{RuleActivationEvidence: 3}, nowTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice")
	if err != nil || profile.Structured.Version != 1 {
		t.Fatalf("contradiction changed profile head: version=%d err=%v", profile.Structured.Version, err)
	}
	storedRule, err := h.store.GetRule(context.Background(), "alice", rule.ID)
	if err != nil || storedRule.Statement != rule.Statement || storedRule.Status != voice.RuleActive || storedRule.EvidenceCount != 3 {
		t.Fatalf("active rule changed: %+v err=%v", storedRule, err)
	}
	confirmations, err := h.store.ListConfirmations(context.Background(), "alice")
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
	versions, err := h.store.ListProfileVersions(context.Background(), "alice")
	if err != nil || len(versions) != 2 || len(versions[0].Profile.Rules) != 1 || versions[0].Profile.Rules[0] != replacedRule {
		t.Fatalf("replacement snapshot=%+v err=%v", versions, err)
	}
}

func TestManualOverrideClearAndRestorePublishImmutableWholeVersions(t *testing.T) {
	h := newVoiceHarness(t)
	base := voice.StructuredProfile{Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: "분석값", Source: voice.SourceAnalyzed}}}
	if _, err := h.store.PublishProfileVersion(context.Background(), "alice", base, "analysis", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	manual := "직접 정한 값"
	profile, err := h.svc.UpdateOverride(context.Background(), "alice", voice.LayerLexical, "description", &manual)
	if err != nil || profile.Structured.Version != 2 || profile.Structured.Lexical.Description.Value != manual || profile.Structured.Lexical.Description.Source != voice.SourceManual {
		t.Fatalf("manual profile=%+v err=%v", profile.Structured, err)
	}
	profile, err = h.svc.UpdateOverride(context.Background(), "alice", voice.LayerLexical, "description", nil)
	if err != nil || profile.Structured.Version != 3 || profile.Structured.Lexical.Description.Value != "분석값" {
		t.Fatalf("cleared profile=%+v err=%v", profile.Structured, err)
	}
	profile, err = h.svc.RestoreVersion(context.Background(), "alice", 2)
	if err != nil || profile.Structured.Version != 4 || profile.Structured.Lexical.Description.Value != manual {
		t.Fatalf("restored profile=%+v err=%v", profile.Structured, err)
	}
	versions, err := h.svc.ListVersions(context.Background(), "alice")
	if err != nil || len(versions) != 4 || versions[0].Origin != "restore" || versions[0].RestoredFromVersion != 2 || versions[3].Profile.Lexical.Description.Value != "분석값" {
		t.Fatalf("versions=%+v err=%v", versions, err)
	}
}

func TestIndependentLearningEventsPromoteRuleAtConfiguredEvidenceCount(t *testing.T) {
	h := newVoiceHarness(t)
	statement := "LLM does formal endings, but I do polite endings"
	for i := 1; i <= 3; i++ {
		nowTime := time.Date(2026, 8, 29, 12, i, 0, 0, time.UTC)
		now := nowTime.Format(time.RFC3339Nano)
		slug := "rule-post-" + string(rune('0'+i))
		if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES(?,'alice','규칙','','review',?,?)", slug, now, now); err != nil {
			t.Fatal(err)
		}
		event := voice.LearningEvent{ID: "rule-event-" + string(rune('0'+i)), UserID: "alice", PostSlug: slug, BaselineRevision: 1, InputHash: "hash-" + slug, BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(), Status: "running", CreatedAt: nowTime}
		if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		result := voice.LearningResult{Source: voice.AuthoredSource{ID: "rule-source-" + string(rune('0'+i)), UserID: "alice", PostSlug: slug, LearningEventID: event.ID, Title: slug, Body: slug, Excerpt: slug, CreatedAt: nowTime}, Rules: []voice.ExtractedRule{{Statement: statement, Layer: voice.LayerEndings}}}
		if err := h.store.ApplyLearningResult(context.Background(), event, result, voice.PersonalizationConfig{RuleActivationEvidence: 3}, nowTime); err != nil {
			t.Fatal(err)
		}
		rules, err := h.store.ListRules(context.Background(), "alice")
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
	_, _, projected, _, err := h.svc.ProfileForPrompt(context.Background(), "alice")
	if err != nil || projected != statement {
		t.Fatalf("active projection=%q err=%v", projected, err)
	}
}

func TestOneLearningEventCountsAtMostOneEvidencePerMatchedRule(t *testing.T) {
	h := newVoiceHarness(t)
	nowTime := time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC)
	now := nowTime.Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec("INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('dedupe','alice','중복','','review',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('shared','alice','LLM does formal endings, but I do polite endings','shared','endings',1,'candidate','diff',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	event := voice.LearningEvent{ID: "event-dedupe", UserID: "alice", PostSlug: "dedupe", BaselineRevision: 1, InputHash: "dedupe-hash", BaselineJSON: `{}`, FinalJSON: `{}`, ModelRef: analyzeRef.String(), Status: "running", CreatedAt: nowTime}
	if err := h.store.InsertLearningEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	result := voice.LearningResult{
		Source: voice.AuthoredSource{ID: "source-dedupe", UserID: "alice", PostSlug: "dedupe", LearningEventID: event.ID, Title: "중복", Body: "본문", Excerpt: "본문", CreatedAt: nowTime},
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
	models := &scriptedPersonalizationModels{}
	svc := voice.NewService(h.store, models, h.jobs)
	svc.ConfigurePersonalization(learningPosts{}, voice.PersonalizationConfig{FewShotTargetCount: 2, FewShotMax: 3, FewShotExcerptTargetChars: 500, FewShotExcerptMaxChars: 800, DiffMaxRules: 3, DiffMinPatternEdits: 2, RuleActivationEvidence: 3, RuleRetireAfter: 180 * 24 * time.Hour, ValidationPostCount: 3, EndingMaxConsecutive: 2})
	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	profile := voice.StructuredProfile{Lexical: voice.LexicalProfile{Description: voice.VoiceValue{Value: "담백함", Source: voice.SourceAnalyzed}}}
	if _, err := h.store.PublishProfileVersion(context.Background(), "alice", profile, "analysis", 0, nowTime); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('candidate','alice','LLM does long sentences, but I do concise sentences','candidate','syntax',1,'candidate','diff',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"source-a", "source-b", "source-c"} {
		body := []string{"첫 번째 원문은 바닷가 기록입니다.", "두 번째 원문은 산책 기록입니다.", "세 번째 원문은 카페 기록입니다."}[i]
		if _, err := h.db.Writer.Exec("INSERT INTO voice_authored_sources(id,user_id,title,tags,body,excerpt,created_at) VALUES(?,'alice',?,'[]',?,?,?)", id, "주제 "+id, body, body, now); err != nil {
			t.Fatal(err)
		}
	}

	targetLength := 900
	comparisonID, jobID, err := svc.StartRuleComparison(context.Background(), "alice", "candidate", "source-a", &targetLength, analyzeRef)
	if err != nil || comparisonID == "" || jobID == "" || models.calls != 0 {
		t.Fatalf("comparison start id=%q job=%q calls=%d err=%v", comparisonID, jobID, models.calls, err)
	}
	models.responses = []string{"비교 본문", "비교 본문"}
	if err = svc.CompareRule(context.Background(), "alice", comparisonID, analyzeRef.String(), func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	comparison, err := svc.GetRuleComparison(context.Background(), "alice", comparisonID)
	if err != nil || comparison.Status != "review" || len(comparison.Candidates) != 2 {
		t.Fatalf("comparison=%+v err=%v", comparison, err)
	}
	if _, err = svc.DecideRuleComparison(context.Background(), "alice", comparisonID, comparison.RuleOnSide); err != nil {
		t.Fatal(err)
	}
	var experimentCount int
	if err = h.db.Reader.QueryRow("SELECT count(*) FROM model_experiments").Scan(&experimentCount); err != nil || experimentCount != 0 {
		t.Fatalf("rule comparison wrote model experiments: count=%d err=%v", experimentCount, err)
	}

	models.responses = []string{"중립 주제입니다.", "새로 작성한 글입니다.", "중립 주제입니다.", "새로 작성한 글입니다.", "중립 주제입니다.", "새로 작성한 글입니다."}
	validationID, validationJobID, err := svc.StartValidation(context.Background(), "alice", analyzeRef, analyzeRef, false)
	if err != nil || validationID == "" || validationJobID == "" {
		t.Fatalf("validation start id=%q job=%q err=%v", validationID, validationJobID, err)
	}
	beforeValidationCalls := models.calls
	if err = svc.ValidateProfile(context.Background(), "alice", validationID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	validation, err := svc.GetValidation(context.Background(), "alice", validationID)
	if err != nil || validation.Status != "done" || validation.TotalCount != 0 || models.calls-beforeValidationCalls != 6 {
		t.Fatalf("validation=%+v calls=%d err=%v", validation, models.calls-beforeValidationCalls, err)
	}
	if _, err = svc.GetValidation(context.Background(), "bob", validationID); err == nil {
		t.Fatal("second account read the first account's validation")
	}
	judgeScores := `{"endings":true,"sentence_rhythm":true,"opening_closing":true,"vocabulary":true,"addressee":true}`
	models.responses = []string{"중립 주제입니다.", "새로 작성한 글입니다.", judgeScores, "중립 주제입니다.", "새로 작성한 글입니다.", judgeScores, "중립 주제입니다.", "새로 작성한 글입니다.", judgeScores}
	judgedID, _, err := svc.StartValidation(context.Background(), "alice", analyzeRef, analyzeRef, true)
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
	current, err := h.store.GetProfile(context.Background(), "alice")
	if err != nil || current.Structured.Version != 2 {
		t.Fatalf("validation mutated profile head: version=%d err=%v", current.Structured.Version, err)
	}
}
