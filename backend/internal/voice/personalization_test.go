package voice

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

type personalizationModels struct {
	responses []string
	calls     int
}

func (m *personalizationModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{}, false, nil
}
func (m *personalizationModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) { return llm.ModelInfo{}, false }
func (m *personalizationModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
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
