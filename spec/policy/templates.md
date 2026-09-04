# Policy — Post templates (템플릿)

Canonical backend and frontend rules. Source: [plan/11](../plan/11.post-templates.md), rebuilt from
[change 25](../changes/archive/25.replace-purposes-with-drag-and-drop-post.md) by job 53. The grammar itself is
[tech/post-template-grammar](../tech/post-template-grammar.md).

A **template** is a reusable, account-owned document that decides the **shape** of a post: its literal text, the
positions it reserves for content the app cannot invent, what repeats per photo, and where prose gets written. Three
authored axes now stand beside each other and each owns one question — the voice decides how sentences sound
([voice](voice.md)), the template decides what shape the post has, and a guideline decides what the post must avoid
([guidelines](guidelines.md)). A post combines exactly one voice and at most one template.

It replaces the retired **purpose** (용도), which was prose *about* shape rather than shape itself: the author had to
describe a frame in Korean and hope, the model re-derived it on every run, a literal line could not be required, and
"one paragraph per photo" was something the model counted for itself.

## Ownership and persistence

- An account owns zero or more templates (`templates`), the user-facing noun 템플릿. A template has exactly three
  authored fields — `name`, `description`, `body` — plus its timestamps. Nothing else belongs to it.
- `body` is the **single source of truth** for the shape. It is what the database stores, what the prompt receives
  (after expansion), and what the builder round-trips; the builder is a parser plus a renderer over that string and
  never a second representation of it.
- Every store query and RPC is scoped by the authenticated user. No request carries a user id. A foreign template id
  is indistinguishable from an unknown one (`NotFound`).
- `templates_id_user` is the composite target `posts.template_id` points at, so a post can never name another
  account's template even if a service check is bypassed. `templates_user_name` keeps display names unique within an
  account.
- There is no tombstone: a deleted template is gone and its name is free again immediately. This is the deliberate
  difference from a voice, which keeps its name so the posts written in it stay readable — a template is not
  something a post was written *in*, only something the next machine run is told.
- Migration `0022_post_templates.sql` creates `templates` and `guideline_templates`, replaces `posts.purpose_id` with
  `posts.template_id` under the same composite account foreign key and detach trigger, renames
  `model_experiments.purpose_name` to `template_name`, and **drops `purposes` and `guideline_purposes`**. It is
  destructive by decision: the deployed database was inspected first and the owner chose to drop the two purposes it
  held rather than convert them. `template_name` is the one exception — renamed, not cleared, because a frozen
  comparison record must keep saying what both candidates were given. Timestamps follow the fixed-width UTC RFC3339
  rule.

## Field rules

| Field | Rule |
|---|---|
| `name` | trimmed, non-empty, ≤ `TEMPLATE_NAME_MAX_CHARS`, unique among the account's templates after trim |
| `description` | trimmed, **may be empty**, ≤ `TEMPLATE_DESCRIPTION_MAX_CHARS`. Management copy only — it never reaches a prompt |
| `body` | trimmed at the edges only, non-empty, ≤ `TEMPLATE_BODY_MAX_CHARS`, **and it must parse** |
| account cap | creating beyond `TEMPLATE_MAX_PER_ACCOUNT` is refused with `FailedPrecondition`. The cap is checked inside the insert transaction, so two concurrent creates cannot both pass it |

Limits are Unicode scalar-value counts on both sides, so a Hangul syllable is one character to the server and to the
counter. The backend is authoritative; the frontend mirrors the three field ceilings only for live counters and never
decides a save.

**Parsing is part of validation.** A body that does not parse has no meaning — it would reach a prompt as prose the
model prints back, and the builder could not open it either — so it cannot be saved at all. The refusal is
`InvalidArgument` with reason `TEMPLATE_PARSE_FAILED` and allowlisted `line` and `reason` params, because the editor
has to point at the offending line without parsing wire prose to find out which one. Only the body's outer whitespace
is trimmed; everything inside is the author's, down to the blank lines.

## Editing

- `UpdateTemplate` is presence-based, like `UpdateVoiceProfile`: only the fields the request carries change. The
  store runs **one statement per present field** inside one transaction, so a field the request omitted is never
  named by any statement — two fields edited from two tabs cannot overwrite each other, and no read-modify-write can
  restore a stale value. A present empty `description` clears it; a present empty `name` or `body` is refused.
- A duplicate name is `AlreadyExists` with reason `TEMPLATE_NAME_TAKEN`; a bound violation is `InvalidArgument` with
  `TEMPLATE_FIELD_TOO_LONG` and allowlisted `field`, `max`, and `actual` params. The frontend renders these stable
  details in the active locale and never treats raw wire prose as translation input.

## Deletion detaches, never cascades

- `DeleteTemplate` removes the template and returns `detached_posts`: how many posts lost their assignment. The
  detach is the schema's `templates_detach_posts_on_delete` trigger, which runs inside the delete's own transaction,
  so the count and the clearing see one snapshot of `posts`.
