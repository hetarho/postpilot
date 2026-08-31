package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

const DiffRulePromptVersion = "voice-diff-rules-v1"

const diffRulePrompt = `You classify edits between an AI draft and the user's finalized Korean prose.
Discard factual, topical, and preference-about-content changes completely. Return JSON only:
{"rules":[{"statement":"LLM does X, but I do Y","layer":"lexical|endings|syntax|structure","citation_indexes":[0,1]}]}.
Every rule needs at least the requested number of distinct same-kind edit citations. Return {"rules":[]} when unsupported.
Do not quote private prose in statements. Generalize only the writing behavior demonstrated by the cited edits.`

const englishDiffRulePrompt = `You classify edits between an AI draft and the user's finalized English prose.
Discard factual, topical, and preference-about-content changes completely. Return JSON only:
{"rules":[{"statement":"LLM does X, but I do Y","layer":"lexical|endings|syntax|structure","citation_indexes":[0,1]}]}.
Every rule needs at least the requested number of distinct same-kind edit citations. Return {"rules":[]} when unsupported.
Treat the endings layer as English register, contraction, or statement/question/exclamation/fragment cadence, never Korean endings.
Do not quote private prose in statements. Generalize only the writing behavior demonstrated by the cited edits.`

const SemanticRulePromptVersion = "voice-semantic-rule-v1"

const semanticRulePrompt = `Compare one newly observed writing-style rule with the account's existing rules.
Return JSON only: {"relation":"same|contradicts|distinct","rule_id":"existing id or empty"}.
"same" means the behavior is semantically equivalent. "contradicts" means the new behavior reverses an active rule.
Otherwise return "distinct" with an empty rule_id.`

const englishSemanticRulePrompt = `Compare one newly observed English writing-style rule with the account's existing English rules.
Return JSON only: {"relation":"same|contradicts|distinct","rule_id":"existing id or empty"}.
"same" means the behavior is semantically equivalent. "contradicts" means the new behavior reverses an active rule.
Compare English register, contractions, cadence, syntax, structure, and lexical behavior without translating them into Korean ending categories.
Otherwise return "distinct" with an empty rule_id.`

type diffRuleWire struct {
	Statement       string `json:"statement"`
	Layer           string `json:"layer"`
	CitationIndexes []int  `json:"citation_indexes"`
}
type diffRulesWire struct {
	Rules []diffRuleWire `json:"rules"`
}
type semanticRuleWire struct {
	Relation string `json:"relation"`
	RuleID   string `json:"rule_id"`
}

type SentenceEdit struct{ Before, After string }

// AlignSentences uses an LCS for stable insert/delete/reorder handling and pairs the
// remaining runs in order. It is deterministic and never sends prose outside the
// explicit learning operation.
func AlignSentences(before, after string) []SentenceEdit {
	a, b := SegmentSentences(before), SegmentSentences(after)
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var edits []SentenceEdit
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if i < len(a) && j < len(b) && a[i] == b[j] {
			i++
			j++
			continue
		}
		var deleted, inserted []string
		for i < len(a) || j < len(b) {
			if i < len(a) && j < len(b) && a[i] == b[j] {
				break
			}
			if i < len(a) && (j >= len(b) || dp[i+1][j] >= dp[i][j+1]) {
				deleted = append(deleted, a[i])
				i++
			} else {
				inserted = append(inserted, b[j])
				j++
			}
		}
		for k := 0; k < max(len(deleted), len(inserted)); k++ {
			e := SentenceEdit{}
			if k < len(deleted) {
				e.Before = deleted[k]
			}
			if k < len(inserted) {
				e.After = inserted[k]
			}
			edits = append(edits, e)
		}
	}
	return edits
}

