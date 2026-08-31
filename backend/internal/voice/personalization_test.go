package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

type personalizationModels struct {
	responses []string
	calls     int
	requests  []llm.Request
}

func (m *personalizationModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{}, false, nil
}
func (m *personalizationModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{}, false
}
func (m *personalizationModels) Complete(_ context.Context, _ llm.ModelRef, request llm.Request) (llm.Response, error) {
	m.requests = append(m.requests, request)
	response := m.responses[m.calls]
	m.calls++
	return llm.Response{Text: response}, nil
}

func TestMeasureKoreanEndingsAndUnicodeLength(t *testing.T) {
	got := Measure("오늘은 좋아요. 정말 좋습니다! 다음에도 온다.\n짧아요.")
	if len(got.Sentences) != 4 || got.AverageSentenceChars <= 0 {
		t.Fatalf("measure = %+v", got)
	}
	ratios := map[string]float64{}
	for _, item := range got.EndingDistribution {
		ratios[item.Ending] = item.Ratio
	}
	if ratios["해요"] != .5 || ratios["습니다"] != .25 || ratios["다"] != .25 {
		t.Fatalf("ending ratios = %+v", ratios)
	}
}

func TestKoreanAnalysisContractsRemainTheDefaultByteForByte(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) }
	text := "오늘은 좋아요. 정말 좋습니다!"
	if got, want := MeasuredProfileForLanguage(text, LanguageKorean, now), MeasuredProfile(text, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("Korean measured profile changed\ngot=%#v\nwant=%#v", got, want)
	}
	if analysisPromptForLanguage(LanguageKorean) != analysisPrompt {
		t.Fatal("Korean analysis prompt changed through language selection")
	}
	if !bytes.Equal(VoiceAnalysisSchemaForLanguage(LanguageKorean), VoiceAnalysisSchema()) {
		t.Fatal("Korean analysis schema changed through language selection")
	}
}

func TestEnglishMeasurementUsesWordsRegisterCadenceAndSixAxes(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) }
	text := "However, we can't ignore this decision. John's proposal is formal and cannot be dismissed. Are you ready? Great! A fragment"
	got := MeasuredProfileForLanguage(text, LanguageEnglish, now)
	if got.Syntax.AverageSentenceWords == nil || *got.Syntax.AverageSentenceWords <= 0 {
		t.Fatalf("average words = %#v", got.Syntax.AverageSentenceWords)
	}
	if !strings.Contains(got.Endings.BaseRegister.Value, "contractions present") {
		t.Fatalf("register = %#v", got.Endings.BaseRegister)
	}
	if !reflect.DeepEqual(got.Syntax.PreferredConnectives, []string{"however"}) {
		t.Fatalf("connectives = %#v", got.Syntax.PreferredConnectives)
	}
	if !strings.Contains(got.Syntax.PassiveTendency.Value, "1 of") {
		t.Fatalf("passive tendency = %#v", got.Syntax.PassiveTendency)
	}
	cadence := map[string]float64{}
	for _, item := range got.Endings.Distribution {
		cadence[item.Ending] = item.Ratio
		if strings.ContainsAny(item.Ending, "가-힣") {
			t.Fatalf("English cadence used a Korean category: %#v", item)
		}
	}
	for _, category := range []string{"statement", "question", "exclamation", "fragment"} {
		if _, ok := cadence[category]; !ok {
			t.Fatalf("cadence category %q missing: %#v", category, cadence)
		}
	}
	for name, value := range map[string]*int{"involvement": got.Axes.Involvement, "narrativity": got.Axes.Narrativity, "persuasion": got.Axes.PersuasionOvertness, "abstractness": got.Axes.Abstractness, "addressee": got.Axes.AddresseeFocus, "humor": got.Axes.Humor} {
		if value == nil || *value < -3 || *value > 3 {
			t.Fatalf("axis %s = %#v", name, value)
		}
	}
}

