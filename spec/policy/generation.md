# Policy — Post generation

Canonical rules that are **currently true** in the code. Source: [plan/06](../plan/06.two-stage-generation-and-contact-sheet.md),
implemented by jobs 10, 11, 23, 24, and 36; voice scoping from
[plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), job 18; language freezing and structured failures
from [plan/13](../plan/13.multilingual-interface-and-target-langua.md), job 32.

## Canonical content

- A generated post is a `PostContent` block array, not HTML. Its flat `Block` types are `TEXT`, `HEADING`, `IMAGE`,
  `QUOTE`, and `LIST`; those exact enum names are the model-facing protojson contract.
- Every model-produced block passes one validation function immediately after parsing. Missing required fields,
  fields forbidden for that block type, empty list items, and unknown types drop that block and log its type and
  offending field. An invalid heading level is clamped to `2`. Because the structured-output schema requires
  `level` on every block, a non-heading `level` is ignored and normalized to `0` without a warning.
- After validation, every `IMAGE.file` is checked against the post's attached filenames with exact,
  case-sensitive matching. Invented or differently-cased filenames are dropped and logged.
- Ordinary generation and a chosen write-experiment result replace `posts.content` wholesale, establish a machine
  baseline, and set `status = review`. A/B candidate completion alone never mutates canonical content. There is no
  generation version history or partial content update.

## Pipeline

- Ordinary generation is a durable `generate` job. `StartGeneration` validates ownership and one explicit active
  write model (plus an explicit vision observe model when photos exist), freezes the required post target language,
  optional target length, the post's optional purpose brief, **and the account's applicable writing-guideline texts**
  in its payload, and returns only `job_id` without making a provider call or creating a model experiment.
- `StartWriteExperiment` is the separate explicit A/B path. It validates two distinct write candidates, freezes one
  shared post/profile/option snapshot, creates the experiment and job, and returns their ids before provider work.
- When photos are attached, observation always precedes writing. Photos are ordered by `created_at, id`, read from
  private object storage as the already-normalized JPEGs, and sent in configurable batches. The server does not
  decode images and the upload path remains browser-to-storage. Reads are capped again at `MaxImageBytes`, both by
  response length and by the actual stream, because a presigned PUT may remain valid briefly after confirmation.
- Each observation batch is matched back by exact filename. Results for unknown filenames are discarded; a missing
  result becomes an empty observation for that attached filename. The merged array is persisted after every batch,
  before its durable progress update, so the count always converges to the attached photo count.
- With no photos, observation makes no provider call, reports `observe 0/0`, clears stale observations, and tells
  the writing model to use the memo without images.
- The writing prompt's stable prefix is static task/format rules → target-specific baseline when applicable → voice
  projection → optional frozen purpose brief (see [purposes](purposes.md)) → optional frozen `[작문 지침]` section
  (see [guidelines](guidelines.md)). Same-language projection contains the
  complete profile/rules/excerpts; cross-language projection contains only the portable structure and six numeric
  axes defined by [languages](languages.md). Per-post title hint, memo, observations, and exact filenames follow it.
  The prompt requires its frozen target for title, summary, tags, prose, alt text, and captions; one paragraph per
  `TEXT` block; only attached filenames; context-appropriate image placement; a one-line summary; and 3–6 tags.
- The purpose brief is resolved from the post once, at enqueue, through the purpose context's published lookup, and
  written into the payload as text. Handlers build the prompt from that payload and never re-read the row, so editing
  or deleting the purpose afterwards — including across a restart-resume or an explicit retry — cannot change work
  already queued. A purpose deleted before the start is simply absent. The observe stage never receives a brief.
- The applicable guideline texts are resolved at the same moment, from the same purpose id, through the guideline
  context's published `ForPrompt`, and frozen the same way. When non-empty they render as exactly one `[작문 지침]`
  section — after the purpose section when the post has one, otherwise directly after `[종결어미 제약]`, always before
  the per-post material — closed by a fixed precedence sentence that makes a guideline outrank a conflicting purpose
  instruction while leaving register to the voice profile. With none applicable there is no section and the prompt is
  baseline-identical. Retries and restart-resumes use the frozen texts; the observe stage never receives them.