func endingOf(sentence string) string {
	v := strings.TrimRightFunc(strings.TrimSpace(sentence), func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
	if strings.HasSuffix(v, "습니다") || strings.HasSuffix(v, "니다") || strings.HasSuffix(v, "ㅂ니다") {
		return "습니다"
	}
	if strings.HasSuffix(v, "해요") || strings.HasSuffix(v, "어요") || strings.HasSuffix(v, "아요") || strings.HasSuffix(v, "요") {
		return "해요"
	}
	if strings.HasSuffix(v, "다") {
		return "다"
	}
	return "기타"
}

func englishStyleEnding(sentence string) string {
	register := "uncontracted"
	for _, word := range englishWords(sentence) {
		if englishContraction(word) {
			register = "contracted"
			break
		}
	}
	return register + "-" + englishCadenceKind(sentence)
}

// ExtractStyleRules rejects factual-only edits and emits a rule only when the same
// ending or length pattern appears in at least minEvidence independent aligned edits.
func ExtractStyleRules(edits []SentenceEdit, minEvidence, maxRules int) []ExtractedRule {
	return ExtractStyleRulesForLanguage(edits, minEvidence, maxRules, LanguageKorean)
}

func ExtractStyleRulesForLanguage(edits []SentenceEdit, minEvidence, maxRules int, language Language) []ExtractedRule {
	type pattern struct {
		before, after string
		citations     []string
	}
	patterns := map[string]*pattern{}
	for _, edit := range edits {
		if edit.Before == "" || edit.After == "" {
			continue
		}
		a, b := utf8.RuneCountInString(edit.Before), utf8.RuneCountInString(edit.After)
		if a > 0 && (b*100/a <= 70 || b*100/a >= 140) {
			key := "length:"
			direction := "shorter"
			if b > a {
				direction = "longer"
			}
			key += direction
			if patterns[key] == nil {
				patterns[key] = &pattern{before: "long sentences", after: direction + " sentences"}
			}
			patterns[key].citations = append(patterns[key].citations, edit.Before+" → "+edit.After)
			continue
		}
		from, to := endingOf(edit.Before), endingOf(edit.After)
		if language == LanguageEnglish {
			from, to = englishStyleEnding(edit.Before), englishStyleEnding(edit.After)
		}
		if from != to {
			key := "ending:" + from + ">" + to
			if patterns[key] == nil {
				patterns[key] = &pattern{before: from, after: to}
			}
			patterns[key].citations = append(patterns[key].citations, edit.Before+" → "+edit.After)
			continue
		}
	}
	keys := make([]string, 0, len(patterns))
	for key := range patterns {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ExtractedRule, 0, maxRules)
	for _, key := range keys {
		p := patterns[key]
		if len(p.citations) < minEvidence {
			continue
		}
		layer := LayerEndings
		if strings.HasPrefix(key, "length:") {
			layer = LayerSyntax
		}
		out = append(out, ExtractedRule{Statement: fmt.Sprintf("LLM uses %s, but I use %s", p.before, p.after), Layer: layer, Citations: p.citations})
		if len(out) == maxRules {
			break
		}
	}
	return out
}

func (s *Service) extractStyleRules(ctx context.Context, ref llm.ModelRef, edits []SentenceEdit) ([]ExtractedRule, error) {
	return s.extractStyleRulesForLanguage(ctx, ref, edits, LanguageKorean)
}

func (s *Service) extractStyleRulesForLanguage(ctx context.Context, ref llm.ModelRef, edits []SentenceEdit, language Language) ([]ExtractedRule, error) {
	eligible := make([]SentenceEdit, 0, len(edits))
	for _, edit := range edits {
		if strings.TrimSpace(edit.Before) != "" && strings.TrimSpace(edit.After) != "" && edit.Before != edit.After {
			eligible = append(eligible, edit)
		}
	}
	if len(eligible) < s.config.DiffMinPatternEdits {
		return nil, nil
	}
	payload, err := json.Marshal(struct {
		PromptVersion string         `json:"prompt_version"`
		Minimum       int            `json:"minimum_same_kind_edits"`
		Maximum       int            `json:"maximum_rules"`
		Edits         []SentenceEdit `json:"edits"`
	}{DiffRulePromptVersion, s.config.DiffMinPatternEdits, s.config.DiffMaxRules, eligible})
	if err != nil {
		return nil, err
	}
	prompt := diffRulePrompt
	if language == LanguageEnglish {
		prompt = englishDiffRulePrompt
	}
	response, err := s.models.Complete(ctx, ref, llm.Request{System: prompt, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(string(payload))}}}})
	if err != nil {
		return nil, err
	}
	var wire diffRulesWire
	if err = json.Unmarshal([]byte(strings.TrimSpace(response.Text)), &wire); err != nil {
		return nil, fmt.Errorf("diff rule extraction returned invalid JSON: %w", err)
	}
	if len(wire.Rules) > s.config.DiffMaxRules {
		return nil, fmt.Errorf("diff rule extraction exceeded the rule cap")
	}
	out := make([]ExtractedRule, 0, len(wire.Rules))
	seenStatements := map[string]struct{}{}
	for _, candidate := range wire.Rules {
		statement := strings.TrimSpace(candidate.Statement)
		if !strings.HasPrefix(statement, "LLM does ") || !strings.Contains(statement, ", but I do ") {
			return nil, fmt.Errorf("diff rule statement does not match the required shape")
		}
		layer := RuleLayer(candidate.Layer)
		if layer != LayerLexical && layer != LayerEndings && layer != LayerSyntax && layer != LayerStructure {
			return nil, fmt.Errorf("diff rule has an invalid layer")
		}
		indexes := uniqueValidIndexes(candidate.CitationIndexes, len(eligible))
		if len(indexes) < s.config.DiffMinPatternEdits {
			return nil, fmt.Errorf("diff rule has insufficient cited edits")
		}
		if !citationsHaveSameStylePatternForLanguage(layer, eligible, indexes, language) {
			return nil, fmt.Errorf("diff rule citations do not demonstrate one repeated style pattern")
		}
		key := canonicalRule(statement)
		if _, duplicate := seenStatements[key]; duplicate {
			continue
		}
		seenStatements[key] = struct{}{}
		citations := make([]string, 0, len(indexes))
		for _, index := range indexes {
			citations = append(citations, fmt.Sprintf("edit:%d", index))
		}
		out = append(out, ExtractedRule{Statement: statement, Layer: layer, Citations: citations})
	}
	return out, nil
}

