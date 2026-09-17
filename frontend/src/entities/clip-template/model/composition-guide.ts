import i18next from 'i18next'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'

export const CLIP_COMPOSITION_EXAMPLE = `<clip version="1">
  <field id="place" label="상호명" required="true">간판에 적힌 이름을 공백 빼고 9자 이내로 적어주세요. 예: 성수 곱창</field>
  <field id="region" label="지역" required="true">동네나 지역을 짧게 적어주세요. 예: 성수동</field>
  <field id="verdict" label="한줄평" required="true">이번 식사를 한 문장으로 적어주세요. 공백 빼고 18자까지 들어갑니다.</field>
  <field id="score" label="평점·사람 말로" required="true">점수를 숫자 대신 말로, 공백 빼고 6자 이내로 적어주세요. 예: 또 갈래요</field>
  <field id="disclosure" label="광고·협찬 등 표시 문구" required="false">화면에 그대로 표시할 문장을 적어주세요. 없으면 비워두세요.</field>
  <group id="menu" label="메뉴" min="1">
    <field id="name" label="메뉴 이름" required="true">먹은 순서대로 메뉴를 하나씩 추가해 주세요. 예: 살치살</field>
    <field id="price" label="가격" required="false">단위까지 적어주세요. 예: 1인분 18,000원</field>
  </group>
  <guide>친구에게 카톡으로 말하듯 자연스러운 존댓말로 쓰세요. 감탄사와 최상급, 광고성 수식어를 남발하지 마세요. 메뉴 이름과 가격은 입력된 그대로 말하고, 화면으로 알 수 없는 맛과 향은 지침에 적힌 만큼만 말하세요.</guide>
  <text id="disclosure_badge" kind="fixed" role="badge" position="top" align="left"><value field="disclosure"/></text>
  <text id="intro" kind="fixed" role="hook"><row kind="fixed"><value field="place"/></row><row kind="fixed"><value field="region"/></row></text>
  <stage name="가게 앞">간판과 외관을 먼저 보여줍니다</stage>
  <stage name="음식">주문한 메뉴가 나오는 순간</stage>
  <text id="taste" kind="ai" role="caption">첫 한 입의 인상을 한 줄로 쓰세요</text>
  <text id="closing" kind="fixed" role="ending"><row kind="fixed"><value field="place"/></row><row kind="fixed"><value field="score"/></row><row kind="fixed"><value field="verdict"/></row></text>
</clip>`

/** Closed grammar shared by both localized guides; examples are parsed in tests. */
const GRAMMAR = `clip(version="1")
clip children, in the order the clip shows them: field, group, guide, stage, text. That order IS the outline and it is the ONLY position an entry declares — a template says what the clip shows from beginning to end and nothing about when or where each entry is drawn. Footage order, cut rhythm and every caption's time are decided at generation from the project instruction and the observed footage; the intro/outro design, the caption styles, the caption pace and the accent are chosen per clip.
field(id, label, required="true|false", chars?) = prompt text; default required=false. chars is the maximum number of characters the answer may hold.
group(id, label?, min?, max?) children: field; group ID "scenes" is reserved. label is the owner-visible group name (default: generic). min/max are non-negative integers, min <= max <= items limit; defaults: min=0, max=items limit. Generation requires at least min items; incomplete drafts can still be saved. An item's facts (name, price) are stated by the narration where the instruction wants them; they are never bound to a scene.
guide = invisible instructions to the writing model, inside clip only: voice, viewpoint, forbidden phrases, what to dwell on. The project instruction comes first and the guide follows it.
stage(name) = one line of intent. A named composition stage the flow follows where the footage allows: it admits and forbids no footage, a stage nothing was filmed for is skipped or merged, and footage matching no stage is kept where it belongs. Root children only. Name up to the label limit, intent up to the prompt limit.
text(id, kind="fixed|ai", role="hook|caption|ending|badge", position="auto|top|upper_mid|lower_mid|bottom|header", align="left|center|right", chars?)
role: hook = one intro line or more, ending = the outro's, caption = one caption over the footage, badge = the disclosure. Any number of each, in any order; hook and ending hold rows, caption and badge hold text directly.
A text declares NO timing — no basis, no start, no end. The intro plays over the output's opening, the outro over its end, the badge over the whole clip, and a caption where the narration places it; the owner changes any of them in ② afterwards.
How many intro and outro lines are drawn is the design the clip chooses: lines fill that design's places in the order you wrote them, and one it cannot hold is left out with a notice rather than refused. Omit position and align on hook and ending; header placement is the badge's.
kind: fixed text is shown exactly as written after value substitution and is never rewritten; an ai text is an instruction to the writer, which answers it with words of its own.
text content: exact literal + <value field="FIELD_ID"/>. Only global fields can be referenced; a group's fields belong to the narration.
hook/ending rows: <row kind="fixed|ai" chars?>literal + value</row>; kind defaults to the parent. Fixed and answer-bound values are exact. AI rows are separate one-line entries. Never add manual newlines.
chars is a positive integer, at most the copy limit below. Undeclared means the number the drawn position already holds, which is the design's and is applied when the clip renders.
Not part of this grammar (refused as unsupported_section, unsupported_role, unsupported_basis): scene, repeat, role="info", and any basis, start or end. Root intro, caption, outro, accent and pace are read from older templates and no longer authored.
IDs: 1–64 ASCII letters/digits/_/-, first letter or digit. Group field IDs are qualified; other IDs are unique.
XML escaping: &amp; &lt; &gt; &quot; &apos; and valid numeric entities. No HTML, scripts, expressions, CSS, DTD, processing instructions, remote references, unknown tags or attributes.`

export function clipCompositionGuide(currentSource?: string) {
  const guide = i18next.t('composition.guide', {
    ns: 'clips',
    grammar: GRAMMAR,
    example: CLIP_COMPOSITION_EXAMPLE,
    limits: Object.entries(CLIP_COMPOSITION_LIMITS)
      .map(([key, value]) => `${key}=${value}`)
      .join(', '),
    interpolation: { escapeValue: false },
  })
  return currentSource
    ? guide +
        '\n\n' +
        i18next.t('composition.design.selectedSource', { ns: 'clips' }) +
        '\n' +
        currentSource
    : guide
}