func TestEnglishContractionsExcludePossessivesAndUncontractedCannot(t *testing.T) {
	for _, word := range []string{"can't", "we're", "I've", "you'll", "I'd", "I'm", "it's"} {
		if !englishContraction(strings.ToLower(word)) {
			t.Fatalf("actual contraction %q was not detected", word)
		}
	}
	for _, word := range []string{"john's", "company's", "cannot", "gonna"} {
		if englishContraction(word) {
			t.Fatalf("non-contraction %q was detected", word)
		}
	}
	formal := MeasuredProfileForLanguage("John's proposal cannot be dismissed.", LanguageEnglish, time.Now)
	if !strings.Contains(formal.Endings.BaseRegister.Value, "formal") {
		t.Fatalf("possessive/cannot made formal prose conversational: %#v", formal.Endings.BaseRegister)
	}
}

func TestEnglishPromptSchemaRuleComparisonAndValidationSelection(t *testing.T) {
	if !strings.Contains(analysisPromptForLanguage(LanguageEnglish), "English writing-style analyst") || strings.Contains(analysisPromptForLanguage(LanguageEnglish), "한국어 문체") {
		t.Fatalf("English analysis prompt = %q", analysisPromptForLanguage(LanguageEnglish))
	}
	var schema map[string]any
	if err := json.Unmarshal(VoiceAnalysisSchemaForLanguage(LanguageEnglish), &schema); err != nil {
		t.Fatalf("English schema is invalid JSON: %v", err)
	}
	snapshot := ruleComparisonSnapshot{Rule: ContrastRule{Statement: "I use contractions"}, Source: AuthoredSource{Title: "A walk", Tags: []string{"travel"}}, SourceLanguage: LanguageEnglish}
	off, on := BuildRuleComparisonPrompts(snapshot)
	if !strings.Contains(off, "Write an English blog post") || !strings.Contains(on, snapshot.Rule.Statement) || strings.Contains(off, "한국어") {
		t.Fatalf("English comparison prompts off=%q on=%q", off, on)
	}
	for _, prompt := range []string{validationSummaryPrompt(LanguageEnglish), validationWritePrompt(LanguageEnglish, StructuredProfile{}, 2), validationJudgePrompt(LanguageEnglish)} {
		if !strings.Contains(prompt, "English") || strings.Contains(prompt, "한국어") {
			t.Fatalf("English validation prompt = %q", prompt)
		}
	}
}

func TestEnglishRuleExtractionAndSemanticClassificationUseEnglishStyleSemantics(t *testing.T) {
	edits := []SentenceEdit{
		{Before: "We are ready.", After: "We're ready!"},
		{Before: "They are here.", After: "They're here!"},
	}
	rules := ExtractStyleRulesForLanguage(edits, 2, 3, LanguageEnglish)
	if len(rules) != 1 || rules[0].Layer != LayerEndings || !strings.Contains(rules[0].Statement, "uncontracted-statement") || !strings.Contains(rules[0].Statement, "contracted-exclamation") {
		t.Fatalf("English deterministic rules = %#v", rules)
	}

	models := &personalizationModels{responses: []string{`{"relation":"same","rule_id":"existing"}`}}
	svc := &Service{models: models}
	candidates := []ExtractedRule{{Statement: "I use compact, conversational sentences", Layer: LayerSyntax}}
	existing := []ContrastRule{{ID: "existing", Statement: "I keep sentences concise", Layer: LayerSyntax, Status: RuleCandidate}}
	classified, err := svc.classifyRuleRelationsForLanguage(context.Background(), llm.ModelRef{ProviderID: "fake", ModelID: "analyze"}, candidates, existing, LanguageEnglish)
	if err != nil || classified[0].MatchRuleID != "existing" {
		t.Fatalf("English semantic relation = %#v, err=%v", classified, err)
	}
	if len(models.requests) != 1 || models.requests[0].System != englishSemanticRulePrompt || strings.Contains(models.requests[0].System, "한국어") {
		t.Fatalf("semantic prompt = %#v", models.requests)
	}
}