func uniqueValidIndexes(values []int, length int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value < 0 || value >= length {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func citationsHaveSameStylePattern(layer RuleLayer, edits []SentenceEdit, indexes []int) bool {
	return citationsHaveSameStylePatternForLanguage(layer, edits, indexes, LanguageKorean)
}

func citationsHaveSameStylePatternForLanguage(layer RuleLayer, edits []SentenceEdit, indexes []int, language Language) bool {
	var signature string
	for _, index := range indexes {
		edit := edits[index]
		current := ""
		switch layer {
		case LayerEndings:
			from, to := endingOf(edit.Before), endingOf(edit.After)
			if language == LanguageEnglish {
				from, to = englishStyleEnding(edit.Before), englishStyleEnding(edit.After)
			}
			if from == to {
				return false
			}
			current = from + ">" + to
		case LayerSyntax:
			before, after := utf8.RuneCountInString(edit.Before), utf8.RuneCountInString(edit.After)
			if before == 0 || (after*100/before > 70 && after*100/before < 140) {
				return false
			}
			if after > before {
				current = "longer"
			} else {
				current = "shorter"
			}
		case LayerLexical, LayerStructure:
			// These dimensions require semantic interpretation. The strict citation
			// count and explicit analysis-model boundary are their evidence gate.
			current = string(layer)
		}
		if signature == "" {
			signature = current
		} else if signature != current {
			return false
		}
	}
	return signature != ""
}

func (s *Service) classifyRuleRelations(ctx context.Context, ref llm.ModelRef, candidates []ExtractedRule, existing []ContrastRule) ([]ExtractedRule, error) {
	return s.classifyRuleRelationsForLanguage(ctx, ref, candidates, existing, LanguageKorean)
}

func (s *Service) classifyRuleRelationsForLanguage(ctx context.Context, ref llm.ModelRef, candidates []ExtractedRule, existing []ContrastRule, language Language) ([]ExtractedRule, error) {
	for i := range candidates {
		candidate := &candidates[i]
		var comparable []ContrastRule
		for _, rule := range existing {
			if rule.Layer != candidate.Layer || rule.Status == RuleRejected {
				continue
			}
			if canonicalRule(rule.Statement) == canonicalRule(candidate.Statement) {
				candidate.MatchRuleID = rule.ID
				break
			}
			if rule.Status == RuleActive && rulesDirectlyContradict(rule.Statement, candidate.Statement) {
				candidate.ContradictsRuleID = rule.ID
				break
			}
			comparable = append(comparable, rule)
		}
		if candidate.MatchRuleID != "" || candidate.ContradictsRuleID != "" || len(comparable) == 0 {
			continue
		}
		payload, err := json.Marshal(struct {
			PromptVersion string         `json:"prompt_version"`
			Candidate     string         `json:"candidate"`
			Existing      []ContrastRule `json:"existing"`
		}{SemanticRulePromptVersion, candidate.Statement, comparable})
		if err != nil {
			return nil, err
		}
		prompt := semanticRulePrompt
		if language == LanguageEnglish {
			prompt = englishSemanticRulePrompt
		}
		response, err := s.models.Complete(ctx, ref, llm.Request{System: prompt, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(string(payload))}}}})
		if err != nil {
			return nil, err
		}
		var relation semanticRuleWire
		if err = json.Unmarshal([]byte(strings.TrimSpace(response.Text)), &relation); err != nil {
			return nil, fmt.Errorf("semantic rule classification returned invalid JSON: %w", err)
		}
		if relation.Relation == "distinct" && relation.RuleID == "" {
			continue
		}
		var matched *ContrastRule
		for j := range comparable {
			if comparable[j].ID == relation.RuleID {
				matched = &comparable[j]
				break
			}
		}
		if matched == nil {
			return nil, fmt.Errorf("semantic rule classification referenced an unknown rule")
		}
		switch relation.Relation {
		case "same":
			candidate.MatchRuleID = matched.ID
		case "contradicts":
			if matched.Status != RuleActive {
				return nil, fmt.Errorf("only an active rule can require contradiction confirmation")
			}
			candidate.ContradictsRuleID = matched.ID
		default:
			return nil, fmt.Errorf("semantic rule classification has an invalid relation")
		}
	}
	return candidates, nil
}

func rulesDirectlyContradict(a, b string) bool {
	strip := func(value string) string {
		value = canonicalRule(value)
		for _, negative := range []string{"않", "말", "금지", "not", "never"} {
			value = strings.ReplaceAll(value, negative, "")
		}
		return value
	}
	negative := func(value string) bool {
		value = strings.ToLower(value)
		return strings.Contains(value, "않") || strings.Contains(value, "말") || strings.Contains(value, "금지") || strings.Contains(value, " not ") || strings.Contains(value, "never")
	}
	return negative(a) != negative(b) && strip(a) == strip(b)
}
