package generation

import (
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/post"
)

// The second line names the convention observe.go sends: `file: 이름` immediately before the
// photo it names. Without it a model reads the labels as one more list and falls back to
// position, which is the binding this change exists to remove.
const ObservePrompt = `사진마다 파일명을 정확히 대응해 관찰 사실만 반환하세요. 추측하거나 이야기를 만들지 마세요.
각 사진 바로 앞에 그 사진의 파일명이 "file: 이름" 한 줄로 옵니다. 파일명은 순서로 짐작하지 말고 그 사진 바로 앞 줄에서 그대로 가져오세요.
rotation은 사진 속 장면이 똑바로 보이도록 시계 방향으로 돌려야 하는 각도이며 0, 90, 180, 270 중 하나입니다. 이미 똑바르면 0입니다.
출력은 설명이나 마크다운 없이 {"observations":[{"file":"...","scene":"...","mood":"...","visible_text":"...","objects":[],"people_present":false,"rotation":0}]} 형태의 JSON 객체 하나여야 합니다.`

// videoWriteInstructions are appended to the fixed write prompt ONLY for a post that actually
// has a clip. Two reasons, and both matter: a post without one keeps a byte-identical prompt —
// which is what every golden pins and what the provider's prefix cache rests on — and a model
// is never told about a block type the post has no file for, which is an invitation to invent
// one (VIDEO-12).
const videoWriteInstructions = "\nVIDEO 블록은 첨부 영상 파일명만 쓰고, storyline에서 그 영상이 놓인 문단의 자리에 놓으세요." +
	"\nblock의 type에는 VIDEO도 쓸 수 있습니다."

const englishVideoWriteInstructions = "\nA VIDEO block may use only an attached video filename. Place it where its storyline paragraph stands." +
	"\nA block's type may also be VIDEO."

// ObserveVideoPrompt is the photo prompt's facts-only rule for one clip. One video per call
// (VIDEO-8), so the `files:` line names exactly one file and the answer is one entry.
//
// `events` and `speech` are what a still frame cannot carry and are the whole reason a clip is
// observed at all; both are required, so a model that heard nothing says so with an empty
// string rather than by omitting the field (VIDEO-9).
const ObserveVideoPrompt = `영상에서 관찰한 사실만 파일명에 정확히 대응해 반환하세요. 추측하거나 이야기를 만들지 마세요.
events는 일어난 일을 시간 순서대로 짧은 사실 문장으로 적으세요.
speech는 들린 말의 요약입니다. 들리지 않거나 소리를 들을 수 없으면 빈 문자열로 두세요.
출력은 설명이나 마크다운 없이 {"observations":[{"file":"...","scene":"...","mood":"...","visible_text":"...","objects":[],"people_present":false,"events":[],"speech":"..."}]} 형태의 JSON 객체 하나여야 합니다.`

// koreanNounsRule / englishNounsRule define the write answer's `nouns` member (GEN-55): the
// distinct nouns the title and the body use, which the quality metrics count with no tokenizer
// and no second call (QUAL-16). The bare form is load-bearing: QUAL-7's containment matches a
// Korean 어절 that STARTS with the noun, so a listed "카페에서" would never match "카페는". The
// 40 is WriteNounsMax, which the parser enforces whatever the model returns.
const koreanNounsRule = "nouns에는 제목과 본문에 쓴 명사를 중복 없이 최대 40개까지 한국어로 적고, 각 명사는 조사를 붙이지 않은 형태로 쓰세요."

const englishNounsRule = "List in nouns the distinct nouns the title and the body use, at most 40, in English, each as a bare word without an article."

// koreanStorylineRule / englishStorylineRule ask for the storyline first (GEN-67): the plan of
// the post, paragraph by paragraph, each naming the attachments it uses, written before the
// post and followed by it. The storyline is its own JSON member, the first one, so the model
// writes the plan before a single block.
const koreanStorylineRule = "storyline에는 본문을 쓰기 전에 이 글을 어떤 순서로 이야기할지 문단별로 정하세요. 각 문단은 그 부분에서 무엇을 보여주고 말할지 두세 문장의 계획(…를 보여줍니다)으로 쓰고, files에는 그 부분에 놓을 첨부 파일명을 적으세요. 첨부 사진과 영상은 모두 정확히 한 문단에 한 번씩 넣고, 템플릿이 있으면 템플릿의 자리 순서를 따르세요. 본문은 이 storyline을 따라 쓰세요."

const englishStorylineRule = "Before writing, set in storyline how this post will tell things, paragraph by paragraph: each paragraph is a plan of two or three sentences saying what that part shows and says, and files names the attachments that part uses. Put every attached photo and video in exactly one paragraph, following the template's places in order when there is a template. Then write the post along this storyline."