func TestExtractStyleRulesRejectsFactsAndRequiresRepeatedPattern(t *testing.T) {
	factual := AlignSentences("제주에 갔다. 카페에 갔다.", "부산에 갔다. 시장에 갔다.")
	if rules := ExtractStyleRules(factual, 2, 3); len(rules) != 0 {
		t.Fatalf("factual edits emitted rules: %+v", rules)
	}
	edits := []SentenceEdit{{Before: "오늘은 좋습니다.", After: "오늘은 좋아요."}, {Before: "풍경이 멋집니다.", After: "풍경이 멋져요."}, {Before: "음식이 맛있습니다.", After: "음식이 맛있어요."}, {Before: "긴 문장입니다.", After: strings.Repeat("아주 ", 30) + "길어요."}}
	rules := ExtractStyleRules(edits, 2, 3)
	if len(rules) != 1 || rules[0].Layer != LayerEndings || len(rules[0].Citations) != 3 {
		t.Fatalf("rules = %+v", rules)
	}
}

func TestRankExcerptsIsStableUniqueAndCapped(t *testing.T) {
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	sources := []AuthoredSource{{ID: "a", Title: "서울 산책", Tags: []string{"서울"}, Excerpt: "A", CreatedAt: now}, {ID: "b", Title: "제주", Tags: []string{"제주"}, Excerpt: "B", CreatedAt: now.Add(time.Hour)}, {ID: "c", Title: "서울 식당", Tags: []string{"서울"}, Excerpt: "A", CreatedAt: now.Add(2 * time.Hour)}, {ID: "d", Title: "서울 카페", Tags: []string{"서울"}, Excerpt: "D", CreatedAt: now.Add(3 * time.Hour)}}
	got := rankExcerpts(sources, "서울 여행", []string{"서울"}, 3)
	if len(got) != 3 || got[0] != "D" || got[1] != "A" || got[2] != "B" {
		t.Fatalf("ranked = %v", got)
	}
}

func TestExcerptUsesFirstBoundaryBetweenTargetAndCap(t *testing.T) {
	body := strings.Repeat("가", 510) + "." + strings.Repeat("나", 400)
	got := excerptAroundTarget(body, 500, 800)
	if len([]rune(got)) != 511 || !strings.HasSuffix(got, ".") {
		t.Fatalf("excerpt length=%d suffix=%q", len([]rune(got)), got[len(got)-1:])
	}
	if got := excerptAroundTarget(strings.Repeat("가", 900), 500, 800); len([]rune(got)) != 800 {
		t.Fatalf("hard-capped excerpt length=%d", len([]rune(got)))
	}
}

func TestComparisonDecisionRequiresTwoSuccessfulOutputs(t *testing.T) {
	valid := RuleComparison{Status: "review", Candidates: []ComparisonCandidate{{Status: "succeeded", Output: "왼쪽"}, {Status: "succeeded", Output: "오른쪽"}}}
	if !comparisonReadyForDecision(valid) {
		t.Fatal("complete comparison was not eligible")
	}
	for _, invalid := range []RuleComparison{
		{Status: "partial", Candidates: valid.Candidates},
		{Status: "review", Candidates: []ComparisonCandidate{{Status: "succeeded", Output: "왼쪽"}, {Status: "failed"}}},
		{Status: "review", Candidates: []ComparisonCandidate{{Status: "succeeded", Output: "왼쪽"}, {Status: "succeeded", Output: ""}}},
	} {
		if comparisonReadyForDecision(invalid) {
			t.Fatalf("incomplete comparison was eligible: %+v", invalid)
		}
	}
}

