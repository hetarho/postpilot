package generation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/post"
)

// koreanReviseScope / englishReviseScope qualify the minimality rule directly above them,
// because without a qualifier that rule loses whole-post requests. Several sentences in this
// prompt — the opening 최소한, the byte-for-byte line and the [작문 지침] scope sentence — all
// push toward touching as little as possible, so
// "톤을 바꿔줘" or "구성을 다시 잡아줘" was answered with one local edit and the post's shape
// never moved. This says what minimality is measured against: the request's scope, not the
// number of blocks. It licenses nothing outside that scope, which is the property to keep.
const koreanReviseScope = "요청이 글 전체를 대상으로 하면(구성, 순서, 분량, 어조, 전체 흐름) 그 범위 전체를 다시 쓰세요. 최소한이라는 것은 요청을 좁게 해석하라는 뜻이 아니라 요청 범위 밖을 건드리지 말라는 뜻입니다."

const englishReviseScope = "When the request addresses the post as a whole — its structure, order, length, tone, or overall flow — rewrite that entire scope. The smallest possible change means leaving everything outside the request alone, not reading the request narrowly."

// koreanReviseLiteral / englishReviseLiteral: the instruction arrives as free prose at the end
// of the user message, and nothing told the model what it IS. A request phrased as a statement
// — "주차장 넓어서 좋았음" — then reads as text to insert, and the post gains the sentence the
// user typed, typos and all. This is the same move templateFactLegend makes for <facts>: name
// what the material is before the model decides what to do with it.
const koreanReviseLiteral = "수정 요청문은 지시이지 본문이 아닙니다. 요청 문장이나 그 표기와 오타를 글에 그대로 옮기지 말고, 요청이 말하는 내용을 위 말투 프로필에 맞춰 다시 쓰세요. 사용자가 따옴표로 정확한 문구를 지정했을 때만 그 문구를 그대로 씁니다."

const englishReviseLiteral = "The request is an instruction, not body text. Never copy its sentences, wording, or typos into the post: write what it asks for in the voice profile above. Reproduce an exact phrase only when the user quoted it as the words to use."

// koreanReviseLiteralNoVoice / englishReviseLiteralNoVoice say the same for a post with 말투
// 없음, which has no profile above to write in (GEN-74): the post's own style is the reference.
const koreanReviseLiteralNoVoice = "수정 요청문은 지시이지 본문이 아닙니다. 요청 문장이나 그 표기와 오타를 글에 그대로 옮기지 말고, 요청이 말하는 내용을 현재 글의 문체에 맞춰 다시 쓰세요. 사용자가 따옴표로 정확한 문구를 지정했을 때만 그 문구를 그대로 씁니다."

const englishReviseLiteralNoVoice = "The request is an instruction, not body text. Never copy its sentences, wording, or typos into the post: write what it asks for in the style of the current post. Reproduce an exact phrase only when the user quoted it as the words to use."

const RevisePrompt = `현재 블로그 글에 사용자의 수정 요청만 최소한으로 반영하세요.
요청과 무관한 문장은 글자 그대로 유지하고, 손대지 않은 블록을 다듬거나 다시 쓰지 마세요.
` + koreanReviseScope + `
` + koreanReviseLiteral + `
제목, 한 줄 요약, 태그는 사용자가 그것들을 고쳐 달라고 한 경우에만 바꾸세요.
IMAGE와 GALLERY 블록은 첨부된 정확한 파일명만 사용할 수 있습니다. 요청이 있으면 순서를 바꾸거나 사진을 묶거나 나누거나 뺄 수 있지만, 파일명을 바꾸거나 새 이미지를 만들지 마세요.
` + koreanGalleryRule + `
출력은 diff가 아니라 완전한 PostContent이며, 설명이나 마크다운 없이 {"title":"...","summary":"...","tags":[],"blocks":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, files, layout, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, GALLERY, QUOTE, LIST 중 하나입니다.` + "\n" + koreanBlockFieldContract + "\n" + koreanRevisionHonestyContract

const englishRevisePrompt = `Apply only the user's requested edit to the current blog post, with the smallest possible change.
Keep every unrelated sentence byte-for-byte and do not polish or rewrite untouched blocks.
` + englishReviseScope + `
` + englishReviseLiteral + `
Change the title, one-line summary, or tags only when the user explicitly asks to change them.
IMAGE and GALLERY blocks may use only exact attached filenames. When requested, photos may be reordered, grouped, split or removed, but never rename a file or invent an image.
` + englishGalleryRule + `
Return a complete replacement PostContent, not a diff: exactly one {"title":"...","summary":"...","tags":[],"blocks":[]} JSON object with no explanation or Markdown.
Each block uses the type, content, level, file, files, layout, alt, caption, and items fields. type must be one of TEXT, HEADING, IMAGE, GALLERY, QUOTE, or LIST.` + "\n" + englishBlockFieldContract + "\n" + englishRevisionHonestyContract