- It is a trigger rather than `ON DELETE SET NULL` because SQLite sets **every** column of a composite child key to
  NULL, which would try to null `posts.user_id` and fail its NOT NULL constraint. The composite foreign key stays for
  the account guarantee it provides.
- The same delete also cascades the template's writing-guideline scope links (`guideline_templates`, whole rows on a
  composite key — the `SET NULL` trap above does not apply when the child row itself goes away). Every guideline row
  survives; one left with no links applies nowhere and lists on `/guidelines` as 적용 대상 없음 until it is rescoped
  (see [guidelines](guidelines.md)). The confirmation keeps naming detached posts only — it does not count affected
  guidelines.
- No post, no content, no photo, no guideline and no history is deleted. Frozen job payloads and experiment
  snapshots keep the rendered body and the guideline texts, so work already done stays explainable.

## Assignment to a post

- `SavePostDraftRequest.template_id` is presence-aware with three meanings: **absent** preserves the current
  assignment (what ordinary autosave sends), **present and empty** clears it (없음), **present and non-empty**
  assigns it.
- The template is validated **before anything else in the request is applied**, so a request naming an unknown or
  foreign template leaves the post exactly as it was — title and memo included — and mints no post on a create.
- Assignment is allowed in **every** status, including `finalized`, and while a job is running. It touches no
  content, revision, machine baseline, finalization field or learn eligibility. This is the deliberate difference
  from a voice reassignment, which withdraws the machine baseline: nothing is ever learned from a template, so
  assigning one costs the post nothing.
- `Post` and `PostSummary` carry a transport-only `TemplateRef {id, name}`, unset when the post has none. The post
  context never reads the `templates` table; the name comes from the template context's published directory through
  a consumer-owned `TemplateDirectory` port.

## Rendering, freezing, and what is never learned

- The template is resolved **once, at enqueue**, and what gets frozen is the **rendered** body: expanded for the
  post's attached photos and turned into prompt text with its copy tokens
  ([tech/post-template-grammar §5](../tech/post-template-grammar.md)). Expansion inside the freeze is what makes
  "attaching a photo after the start cannot change the run" true, and it is the only place the expansion bound can
  refuse before a provider is called.
- One `[글 템플릿: {name}]` section enters the write and revise prompts at the position the retired `[글의 용도]`
  section held: after the **complete** voice profile (styleguide → active rules → excerpts → user rules → ending
  constraint) and before the per-post material, with `[작문 지침]` after it. It carries a fixed grammar legend, the
  rendered body between `---` fences, then a fixed precedence sentence. Absent a template the builder adds no
  template bytes at all.
