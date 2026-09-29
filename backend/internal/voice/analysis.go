package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// analysisPrompt is the one analysis call's instruction (VOICE-23, VOICE-24): the product has
// counted the fingerprint already, and the call writes only what cannot be counted.
const analysisPrompt = `당신은 한 사람의 글버릇을 읽는 분석가입니다. 아래 학습 글은 모두 같은 사람이 직접 쓴 글이고, 제품이 이미 센 습관 수치가 함께 있습니다. 수치는 다시 세지 말고, 셀 수 없는 것만 쓰세요. impression: 이 사람의 글이 주는 전체 인상을 1~2문장으로. tics: 자주 나오는 말버릇과 그 말이 나오는 상황(최대 5개). signature_phrases: 이 사람만의 표현(최대 8개) — 음식·장소·상품 이름처럼 주제에 딸린 단어는 빼세요. examples: 위 셋을 보여 주는 문장을 학습 글에서 그대로 인용하고, 없는 문장은 만들지 마세요. 근거가 약하면 빈 목록으로 두세요.
JSON 객체 하나로만 답하세요: {"impression": 문자열, "tics": [{"phrase": 문자열, "when": 문자열}], "signature_phrases": [문자열], "examples": [{"field": "impression" | "tics" | "signature_phrases", "sentence": 문자열}]}`

// The AI part's ceilings: the first two sentences of the impression, five tics, eight phrases.
const (
	maxImpressionSentences = 2
	maxTics                = 5
	maxSignaturePhrases    = 8
)

var errIncompleteAnalysis = errors.New("말투 분석 결과에 빠진 항목이 있어요. 다시 시도해 주세요")

// analysisInput is the call's user message: the counted summary, then the 학습 글's prose.
func analysisInput(counted Fingerprint, samples []Sample) string {
	var b strings.Builder
	b.WriteString("[제품이 센 습관]\n")
	for _, line := range countedLines(counted) {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n[학습 글]\n")
	b.WriteString(AssembleCorpus(samples))
	return b.String()
}

// completeAnalysis makes the one analysis call and reads its AI part. It attaches the embedded
// schema when the resolved model declares structured output (VOICE-27) and fails a result
// missing any field (VOICE-24). Examples not found verbatim in a 학습 글 are dropped.
func (s *Service) completeAnalysis(ctx context.Context, ref llm.ModelRef, counted Fingerprint, samples []Sample) (AIPart, error) {
	request := llm.Request{
		System:   analysisPrompt,
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(analysisInput(counted, samples))}}},
		// Named so the registry can resolve the operator's analysis override. No Reasoning is
		// set: analysis sends no `reasoning` key by default.
		Stage: llm.StageNameAnalyze,
	}
	if info, ok := s.models.Resolve(ref); ok && info.StructuredOutput {
		request.JSONSchema = VoiceAnalysisSchema()
	}
	response, err := s.models.Complete(ctx, ref, request)
	if err != nil {
		return AIPart{}, err
	}
	return parseAIPart(response.Text, samples)
}

type aiAnswer struct {
	Impression       *string   `json:"impression"`
	Tics             *[]Tic    `json:"tics"`
	SignaturePhrases *[]string `json:"signature_phrases"`
	Examples         *[]struct {
		Field    string `json:"field"`
		Sentence string `json:"sentence"`
	} `json:"examples"`
}

func parseAIPart(text string, samples []Sample) (AIPart, error) {
	var answer aiAnswer
	if err := json.Unmarshal([]byte(stripCodeFence(text)), &answer); err != nil {
		return AIPart{}, fmt.Errorf("말투 분석 결과를 읽지 못했어요: %w", err)
	}
	if answer.Impression == nil || answer.Tics == nil || answer.SignaturePhrases == nil || answer.Examples == nil {
		return AIPart{}, errIncompleteAnalysis
	}
	part := AIPart{Impression: firstSentences(strings.TrimSpace(*answer.Impression), maxImpressionSentences)}
	for _, tic := range *answer.Tics {
		tic.Phrase, tic.When = strings.TrimSpace(tic.Phrase), strings.TrimSpace(tic.When)
		if tic.Phrase != "" && len(part.Tics) < maxTics {
			part.Tics = append(part.Tics, tic)
		}
	}
	for _, phrase := range *answer.SignaturePhrases {
		if phrase = strings.TrimSpace(phrase); phrase != "" && len(part.SignaturePhrases) < maxSignaturePhrases {
			part.SignaturePhrases = append(part.SignaturePhrases, phrase)
		}
	}
	for _, example := range *answer.Examples {
		field := AIField(example.Field)
		if field != AIImpression && field != AITics && field != AISignaturePhrases {
			continue
		}
		if id := verbatimSource(example.Sentence, samples); id != "" {
			part.Examples = append(part.Examples, AIExample{Field: field, Sentence: strings.TrimSpace(example.Sentence), MaterialID: id})
		}
	}
	return part, nil
}

// verbatimSource is the 학습 글 whose prose holds the sentence word for word once whitespace is
// collapsed on both sides, or "" — an example the AI did not quote is dropped (VOICE-26).
func verbatimSource(sentence string, samples []Sample) string {
	want := collapseSpace(sentence)
	if want == "" {
		return ""
	}
	for _, sample := range samples {
		if strings.Contains(collapseSpace(ProseText(sample.Body)), want) {
			return sample.ID
		}
	}
	return ""
}

func collapseSpace(value string) string { return strings.Join(strings.Fields(value), " ") }

func firstSentences(text string, n int) string {
	sentences := SegmentSentences(text)
	if len(sentences) > n {
		sentences = sentences[:n]
	}
	return strings.Join(sentences, " ")
}

// stripCodeFence reads a model answer wrapped in a ``` fence the way it reads a bare one.
func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimPrefix(text, "json")
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))
}