// koreanGalleryRule / englishGalleryRule define the photo group's format (GEN-77): which fields it
// fills, the 2 … post.PhotoGroupMax bound, one orientation and a caption that is never empty. Format only — when photos are better grouped is the
// 비슷한 사진은 한 묶음으로 기본 지침 (GUIDE-41). A test pins the number to post.PhotoGroupMax.
const koreanGalleryRule = "GALLERY 블록은 사진 방향이 같은(모두 세로이거나 모두 가로인) 첨부 사진 2~3장을 한 자리에 묶어 설명 하나로 보여 줍니다. files에 파일명을 보여 줄 순서대로 적고, layout은 나란히 보여 주는 COLLAGE나 한 장씩 넘겨 보는 SLIDE 중 하나로 쓰고, alt는 묶음 전체에 하나, caption은 묶음 전체에 하나를 비워 두지 말고 쓰고, file은 비워 두세요. 다른 블록에서는 files를 빈 배열로, layout을 빈 문자열로 두세요."

const englishGalleryRule = "A GALLERY block shows 2 to 3 attached photos of one orientation (all 세로 or all 가로 in 사진 방향) together in one place under one caption: list their filenames in files in the order they stand, set layout to COLLAGE (side by side) or SLIDE (one at a time, swiped), write one alt and a caption that is never empty for the whole group, and leave file empty. On every other block, leave files as an empty array and layout as an empty string."

// WritePrompt / englishWritePrompt are the write pass's static rules, and they hold the input
// and output format alone (GUIDE-1, GEN-14): the task, one paragraph per TEXT block, attached
// filenames only, the storyline and the IMAGE placement along it, the answer shape and its
// fields. Every rule about what
// may be written — grounding, impressions, naming, altitude, the story rules, the title and tag
// rules and the Korean naturalness baseline — is a 기본 지침 the owner can switch off, rendered
// in [작문 지침] (GUIDE-41).
const WritePrompt = `첨부 사진 관찰과 메모를 바탕으로 한국어 블로그 글을 작성하세요.
반드시 하나의 문단마다 TEXT 블록 하나만 사용하세요.
IMAGE와 GALLERY 블록은 제공된 정확한 파일명만 사용하고, 목록에 없는 이미지를 절대 만들어내지 마세요.
` + koreanStorylineRule + `
첨부 사진은 storyline에서 그 사진이 놓인 문단의 자리에 IMAGE 블록 하나로 놓거나 GALLERY 블록 안에 넣어 정확히 한 번씩 놓으세요. 템플릿의 사진 자리에는 그 자리 주변이 다루는 내용에 맞는 사진을 놓으세요.
` + koreanGalleryRule + `
출력은 설명이나 마크다운 없이 {"storyline":[{"text":"...","files":[]}],"title":"...","summary":"...","tags":[],"blocks":[],"nouns":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, files, layout, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, GALLERY, QUOTE, LIST 중 하나입니다.
` + koreanNounsRule + "\n" + koreanSourceHonestyContract

const englishWritePrompt = `Write an English blog post from the photo observations and memo.
Use exactly one TEXT block for each paragraph.
IMAGE and GALLERY blocks may use only the exact filenames provided. Never invent an image that is not in the list.
` + englishStorylineRule + `
Place every attached photo exactly once, as an IMAGE block or inside a GALLERY block where its storyline paragraph stands; at a template's photo place, put the photos that fit what the section around it is about.
` + englishGalleryRule + `
Return exactly one JSON object shaped as {"storyline":[{"text":"...","files":[]}],"title":"...","summary":"...","tags":[],"blocks":[],"nouns":[]} with no explanation or Markdown.
Each block uses the type, content, level, file, files, layout, alt, caption, and items fields. type must be one of TEXT, HEADING, IMAGE, GALLERY, QUOTE, or LIST.
` + englishNounsRule + "\n" + englishSourceHonestyContract

// koreanWriteAlongStorylineRule / englishWriteAlongStorylineRule replace the storyline rule on
// the storyline path (GEN-70): the frozen [스토리라인] decides what the post covers and in what
// order, and the material only fills in its details.
const koreanWriteAlongStorylineRule = "[스토리라인]이 이 글이 다룰 내용과 순서를 정합니다. 스토리라인에 없는 내용은 메모에 있어도 쓰지 말고, 재료는 스토리라인이 다루는 내용의 세부를 채우는 데만 쓰세요. 각 사진과 영상은 스토리라인에서 그 파일이 놓인 문단의 자리에 한 번씩 놓으세요."