func TestRuleComparisonPromptsDifferOnlyBySelectedRule(t *testing.T) {
	targetLength := 900
	snapshot := ruleComparisonSnapshot{Rule: ContrastRule{Statement: "LLM does long sentences, but I do concise sentences"}, Source: AuthoredSource{Title: "서울 산책", Tags: []string{"서울"}}, TargetLength: &targetLength, EndingMaxConsecutive: 2}
	off, on := BuildRuleComparisonPrompts(snapshot)
	if strings.Replace(on, snapshot.Rule.Statement+"\n", "", 1) != off {
		t.Fatalf("comparison prompts differ beyond the rule\noff=%q\non=%q", off, on)
	}
	if !strings.Contains(off, "900") {
		t.Fatalf("configured target missing: %s", off)
	}
	withoutTarget, _ := BuildRuleComparisonPrompts(ruleComparisonSnapshot{Rule: snapshot.Rule, Source: snapshot.Source, EndingMaxConsecutive: 2})
	if strings.Contains(withoutTarget, "목표 길이") || strings.Contains(withoutTarget, "1200") {
		t.Fatalf("absent target leaked a numeric constraint: %s", withoutTarget)
	}
}

func TestJudgeScoresRequireExactlyFiveDimensions(t *testing.T) {
	valid := map[string]bool{"endings": true, "sentence_rhythm": false, "opening_closing": true, "vocabulary": true, "addressee": false}
	if !validJudgeScores(valid) {
		t.Fatal("valid judge result was rejected")
	}
	delete(valid, "endings")
	if validJudgeScores(valid) {
		t.Fatal("missing judge dimension was accepted")
	}
	valid["endings"] = true
	valid["extra"] = true
	if validJudgeScores(valid) {
		t.Fatal("extra judge dimension was accepted")
	}
}

func TestAnalysisModelExtractsOnlyStrictRepeatedStyleRules(t *testing.T) {
	models := &personalizationModels{responses: []string{`{"rules":[{"statement":"LLM does formal endings, but I do polite endings","layer":"endings","citation_indexes":[0,1]}]}`}}
	svc := &Service{models: models, config: PersonalizationConfig{DiffMinPatternEdits: 2, DiffMaxRules: 3}}
	edits := []SentenceEdit{{Before: "오늘은 좋습니다.", After: "오늘은 좋아요."}, {Before: "풍경이 멋집니다.", After: "풍경이 멋져요."}}
	rules, err := svc.extractStyleRules(context.Background(), llm.ModelRef{ProviderID: "fake", ModelID: "analyze"}, edits)
	if err != nil || len(rules) != 1 || rules[0].Layer != LayerEndings || len(rules[0].Citations) != 2 {
		t.Fatalf("rules=%+v err=%v", rules, err)
	}

	models.responses = append(models.responses, `{"rules":[{"statement":"LLM does one-off wording, but I do another","layer":"lexical","citation_indexes":[0,0]}]}`)
	if _, err = svc.extractStyleRules(context.Background(), llm.ModelRef{ProviderID: "fake", ModelID: "analyze"}, edits); err == nil {
		t.Fatal("duplicate citations were accepted as independent evidence")
	}
}

func TestSemanticRuleClassificationUsesExactChecksBeforeModel(t *testing.T) {
	models := &personalizationModels{responses: []string{`{"relation":"same","rule_id":"r2"}`}}
	svc := &Service{models: models}
	existing := []ContrastRule{
		{ID: "r1", Statement: "LLM does formal endings, but I do polite endings", Layer: LayerEndings, Status: RuleCandidate},
		{ID: "r2", Statement: "LLM does long sentences, but I do concise sentences", Layer: LayerSyntax, Status: RuleCandidate},
	}
	candidates := []ExtractedRule{
		{Statement: "LLM does formal endings, but I do polite endings", Layer: LayerEndings},
		{Statement: "LLM does verbose sentences, but I do compact sentences", Layer: LayerSyntax},
	}
	classified, err := svc.classifyRuleRelations(context.Background(), llm.ModelRef{ProviderID: "fake", ModelID: "analyze"}, candidates, existing)
	if err != nil {
		t.Fatal(err)
	}
	if classified[0].MatchRuleID != "r1" || classified[1].MatchRuleID != "r2" || models.calls != 1 {
		t.Fatalf("classified=%+v calls=%d", classified, models.calls)
	}
}