- The static task/format rules carry a universal **grounding constraint** (code, not config, not a row): the writer
  may state no concrete fact absent from the memo and the photo observations, and may invent no interaction, facility,
  service, conversation or price. The write pass, which holds that material, is additionally told to omit or confine
  to the observed range whatever it cannot confirm; the revise pass, which holds neither, is instead scoped to the
  sentences the request touches (see [revision](revision.md)). The shared core is present in every write and revise
  prompt in both languages, is disjoint from the naturalness baseline below, and never enters the observe prompt.
- The A/B snapshot freezes the same target, voice projection, brief, and guideline texts, so both candidates get
  byte-identical inputs except for model ref. The input hash changes with target, purpose, or the applicable guideline
  set, and the experiment records both target and `purpose_name`; the guideline texts are not projected for display.
- Observation runs at most once in either mode. Ordinary generation calls one writer and directly persists validated
  content. A/B generation gives both writers byte-identical prepared snapshots/schema/options except for model ref,
  runs them concurrently, and stores validated candidate output under the experiment until an explicit verdict.
- A model declaring structured-output support receives the relevant JSON schema. Other models use the same parser,
  which accepts direct JSON, fenced JSON, or the first complete JSON object. Unparseable output becomes the stable
  durable reason `MODEL_OUTPUT_INVALID`; raw response detail is diagnostic and never primary user-facing copy.
- Every provider call is bounded by the registry's five-minute stage timeout. No database transaction spans a
  provider call; observations, progress, and final content are separate short writes.
- Observation and writing/revision request `low` reasoning effort. A strict model-level override may replace that
  value or deliberately omit the wire key; voice analysis continues to send no reasoning preference. The shared
  completion cap is 8,192 tokens for reasoning plus visible output.
- A write or observe candidate returns provider-reported usage even when its call fails. Output with terminal reason
  `length` and no usable content—including non-empty partial JSON—is reported as budget exhaustion with a
  shorter-target/different-model remedy. Revision uses the same classification.

## Korean naturalness baseline

- `NaturalnessBaseline` is one code-owned constant shared by write and revise. It appears exactly once after the
  static task/format rules for Korean output and before the voice projection; an empty voice does not remove it.
- The section is subtraction-only: it caps stock antithesis, formulaic closers, obligation-ended paragraphs,
  connective-ending commas, uniform sentence structure, generic policy verbs, piled invented metaphors,
  abstract-noun chains, hype, and unwarranted rhetoric. It applies only to newly written or explicitly revised
  `TEXT` prose; titles, summaries, `HEADING`/`LIST` content, and untouched revision text are excluded. It does not add
  a rewrite/verification pass or provider call.
- The corpus-rejected folk rules from archived change 10 are intentionally absent. A future prompt edit must not add
  bans on `~에 대해`, `~를 통해`, `~것이다`, or sentence-initial conjunctions.
- The closing precedence line makes the voice profile, active contrast rules, and user rules authoritative when they
  conflict with the baseline. The measured voice remains the source of personal register.
- Ordinary generation and both A/B candidates inherit identical baseline bytes through `BuildWritePrompt` for a
  Korean target and omit the complete section for an English target. `ObservePrompt` is target-independent and
  unchanged across otherwise-equal Korean/English jobs.

## Start preconditions and ownership

- The acting user comes only from the authenticated session. A foreign post is `PermissionDenied`; an unknown post
  is `NotFound`.
- Ordinary generation requires one enabled explicit write model and never depends on an A/B pair. A/B generation
  separately requires two distinct enabled write candidates. With photos, both require an enabled vision-capable
  observe model; with no photos, observation is omitted and its ref is stored empty.
- The job queue's one-active-job-per-post constraint is authoritative under concurrency. A collision is
  `FailedPrecondition` and includes the active job id so the client can attach to it.
- The post's voice must be active. A post whose voice is deleted is refused with `FailedPrecondition` before any
  enqueue or provider call — both for ordinary generation and for A/B generation — and the server never falls back
  to the default or another voice.

## Contact sheet and reading view

- The contact sheet pairs each attached image with its persisted observation by exact filename and displays
  `scene`, `mood`, `visible_text`, and `objects`. During observation, completed entries appear immediately and the
  remainder say `관찰 대기`; each non-terminal job snapshot refreshes the post read model.
