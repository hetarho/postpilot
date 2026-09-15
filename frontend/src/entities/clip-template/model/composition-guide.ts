import i18next from 'i18next'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'

export const CLIP_COMPOSITION_EXAMPLE = `<clip version="1" intro="b" caption="bold" outro="e" accent="teal" pace="steady">
  <field id="place" label="장소" required="false">촬영한 장소</field>
  <group id="menu">
    <field id="name" label="메뉴 이름" required="true"/>
    <field id="price" label="가격" required="false">예: 1인분 12,000원</field>
  </group>
  <guide>선택한 영상 속 장면과 각 메뉴의 정보를 연결하세요. 없는 장면이나 숫자는 만들지 마세요.</guide>
  <text id="disclosure" kind="fixed" role="badge" position="header" basis="whole">직접 구매한 메뉴입니다</text>
  <text id="opening" kind="ai" role="hook" basis="output-start"><row>관찰한 음식의 특징을 9자 이내로 설명하세요.</row></text>
  <repeat for="menu">
    <scene id="dish" scope="item">
      <guide>이 메뉴가 실제로 보이는 장면을 사용하세요.</guide>
      <text id="caption" kind="ai" role="caption" basis="cut">화면에 보이는 <value field="menu.name"/>의 모습만 짧게 설명하세요.</text>
      <text id="price" kind="fixed" role="info" basis="cut"><value field="menu.name"/> · <value field="menu.price"/></text>
    </scene>
  </repeat>
  <text id="closing" kind="fixed" role="ending" basis="output-end" start="-3" end="0">
    <row>다음 방문</row>
    <row></row>
    <row><value field="place"/></row>
  </text>
</clip>`

/** Closed grammar shared by both localized guides; examples are parsed in tests. */
const GRAMMAR = `clip(version="1", intro="a|b", caption="bold", outro="b|e", accent="coral|amber|lime|teal|blue|violet|pink|", pace="steady|rapid")
clip children: field, group, guide, scene, repeat, text
field(id, label, required="true|false") = prompt text; default required=false
group(id, label?, min?, max?) children: field; group ID "scenes" is reserved. label is the owner-visible group name (default: generic). min/max are non-negative integers, min <= max <= items limit; defaults: min=0, max=items limit. Generation requires at least min items; incomplete drafts can still be saved.
guide = invisible instructions, inside clip or scene
scene(id, scope="scene|item|context") children: guide, text
repeat(for="scenes|GROUP_ID") children: scene; group repetition requires scope=item; no nested repeat
text(id, kind="fixed|ai", role="caption|info|badge|hook|ending", position="auto|top|upper_mid|lower_mid|bottom|header", align="left|center|right", basis="whole|output-start|output-end|cut", start, end)
Exactly one hook and one ending must be direct clip children. Hook basis=output-start, ending basis=output-end; omit position and align. Hook/ending contain only rows, without role. Omitted intervals default to 0–2.5 seconds and -3–0 seconds respectively. At most 2 intro rows, 2 outro-B rows or 3 outro-E rows. Empty slots keep their positions; overflow is invalid_skeleton. All three root design attributes are required.
text content: exact literal + <value field="FIELD_ID"/> or <value field="GROUP_ID.FIELD_ID"/>
hook/ending rows: <row kind="fixed|ai">literal + value</row>; kind defaults to the parent. Fixed and answer-bound values are exact. AI rows are separate one-line slots. Character limits: intro A 8/8, intro B 9/9, outro B 9/9, outro E 16/8/16. Never add manual newlines or an extra row.
info rows: <row role="label|caption">literal + value</row>.
whole: omit start/end. output-start: 0 <= start < end. output-end: start < end <= 0. cut: only inside scene; omit both endpoints for automatic timing or use 0 <= start < end.
Seconds have up to 3 decimal places. Explicit intervals must fit the output/cut, without clamping. Header is only for info/badge. Defaults: position=auto, align=center (caption/info/badge only).
IDs: 1–64 ASCII letters/digits/_/-, first letter or digit. Group field IDs are qualified; other IDs are unique.
XML escaping: &amp; &lt; &gt; &quot; &apos; and valid numeric entities. No HTML, scripts, expressions, CSS, DTD, processing instructions, remote references, unknown tags or attributes.`

export function clipCompositionGuide() {
  return i18next.t('composition.guide', {
    ns: 'clips',
    grammar: GRAMMAR,
    example: CLIP_COMPOSITION_EXAMPLE,
    limits: Object.entries(CLIP_COMPOSITION_LIMITS)
      .map(([key, value]) => `${key}=${value}`)
      .join(', '),
    interpolation: { escapeValue: false },
  })
}
