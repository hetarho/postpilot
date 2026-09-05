# Tech — Post template grammar (글 템플릿 문법)

The authoring contract for a **template** body: its five constructs, how a body is parsed, how it is expanded and
rendered into the write/revise prompts, and how the slots it declares come back in the canonical block array.
Source: [changes/25](../changes/archive/25.replace-purposes-with-drag-and-drop-post.md), built by job 53.

A template body is the **single source of truth**: it is what `templates.body` stores, what the prompt receives
(after expansion), and what the drag-and-drop builder round-trips. The builder is a parser plus a renderer over that
text, never a second representation of it.

**The grammar is internal.** It is the contract between the builder and the write prompt, and since
[change 30](../changes/archive/30.rework-the-template-screen-into-a-list-a.md) it is **never rendered to the user**:
there is no source view, no mode switch, and no worked example written in tag or brace syntax anywhere in the
product. What a person authors is blocks; what they read back is each block's own text. A body the parser cannot
read is therefore not repairable by hand — the screen says so and offers to start the composition over — which is
the one behavior this document's audience has to know about, because it is the only case where a body is discarded
rather than round-tripped.

## 1. Why tags

The grammar is XML-tag delimited rather than brace/bracket/comment delimited, for four reasons that are all about the
consumers rather than about taste:

- An LLM reads tag-delimited structure most reliably, and the write prompt already asks the same model for
  protojson — a second brace language in the same prompt is the one thing to avoid.
- Closing tags make nesting unambiguous to a parser. `{}`/`[]`/`/* */` forms are not decidable without a lookahead
  the builder would have to reimplement.
- Attributes carry the machine-readable parts (`kind`, `label`, `each`) without being confusable with prose.
- `[사진 …]` is **already taken**: it is the Naver export's photo marker in copied body text
  ([policy/export](../policy/export.md)). A bracket grammar would collide with it in the one output the product
  exists to produce.

## 2. The five constructs

| Construct | Meaning | In the prompt | In the post |
|---|---|---|---|
| bare text | literal | verbatim | verbatim |
| `<write>지시</write>` | write prose here, per the instruction | the instruction, tags intact | generated prose |
| `<slot kind="…" label="…"/>` | a reserved content position | a copy token (§5) | a resolved block or an unfilled slot |
| `<repeat each="photo">…</repeat>` | expand once per attachment | N expanded copies | N repetitions |
| `<note>지시</note>` | an instruction for the model only | verbatim, tags intact | nothing |

- `slot` is **self-closing only**. `kind` is required and is one of `photo` · `place` · `link`. `label` is optional
  free text shown to the user and carried into the block; it is never an instruction to the model.
- `repeat` requires `each`, whose only value is `photo`. Its children may be literal · `write` · `slot` · `note`.
- `write` and `note` hold text only — no nested tags.
- Everything else is a parse error (§4). There is no lenient fallback: a body that half-parses would silently drop
  the structure the user asked for.

## 3. Parsing

A body parses to an ordered node list. **Literal and text-holding nodes keep their raw source slice**, and
serialization re-emits those slices verbatim, so `parse → serialize` is byte-exact by construction — for a
hand-written body as much as for a builder-produced one. This is what backs the round-trip guarantee (change 25 AC8)
without a canonical-formatting pass that would reflow a user's own spacing.

`<` is a tag start when it is followed by an identifier (or `/` plus one) and then a delimiter — whitespace, `>`, or
`/`. A `<` followed by whitespace, a digit or punctuation is literal text, so prose like `3 < 5` and `<--` need no
escape. An identifier that is **not** one of the five is `unknown_tag` rather than literal text: a mistyped
`<repaet each="photo">` silently printing itself into every post is a worse failure than needing `&lt;b&gt;` for the
rare prose that wants a literal tag. `&lt;` always yields a literal `<`.
Entity decoding (`&lt;` `&gt;` `&amp;`) happens when a node's text is *read* — for the prompt and for the builder —
never in the stored slice, so a bare `&` in prose round-trips unchanged.

## 4. Parse errors

A body that does not parse **cannot be saved**. Every error names a 1-based line and one reason:

| Reason | Case |
|---|---|
| `unknown_tag` | `<` + an identifier that is not one of the five |
| `unclosed_tag` | `write` · `note` · `repeat` opened and never closed |
| `unexpected_close` | a closing tag with no open counterpart |
| `malformed_tag` | an unparsable attribute list, or `slot` written with a closing tag |
| `missing_attribute` | `slot` without `kind`, `repeat` without `each` |
| `unknown_slot_kind` | `kind` outside `photo` · `place` · `link` |
| `unknown_repeat_each` | `each` outside `photo` |
| `nested_repeat` | `repeat` inside `repeat` |
| `empty_write` | `<write></write>` — a write node with no instruction has nothing to ask for |

