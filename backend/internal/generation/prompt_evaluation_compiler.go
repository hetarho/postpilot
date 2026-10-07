package generation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// PromptEvaluationCompilerVersion identifies the developer-only LANG-29 condition.
// It is not an output language, persisted job option, or MODEL-30 test factor.
const PromptEvaluationCompilerVersion = "common-instruction-language-v1"

type PromptInstructionLanguage string

const (
	PromptInstructionsKorean        PromptInstructionLanguage = "korean-baseline"
	PromptInstructionsEnglishCommon PromptInstructionLanguage = "english-common-instructions"
)

type instructionFragmentPair struct{ korean, english string }

// Only common task and format sentences have counterparts here. The production
// constants own their Korean text; this condition deliberately leaves source,
// precedence, revision scope, voice, example, tag and output-language policies
// in their original language. No material is translated or reparsed.
func commonInstructionPairs(mode string) []instructionFragmentPair {
	blockFormat := []instructionFragmentPair{
		{koreanBlockFields, englishBlockFields},
		{koreanBlockFieldContract, englishBlockFieldContract},
	}
	switch mode {
	case "direct", "full-test-direct", "frozen-storyline":
		format := koreanWriteFormat
		if mode == "frozen-storyline" {
			format = strings.Replace(format, `{"storyline":[{"text":"...","files":[]}],"title"`, `{"title"`, 1)
		}
		return append([]instructionFragmentPair{
			{koreanWriteTask, "Write a Korean blog post from the attached photo observations and memo."},
			{koreanParagraphFormat, "Use exactly one TEXT block for each paragraph."},
			{format, englishJSONFormat(format)},
		}, blockFormat...)
	case "storyline-create", "storyline-rewrite":
		return []instructionFragmentPair{{koreanStorylineTask, englishStorylineTask}, {koreanStorylineFormat, englishStorylineFormat}}
	case "revision":
		return append([]instructionFragmentPair{
			{koreanRevisionTask, "Apply only the user's requested edit to the current blog post, with the smallest possible change."},
			{koreanRevisionFormat, `Return a complete replacement PostContent, not a diff: exactly one {"title":"...","summary":"...","tags":[],"blocks":[]} JSON object with no explanation or Markdown.`},
		}, blockFormat...)
	case "photo-observation", "full-test-photo-observation":
		return []instructionFragmentPair{
			{koreanPhotoObservationTask, "Return identifiable visual evidence matched exactly to each photo's filename."},
			{koreanPhotoObservationFormat, englishJSONFormat(koreanPhotoObservationFormat)},
		}
	case "video-observation", "full-test-video-observation":
		return []instructionFragmentPair{
			{koreanVideoObservationTask, "Return identifiable visual and audible evidence from the video matched exactly to its filename."},
			{koreanVideoObservationFormat, englishJSONFormat(koreanVideoObservationFormat)},
		}
	default:
		return nil
	}
}

func englishJSONFormat(korean string) string {
	shape := strings.TrimSuffix(strings.TrimPrefix(korean, "출력은 설명이나 마크다운 없이 "), " 형태의 JSON 객체 하나여야 합니다.")
	return "Return exactly one JSON object shaped as " + shape + " with no explanation or Markdown."
}

func evaluationContractFragment(id string) bool {
	switch id {
	case "write-contract.frame.0", "plan-contract.frame.0", "revision-contract.frame.0", "photo-contract.frame.0", "video-contract.frame.0":
		return true
	default:
		return false
	}
}

// compilePromptEvaluationRequest accepts the final application request and uses
// its emitted code-owned fragment boundaries as compiler inputs. It cannot read
// owner sources, resolve settings, call a provider, or alter user message bytes.
// Production request preparation never calls this developer compiler.
func compilePromptEvaluationRequest(request llm.Request, condition PromptInstructionLanguage) (llm.Request, error) {
	if condition != PromptInstructionsKorean && condition != PromptInstructionsEnglishCommon {
		return llm.Request{}, fmt.Errorf("unsupported instruction-language evaluation condition %q", condition)
	}
	if request.Composition == nil {
		return llm.Request{}, fmt.Errorf("instruction-language evaluation requires an actual composition")
	}
	var baseline strings.Builder
	for _, fragment := range request.Composition.Fragments {
		if fragment.Role == llm.InspectionRoleSystem {
			baseline.WriteString(fragment.Text)
		}
	}
	if baseline.String() != request.System {
		return llm.Request{}, fmt.Errorf("instruction-language evaluation composition differs from the actual system prompt")
	}
	if condition == PromptInstructionsKorean {
		return request, nil
	}
	pairs := commonInstructionPairs(request.Composition.Mode)
	if len(pairs) == 0 {
		return llm.Request{}, fmt.Errorf("no common instruction condition for mode %q", request.Composition.Mode)
	}
	composition := *request.Composition
	composition.Fragments = append([]llm.RequestFragment(nil), composition.Fragments...)
	composition.SourceFiles = append([]string(nil), composition.SourceFiles...)
	if !slices.Contains(composition.SourceFiles, "internal/generation/prompt_evaluation_compiler.go") {
		composition.SourceFiles = append(composition.SourceFiles, "internal/generation/prompt_evaluation_compiler.go")
	}
	composition.PromptVersion += "+" + PromptEvaluationCompilerVersion
	seen := make([]int, len(pairs))
	var system strings.Builder
	for i, fragment := range composition.Fragments {
		if evaluationContractFragment(fragment.ID) {
			if fragment.Role != llm.InspectionRoleSystem || fragment.Authorship != llm.FragmentAuthorshipCode {
				return llm.Request{}, fmt.Errorf("instruction-language evaluation contract is not code-owned system material")
			}
			for j, pair := range pairs {
				seen[j] += strings.Count(fragment.Text, pair.korean)
				fragment.Text = strings.ReplaceAll(fragment.Text, pair.korean, pair.english)
			}
			fragment.SourceFiles = append(append([]string(nil), fragment.SourceFiles...), "internal/generation/prompt_evaluation_compiler.go")
			composition.Fragments[i] = fragment
		}
		if fragment.Role == llm.InspectionRoleSystem {
			system.WriteString(fragment.Text)
		}
	}
	for i, count := range seen {
		if count != 1 {
			return llm.Request{}, fmt.Errorf("common instruction fragment %d occurs %d times in mode %q", i, count, composition.Mode)
		}
	}
	request.System, request.Composition = system.String(), &composition
	return request, nil
}