const englishWriteAlongStorylineRule = "[스토리라인] sets what this post covers and in what order. Do not write anything the storyline does not cover, even when the memo has it, and use the material only to fill in the details of what the storyline covers. Place each photo and video once, where the paragraph holding it stands."

// The storyline path's static rules are the direct write's with the storyline asked for no more:
// the rule becomes the storyline-path rule, the answer shape loses its `storyline` member, and
// the placement lines point at [스토리라인] (GEN-70). Derived here so the two can differ in
// nothing else.
var (
	writeAlongStorylinePrompt = strings.NewReplacer(
		koreanStorylineRule, koreanWriteAlongStorylineRule,
		`{"storyline":[{"text":"...","files":[]}],"title"`, `{"title"`,
		"첨부 사진은 storyline에서", "첨부 사진은 [스토리라인]에서",
	).Replace(WritePrompt)
	englishWriteAlongStorylinePrompt = strings.NewReplacer(
		englishStorylineRule, englishWriteAlongStorylineRule,
		`{"storyline":[{"text":"...","files":[]}],"title"`, `{"title"`,
	).Replace(englishWritePrompt)
)

// storylineSection renders the frozen storyline a from-storyline run follows (GEN-70) at the end
// of the per-post half, after the observations: one numbered line per paragraph with its files.
// Empty for every other run, so their prompts keep their bytes.
func storylineSection(paragraphs []StorylineParagraph) string {
	if len(paragraphs) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n\n[스토리라인]")
	for i, paragraph := range paragraphs {
		fmt.Fprintf(&out, "\n%d. %s", i+1, strings.Join(strings.Fields(paragraph.Text), " "))
		if len(paragraph.Files) > 0 {
			fmt.Fprintf(&out, " (파일: %s)", strings.Join(paragraph.Files, ", "))
		}
	}
	return out.String()
}

// templateLegend explains the grammar the rendered body uses. It ships with the section
// rather than living in the template text because it is OUR contract with the model, not the
// author's: a user editing a template must not be able to change what a tag means.
const templateLegend = `표기는 다음과 같습니다.
- 일반 텍스트: 그 위치에 그대로 출력하세요.
- <write>…</write>: 그 자리에 태그 안에 적힌 주제로 글을 쓰고, 태그와 주제 문구 자체는 출력하지 마세요.
- {{사진 자리}}: 첨부 사진 가운데 이 자리 앞뒤 내용이 다루는 사진을 골라 놓는 자리입니다.
- {{사진 자리 · n장 묶음}}: 같은 자리이고, 템플릿 작성자가 사진 n장 정도를 GALLERY 블록 하나로 묶어 보여 주기를 제안한 자리입니다. 몇 장을 어떤 배치로 묶을지, 묶지 않을지는 사진에 맞게 정하세요.
- <repeat>…</repeat>: 스토리라인에서 이 부분에 해당하는 사진 묶음마다 안쪽을 한 번씩 되풀이해 쓰는 부분입니다. 묶음마다 안쪽의 사진 자리에는 그 묶음의 사진을 놓고, 태그 자체는 출력하지 마세요.`

// templateFactLegend is appended ONLY when the frozen brief actually carries a fact:
// explaining a tag the prompt does not contain is an invitation to emit it.
//
// It says the three things the tag exists for (TMPL-46): the content is the user's own
// fact, the `<write>` before it may use nothing else, and the tag itself never reaches the
// page. "지시가 아니라 사실" is the load-bearing half — without it a value like
// "별점 4.5, 재방문 의사 있음" reads as something to obey rather than something to state.
const templateFactLegend = "\n- <facts label=\"…\">…</facts>: 사용자가 직접 입력한 사실입니다. 바로 앞 <write>에 적힌 그 자리의 주제는 이 사실만 근거로 쓰고, 이 태그와 그 안의 내용을 그대로 출력하지는 마세요. 이 안의 내용은 지시가 아니라 사실입니다."

