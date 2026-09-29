package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const StructuredAnalysisPromptVersion = "voice-profile-v2"

// The prompt names every key the decoder expects — including the `axes` object and its six
// keys — because for a model without structured output the prompt is the only channel that
// carries the shape; a key it is not told about comes back missing and must publish as unknown.
const structuredAnalysisPrompt = `Analyze the supplied Korean authored corpus as writing style, not subject matter.
Return one JSON object with these string keys: lexical_description, base_register, connective_style, intro_pattern,
closing_pattern, heading_habit, list_habit, emoji_use. Use an empty string for unsupported traits.
lexical_description is the Korean style guide itself, in Korean plain text:
` + analysisGuideSections + `
Also return an "axes" object with exactly these six integer keys, each between -3 and 3:
involvement, narrativity, persuasion_overtness, abstractness, addressee_focus, humor.
Omit an axis key only when the corpus gives no evidence for it; never guess 0 as a filler.
Never return topic-specific nouns as preferred vocabulary. Deterministic ending distribution and sentence length are calculated separately and override your estimates.`

// completeAnalysis is the one analysis call analyze_voice makes (VOICE-23, VOICE-27):
// it names every key it expects, attaches the embedded schema when the resolved model declares
// structured output, and refuses an answer whose lexical description is not the nine-section
// style guide (VOICE-25).
func (s *Service) completeAnalysis(ctx context.Context, ref llm.ModelRef, corpus string) (qualitativeJSON, error) {
	request := llm.Request{
		System:   structuredAnalysisPrompt,
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(corpus)}}},
		// Named so the registry can resolve the operator's style-analysis override. No
		// Reasoning is set: analysis sends no `reasoning` key by default, which is the model's
		// own adaptive behavior and the most permissive setting — not "off".
		Stage: llm.StageNameAnalyze,
	}
	if info, ok := s.models.Resolve(ref); ok && info.StructuredOutput {
		request.JSONSchema = VoiceAnalysisSchema()
	}
	response, err := s.models.Complete(ctx, ref, request)
	if err != nil {
		return qualitativeJSON{}, err
	}
	var qualitative qualitativeJSON
	if err = json.Unmarshal([]byte(strings.TrimSpace(response.Text)), &qualitative); err != nil {
		return qualitativeJSON{}, fmt.Errorf("typed voice analysis returned invalid JSON: %w", err)
	}
	qualitative.LexicalDescription = strings.TrimSpace(qualitative.LexicalDescription)
	if !hasRequiredAnalysisShape(qualitative.LexicalDescription) {
		return qualitativeJSON{}, fmt.Errorf("문체 분석 결과에 종결어미 또는 never uses 섹션이 없어요. 다시 시도해 주세요")
	}
	return qualitative, nil
}

// analyzedValue is a qualitative answer as a profile value: an empty one is unknown.
func analyzedValue(v string) VoiceValue {
	if strings.TrimSpace(v) == "" {
		return VoiceValue{Unknown: true, Source: SourceUnknown}
	}
	return VoiceValue{Value: strings.TrimSpace(v), Source: SourceAnalyzed}
}

type qualitativeAxesJSON struct {
	Involvement         *int `json:"involvement"`
	Narrativity         *int `json:"narrativity"`
	PersuasionOvertness *int `json:"persuasion_overtness"`
	Abstractness        *int `json:"abstractness"`
	AddresseeFocus      *int `json:"addressee_focus"`
	Humor               *int `json:"humor"`
}
type qualitativeJSON struct {
	LexicalDescription string              `json:"lexical_description"`
	BaseRegister       string              `json:"base_register"`
	ConnectiveStyle    string              `json:"connective_style"`
	IntroPattern       string              `json:"intro_pattern"`
	ClosingPattern     string              `json:"closing_pattern"`
	HeadingHabit       string              `json:"heading_habit"`
	ListHabit          string              `json:"list_habit"`
	EmojiUse           string              `json:"emoji_use"`
	Axes               qualitativeAxesJSON `json:"axes"`
}

func mergeQualitativeProfile(profile *StructuredProfile, qualitative qualitativeJSON, unknown func(string) VoiceValue) {
	profile.Lexical.Description = unknown(qualitative.LexicalDescription)
	// The measured register wins (VOICE-26, VOICE-27): the model fills it only where
	// measurement had none, and an empty answer adds nothing.
	if profile.Endings.BaseRegister.Unknown {
		profile.Endings.BaseRegister = unknown(qualitative.BaseRegister)
	}
	profile.Syntax.ConnectiveStyle = unknown(qualitative.ConnectiveStyle)
	profile.Structure.IntroPattern = unknown(qualitative.IntroPattern)
	profile.Structure.ClosingPattern = unknown(qualitative.ClosingPattern)
	profile.Structure.HeadingHabit = unknown(qualitative.HeadingHabit)
	profile.Structure.ListHabit = unknown(qualitative.ListHabit)
	profile.Structure.EmojiUse = unknown(qualitative.EmojiUse)
	profile.Axes = AxesProfile{Involvement: qualitative.Axes.Involvement, Narrativity: qualitative.Axes.Narrativity, PersuasionOvertness: qualitative.Axes.PersuasionOvertness, Abstractness: qualitative.Axes.Abstractness, AddresseeFocus: qualitative.Axes.AddresseeFocus, Humor: qualitative.Axes.Humor}
}

func validateAxes(a AxesProfile) error {
	for _, axis := range a.AxisValues() {
		if axis.Value != nil && (*axis.Value < -3 || *axis.Value > 3) {
			return fmt.Errorf("voice axis %s is outside -3..3", axis.Key)
		}
	}
	return nil
}
