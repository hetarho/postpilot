# Portable clip composition v1

`Parse` validates source into a typed `Document`; `Resolve` binds explicit answers
and selected footage cuts. Neither function chooses footage, calls AI or renders.
An AI element's resolved text is its **writer instruction**, while a fixed
element's resolved text is the exact output after explicit value substitution.

```xml
<clip version="1" styles="clean memo" pace="steady" accent="teal">
  <field id="location" label="지역">촬영한 지역</field>
  <group id="menu">
    <field id="name" label="메뉴" required="true"/>
    <field id="price" label="가격"/>
  </group>
  <guide>촬영한 순서와 실제 장면에 맞춰 구성하세요.</guide>
  <text id="disclosure" kind="fixed" role="badge" position="header"
        basis="whole">협찬받아 촬영한 영상입니다.</text>
  <repeat for="menu">
    <scene id="dish" scope="item">
      <text id="caption" kind="ai" role="caption" basis="cut">
        화면 속 <value field="menu.name"/>의 관찰 가능한 모습만 설명하세요.
      </text>
      <text id="price" kind="fixed" role="info" basis="cut">
        <value field="menu.name"/> · <value field="menu.price"/>
      </text>
    </scene>
  </repeat>
  <text id="closing" kind="fixed" role="ending"
        basis="output-end" start="-2.5" end="0">
    <row role="title">다음에 함께 가요</row>
    <row role="body">마음에 들면 저장해 주세요</row>
  </text>
</clip>
```

- Root children: `field`, `group`, invisible `guide`, `scene`, `repeat`, `text`.
  A group contains fields. A scene contains guides/text and selects actual
  footage with `scope="scene|item|context"`. Repetition contains scenes and uses
  `for="scenes"` or a declared group. Nested repetition is refused; `scenes` is
  reserved as a group ID. Unfilmed items do not create synthetic footage.
- Template IDs use 1–64 ASCII letters, digits, underscores or hyphens, starting
  with a letter or digit. Fields inside groups have qualified identity, e.g.
  `menu.price`. Visible element and scene IDs are unique across the document.
- A text declares `kind="fixed|ai"`, `role="caption|info|badge|hook|ending"` and
  `basis="whole|output-start|output-end|cut"`. Style defaults to `auto`, position
  to `auto`, alignment to `center`; explicit styles must belong to the root's
  approved set. Header placement is only for info/badge. Card rows (hook,
  ending, info) use `hook|title|mark|body|caption|label|badge` typography roles.
- `value` is the only substitution. Global fields use their ID; grouped fields
  require `group.field` and the current matched item. Optional blank values omit
  the dependent element. A required unresolved value is an error; another item's
  answer is never a fallback. No role inserts an undeclared sibling element.
- Seconds use at most three decimal places. Whole-output intervals have no
  endpoints; start-relative endpoints are nonnegative, end-relative endpoints
  nonpositive. Cut intervals are cut-local and may omit both endpoints to use
  the configured automatic inset. Invalid intervals fail instead of clamping.
  Output cut starts subtract incoming transition overlap. Stable cut IDs carry
  elements through reordering; deleting a cut deletes only its own instances.
- `Source` is unchanged, with half-open Unicode scalar spans and 1-based lines.
  No-edit mode switches use `Source`; builder edits call `ReplaceNode` with a
  qualified ID or `ReplaceSpan` with a current node span. Only that subtree is
  serialized. XML entities decode once; literal whitespace is not normalized.
- No scripts, expressions, HTML, DTD, processing instructions, remote entities,
  includes, arbitrary CSS or unknown attributes are accepted. Source and inputs
  obey `config.ClipCompositionLimits`; the frontend mirrors those bounds in
  `shared/config/clip-composition.ts`, checked by the same corpus. The resolver
  also accepts the caller's remaining expanded UTF-8 text budget. The writer
  must separately check its complete serialized provider request before charging.

`testdata/corpus.json` is the shared Go/TypeScript oracle for semantics, errors,
Unicode source spans, bindings, timing and finite bounds. Keep it shared rather
than generating one implementation's expected results from the other.