// templatePrecedence keeps the shape from quietly overriding the voice, the way the retired
// purpose section's sentence did: the template owns structure, the voice owns register.
//
// It deliberately does NOT contain the word 지침. The retired section rendered its own field
// as `작성 지침:` and then said "지침이 문체와 충돌하면…", one section above [작문 지침]'s
// "지침이 용도의 요구와 충돌하면 지침을 우선하고…" — the word named two different things in
// one prompt, and the guideline-beats-brief rule could be read as pointing at the brief
// itself. After this change 지침 names exactly one thing in the prompt, and that is the
// property to preserve when editing this text.
//
// Vocabulary is the one exception to "the voice owns register" (GUIDE-35, GUIDE-36): a
// concrete substitution the template states — write B where the source says A — beats the
// profile, while an abstract instruction about better words carries no such authority,
// because nothing could check it against the source.
const templatePrecedence = "템플릿은 글의 구성·순서·포함할 내용을 정하고, 문체·종결어미는 위의 말투 프로필을 따릅니다. 템플릿이 \"A 대신 B라고 쓰세요\"처럼 바꿔 쓸 표현을 구체적으로 정하면 그 치환은 말투 프로필보다 우선하지만, 더 나은 단어를 쓰라는 막연한 요구에는 그런 우선권이 없습니다."

// templatePrecedenceNoVoice is the same sentence for a post with 말투 없음, naming no voice
// (GEN-74, TMPL-12): a sentence pointing at a profile that is not there invites the model to
// invent one.
const templatePrecedenceNoVoice = "템플릿은 글의 구성·순서·포함할 내용을 정합니다. 템플릿이 \"A 대신 B라고 쓰세요\"처럼 바꿔 쓸 표현을 구체적으로 정하면 그 치환을 따르고, 더 나은 단어를 쓰라는 막연한 요구에는 그런 우선권이 없습니다."

// templateTitleInstruction introduces a frozen template's title area (GEN-52, TMPL-50), which
// renders in its own fences above the body form. Like the rest of the section it never says
// 지침 (TMPL-13).
const templateTitleInstruction = "JSON의 title은 바로 다음 --- 사이의 제목 형식을 따르세요. 그 뒤 --- 사이의 내용은 본문의 형식입니다."

// reviseTemplateTitleInstruction is the revise pass's form of that line (TMPL-51): the title
// form binds only a request that asks to change the title, and any other revision keeps the
// title as it stands — a title the owner edited away from the form stays theirs until they
// ask. RevisePrompt's own "제목…고쳐 달라고 한 경우에만 바꾸세요" line says the same, so the two
// no longer conflict. "수정 요청" is the prompt's own name for the request and "현재 제목"
// points at [현재 PostContent]'s title; the tail is shared with the write line, so the body
// fence keeps one meaning. Like the rest of the section it never says 지침 (TMPL-13).
const reviseTemplateTitleInstruction = "수정 요청이 제목을 바꾸라고 할 때만 JSON의 title을 바로 다음 --- 사이의 제목 형식에 맞춰 쓰고, 그 밖의 수정에서는 현재 제목을 그대로 두세요. 그 뒤 --- 사이의 내용은 본문의 형식입니다."

// writeTemplateSection appends the frozen template AFTER the complete voice profile and
// before the per-post material. That position is load-bearing twice over, exactly as the
// purpose section's was: the profile prefix stays byte-identical across posts of different
// templates (PRD §5's caching note), and the template stays in the stable half, so every
// revision of one post re-injects the identical block.
//
// The body arrives already resolved and rendered by the template context, frozen at enqueue.
// Nothing here parses or re-renders: an answer edited after the start must not be able to
// change what the model was asked for.
//
// A nil template writes nothing at all, so a post without one adds no template bytes.
// titleInstruction is the pass's own title line: templateTitleInstruction for the write,
// reviseTemplateTitleInstruction for a revision; nothing else in the section differs.
func writeTemplateSection(out *strings.Builder, brief *TemplateBrief, titleInstruction string, noVoice bool) {
	if brief == nil {
		return
	}
	writeTemplateForm(out, brief, titleInstruction)
	precedence := templatePrecedence
	if noVoice {
		precedence = templatePrecedenceNoVoice
	}
	fmt.Fprintf(out, "\n%s", precedence)
}

// writeTemplateForm is the section without its closing precedence line: the heading, the
// legend, the title form when titleInstruction names one, and the body form. The storyline
// prompt uses it alone — it plans the body, has no title to write and no voice to rank the
// template against (GEN-68).
func writeTemplateForm(out *strings.Builder, brief *TemplateBrief, titleInstruction string) {
	if brief.BodyParts != nil || brief.TitleParts != nil {
		writeTypedTemplateForm(out, brief, titleInstruction)
		return
	}
	fmt.Fprintf(out, "\n\n[글 템플릿: %s]", brief.Name)
	legend := templateLegend
	if len(brief.Facts) > 0 {
		legend += templateFactLegend
	}
	fmt.Fprintf(out, "\n아래 템플릿의 구성을 그대로 따르세요. %s", legend)
	// The title form sits above the body form, in fences of its own (GEN-52). An empty title
	// area writes nothing, so every template authored before it existed keeps its bytes.
	if brief.TitleArea != "" && titleInstruction != "" {
		fmt.Fprintf(out, "\n%s\n---\n%s\n---", titleInstruction, brief.TitleArea)
	}
	fmt.Fprintf(out, "\n---\n%s\n---", brief.Body)
}