// A payload written while 규칙으로 저장 existed still carries `save_as_rule`; it decodes
// because encoding/json ignores a key the struct no longer names.
type revisionPayloadJSON struct {
	OriginProtocolVersion int             `json:"origin_protocol_version,omitempty"`
	CompletionTokens      int             `json:"completion_tokens,omitempty"`
	Profile               *profilePayload `json:"voice_profile,omitempty"`
	Instruction           string          `json:"instruction"`
	ContentLanguage       Language        `json:"content_language,omitempty"`
	// Frozen at enqueue exactly as the generate payload freezes it. A payload written
	// before templates existed decodes with this absent, which is "no template".
	Template *templatePayload `json:"template,omitempty"`
	// Likewise for the owner's applicable guideline texts, in injection order, and the enabled
	// 기본 지침 ahead of them; absent is none (GUIDE-17).
	Guidelines        []string              `json:"guidelines,omitempty"`
	StockGuidelines   *[]stockGuidelineJSON `json:"stock_guidelines,omitempty"`
	DefaultGuidelines []string              `json:"default_guidelines,omitempty"`
	// Frozen at Start like the brief (GEN-46); a payload from before the member decodes 0,
	// which the handler resolves to the default.
	TagCount          int  `json:"tag_count,omitempty"`
	WriteNativeEffort bool `json:"write_native_effort,omitempty"`
}

func encodeRevisionPayload(instruction string, template *TemplateBrief, guidelines []string) ([]byte, error) {
	return encodeRevisionPayloadForLanguage(instruction, LanguageKorean, template, FrozenGuidelines{Owner: guidelines}, post.TagCountRange.Default, false)
}

func encodeRevisionPayloadForLanguage(instruction string, language Language, template *TemplateBrief, guidelines FrozenGuidelines, tagCount int, nativeEffort bool, frozen ...Profile) ([]byte, error) {
	if !language.Valid() {
		return nil, ErrContentLanguageRequired
	}
	var profile *profilePayload
	if len(frozen) > 0 {
		profile = encodeProfile(&frozen[0])
	}
	return json.Marshal(revisionPayloadJSON{
		Profile:     profile,
		Instruction: instruction, ContentLanguage: language,
		Template: encodeTemplate(template), Guidelines: cloneTexts(guidelines.Owner),
		DefaultGuidelines: cloneTexts(guidelines.Defaults), StockGuidelines: encodeStockGuidelines(guidelines.Stock),
		TagCount: tagCount, WriteNativeEffort: nativeEffort,
	})
}

