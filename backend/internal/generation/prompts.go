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
출력은 설명이나 마크다운 없이 {"observations":[{"file":"...","scene":"...","mood":"...","visible_text":"...","objects":[],"people_present":false}]} 형태의 JSON 객체 하나여야 합니다.`

// videoWriteInstructions are appended to the fixed write prompt ONLY for a post that actually
// has a clip. Two reasons, and both matter: a post without one keeps a byte-identical prompt —
// which is what every golden pins and what the provider's prefix cache rests on — and a model
// is never told about a block type the post has no file for, which is an invitation to invent
// one (VIDEO-12).
const videoWriteInstructions = "\nVIDEO 블록은 첨부 영상 파일명만 쓰고, 영상이 보여주는 내용이 글에서 언급되는 위치에 놓으세요." +
	"\nblock의 type에는 VIDEO도 쓸 수 있습니다."

const englishVideoWriteInstructions = "\nA VIDEO block may use only an attached video filename. Place it where the post mentions what the clip shows." +
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

// koreanGrounding / englishGrounding are the built-in grounding constraint (plan 16): the
// writer may state no concrete fact the memo, the photo observations and the template's data
// fields do not carry (GEN-16). The third source is named unconditionally, even for a post
// with no template: this text sits in the STATIC rules ahead of the voice profile, and making
// it conditional would break the byte-stable prefix TEMPLATE-12 and GEN-14 protect for prompt
// caching — one clause naming a source this post has none of costs nothing. It
// ships as fixed prompt text because the invented-fact failure — "주인분에게 건네받았다" for
// an unmanned store — is one every account hits with zero setup, so it cannot wait for a
// user-authored 지침. It is deliberately disjoint from NaturalnessBaseline, which owns style
// and nothing else, and from the observe prompt, which never sees it ([I3]).
const koreanGrounding = "메모, 사진 관찰, 그리고 템플릿 입력란에 주어진 사실에 없는 구체적 사실은 쓰지 마세요. 사람과의 상호작용, 시설, 서비스, 대화, 가격처럼 확인되지 않은 내용을 지어내지 마세요."

const englishGrounding = "State no concrete fact that the memo, the photo observations, and the facts given in the template's fields do not carry. Do not invent interactions with people, facilities, services, conversations, or prices."

// The write pass holds the memo and the observations, so it can also be told to omit what it
// cannot confirm. The revise pass does not receive either one, so the same instruction there
// would license stripping real facts out of blocks the request never mentioned — directly
// against the byte-for-byte preservation rule beside it. Hence the split: the core prohibition
// is shared, and each pass carries the clause its own material can support.
const koreanGroundingWriteScope = "이 제약은 사실 진술에만 적용됩니다. 확인할 수 없는 사실은 생략하거나 관찰된 범위 안에서만 쓰고, 글쓴이의 감상과 의견, 화제 전환은 자유롭게 쓰세요."

const englishGroundingWriteScope = "This binds factual claims alone: omit whatever fact you cannot confirm, or keep it within what was observed, while the author's impressions, opinions, and changes of subject stay yours to write."

const koreanGroundingReviseScope = "이 기준은 수정 요청으로 새로 쓰거나 손대는 문장에만 적용하고, 요청 밖의 기존 문장은 사실 확인 없이 그대로 두세요."

const englishGroundingReviseScope = "Apply this only to sentences the request makes you write or touch; leave every sentence outside the request exactly as it is, without re-checking its facts."

// koreanAltitude / englishAltitude are the fourth grounding constant (GEN-47) and, like the
// naming rule, they are write-only: the revise pass holds no observations to stay above. The
// grounding constraint above says what may be stated; without this one the model answered it
// by covering the observation array entry by entry, so a post read as a caption for every
// photograph. It is subtraction, worded the way NaturalnessBaseline is: it forbids describing
// and licenses nothing new — whatever is written still has to survive the line above.
const koreanAltitude = "사진 관찰은 글의 근거이자 사진을 놓을 자리를 알려주는 자료이지, 하나씩 묘사해야 할 목록이 아닙니다. 어떤 문단도 사진을 가리키며 설명할 필요가 없습니다. 조명, 벽, 천장, 집기, 공간 배치처럼 글에 아무것도 더하지 않는 시각적 묘사는 쓰지 마세요."

const englishAltitude = "The photo observations are grounding and placement material, not a list to describe one by one. No paragraph has to point at a photo. Do not write visual detail that carries nothing for the post: lighting, walls, ceilings, fixtures, the arrangement of a room."

// koreanNaming / englishNaming are write-only, and they are about IDENTITY, not only about a
// proper name. The observation stage sees one photo at a time and is told to state facts
// alone, so it can only ever say what a thing LOOKS like: "노을 지는 콘크리트 건물" for what
// the memo already called 별채. A rule covering only names left that case out — 별채 is a
// role, not a name — so the generic wording survived into the caption and the post read as a
// stranger's description of the user's own trip.
//
// The three sources are the same three koreanGrounding names, and the trailing clause is the
// bound: identity comes from the memo and the template, never a guess about the frame.
const koreanNaming = "사진 관찰은 사진을 한 장씩 따로 본 결과라 그 대상이 무엇인지 모르는 채 적혀 있습니다. 메모나 템플릿 입력란이 그 대상의 정체나 이름을 알려주면 — 관찰의 \"콘크리트 건물\"이 메모의 \"별채\"라면 — 본문과 IMAGE alt 및 caption에서 관찰의 일반적인 표현 대신 그쪽을 쓰세요. 다만 사진 관찰에 없는 것을 사진 안에 있는 것처럼 쓰지는 마세요."

const englishNaming = "A photo observation was made one photo at a time, without knowing what its subject is. When the memo or a template field gives that subject an identity or a name — the observation's \"concrete building\" is the memo's \"annex\" — use that in prose and in IMAGE alt and caption instead of the observation's generic wording. Do not write anything the photo observations do not show as being in the frame."

// koreanTitleProhibitions / englishTitleProhibitions are the only two title rules with a
// primary source, and the only two that ship (GEN-49). Google Search Central names boilerplate
// titles — long text in which only a single piece of information varies — as a practice to
// avoid, and Naver 서치어드바이저's content-markup guide warns that a keyword repeated twice or
// more in one title risks a penalty (QUAL-6). Everything else said about titles — ideal length,
// keyword position, bracket forms, number-first openings — has no primary source and stays out
// (spec/ideation/post-quality-and-related-links.md). Write-only, like the naming rule: a
// revision changes a title only when asked to.
const koreanTitleProhibitions = "제목을 정보 하나만 바꿔 넣은 긴 상투 문구로 쓰지 말고, 한 제목 안에서 같은 키워드를 두 번 이상 쓰지 마세요."

const englishTitleProhibitions = "Do not write the title as long boilerplate in which only a single piece of information changes, and do not use any keyword twice or more within one title."

// koreanTitleFormProhibitions / englishTitleFormProhibitions take the plain line's place when
// the frozen template authored a title area (TMPL-52): the owner's form outranks both
// prohibitions, which then bind only what the model writes inside the form's <write>. Neither
// variant contains the other, so a prompt can be checked for carrying exactly one of them.
const koreanTitleFormProhibitions = "제목은 템플릿의 제목 형식을 우선해 따르세요. 그 형식의 <write> 안에서 직접 쓰는 부분은 정보 하나만 바꿔 넣은 긴 상투 문구로 쓰지 말고, 그 안에서 같은 키워드를 두 번 이상 쓰지 마세요."

const englishTitleFormProhibitions = "The template's title form comes first. In what you write inside its <write>, do not use long boilerplate in which only a single piece of information changes, and do not use any keyword twice or more."

// koreanTagRule / englishTagRule are the one tag rule (GEN-50): choose among labels already
// true of the post rather than the first ones reached for. It reads nothing but the post's own
// material — no search-demand data is read anywhere (QUAL-34) — so it cannot license a tag the
// post does not bear out.
const koreanTagRule = "tags는 먼저 떠오르는 단어를 그대로 쓰지 말고, 이 글의 내용에 이미 들어맞는 이름 가운데서 고르세요."

const englishTagRule = "Choose the tags among labels already true of this post, not the first words that come to mind."

// koreanNounsRule / englishNounsRule define the write answer's `nouns` member (GEN-55): the
// distinct nouns the title and the body use, which the quality metrics count with no tokenizer
// and no second call (QUAL-16). The bare form is load-bearing: QUAL-7's containment matches a
// Korean 어절 that STARTS with the noun, so a listed "카페에서" would never match "카페는". The
// 40 is WriteNounsMax, which the parser enforces whatever the model returns.
const koreanNounsRule = "nouns에는 제목과 본문에 쓴 명사를 중복 없이 최대 40개까지 한국어로 적고, 각 명사는 조사를 붙이지 않은 형태로 쓰세요."

const englishNounsRule = "List in nouns the distinct nouns the title and the body use, at most 40, in English, each as a bare word without an article."

const WritePrompt = `첨부 사진 관찰과 메모를 바탕으로 자연스러운 한국어 블로그 글을 작성하세요.
` + koreanGrounding + " " + koreanGroundingWriteScope + `
` + koreanAltitude + `
` + koreanNaming + `
` + koreanTitleProhibitions + `
` + koreanTagRule + `
반드시 하나의 문단마다 TEXT 블록 하나만 사용하세요.
IMAGE 블록은 제공된 정확한 파일명만 사용하고, 목록에 없는 이미지를 절대 만들어내지 마세요.
IMAGE 블록은 사진이 글의 흐름상 가장 자연스러운 위치에 오도록 배치하세요.
출력은 설명이나 마크다운 없이 {"title":"...","summary":"...","tags":[],"blocks":[],"nouns":[]} 형태의 JSON 객체 하나여야 합니다.
각 block은 type, content, level, file, alt, caption, items 필드를 사용하며 type은 TEXT, HEADING, IMAGE, QUOTE, LIST 중 하나입니다.
` + koreanNounsRule

const englishWritePrompt = `Write a natural English blog post from the photo observations and memo.
` + englishGrounding + " " + englishGroundingWriteScope + `
` + englishAltitude + `
` + englishNaming + `
` + englishTitleProhibitions + `
` + englishTagRule + `
Use exactly one TEXT block for each paragraph.
IMAGE blocks may use only the exact filenames provided. Never invent an image that is not in the list.
Place each IMAGE block where the photo fits most naturally in the flow of the post.
Return exactly one JSON object shaped as {"title":"...","summary":"...","tags":[],"blocks":[],"nouns":[]} with no explanation or Markdown.
Each block uses the type, content, level, file, alt, caption, and items fields. type must be one of TEXT, HEADING, IMAGE, QUOTE, or LIST.
` + englishNounsRule

const NaturalnessBaseline = `[한국어 자연 문체 기준선]
- 아래 기준은 새로 쓰거나 수정 요청으로 손대는 TEXT 본문에만 적용하세요. 제목·요약·HEADING·LIST에는 적용하지 말고, 수정에서는 요청 밖의 기존 문장을 그대로 두세요.
- 대조 수사는 글 전체에서 “A가 아니라 B”, “~것이 아니라” 꼴을 합쳐 한 번만 쓰세요.
- 문단을 “필요한·중요한·핵심은 …이다”, “결국 …로 이어진다”, “~하는 이유다”로 닫지 마세요. “중요한 것은 실행력이다”보다 “오늘 할 일을 바로 적고 실행하세요”처럼 사실과 동작을 직접 쓰세요.
- 구체적 시점 없는 “향후·앞으로” 전망이나 내용 없는 “과제도 남아 있다”로 문단을 닫지 마세요. “~해야 한다”로 끝나는 문단은 글 전체에서 하나만 허용합니다.
- 연결어미 -고/-며/-지만/-면서/-아서 바로 뒤에는 쉼표를 놓지 말고, 대부분 문장은 쉼표 없이 쓰세요.
- 한 문단 안에서 짧은 문장과 긴 복문, 단문과 복문을 섞어 길이와 구조에 변화를 주세요.
- 확대·강화·개선·확보·구축 같은 포괄적 동사를 되풀이하지 말고 구체적인 동작을 쓰세요. 잠식·청사진·신호탄 같은 지어낸 비유를 겹치거나 과장 형용사를 쌓지 말고, “~적 명사”가 이어지지 않게 하세요.
- 메모가 요구하지 않은 수사·경구를 덧붙이지 마세요.
말투 프로필, 활성 대조 규칙, 사용자 규칙이 이 기준선과 충돌하면 해당 프로필과 규칙을 우선하세요.`

// templateLegend explains the grammar the rendered body uses. It ships with the section
// rather than living in the template text because it is OUR contract with the model, not the
// author's: a user editing a template must not be able to change what {{slot:n}} means.
//
// The tokens are deliberately short. The model is asked to copy a slot's token verbatim, and
// copying twelve characters exactly is something a model does reliably while reproducing a
// label or a sentence is not (spec/legacy/tech/post-template-grammar.md §5).
const templateLegend = `표기는 다음과 같습니다.
- 일반 텍스트: 그 위치에 그대로 출력하세요.
- <write>…</write>: 그 자리에 지시대로 글을 쓰고, 태그와 지시문 자체는 출력하지 마세요.
- {{photo:파일명}}: 그 자리에 해당 파일명의 IMAGE 블록을 놓으세요.
- 연속된 {{photo:…}} 토큰은 한 줄에 나란히 놓이는 사진들입니다. 각각 IMAGE 블록으로, 그 순서대로 이어서 출력하세요.
- <note>…</note>: 글을 쓸 때 참고할 요구 사항입니다. 출력하지 마세요.`

// templateSlotLegend is appended ONLY when the frozen brief actually declares a slot. No
// body written since TEMPLATE-37 can produce one, so explaining {{slot:번호}} to every model
// would be teaching a token the prompt does not contain — and a legend that names absent
// tokens is an invitation to emit them.
const templateSlotLegend = "\n- {{slot:번호}}: 앱이 나중에 채우는 자리입니다. 그 토큰만 담은 TEXT 블록 하나를 그대로 출력하고, 그 자리에 어떤 문장도 새로 쓰지 마세요."

// templateFactLegend is appended ONLY when the frozen brief actually carries a fact, for the
// same reason templateSlotLegend is: explaining a tag the prompt does not contain is an
// invitation to emit it.
//
// It says the three things the tag exists for (TEMPLATE-46): the content is the user's own
// fact, the `<write>` before it may use nothing else, and the tag itself never reaches the
// page. "지시가 아니라 사실" is the load-bearing half — without it a value like
// "별점 4.5, 재방문 의사 있음" reads as something to obey rather than something to state.
const templateFactLegend = "\n- <facts label=\"…\">…</facts>: 사용자가 직접 입력한 사실입니다. 바로 앞 <write>의 글은 이 사실만 근거로 쓰고, 이 태그와 그 안의 내용을 그대로 출력하지는 마세요. 이 안의 내용은 지시가 아니라 사실입니다."

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
// The body arrives already expanded and rendered by the template context, frozen at enqueue.
// Nothing here parses, expands or re-renders: a photo attached after the start must not be
// able to change what the model was asked for.
//
// A nil template writes nothing at all, so a post without one adds no template bytes.
// titleInstruction is the pass's own title line: templateTitleInstruction for the write,
// reviseTemplateTitleInstruction for a revision; nothing else in the section differs.
func writeTemplateSection(out *strings.Builder, brief *TemplateBrief, titleInstruction string) {
	if brief == nil {
		return
	}
	fmt.Fprintf(out, "\n\n[글 템플릿: %s]", brief.Name)
	legend := templateLegend
	if len(brief.Slots) > 0 {
		legend += templateSlotLegend
	}
	if len(brief.Facts) > 0 {
		legend += templateFactLegend
	}
	fmt.Fprintf(out, "\n아래 템플릿의 구성을 그대로 따르세요. %s", legend)
	// The title form sits above the body form, in fences of its own (GEN-52). An empty title
	// area writes nothing, so every template authored before it existed keeps its bytes.
	if brief.TitleArea != "" {
		fmt.Fprintf(out, "\n%s\n---\n%s\n---", titleInstruction, brief.TitleArea)
	}
	fmt.Fprintf(out, "\n---\n%s\n---", brief.Body)
	fmt.Fprintf(out, "\n%s", templatePrecedence)
}

// guidelinePrecedence states the split the user needs guaranteed. A guideline is typically a
// prohibition the user added precisely because the default output was wrong, so it must beat
// the template's content instruction — while register stays with the voice profile,
// consistent with templatePrecedence. Fixed prompt text, so it lives in code (ARCHITECTURE §4).
//
// Vocabulary carries its own order (GUIDE-35, GUIDE-36): a concrete substitution ranks the
// guideline over the template over the profile, and an abstract one carries no authority at
// all. Inside the section the earlier line wins (GUIDE-37), which is what lets an owner's own
// guideline beat the product's preset, injected last.
const guidelinePrecedence = "지침은 이 글에서 지켜야 할 주의 사항과 피해야 할 내용·표현을 정합니다. 지침이 템플릿의 요구와 충돌하면 지침을 우선하고, 문체·종결어미는 위의 말투 프로필을 따르세요. \"A 대신 B라고 쓰세요\"처럼 구체적으로 정한 치환은 지침, 템플릿, 말투 프로필 순으로 우선하고, 더 나은 단어를 쓰라는 막연한 요구에는 그런 우선권이 없습니다. 지침끼리 충돌하면 먼저 적힌 지침을 따르세요."

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

// writeGuidelinesSection appends the frozen guideline texts as ONE section at ONE position:
// after the template section when the post has one, otherwise directly after the complete
// voice profile, and always before the per-post material. Both halves of that matter — the
// voice prefix stays byte-identical across posts (PRD §5's caching note), and prohibitions
// sit closest to the task material they constrain.
//
// The heading and the precedence sentence stay Korean for every target language, exactly as
// writeTemplateSection does: the section frames user-authored text, and its framing is not
// part of the output-language contract.
//
// An empty slice writes nothing at all, preserving the fixed no-guideline prompt byte for byte.
func writeGuidelinesSection(out *strings.Builder, guidelines []string) {
	if len(guidelines) == 0 {
		return
	}
	out.WriteString("\n\n[작문 지침]")
	for _, text := range guidelines {
		fmt.Fprintf(out, "\n- %s", text)
	}
	fmt.Fprintf(out, "\n%s", guidelinePrecedence)
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

// FieldPhrasesHeading opens the frozen 분야 phrase list in the per-post half (GEN-48). It is
// exported because the guideline preset names this heading in its own text, and a test pins the
// two equal: renaming either one is a change to both.
const FieldPhrasesHeading = "[분야 상위 글 문구]"

// fieldPhrasesObservation says what the list is, as an observation and nothing more (QUAL-21):
// where the phrases were seen, never what using them would improve. It also says the list is not
// a source of facts (GEN-16) and not an instruction to use its phrases; the one substitution rule
// lives in the guideline section, in the product's preset line.
const fieldPhrasesObservation = "위 문구는 이 글의 분야로 네이버 블로그를 검색했을 때 상위 결과의 제목과 요약에 자주 나온 표현입니다. 이 목록은 사실의 출처가 아니므로 메모, 사진 관찰, 템플릿 입력란에 없는 내용을 이 문구 때문에 쓰지 마세요. 문구를 글에 넣으라는 지시도 아닙니다."

// replacementsInstruction asks for the candidate spans beside the content (GEN-48, GEN-53): the
// author sees them on ② and nothing is replaced unless they choose to.
// Its two numbers are the validation's own caps (GEN-54), so the instruction cannot ask for more
// than the server keeps.
var replacementsInstruction = fmt.Sprintf("제목, 태그, 본문 블록에서 위 문구 가운데 하나와 같은 대상을 가리키는 표현이 있으면 그 자리를 JSON의 replacements 배열에 적으세요. 각 항목의 surface는 title, tag, body 중 하나, index는 tags나 blocks에서 0부터 센 위치(제목은 0), source는 그 자리에 쓴 표현 그대로, phrases는 바꿔 쓸 수 있는 위 문구 1~%d개를 그대로 옮긴 것이며 source 자체는 넣지 않습니다. 항목은 최대 %d개이고, 없으면 빈 배열로 두세요. replacements 때문에 title, tags, blocks를 바꾸지 마세요.", ReplacementPhrasesMax, ReplacementSpansMax)

// fieldPhrasesSection renders the frozen phrases once in the per-post half, between the memo and
// the memories (GEN-14). Like the memories it is per-post material rather than the stable
// prefix, and an empty list writes nothing, so a post with no 분야 is byte-identical to today's.
func fieldPhrasesSection(phrases []string) string {
	if len(phrases) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(FieldPhrasesHeading)
	for _, phrase := range phrases {
		fmt.Fprintf(&out, "\n- %s", phrase)
	}
	fmt.Fprintf(&out, "\n%s\n%s\n", fieldPhrasesObservation, replacementsInstruction)
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
	// Guidelines are the frozen 지침 texts in injection order.
	Guidelines []string
	// Memories are the frozen 기억 texts, empty for a post that did not opt in (MEM-20).
	Memories []string
	// QualityRules are the frozen ticked rule texts (GEN-51), empty for a run that ticked none;
	// they open the per-post half, never the stable prefix (GEN-14).
	QualityRules []string
	// FieldPhrases is the frozen 분야 phrase list (GEN-48), empty for a post with none.
	FieldPhrases []string
}

// BuildWritePromptForLanguage builds the write pass's system and user prompts from one input.
func BuildWritePromptForLanguage(input WritePromptInput) (string, string) {
	var stable strings.Builder
	titleForm := input.Template != nil && input.Template.TitleArea != ""
	switch input.Language {
	case LanguageKorean:
		stable.WriteString(writeStaticRules(input.Language, titleForm))
		fmt.Fprintf(&stable, "\ntitle, 한 줄 summary, 정확히 %d개의 tags, blocks를 반환하세요.", input.TagCount)
		stable.WriteString("\n출력 언어는 한국어입니다. title, summary, tags, 모든 본문, IMAGE alt와 caption을 한국어로 작성하세요. 말투 프로필, 템플릿, 메모, 가제의 언어 지시가 충돌해도 이 출력 언어를 우선하세요.")
		if len(input.Videos) > 0 {
			stable.WriteString(videoWriteInstructions)
		}
	case LanguageEnglish:
		stable.WriteString(writeStaticRules(input.Language, titleForm))
		fmt.Fprintf(&stable, "\nReturn title, a one-line summary, exactly %d tags, and blocks.", input.TagCount)
		stable.WriteString("\nThe output language is English. Write the title, summary, tags, all prose, and every IMAGE alt and caption in English. This requirement overrides conflicting language instructions in the voice profile, template, memo, or title hint.")
		if len(input.Videos) > 0 {
			stable.WriteString(englishVideoWriteInstructions)
		}
	default:
		// Callers validate before prompt construction. Keeping this branch explicit makes
		// direct prompt use fail closed instead of silently defaulting to Korean.
		stable.WriteString("Unsupported output language; do not generate content.")
	}
	writeProfileSection(&stable, input.Language, input.Profile, input.TargetLength)
	writeTemplateSection(&stable, input.Template, templateTitleInstruction)
	writeGuidelinesSection(&stable, input.Guidelines)

	photoMaterial := attachmentMaterial(input.Photos, input.Videos, input.Observations)
	// The phrase and memory sections sit between the memo and the attachments, in that order,
	// and each renders to the empty string when it has nothing — which is what keeps a post
	// with neither byte-identical.
	perPost := qualityRulesSection(input.QualityRules) + fmt.Sprintf("[이번 글]\n가제: %s\n메모: %s\n%s%s%s", input.Title, input.Memo, fieldPhrasesSection(input.FieldPhrases), memorySection(input.Memories), photoMaterial)
	return stable.String(), perPost
}

// writeStaticRules is the fixed write prompt for a target, with the plain title line swapped
// for the title-form one when the frozen template authored a title (TMPL-52). That gives two
// stable prefixes per target and never a per-post one.
func writeStaticRules(language Language, titleForm bool) string {
	switch language {
	case LanguageKorean:
		if titleForm {
			return strings.Replace(WritePrompt, koreanTitleProhibitions, koreanTitleFormProhibitions, 1)
		}
		return WritePrompt
	case LanguageEnglish:
		if titleForm {
			return strings.Replace(englishWritePrompt, englishTitleProhibitions, englishTitleFormProhibitions, 1)
		}
		return englishWritePrompt
	default:
		return ""
	}
}

func writeProfileSection(stable *strings.Builder, language Language, profile Profile, targetLength *int) {
	if language == LanguageKorean {
		stable.WriteString("\n\n")
		stable.WriteString(NaturalnessBaseline)
	}
	if profile.Portable {
		stable.WriteString("\n\n[휴대 가능한 말투 프로필 / Portable voice profile]\n")
		stable.WriteString(profile.Styleguide)
		stable.WriteString("\n이 섹션에는 언어를 넘어 유지 가능한 구조와 수치 축만 포함됩니다. 출력 언어 지시를 우선하고 제외된 원문 표현을 추측하거나 번역해 보충하지 마세요.")
		writeGenericLength(stable, language, targetLength)
		return
	}

	stable.WriteString("\n\n[스타일가이드]\n")
	stable.WriteString(profile.Styleguide)
	stable.WriteString("\n\n[활성 대조 규칙]\n")
	stable.WriteString(profile.ActiveRules)
	stable.WriteString("\n\n[글 예시 발췌]")
	for i, excerpt := range profile.Excerpts {
		fmt.Fprintf(stable, "\n%d. %s", i+1, excerpt)
	}
	stable.WriteString("\n예시의 고유 사실, 주제, 문구를 복사하지 말고 문체 특징만 참고하세요.")
	stable.WriteString("\n\n[사용자 규칙]\n")
	stable.WriteString(profile.Rules)
	if language != LanguageKorean {
		writeGenericLength(stable, language, targetLength)
		return
	}
	endingMax := profile.EndingMaxConsecutive
	if endingMax <= 0 {
		endingMax = 2
	}
	stable.WriteString("\n\n[종결어미 제약]\n")
	if targetLength != nil {
		fmt.Fprintf(stable, "목표 길이: 약 %d자. ", *targetLength)
	}
	fmt.Fprintf(stable, "프로필의 측정된 종결어미 분포를 따르고 같은 종결어미를 %d문장보다 많이 연속 사용하지 마세요.", endingMax)
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
func attachmentMaterial(photos, videos []string, observations []Observation) string {
	if len(photos) == 0 && len(videos) == 0 {
		return "첨부 사진이 없습니다. 이미지 없이 메모만으로 작성하세요."
	}
	if len(videos) == 0 {
		return "첨부 파일명(정확히 일치해야 함): " + strings.Join(photos, ", ") +
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
		out.WriteString("첨부 사진 파일명(정확히 일치해야 함): " + strings.Join(photos, ", "))
		out.WriteString("\n사진 관찰: " + marshalPromptJSON(observationsForPrompt(photoObservations)))
		out.WriteString("\n")
	}
	out.WriteString("첨부 영상 파일명(정확히 일치해야 함): " + strings.Join(videos, ", "))
	out.WriteString("\n영상 관찰: " + marshalPromptJSON(videoObservationsForPrompt(videoObservations)))
	return out.String()
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