- The position is load-bearing twice: the voice-profile prefix stays byte-stable across posts of different templates
  (PRD §5's caching note), and the section stays in the stable half, so every revision of one post re-injects the
  identical block.
- **The section never uses the word 지침.** The retired purpose section rendered its own field as `작성 지침:` and
  then said *"지침이 문체와 충돌하면…"*, one section above `[작문 지침]`'s *"지침이 용도의 요구와 충돌하면 지침을
  우선하고…"* — the word named two different things in one prompt, and the guideline-beats-brief rule could be read
  as pointing at the brief itself. After this change 지침 names exactly one thing in the prompt, and that is the
  property to preserve when editing the text.
- Handlers read the frozen copy from the payload and never the live row. Editing or deleting the template after
  `StartGeneration` / `StartRevision` returns — including across a restart-resume or an explicit retry — cannot
  change the prompt. A template deleted between the save and the start is simply absent: a post with no template,
  not a failed start. A payload written before templates existed decodes as "no template" rather than failing.
- A write A/B experiment freezes the same rendered body into the shared prepared snapshot, so both candidates
  receive byte-identical system prompts and a different template is a different input hash.
  `ModelExperiment.template_name` records it by name, stored rather than derived, so the comparison detail keeps
  reading correctly after the template is renamed or deleted and after the retention sweep clears the snapshot.
- **Nothing about a template is learned, inferred, suggested or written by a model** ([I4] stays entirely with
  voice). No template is selected from a title, memo, tags, photos or content; there is no default and no seeded
  library. The observe stage never sees one — observation stays facts-only about photos ([I3]).
- No template surface makes a provider call or enqueues a job ([I5]): listing, selecting, creating, editing,
  deleting, parsing, the builder and its source view are plain CRUD round trips and pure client work.

## Slots in the canonical post

A slot is content the app cannot invent, so it stays **honest rather than filled**. [I2] is unchanged: the canonical
post is still a block array, and an unfilled slot is a `TEXT` block carrying an optional typed `slot {kind, label}`
— **not** a sixth `BlockType`. A new enum member would force every existing switch (reading view, block editor, the
four export mappings, revision, the validator) to change merely to stay correct.

- After block validation and attachment filtering, one template pass resolves the copy tokens into slot blocks and
  **inserts any slot the model omitted** — after the block that resolved the nearest preceding slot, or at the end
  when none resolved. A token buried inside a longer paragraph is stripped from the prose and the slot inserted
  before it; a repeated token resolves once.
- The slot block's **content is the token**, not the label. A revision receives the current content and hands
  untouched blocks back byte for byte, but the structured-output schema cannot carry the `slot` marker (its block
  object is closed) — a token in the content survives that round trip, which is what lets the pass find the slot
  again at its own position instead of re-appending it at the end on every revision. The label is what slot-aware
  surfaces display.
- The pass is idempotent, and it runs again after every revision.
- Literal fidelity and section ordering are **not** verified, and no template deviation fails a run or triggers a
  retry: a generation that came back usable is never thrown away over a punctuation drift.
- Unfilled slots **warn, they never gate.** Finalization and export are not blocked. The editor states how many
  remain and the reading view shows where each one is; the export panel states the count and the four derived
  outputs render each slot as a bracketed placeholder in marker order, which is deliberately distinguishable from
  the Naver `[사진 …]` photo markers by their filename suffix ([export](export.md)).

## Frontend

- `/templates` is a top-level destination (템플릿, in 용도's position after 말투, lazily split like its siblings). It
  lists the account's templates with their post counts, edits each field read-first and one at a time, and deletes
  behind a confirmation that states the detach count. The empty state shows one worked example **as copy** and
  creates no row.
- The body is authored through one control in two modes over the same string: a **structure** view (a drag-and-drop
  list of blocks with a visible palette) and a **원문** view (the body text). The structure view offers both pointer
  drag and move buttons — HTML5 drag events do not fire on touch, and the base breakpoint is a 360px phone, so the
  buttons are the only way the primary device can reorder at all.
- The structure view holds its rows in local state and contributes **only complete blocks** to the body. A block
  exists as a row the moment it is added, before anything is typed into it, and an empty `<write></write>` does not
  parse — so without this the builder would produce a body its own parser refuses the instant a block is added. A
  value that is not what the editor last emitted came from outside (the source view, a refetch) and only then are
  the rows reseeded.
- A body that does not parse **forces the source view**, where its error is named on the offending line, and the
  structure tab is disabled: there is nothing for it to render, and guessing would drop the structure the author
  asked for. Such a body cannot be saved.
- Builder → body → builder is byte-identical for anything the builder produced. A hand-written body that parses
  opens in the structure view too, but editing it there normalizes its spacing to the canonical form, which is why
  the source view exists.
- The directory query is re-read on every mount (`staleTime: 0`, `refetchOnMount: 'always'`). `post_count` is a
  projection over *posts*, so assigning a template in the editor changes it without touching any template and no
  template mutation can invalidate it — and it is the number the user confirms a destructive detach against.
- A rename invalidates the post caches as well as the directory: the name is projected onto every assigned post and
  rendered on the list, so those rows change what they display without changing at all.
- The editor and `/posts/new` show a `템플릿` picker defaulting to 없음. It rides 글 생성's dock row beside the 말투
  picker and the writing brief's trigger ([posts.md](posts.md) *Editor presentation*), because a template is chosen
  per draft and silently changes what a run produces. That row is three controls wide on a 360px screen, so the
  picker carries neither a visible caption (its `sr-only` label keeps it announced as `템플릿 <값>`), nor the
  selected template's description, nor a `템플릿 관리` link. Selecting writes a presence-aware patch through the
  per-post draft queue, so a delayed title save cannot revert a newer selection. On a create, 없음 sends **no** field
  at all rather than an empty one.
- The select stays enabled while a job runs and says that the running job keeps the template it started with. A
  failed directory read is shown as a failure with a retry — never as an empty directory, which would leave clearing
  as the only available action.
- The post list shows the template name beside the voice for an assigned post and nothing for an unassigned one.
- The body editor lives in `entities/template/ui` rather than in a feature: it performs no mutation, both
  `create-template` and `edit-template` need it, and two features importing each other is what FSD forbids.

## Configuration

| Value | Owner | Default |
|---|---|---|
| `TEMPLATE_NAME_MAX_CHARS` | BE `platform/config` · FE `shared/config` (`VITE_TEMPLATE_NAME_MAX_CHARS`) | `40` |
| `TEMPLATE_DESCRIPTION_MAX_CHARS` | same pair | `200` |
| `TEMPLATE_BODY_MAX_CHARS` | same pair | `4000` |
| `TEMPLATE_MAX_PER_ACCOUNT` | BE only | `50` |
| `TEMPLATE_MAX_REPEAT_EXPANSION` | BE only | `40` |

The last two stay server-side: the first is a storage guard, and the second bounds how large one expanded template
may grow before it reaches a provider, which depends on the post's photo count rather than on the template being
edited. A non-positive or malformed backend value is boot-fatal. The frontend falls back to the default rather than
disabling its counter, so a build-time typo cannot silently remove the client-side bound. The grammar's tag names,
the token forms, the slot kinds, section headings, the legend, the precedence sentence, and the proto/SQL schema are
code, not configuration (ARCHITECTURE §4).