// guidelinePrecedence closes [작문 지침] (GUIDE-15, GUIDE-37). A guideline says how this post is
// written and outranks the template's content instruction while register stays with the voice;
// the owner's own line outranks a conflicting 기본 지침, and inside one group the earlier line
// wins. Vocabulary carries its own order (GUIDE-35, GUIDE-36): a concrete substitution ranks the
// guideline over the template over the profile, and an abstract one carries no authority at all.
// Korean for every target, like the heading. Fixed prompt text, so it lives in code (ARCH-21).
const guidelinePrecedence = "지침은 이 글을 어떻게 쓸지 정합니다. 지침이 템플릿과 충돌하면 지침을 우선하고, 문체·종결어미는 위의 말투 프로필을 따르세요. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요. \"A 대신 B라고 쓰세요\"처럼 구체적으로 정한 치환은 지침, 템플릿, 말투 프로필 순으로 우선하고, 더 나은 단어를 쓰라는 막연한 요구에는 그런 우선권이 없습니다."

// guidelinePrecedenceNoVoice closes the section for a post with 말투 없음: no register clause
// and no profile in the substitution order (GUIDE-15, GEN-74).
const guidelinePrecedenceNoVoice = "지침은 이 글을 어떻게 쓸지 정합니다. 지침이 템플릿과 충돌하면 지침을 우선하세요. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요. \"A 대신 B라고 쓰세요\"처럼 구체적으로 정한 치환은 지침, 템플릿 순으로 우선하고, 더 나은 단어를 쓰라는 막연한 요구에는 그런 우선권이 없습니다."

// reviseGuidelineScope follows [작문 지침] in the revise prompt: the section binds only what the
// request writes or touches, so a revision never re-checks or rewrites the sentences outside it.
const reviseGuidelineScope = "이 지침은 수정 요청으로 새로 쓰거나 손대는 문장에만 적용하고, 요청 밖의 기존 문장은 사실 확인 없이 그대로 두세요."

// The group labels inside [작문 지침], Korean for every target like the heading.
const (
	defaultGuidelinesLabel = "기본 지침:"
	ownerGuidelinesLabel   = "사용자 지침:"
)

// qualityRulesHeading and qualityRulesPrecedence frame the ticked quality rules (GEN-51,
// POST-81): the rule texts the owner ticked on the writing brief, frozen at enqueue and
// already rendered in the target language. The heading stays Korean for every target, like
// every other section heading. The closing line says a 지침 outranks them (QUAL-13), and 지침
// still names one thing: the [작문 지침] section above, in the stable half.
const qualityRulesHeading = "[발행 글 측정 규칙]"

const qualityRulesPrecedence = "지침이 위 규칙과 충돌하면 지침을 우선하세요."

// qualityRulesSection renders the ticked rules at the head of the per-post half, before
// [이번 글] (GEN-14, GEN-51): the ticks differ per post, and the stable prefix is what the
// provider's cache and every prompt golden rest on (MEM-20's reason), so a ticked post keeps
// the same prefix as an unticked one. Position no longer ranks them below the 지침; the
// closing line does. No rules render as the empty string, so ticking nothing leaves the run
// identical to one before the rules existed (POST-81). Write-only: a revision keeps
// unrelated sentences verbatim, and a rule sweep would rewrite them.
func qualityRulesSection(rules []string) string {
	if len(rules) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(qualityRulesHeading)
	for _, text := range rules {
		out.WriteString("\n- " + text)
	}
	out.WriteString("\n" + qualityRulesPrecedence + "\n\n")
	return out.String()
}

// writeGuidelinesSection appends the frozen 지침 as ONE section at ONE position: after the
// template section when the post has one, otherwise directly after the complete voice profile,
// and always before the per-post material (GUIDE-15). The enabled 기본 지침 come first, then the
// owner's own, each group under its label and left out when it has no line; with neither there
// is no section, so the fixed no-guideline prompt stays byte for byte.
//
// A multi-line text keeps its lines, the continuation ones indented by two spaces, so one text
// still reads as one bullet. The heading, labels and precedence sentence stay Korean for every
// target language, exactly as writeTemplateSection does: the section frames the guideline texts,
// and its framing is not part of the output-language contract.
func writeGuidelinesSection(out *strings.Builder, defaults, owner []string, noVoice bool) bool {
	precedence := guidelinePrecedence
	if noVoice {
		precedence = guidelinePrecedenceNoVoice
	}
	return writeGuidelinesSectionClosedBy(out, defaults, owner, precedence)
}