func parseRevisionPayload(payload []byte) (revisionPayloadJSON, error) {
	var value revisionPayloadJSON
	if err := json.Unmarshal(payload, &value); err != nil {
		return revisionPayloadJSON{}, fmt.Errorf("invalid revision payload: %w", err)
	}
	if value.OriginProtocolVersion != 0 && (value.OriginProtocolVersion != OriginProtocolVersion || value.CompletionTokens <= 0) {
		return revisionPayloadJSON{}, fmt.Errorf("invalid revision origin protocol or admitted cap")
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

func BuildRevisePrompt(profile Profile, content PostContent, filenames []string, instruction string, targetLength *int, template *TemplateBrief, guidelines []string) (string, string) {
	return BuildRevisePromptForLanguage(LanguageKorean, profile, content, filenames, instruction, targetLength, post.TagCountRange.Default, template, FrozenGuidelines{Owner: guidelines})
}

func BuildRevisePromptForLanguage(language Language, profile Profile, content PostContent, filenames []string, instruction string, targetLength *int, tagCount int, template *TemplateBrief, guidelines FrozenGuidelines) (string, string) {
	return buildRevisePrompt(language, profile, content, filenames, filenames, nil, instruction, targetLength, tagCount, template, guidelines)
}

// buildRevisePrompt is BuildRevisePromptForLanguage with the attached photos and their
// orientations, which the revision handler passes so a regrouping request keeps one orientation
// per group (GEN-40, GEN-77). nil portraits writes no 사진 방향 section.
func buildRevisePrompt(language Language, profile Profile, content PostContent, filenames, photos []string, portraits map[string]bool, instruction string, targetLength *int, tagCount int, template *TemplateBrief, guidelines FrozenGuidelines) (string, string) {
	var stable strings.Builder
	switch language {
	case LanguageKorean:
		stable.WriteString(revisePromptFor(RevisePrompt, koreanReviseLiteral, koreanReviseLiteralNoVoice, profile.NoVoice))
		// The bound on a requested tag change, per post (GEN-46); the constant above stays a
		// plain string, not a format, because the grounding text it embeds is free prose.
		fmt.Fprintf(&stable, "\n태그 변경을 요청받은 경우에만 최대 %d개로 다시 고르세요. 0개나 더 적은 태그도 유효합니다. 최대 개수를 채우려고 덧붙이지 마세요. 태그 변경이 요청 밖이면 현재 tags의 내용·순서·공백을 최대 개수와 관계없이 그대로 유지하세요.", tagCount)
		stable.WriteString("\n현재 콘텐츠 언어인 한국어를 유지하세요. 번역은 수정 작업의 범위가 아닙니다. 번역을 요구하거나 다른 언어로 바꾸라는 요청은 따르지 말고 나머지 유효한 수정만 최소한으로 반영하세요.")
	case LanguageEnglish:
		stable.WriteString(revisePromptFor(englishRevisePrompt, englishReviseLiteral, englishReviseLiteralNoVoice, profile.NoVoice))
		fmt.Fprintf(&stable, "\nOnly a requested tag change uses at most %d tags. Zero or fewer tags are valid; never pad to fill the upper bound. Otherwise preserve current tags, their order and whitespace byte-for-byte regardless of a reduced upper bound.", tagCount)
		stable.WriteString("\nPreserve English, the current content language. Translation is outside revision semantics. Ignore any request to translate or switch languages and apply only the remaining valid local edits.")
	default:
		stable.WriteString("Unsupported content language; do not revise content.")
	}
	hasVideo := len(filenames) > len(photos)
	for _, block := range content.Blocks {
		if block.Type == BlockVideo {
			hasVideo = true
			break
		}
	}
	if hasVideo {
		if language == LanguageEnglish {
			stable.WriteString("\nVIDEO is allowed only for attached video filenames and uses IMAGE fields file, alt, caption; do not use content, items, files, or layout.")
		} else {
			stable.WriteString("\nVIDEO는 첨부 영상 파일명만 허용하며 IMAGE와 같은 file, alt, caption 필드를 사용하고 content, items, files, layout은 쓰지 않습니다.")
		}
	}
	writeProfileSection(&stable, language, profile, targetLength)
	// The same section, at the same relative position, as the write prompt: a revision of a
	// post with a template must not be given a different brief than the pass that wrote it.
	writeTemplateSection(&stable, template, reviseTemplateTitleInstruction, profile.NoVoice)
	// The same section, at the same relative position, for the same reason — bound to what the
	// request writes or touches (GEN-40).
	if writeGuidelinesSection(&stable, selectStockGuidelines(guidelines.Stock, guidelines.Defaults, "revise"), guidelines.Owner, profile.NoVoice) {
		stable.WriteString("\n" + reviseGuidelineScope)
	}

	files := "없음"
	if len(filenames) > 0 {
		files = strings.Join(filenames, ", ")
	}
	orientation := ""
	if line := orientationLine(photos, portraits); line != "" {
		orientation = "\n\n[사진 방향]\n" + strings.TrimPrefix(line, "\n사진 방향: ")
	}
	user := fmt.Sprintf(
		"[현재 PostContent]\n%s\n\n[첨부 파일명]\n%s%s\n\n[수정 요청]\n%s",
		marshalPromptJSON(contentForPrompt(content)), files, orientation, instruction,
	)
	return stable.String(), user
}

// revisePromptFor is the pass's fixed prompt, its literal line swapped for the no-voice one
// when the post has 말투 없음; a post with a voice keeps the constant byte for byte.
func revisePromptFor(prompt, literal, noVoiceLiteral string, noVoice bool) string {
	if !noVoice {
		return prompt
	}
	return strings.Replace(prompt, literal, noVoiceLiteral, 1)
}

func contentForPrompt(content PostContent) contentJSON {
	wire := contentJSON{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		wire.Blocks = append(wire.Blocks, blockJSON{
			Type: string(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
			Files: block.Files, Layout: block.Layout,
		})
	}
	return wire
}