- On a phone the contact sheet is one **horizontal snap carousel**: cards narrower than the strip so a sliver of the
  next one is visible, a `현재 / 전체` position indicator under it, and no inner vertical scroller — a verbose
  observation makes its own card taller. From `sm:` up it keeps its existing fixed-width strip.
- Contact-sheet thumbnails use only the presigned GET `view_url` returned by `GetPost`. A temporary browser `blob:`
  upload preview is never treated as a server-read capability.
- The generated reading view renders the canonical `PostContent` block array directly. It shows title, summary,
  tags, every canonical block type, and resolves IMAGE blocks against attached filenames; it does not store or
  render canonical HTML.
- Everything the next run is GIVEN is set in 글 생성's dock and rendered nowhere else in the editor
  ([posts.md](posts.md) *Editor presentation*): 관찰 모델, 작성 모델, 작성 A/B 후보, 목표 언어 and 목표 분량 behind a
  single options trigger, and 말투 and 용도 on the dock's own row beside it. The A/B pair is set THERE rather than
  linked to the AI 모델 page: it is two dropdowns, and following a link out of a docked bar mid-draft cost the user
  their place for them. Relocating a control changes the screen, never what is sent: every model choice stays
  explicit and is never auto-applied ([I3]), and a pair is saved only once both candidates name a different model,
  because the server refuses anything else. Those two fields offer **only rows this surface can carry out**: no
  blank entry, since `SaveComparisonPair` refuses an empty ref and no RPC clears a pair, and neither field lists
  the model its neighbour holds, since a pair of one model twice is the other state the server refuses. Either
  entry would have emptied or duplicated the FIELD while the saved pair went on running underneath it. Excluding
  the neighbour leaves the two unable to swap A for B, which costs nothing: the experiment shows its candidates
  blind and fixes their sides once it starts, so A/B and B/A are the same run. A field always keeps its own
  current value, so nothing renders as empty because of the exclusion. The AI 모델 page's copy of the same form
  keeps both entries — its explicit 저장 button disables and says the choice is not in effect, which a surface
  whose fields save themselves has nowhere to say.
- The editor exposes separate `생성` and `A/B 비교` actions with independent model blockers and pending states.
  Both await the latest title/memo save and refuse concurrent post work. A missing pair blocks only A/B; a missing
  active writer blocks only ordinary generation. A zero-photo post does not require observe. A deleted voice blocks
  both, before every other reason, with the shared deleted-voice message; the model lab's write comparison applies
  the same precondition.

## Configuration

| Value | Owner | Value |
|---|---|---|
| `OBSERVE_BATCH_SIZE` | BE `internal/platform/config` | env, default `4`, positive integer |
| `LLMStageTimeout` | BE `internal/platform/config` | `5m` per provider call |
| `LLMMaxTokensDefault` | BE `internal/platform/config` | `8192` shared reasoning/output tokens |
| stage reasoning policy | BE `internal/platform/config` | observe `low` · write/revise `low` · analyze has **no field**: it carries no stage value, which is how "send nothing" is expressed |
| `TagsMin` / `TagsMax` | BE `internal/generation` | `3` / `6` |
| `BadOutputErrorHeadChars` | BE `internal/generation` | `200` runes |

## Progressive voice and optional target length

- A collapsed options popover saves or clears an optional target length separately from content. Absence is carried
  as absence through ordinary-job payloads, A/B snapshots, revision, and prompts; there is no hidden 1,200-character
  fallback. A configured positive value is frozen exactly, but machine output never rewrites the saved option.
- Start freezes the owned post's exact active `voice_id` with the input; the handler resolves that voice's profile
  version and language projection and nothing else. Equal source/target languages receive the complete typed profile,
  legacy guidance, rules, and 0–3 excerpts; a mismatch receives only the portable allowlist. Retrieval text and tag
  matches lead only for the full projection. Candidate/retired/rejected rules, excluded cross-language fields, and
  any other voice's data never enter the prompt.
- A machine result establishes a baseline carrying the frozen voice id; a result whose frozen voice no longer matches
  the post (reassigned mid-flight) or is deleted is refused rather than written. Applying an A/B winner rechecks the
  same rule.
- Voice input precedes per-post material and forbids copying example facts/phrases. Measured ending distribution is a
  first-class constraint, and the prompt forbids a third consecutive identical ending.
- Zero samples/sources is valid. Generation never requires history or starts learning, rule comparison, validation,
  embedding, or judge work.
