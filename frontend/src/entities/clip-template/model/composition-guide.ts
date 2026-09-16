import i18next from 'i18next'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'

export const CLIP_COMPOSITION_EXAMPLE = `<clip version="1" intro="b" caption="bold" outro="e">
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
  <text id="disclosure_badge" kind="fixed" role="badge" position="top" align="left" basis="whole"><value field="disclosure"/></text>
  <text id="intro" kind="fixed" role="hook" basis="output-start"><row kind="fixed"><value field="place"/></row><row kind="fixed"><value field="region"/></row></text>
  <text id="closing" kind="fixed" role="ending" basis="output-end"><row kind="fixed"><value field="place"/></row><row kind="fixed"><value field="score"/></row><row kind="fixed"><value field="verdict"/></row></text>
</clip>`

/** Closed grammar shared by both localized guides; examples are parsed in tests. */
const GRAMMAR = `clip(version="1", intro="a|b", caption="bold", outro="b|e")
clip children: field, group, guide, text. A template declares only what every clip made from it must carry: the design, the intro/outro slot text, an optional badge, the information to collect and one optional guide. Footage order, cut rhythm and every caption are decided at generation from the project instruction and the observed footage; caption pace and accent are chosen per project.
field(id, label, required="true|false", chars?) = prompt text; default required=false. chars is the maximum number of characters the answer may hold.
group(id, label?, min?, max?) children: field; group ID "scenes" is reserved. label is the owner-visible group name (default: generic). min/max are non-negative integers, min <= max <= items limit; defaults: min=0, max=items limit. Generation requires at least min items; incomplete drafts can still be saved. An item's facts (name, price) are stated by the narration where the instruction wants them; they are never bound to a scene.
guide = invisible instructions to the writing model, inside clip only: voice, viewpoint, forbidden phrases, what to dwell on. The project instruction comes first and the guide follows it.
text(id, kind="fixed|ai", role="badge|hook|ending", position="auto|top|upper_mid|lower_mid|bottom|header", align="left|center|right", basis="whole|output-start|output-end", start, end, chars?)
Exactly one hook and one ending must be direct clip children. Hook basis=output-start, ending basis=output-end; omit position and align. Hook/ending contain only rows, without role. Omitted intervals default to 0–2.5 seconds and -3–0 seconds respectively. At most 2 intro rows, 2 outro-B rows or 3 outro-E rows. Empty slots keep their positions; overflow is invalid_skeleton. All three root design attributes are required. Only slot text/authorship is authorable; never change slot order, count, type, position or decoration.
badge is the only other visible element: kind="fixed", exact literal + value, basis whole or an explicit output interval; header, top or bottom position.
Not part of this grammar (refused as unsupported_section, unsupported_role, unsupported_basis): scene, repeat, role="caption"|"info", basis="cut". Root accent and pace are read from older templates but no longer authored.
text content: exact literal + <value field="FIELD_ID"/>. Only global fields can be referenced; a group's fields belong to the narration.
hook/ending rows: <row kind="fixed|ai" chars?>literal + value</row>; kind defaults to the parent. Fixed and answer-bound values are exact. AI rows are separate one-line slots. Character limits: intro A 8/22, intro B 9/22, outro B 9/14, outro E 22/6/18. Never add manual newlines or an extra row.
chars is a positive integer and may only be SMALLER than what the position already holds; a larger one is invalid_max. Undeclared means the position's own number. Those numbers, counted as Korean syllables excluding spaces and punctuation: intro A 8/22, intro B 9/22, outro B 9/14, outro E 22/6/18; a field without a declared number follows the positions its answer reaches; badge text has no number of its own.
whole: omit start/end. output-start: 0 <= start < end. output-end: start < end <= 0.
Seconds have up to 3 decimal places. Explicit intervals must fit the output, without clamping. Header is only for the badge. Defaults: position=auto, align=center.
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
