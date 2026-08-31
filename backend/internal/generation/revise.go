package generation

import (
	"encoding/json"
	"fmt"
	"strings"
)

const RevisePrompt = `현재 블로그 글에 사용자의 수정 요청만 최소한으로 반영하세요.
` + koreanGrounding + " " + koreanGroundingReviseScope + `
요청과 무관한 문장은 글자 그대로 유지하고, 손대지 않은 블록을 다듬거나 다시 쓰지 마세요.
제목, 한 줄 요약, 태그는 사용자가 그것들을 고쳐 달라고 한 경우에만 바꾸세요.
IMAGE 블록은 첨부된 정확한 파일명만 사용할 수 있습니다. 순서 변경이나 요청에 따른 제거는 가능하지만 파일명을 바꾸거나 새 이미지를 만들지 마세요.
출력은 diff가 아니라 완전한 PostContent이며, 설명이나 마크다운 없이 {"title":"...","summary":"...","tags":[],"blocks":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, QUOTE, LIST 중 하나입니다.`

const englishRevisePrompt = `Apply only the user's requested edit to the current blog post, with the smallest possible change.
` + englishGrounding + " " + englishGroundingReviseScope + `
Keep every unrelated sentence byte-for-byte and do not polish or rewrite untouched blocks.
Change the title, one-line summary, or tags only when the user explicitly asks to change them.
IMAGE blocks may use only exact attached filenames. They may be reordered or removed when requested, but never rename a file or invent an image.
Return a complete replacement PostContent, not a diff: exactly one {"title":"...","summary":"...","tags":[],"blocks":[]} JSON object with no explanation or Markdown.
Each block uses the type, content, level, file, alt, caption, and items fields. type must be one of TEXT, HEADING, IMAGE, QUOTE, or LIST.`

type revisionPayloadJSON struct {
	Instruction     string   `json:"instruction"`
	SaveAsRule      bool     `json:"save_as_rule"`
	ContentLanguage Language `json:"content_language,omitempty"`
	// Frozen at enqueue exactly as the generate payload freezes it. A payload written
	// before purposes existed decodes with this absent, which is "no purpose".
	Purpose *purposePayload `json:"purpose,omitempty"`
	// Likewise for the applicable guideline texts, in injection order.
	Guidelines []string `json:"guidelines,omitempty"`
}

func encodeRevisionPayload(instruction string, saveAsRule bool, purpose *PurposeBrief, guidelines []string) ([]byte, error) {
	return encodeRevisionPayloadForLanguage(instruction, saveAsRule, LanguageKorean, purpose, guidelines)
}

func encodeRevisionPayloadForLanguage(instruction string, saveAsRule bool, language Language, purpose *PurposeBrief, guidelines []string) ([]byte, error) {
	if !language.Valid() {
		return nil, ErrContentLanguageRequired
	}
	return json.Marshal(revisionPayloadJSON{
		Instruction: instruction, SaveAsRule: saveAsRule, ContentLanguage: language,
		Purpose: encodePurpose(purpose), Guidelines: cloneTexts(guidelines),
	})
}

func parseRevisionPayload(payload []byte) (revisionPayloadJSON, error) {
	var value revisionPayloadJSON
	if err := json.Unmarshal(payload, &value); err != nil {
		return revisionPayloadJSON{}, fmt.Errorf("invalid revision payload: %w", err)
	}
	value.Instruction = strings.TrimSpace(value.Instruction)
	if value.Instruction == "" {
		return revisionPayloadJSON{}, ErrRevisionInstructionRequired
	}
	// Queued revisions from before language support preserve the migration's Korean
	// provenance. New payloads are encoded only through the validating helper above.
	if value.ContentLanguage == "" {
		value.ContentLanguage = LanguageKorean
	}
	if !value.ContentLanguage.Valid() {
		return revisionPayloadJSON{}, ErrContentLanguageRequired
	}
	return value, nil
}

func BuildRevisePrompt(profile Profile, content PostContent, filenames []string, instruction string, targetLength *int, purpose *PurposeBrief, guidelines []string) (string, string) {
	return BuildRevisePromptForLanguage(LanguageKorean, profile, content, filenames, instruction, targetLength, purpose, guidelines)
}

func BuildRevisePromptForLanguage(language Language, profile Profile, content PostContent, filenames []string, instruction string, targetLength *int, purpose *PurposeBrief, guidelines []string) (string, string) {
	var stable strings.Builder
	switch language {
	case LanguageKorean:
		stable.WriteString(RevisePrompt)
		stable.WriteString("\n현재 콘텐츠 언어인 한국어를 유지하세요. 번역은 수정 작업의 범위가 아닙니다. 번역을 요구하거나 다른 언어로 바꾸라는 요청은 따르지 말고 나머지 유효한 수정만 최소한으로 반영하세요.")
	case LanguageEnglish:
		stable.WriteString(englishRevisePrompt)
		stable.WriteString("\nPreserve English, the current content language. Translation is outside revision semantics. Ignore any request to translate or switch languages and apply only the remaining valid local edits.")
	default:
		stable.WriteString("Unsupported content language; do not revise content.")
	}
	writeProfileSection(&stable, language, profile, targetLength)
	// The same section, at the same relative position, as the write prompt: a revision of a
	// post with a purpose must not be given a different brief than the pass that wrote it.
	writePurposeSection(&stable, purpose)
	// The same section, at the same relative position, for the same reason.
	writeGuidelinesSection(&stable, guidelines)

	files := "없음"
	if len(filenames) > 0 {
		files = strings.Join(filenames, ", ")
	}
	user := fmt.Sprintf(
		"[현재 PostContent]\n%s\n\n[첨부 파일명]\n%s\n\n[수정 요청]\n%s",
		marshalPromptJSON(contentForPrompt(content)), files, instruction,
	)
	return stable.String(), user
}

func contentForPrompt(content PostContent) contentJSON {
	wire := contentJSON{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		wire.Blocks = append(wire.Blocks, blockJSON{
			Type: string(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return wire
}