Both parsers — Go (`backend/internal/template`) and TypeScript (`frontend/src/entities/template/lib`) — are tested
against **one shared fixture file**, `backend/internal/template/testdata/grammar/cases.json`. That file is the only
mechanism keeping two implementations of one grammar honest; a new rule lands there first.

## 5. Expansion and prompt rendering

Resolution happens **once, at enqueue**, and the result is what gets frozen:

1. **Expand.** Each `<repeat each="photo">` is replaced by one copy of its children per attached photo, with the
   photo bound by filename in attachment order. **Zero photos drops the whole block, including its literals.** An
   expansion whose total node count would exceed `TEMPLATE_MAX_REPEAT_EXPANSION` is refused at start rather than
   sending an unbounded prompt.
2. **Render.** The expanded nodes become the prompt's template text: literals and `write`/`note` tags appear as
   written, and each `slot` becomes a **copy token** the model is told to reproduce verbatim:

   | Slot | Token | What the model is asked to do |
   |---|---|---|
   | `photo` | `{{photo:<filename>}}` | emit an `IMAGE` block whose `file` is exactly that filename |
   | `place` · `link` | `{{slot:<n>}}` | emit a `TEXT` block containing exactly that token and nothing else |

   A short numeric token is used deliberately: copying twelve characters exactly is something a model does reliably,
   while reproducing a label or a sentence is not.
3. **Freeze.** The rendered text and the ordered slot specs go into the job payload / experiment snapshot beside
   `target_length`. Handlers read only the frozen copy, so editing or deleting the template after a start — across a
   restart-resume or an explicit retry — cannot change the prompt, and photos attached after the start cannot change
   the expansion.

## 6. The prompt section

One `[글 템플릿]` section sits at the position the retired `[글의 용도]` section held: after the **complete** voice
profile (styleguide → active rules → excerpts → user rules → ending constraint) and before the per-post material,
with `[작문 지침]` after it. It carries a fixed legend, the rendered body, then a fixed precedence sentence.

That position is load-bearing twice over, exactly as plan 11's was: the voice-profile prefix stays byte-identical
across posts of different templates (PRD §5's caching note), and the template stays in the stable half, so every
revision of one post re-injects the identical block. A post with no template adds **no bytes at all**.

The heading, the legend and the precedence sentence **must not contain the word 지침**. The retired purpose section
rendered its own field as `작성 지침:` and then said *"지침이 문체와 충돌하면…"*, one section above
`[작문 지침]`'s *"지침이 용도의 요구와 충돌하면 지침을 우선하고…"* — the word named two different things in one
prompt, and the guideline-beats-brief rule could be read as pointing at the brief itself. After this grammar, 지침
names exactly one thing in the prompt.

The framing stays Korean for every target language, as the purpose section's did: it frames user-authored text and is
not part of the output-language contract. Plan 13's rule stands — the target language outranks a conflicting language
instruction inside a template.

## 7. Slots in the canonical post

A slot is content the app cannot invent, so it stays **honest rather than filled**. [I2] is unchanged: the canonical
post is still a block array, and an unfilled slot is a `TEXT` block carrying an optional typed
`slot {kind, label}` — **not** a sixth `BlockType`. A new enum member would force every existing switch (reading
view, block editor, export mappings, revision, validator) to change merely to stay correct; an optional field on a
type they already handle means only the surfaces that render slots specially need to know they exist.

After the existing block validation and attachment filtering, one template pass runs:

- a `TEXT` block whose content is exactly `{{slot:n}}` becomes slot *n*'s block;
- a token embedded inside a longer paragraph is stripped from the prose and the slot block is inserted before it;
- a slot with no token anywhere in the output is **inserted** — immediately after the block that resolved the
  nearest preceding slot, or at the end when there is none.

Literal fidelity and section ordering are **not** verified, and no template deviation fails a run or triggers a
retry: a generation that came back usable is never thrown away over a punctuation drift. A slot then survives
generation, an AI revision that does not mention it, finalization and export, holding its position throughout.

## 8. Bounds

| Value | Owner | Default |
|---|---|---|
| `TEMPLATE_NAME_MAX_CHARS` | BE `platform/config` · FE `shared/config` (`VITE_TEMPLATE_NAME_MAX_CHARS`) | `40` |
| `TEMPLATE_DESCRIPTION_MAX_CHARS` | same pair | `200` |
| `TEMPLATE_BODY_MAX_CHARS` | same pair | `4000` |
| `TEMPLATE_MAX_PER_ACCOUNT` | BE only | `50` |
| `TEMPLATE_MAX_REPEAT_EXPANSION` | BE only | `40` |

Counts are Unicode scalar values on both sides. A non-positive or malformed backend value is boot-fatal; a malformed
frontend override falls back to the default rather than losing its counter. The tag names, the token forms, the slot
kinds, the section heading, the legend, the precedence sentence and the proto/SQL schema are **code, not
configuration** (ARCHITECTURE §4).