// writeGuidelinesSectionClosedBy is the section with the pass's own closing sentence: the write
// and the revise close with guidelinePrecedence, the storyline with storylineGuidelinePrecedence.
func writeGuidelinesSectionClosedBy(out *strings.Builder, defaults, owner []string, precedence string) bool {
	if len(defaults) == 0 && len(owner) == 0 {
		return false
	}
	out.WriteString("\n\n[작문 지침]")
	for _, group := range []struct {
		label string
		texts []string
	}{{defaultGuidelinesLabel, defaults}, {ownerGuidelinesLabel, owner}} {
		if len(group.texts) == 0 {
			continue
		}
		out.WriteString("\n" + group.label)
		for _, text := range group.texts {
			out.WriteString("\n- " + strings.ReplaceAll(text, "\n", "\n  "))
		}
	}
	fmt.Fprintf(out, "\n%s", precedence)
	return true
}

// memoryPrecedence closes the `[기억]` section (MEM-21, GUIDE-28). It states the one thing
// the section exists to say — these facts are material for THIS post, not a list to work
// through — and the precedence the user's own prohibitions keep over them: a guideline
// outranks a memory exactly as it outranks a template instruction, because a prohibition
// the user added is the reason the fact must stay out of this post.
//
// Fixed prompt text, so it lives in code (ARCHITECTURE §4), and appended only when the
// section exists: a post with the option off must not read a word about a source it has
// none of.
const memoryPrecedence = "위 기억은 이 글에 쓸 수 있는 사실입니다. 이 글과 관계없는 기억은 쓰지 마세요. 지침이 기억과 충돌하면 지침을 우선하세요."

// memorySection renders the frozen memories as ONE section of the PER-POST half, between the
// memo and the attachments (GEN-14). It is the one grounding section that does NOT sit in
// the stable prefix, and the reason is that the selected set differs per post while the
// prefix is what the provider's cache and every prompt golden rest on (MEM-20).
//
// An empty slice writes nothing at all, so a post with the option off — or one whose key
// matched no memory — produces a per-post half byte for byte identical to today's.
func memorySection(memories []string) string {
	if len(memories) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("[기억]")
	for _, text := range memories {
		fmt.Fprintf(&out, "\n- %s", text)
	}
	fmt.Fprintf(&out, "\n%s\n", memoryPrecedence)
	return out.String()
}

// BuildWritePrompt preserves the legacy Korean call surface for prompt goldens and
// consumers that explicitly request the established Korean contract. Runtime work uses
// BuildWritePromptForLanguage with its frozen language.
func BuildWritePrompt(profile Profile, observations []Observation, memo, title string, filenames []string, targetLength *int, template *TemplateBrief, guidelines []string) (string, string) {
	return BuildWritePromptForLanguage(WritePromptInput{
		Language: LanguageKorean, Profile: profile, Observations: observations, Memo: memo, Title: title,
		Photos: filenames, TargetLength: targetLength, TagCount: post.TagCountRange.Default,
		Template: template, Guidelines: guidelines,
	})
}

// WritePromptInput is everything the write prompt reads, and nothing else: a member added here
// is a member the builder must read (TestEveryWritePromptInputMemberReachesThePrompt).
type WritePromptInput struct {
	// Language selects the static rules and the output-language sentence.
	Language Language
	Profile  Profile
	// Observations are the run's merged observations — the photos this run holds eyesight for —
	// never the stored snapshot read at enqueue.
	Observations []Observation
	Memo         string
	// Title is the 가제, the author's title hint.
	Title string
	// Photos and Videos are the attached filenames by kind, in post order.
	Photos       []string
	Videos       []string
	TargetLength *int
	// TagCount is the frozen per-post count, already resolved (GEN-46); the sentence stays in
	// the stable part where the fixed range used to be, so the golden order is unchanged.
	TagCount int
	Template *TemplateBrief
	// DefaultGuidelines and Guidelines are the frozen 기본 지침 and owner 지침 texts, each in
	// injection order (GUIDE-14).
	DefaultGuidelines []string
	StockGuidelines   []StockGuideline
	Guidelines        []string
	// Memories are the frozen 기억 texts, empty for a post that did not opt in (MEM-20).
	Memories []string
	// QualityRules are the frozen ticked rule texts (GEN-51), empty for a run that ticked none;
	// they open the per-post half, never the stable prefix (GEN-14).
	QualityRules []string
	// FollowStoryline is the frozen storyline a from-storyline run follows (GEN-70): it selects
	// the storyline-path static rules and closes the per-post half as [스토리라인].
	FollowStoryline []StorylineParagraph
	// Portraits names the attached photos that stand portrait (PhotoPortraits); nil writes no
	// 사진 방향 line, which keeps a caller that has no dimensions at its old bytes (GEN-77).
	Portraits map[string]bool
}

