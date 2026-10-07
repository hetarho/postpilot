package generation

import (
	"fmt"
	"strings"
)

// The storyline prompt's static rules (GEN-68): the task, the paragraph rule, the answer shape
// and the output-language sentence. It carries no voice section — a storyline is a plan, not
// prose in the owner's voice — so nothing here names the voice profile.
const koreanStorylineTask = "아래 재료로 블로그 글의 스토리라인을 짜세요. 본문은 아직 쓰지 않습니다."

const englishStorylineTask = "Plan the storyline of a blog post from the material below. Do not write the post yet."

// The write's storyline rule without its last sentence (the post follows the storyline), plus
// how a paragraph reads: a plan, not prose.
var (
	koreanStorylineParagraphRule = strings.TrimSuffix(koreanStorylineRule, " 본문은 이 storyline을 따라 쓰세요.") +
		" 각 문단은 계획을 말하는 문장(…를 보여줍니다, …를 이야기합니다)으로 쓰세요."
	englishStorylineParagraphRule = strings.TrimSuffix(englishStorylineRule, " Then write the post along this storyline.") +
		" Write each paragraph as sentences that state the plan (it shows …, it tells …)."
)

const storylineAnswerShape = `{"storyline":[{"text":"...","files":[]}]}`

const koreanStorylineLanguage = "출력 언어는 한국어입니다. storyline의 text를 한국어로 작성하세요. 템플릿, 메모, 가제의 언어 지시가 충돌해도 이 출력 언어를 우선하세요."

const englishStorylineLanguage = "The output language is English. Write every storyline text in English. This requirement overrides conflicting language instructions in the template, memo, or title hint."

// The request prompt's rule (GEN-69): the stored storyline changes as the request asks and only
// there, and every attachment still stands in exactly one paragraph.
const koreanStorylineRequestRule = "[현재 스토리라인]을 [수정 요청]대로 고치세요. 요청이 다루지 않는 문단과 사진 배치는 그대로 두고, 첨부 사진과 영상은 모두 정확히 한 문단에 한 번씩 두세요."

const koreanPlanMeaningContract = "스토리라인은 앞으로 쓸 내용과 배열의 계획입니다. 문단 text는 계획을 말하고 files는 첨부 배치를 나타내며, 계획 문장이나 그 배열을 실제 경험·사건의 근거로 확정하지 않습니다. 작성할 문단과 files만 반환하고 최종 제목·태그·본문 블록을 작성하지 마세요. 기존 계획을 선택하거나 표현만 고쳐도 다른 AI 주장이 사용자 사실로 바뀌지 않습니다. 수정 요청에 새 사실이 명시적으로 제공된 경우와 배열 승인만을 구분하세요."
const englishPlanMeaningContract = "A storyline proposes content and arrangement: paragraph text states a plan and files assigns attachment placement. Plan wording or arrangement is not evidence of actual experience or events. Return only the planned paragraphs and files, never final title, tags, or prose blocks. Selecting a plan or rewording it does not turn other AI claims into owner facts. Distinguish explicitly supplied new edit facts from approval of arrangement alone."

const englishStorylineRequestRule = "Revise [현재 스토리라인] as [수정 요청] asks. Leave the paragraphs and the photo placement the request does not touch as they are, and keep every attached photo and video in exactly one paragraph."

// storylineGuidelinePrecedence closes [작문 지침] in the storyline prompts (GUIDE-15): the same
// ranking as the write's, without the voice, which the storyline prompt does not carry. Korean
// for every target, like the section's heading and labels.
const storylineGuidelinePrecedence = "지침은 이 스토리라인을 어떻게 짤지 정합니다. 지침이 템플릿과 충돌하면 지침을 우선하세요. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요."

// StorylinePromptInput is everything the storyline prompts read. Request and Current are the
// request prompt's own (GEN-69); a storyline job leaves both empty.
type StorylinePromptInput struct {
	Language          Language
	Title             string
	Memo              string
	Photos            []string
	Videos            []string
	Observations      []Observation
	Template          *TemplateBrief
	DefaultGuidelines []string
	StockGuidelines   []StockGuideline
	Guidelines        []string
	Memories          []string
	// Current is the stored storyline the request rewrites; Request is the owner's request.
	Current []StorylineParagraph
	Request string
}

// BuildStorylinePromptForLanguage builds a storyline job's system and user prompts (GEN-68):
// the static rules, then the template form, then [작문 지침], and in the user half the post's
// 가제 and 메모, the opted-in memories and the attachment material. With a Request it is the
// request prompt (GEN-69), which adds the request rule and, after the material, the current
// storyline and the request.
func BuildStorylinePromptForLanguage(input StorylinePromptInput) (string, string) {
	var stable strings.Builder
	revising := input.Request != ""
	switch input.Language {
	case LanguageKorean:
		stable.WriteString(koreanStorylineTask)
		stable.WriteString("\n" + koreanStorylineParagraphRule)
		stable.WriteString("\n출력은 설명이나 마크다운 없이 " + storylineAnswerShape + " 형태의 JSON 객체 하나여야 합니다.")
		stable.WriteString("\n" + koreanStorylineLanguage)
		stable.WriteString("\n" + koreanPlanMeaningContract)
		if revising {
			stable.WriteString("\n" + koreanStorylineRequestRule)
		}
	case LanguageEnglish:
		stable.WriteString(englishStorylineTask)
		stable.WriteString("\n" + englishStorylineParagraphRule)
		stable.WriteString("\nReturn exactly one JSON object shaped as " + storylineAnswerShape + " with no explanation or Markdown.")
		stable.WriteString("\n" + englishStorylineLanguage)
		stable.WriteString("\n" + englishPlanMeaningContract)
		if revising {
			stable.WriteString("\n" + englishStorylineRequestRule)
		}
	default:
		stable.WriteString("Unsupported output language; do not plan a storyline.")
	}
	writeSourceHonestyContract(&stable, input.Language)
	if input.Template != nil {
		writeTemplateForm(&stable, input.Template, "")
	}
	writeGuidelinesSectionClosedBy(&stable, selectStockGuidelines(input.StockGuidelines, input.DefaultGuidelines, "storyline"), input.Guidelines, storylineGuidelinePrecedence)

	user := fmt.Sprintf("[이번 글]\n가제: %s\n메모: %s\n%s%s", input.Title, input.Memo, memorySection(input.Memories), attachmentMaterial(input.Photos, input.Videos, input.Observations, nil))
	if revising {
		user += fmt.Sprintf("\n\n[현재 스토리라인]\n%s\n\n[수정 요청]\n%s", marshalPromptJSON(storylineForPrompt(input.Current)), input.Request)
	}
	return stable.String(), user
}

// storylineForPrompt is the stored storyline in the answer's own shape, so the request prompt
// shows the model exactly what it is asked to return, changed.
func storylineForPrompt(paragraphs []StorylineParagraph) map[string][]storylineParagraphJSON {
	wire := make([]storylineParagraphJSON, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		files := paragraph.Files
		if files == nil {
			files = []string{}
		}
		wire = append(wire, storylineParagraphJSON{Text: paragraph.Text, Files: files})
	}
	return map[string][]storylineParagraphJSON{"storyline": wire}
}
