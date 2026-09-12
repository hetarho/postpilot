import i18next from 'i18next'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'

export const CLIP_COMPOSITION_EXAMPLE = `<clip version="1" styles="clean memo simple" accent="teal" pace="steady">
  <field id="place" label="장소" required="false">촬영한 장소</field>
  <group id="menu">
    <field id="name" label="메뉴 이름" required="true"/>
    <field id="price" label="가격" required="false">예: 1인분 12,000원</field>
  </group>
  <guide>선택한 영상 속 장면과 각 메뉴의 정보를 연결하세요. 없는 장면이나 숫자는 만들지 마세요.</guide>
  <text id="disclosure" kind="fixed" role="badge" position="header" basis="whole">직접 구매한 메뉴입니다</text>
  <repeat for="menu">
    <scene id="dish" scope="item">
      <guide>이 메뉴가 실제로 보이는 장면을 사용하세요.</guide>
      <text id="caption" kind="ai" role="caption" style="auto" basis="cut">화면에 보이는 <value field="menu.name"/>의 모습만 짧게 설명하세요.</text>
      <text id="price" kind="fixed" role="info" basis="cut"><value field="menu.name"/> · <value field="menu.price"/></text>
    </scene>
  </repeat>
  <text id="closing" kind="fixed" role="ending" basis="output-end" start="-3" end="0">
    <row role="title">다음에 함께 가요</row>
    <row role="body"><value field="place"/>에서 촬영했어요</row>
  </text>
</clip>`

/** Closed grammar shared by both localized guides; examples are parsed in tests. */
const GRAMMAR = `clip(version="1", styles="clean memo bold mark simple", accent="coral|amber|lime|teal|blue|violet|pink|", pace="steady|rapid")
clip children: field, group, guide, scene, repeat, text
field(id, label, required="true|false") = prompt text; default required=false
group(id) children: field; group ID "scenes" is reserved
guide = invisible instructions, inside clip or scene
scene(id, scope="scene|item|context") children: guide, text
repeat(for="scenes|GROUP_ID") children: scene; group repetition requires scope=item; no nested repeat
text(id, kind="fixed|ai", role="caption|info|badge|hook|ending", style="auto|APPROVED_STYLE", position="auto|top|upper_mid|lower_mid|bottom|header", align="left|center|right", basis="whole|output-start|output-end|cut", start, end)
text content: exact literal + <value field="FIELD_ID"/> or <value field="GROUP_ID.FIELD_ID"/>
text rows (hook/ending/info only): <row role="hook|title|mark|body|caption|label|badge">literal + value</row>
whole: omit start/end. output-start: 0 <= start < end. output-end: start < end <= 0. cut: only inside scene; omit both endpoints for automatic timing or use 0 <= start < end.
Seconds have up to 3 decimal places. Explicit intervals must fit the output/cut, without clamping. Header is only for info/badge. Defaults: style=auto, position=auto, align=center.
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