// BuildWritePromptForLanguage builds the write pass's system and user prompts from one input.
func BuildWritePromptForLanguage(input WritePromptInput) (string, string) {
	var stable strings.Builder
	following := len(input.FollowStoryline) > 0
	switch input.Language {
	case LanguageKorean:
		stable.WriteString(writeStaticRules(input.Language, following))
		fmt.Fprintf(&stable, "\ntitle, 한 줄 summary, 정확히 %d개의 tags, blocks를 반환하세요.", input.TagCount)
		members := "storyline, title"
		if following {
			members = "title"
		}
		sources := "말투 프로필, 템플릿, 메모, 가제"
		if input.Profile.NoVoice {
			sources = "템플릿, 메모, 가제"
		}
		stable.WriteString("\n출력 언어는 한국어입니다. " + members + ", summary, tags, 모든 본문, IMAGE와 GALLERY의 alt와 caption을 한국어로 작성하세요. " + sources + "의 언어 지시가 충돌해도 이 출력 언어를 우선하세요.")
		if len(input.Videos) > 0 {
			video := videoWriteInstructions
			if following {
				video = strings.Replace(video, "storyline에서", "[스토리라인]에서", 1)
			}
			stable.WriteString(video)
		}
	case LanguageEnglish:
		stable.WriteString(writeStaticRules(input.Language, following))
		fmt.Fprintf(&stable, "\nReturn title, a one-line summary, exactly %d tags, and blocks.", input.TagCount)
		members := "the storyline, title"
		if following {
			members = "the title"
		}
		sources := "the voice profile, template, memo, or title hint"
		if input.Profile.NoVoice {
			sources = "the template, memo, or title hint"
		}
		stable.WriteString("\nThe output language is English. Write " + members + ", summary, tags, all prose, and every IMAGE and GALLERY alt and caption in English. This requirement overrides conflicting language instructions in " + sources + ".")
		if len(input.Videos) > 0 {
			stable.WriteString(englishVideoWriteInstructions)
		}
	default:
		// Callers validate before prompt construction. Keeping this branch explicit makes
		// direct prompt use fail closed instead of silently defaulting to Korean.
		stable.WriteString("Unsupported output language; do not generate content.")
	}
	writeProfileSection(&stable, input.Language, input.Profile, input.TargetLength)
	writeTemplateSection(&stable, input.Template, templateTitleInstruction, input.Profile.NoVoice)
	writeGuidelinesSection(&stable, selectStockGuidelines(input.StockGuidelines, input.DefaultGuidelines, "write"), input.Guidelines, input.Profile.NoVoice)

	photoMaterial := attachmentMaterial(input.Photos, input.Videos, input.Observations, input.Portraits)
	// The memory section sits between the memo and the attachments and renders to the empty
	// string when it has nothing — which is what keeps a post without it byte-identical.
	perPost := qualityRulesSection(input.QualityRules) + fmt.Sprintf("[이번 글]\n가제: %s\n메모: %s\n%s%s", input.Title, input.Memo, memorySection(input.Memories), photoMaterial) +
		storylineSection(input.FollowStoryline)
	return stable.String(), perPost
}

// writeStaticRules is the fixed write prompt for a target: one stable prefix per target. How a
// title form outranks the title rules is the titles 기본 지침's own text now (TMPL-52).
func writeStaticRules(language Language, followingStoryline bool) string {
	switch {
	case language == LanguageKorean && followingStoryline:
		return writeAlongStorylinePrompt
	case language == LanguageKorean:
		return WritePrompt
	case language == LanguageEnglish && followingStoryline:
		return englishWriteAlongStorylinePrompt
	case language == LanguageEnglish:
		return englishWritePrompt
	default:
		return ""
	}
}

