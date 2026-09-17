# Portable clip composition v1

`Parse` validates source into a typed `Document`; `Resolve` binds explicit answers
and selected footage cuts. Neither function chooses footage, calls AI or renders.

Three entries read the same grammar at different strictness. `ParseTemplate` is
what a saved TEMPLATE body must satisfy (CLIP-4, CLIP-59, CLIP-112): an ordered
outline of named composition stages and visible text entries — intro, caption
and outro — beside the badge, the fields, the groups and the invisible guides.
Where an entry stands in that outline is the only position it declares, so it
refuses `scene`, `repeat` (`unsupported_section`), a text whose role is `info`
(`unsupported_role`) and any `basis`, `start` or `end` (`unsupported_basis`).
The root still accepts `intro`, `caption` and `outro` and reads nothing from
them: the presets a clip renders in are the project's (CLIP-14, CLIP-139), and a
surplus region line is a render notice rather than a refusal (CLIP-147). `Parse`
still accepts every construct below so a project's frozen snapshot keeps
resolving and rendering exactly as it was frozen (CLIP-140), and `ReadStored`
additionally tolerates retired attributes. `ConvertLegacyTemplate` carries a
section body's guides and scene-bound texts into one root `guide` as prose for
the template read projection; the stored body changes only when the owner saves.
The example below therefore shows the full snapshot grammar, not what a new
template may declare.

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
  <stage name="식당 소개">간판과 외관을 먼저 보여준다</stage>
  <stage name="음식">주문한 메뉴가 나오는 순간</stage>
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

- Root children: `field`, `group`, invisible `guide`, `stage`, `scene`,
  `repeat`, `text`. `Document.Outline` carries the root's stages and texts in
  document order — the outline CLIP-112 makes the entry's only position — while
  the per-kind slices stay beside it for readers that need one kind.
  A group contains fields. A scene contains guides/text and selects actual
  footage with `scope="scene|item|context"`. Repetition contains scenes and uses
  `for="scenes"` or a declared group. Nested repetition is refused; `scenes` is
  reserved as a group ID. Unfilmed items do not create synthetic footage.
- `stage` is one named composition stage (CLIP-141): a `name` within the label
  count and one line of intent within the prompt count, both required, at most
  `Stages` of them and root children only — a stage is a property of the whole
  clip, so inside a scene or a repetition `stage` stays an unknown tag. They are
  ordered guidance handed to the flow call in the template guide's position and
  nothing else reads them: no stage admits footage, refuses it or is reported
  missing.
- Template IDs use 1–64 ASCII letters, digits, underscores or hyphens, starting
  with a letter or digit. Fields inside groups have qualified identity, e.g.
  `menu.price`. Visible element and scene IDs are unique across the document.
- A text declares `kind="fixed|ai"`, `role="caption|info|badge|hook|ending"` and,
  in a snapshot only, `basis="whole|output-start|output-end|cut"`. An entry that
  declares no basis takes the span its role is drawn at: the output's opening for
  `hook`, its end for `ending`, the whole output for a `badge` or a `caption` the
  narration has yet to time. Style defaults to `auto`, position
  to `auto`, alignment to `center`; named styles apply only to caption roles and must belong to the root's
  approved set. Badge, info, hook and ending require `auto` (or omit style). Header placement is only for info/badge. Card rows (hook,
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