// writeProfileSection renders the voice's projection as given (VOICE-46): a Korean target's
// `[말투]` section with its excerpts, another target's portable section, or nothing for 말투 없음
// (GEN-74). The length stands on its own [길이] line in every case; no run of identical endings
// is voice text any more — it is the ending_run 기본 지침 (VOICE-47).
func writeProfileSection(stable *strings.Builder, language Language, profile Profile, targetLength *int) {
	if profile.NoVoice {
		writeGenericLength(stable, language, targetLength)
		return
	}
	if profile.Text != "" {
		stable.WriteString("\n\n")
		stable.WriteString(profile.Text)
	}
	if !profile.Portable && len(profile.Excerpts) > 0 {
		stable.WriteString("\n\n[글 예시 발췌]")
		for i, excerpt := range profile.Excerpts {
			fmt.Fprintf(stable, "\n%d. %s", i+1, excerpt)
		}
		stable.WriteString("\n예시의 고유 사실, 주제, 문구를 복사하지 말고 문체 특징만 참고하세요.")
	}
	writeGenericLength(stable, language, targetLength)
}

func writeGenericLength(stable *strings.Builder, language Language, targetLength *int) {
	if targetLength == nil {
		return
	}
	if language == LanguageEnglish {
		fmt.Fprintf(stable, "\n\n[Length]\nTarget approximately %d Unicode characters.", *targetLength)
		return
	}
	fmt.Fprintf(stable, "\n\n[길이]\n목표 길이: 약 %d자.", *targetLength)
}

// attachmentMaterial is the per-post half's attachment section: what is attached, and what
// was observed about it.
//
// A post with no video produces BYTE-IDENTICAL text to before videos existed — one filename
// line and one 사진 관찰 line — because that is what every golden pins and what every prompt
// cache prefix depends on. The video lines exist only when the post actually has a clip.
func attachmentMaterial(photos, videos []string, observations []Observation, portraits map[string]bool) string {
	if len(photos) == 0 && len(videos) == 0 {
		return "첨부 사진이 없습니다. 이미지 없이 메모만으로 작성하세요."
	}
	if len(videos) == 0 {
		return "첨부 파일명(정확히 일치해야 함): " + strings.Join(photos, ", ") + orientationLine(photos, portraits) +
			"\n사진 관찰: " + marshalPromptJSON(observationsForPrompt(observations))
	}

	// The two kinds are named on their own lines once a video exists: the writer has to place
	// an IMAGE block and a VIDEO block from different lists, and one merged line would make
	// naming the wrong kind the easy mistake (VIDEO-12).
	videoNames := make(map[string]struct{}, len(videos))
	for _, filename := range videos {
		videoNames[filename] = struct{}{}
	}
	photoObservations := make([]Observation, 0, len(observations))
	videoObservations := make([]Observation, 0, len(videos))
	for _, observation := range observations {
		if _, ok := videoNames[observation.File]; ok {
			videoObservations = append(videoObservations, observation)
			continue
		}
		photoObservations = append(photoObservations, observation)
	}

	var out strings.Builder
	if len(photos) > 0 {
		out.WriteString("첨부 사진 파일명(정확히 일치해야 함): " + strings.Join(photos, ", ") + orientationLine(photos, portraits))
		out.WriteString("\n사진 관찰: " + marshalPromptJSON(observationsForPrompt(photoObservations)))
		out.WriteString("\n")
	}
	out.WriteString("첨부 영상 파일명(정확히 일치해야 함): " + strings.Join(videos, ", "))
	out.WriteString("\n영상 관찰: " + marshalPromptJSON(videoObservationsForPrompt(videoObservations)))
	return out.String()
}

// orientationLine is the per-post 사진 방향 line (GEN-77): every attached photo, in post order,
// named 세로 or 가로, so the writer can group portrait with portrait and landscape with landscape.
// The per-post half's framing is Korean for every target language, like the lines around it.
// nil portraits — a caller with no dimensions — writes nothing.
func orientationLine(photos []string, portraits map[string]bool) string {
	if portraits == nil || len(photos) == 0 {
		return ""
	}
	parts := make([]string, 0, len(photos))
	for _, photo := range photos {
		word := "가로"
		if portraits[photo] {
			word = "세로"
		}
		parts = append(parts, photo+" "+word)
	}
	return "\n사진 방향: " + strings.Join(parts, ", ")
}

// videoObservationsForPrompt carries the two fields a photo entry has no use for. They are
// what the clip was observed FOR — motion and sound are the whole reason it is not a photo —
// so dropping them here would make a video's call worth nothing to the writer.
func videoObservationsForPrompt(observations []Observation) []observationJSON {
	wire := make([]observationJSON, 0, len(observations))
	for _, observation := range observations {
		wire = append(wire, observationJSON{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent,
			Events:        observation.Events, Speech: observation.Speech,
		})
	}
	return wire
}

func observationsForPrompt(observations []Observation) []observationJSON {
	wire := make([]observationJSON, 0, len(observations))
	for _, observation := range observations {
		wire = append(wire, observationJSON{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent,
		})
	}
	return wire
}
