# Policy — Post generation

Canonical rules that are **currently true** in the code. Source: [plan/06](../plan/06.two-stage-generation-and-contact-sheet.md),
built by jobs 10, 11, and 23; voice scoping from
[plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), job 18.

## Canonical content

- A generated post is a `PostContent` block array, not HTML. Its flat `Block` types are `TEXT`, `HEADING`, `IMAGE`,
  `QUOTE`, and `LIST`; those exact enum names are the model-facing protojson contract.
- Every model-produced block passes one validation function immediately after parsing. Missing required fields,
  fields forbidden for that block type, empty list items, and unknown types drop that block and log its type and
  offending field. An invalid heading level is clamped to `2`.
- After validation, every `IMAGE.file` is checked against the post's attached filenames with exact,
  case-sensitive matching. Invented or differently-cased filenames are dropped and logged.
- Ordinary generation and a chosen write-experiment result replace `posts.content` wholesale, establish a machine
  baseline, and set `status = review`. A/B candidate completion alone never mutates canonical content. There is no
  generation version history or partial content update.

## Pipeline

- Ordinary generation is a durable `generate` job. `StartGeneration` validates ownership and one explicit active
  write model (plus an explicit vision observe model when photos exist), freezes the optional target length in its
  payload, and returns only `job_id` without making a provider call or creating a model experiment.
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
- The writing prompt's stable prefix is always styleguide → recent excerpts → user rules. Per-post title hint, memo,
  observations, and exact filenames follow it. The prompt requires Korean output, one paragraph per `TEXT` block,
  only attached filenames, context-appropriate image placement, a one-line summary, and 3–6 tags.
- Observation runs at most once in either mode. Ordinary generation calls one writer and directly persists validated
  content. A/B generation gives both writers byte-identical prepared snapshots/schema/options except for model ref,
  runs them concurrently, and stores validated candidate output under the experiment until an explicit verdict.
- A model declaring structured-output support receives the relevant JSON schema. Other models use the same parser,
  which accepts direct JSON, fenced JSON, or the first complete JSON object. Unparseable output fails with
  `모델이 JSON 대신 다른 답을 돌려줬어요: ` plus at most 200 characters of the raw response.
- Every provider call is bounded by the registry's five-minute stage timeout. No database transaction spans a
  provider call; observations, progress, and final content are separate short writes.
- Observation and writing/revision request `low` reasoning effort. A strict model-level override may replace that
  value or deliberately omit the wire key; voice analysis continues to send no reasoning preference. The shared
  completion cap is 8,192 tokens for reasoning plus visible output.
- A write or observe candidate returns provider-reported usage even when its call fails. Output with terminal reason
  `length` and no usable content—including non-empty partial JSON—is reported as budget exhaustion with a
  shorter-target/different-model remedy. Revision uses the same classification.

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
- Contact-sheet thumbnails use only the presigned GET `view_url` returned by `GetPost`. A temporary browser `blob:`
  upload preview is never treated as a server-read capability.
- The generated reading view renders the canonical `PostContent` block array directly. It shows title, summary,
  tags, every canonical block type, and resolves IMAGE blocks against attached filenames; it does not store or
  render canonical HTML.
- The editor exposes separate `생성` and `A/B 비교 생성` actions with independent model blockers and pending states.
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
| stage reasoning policy | BE `internal/platform/config` | observe `low` · write/revise `low` · analyze omitted |
| `TagsMin` / `TagsMax` | BE `internal/generation` | `3` / `6` |
| `BadOutputErrorHeadChars` | BE `internal/generation` | `200` runes |

## Progressive voice and optional target length

- A collapsed options popover saves or clears an optional target length separately from content. Absence is carried
  as absence through ordinary-job payloads, A/B snapshots, revision, and prompts; there is no hidden 1,200-character
  fallback. A configured positive value is frozen exactly, but machine output never rewrites the saved option.
- Start freezes the owned post's exact active `voice_id` with the input; the handler resolves that voice's profile
  version and projection and nothing else. The projection contains that voice's typed descriptors, legacy manual
  guidance, bans, evidence-ranked active rules, and 0–3 unique excerpts. Retrieval text (title/memo) and tag matches
  lead; stable recent fallback keeps a single unrelated finalized post useful. Candidate/retired/rejected rules and
  any other voice's data never enter the prompt.
- A machine result establishes a baseline carrying the frozen voice id; a result whose frozen voice no longer matches
  the post (reassigned mid-flight) or is deleted is refused rather than written. Applying an A/B winner rechecks the
  same rule.
- Voice input precedes per-post material and forbids copying example facts/phrases. Measured ending distribution is a
  first-class constraint, and the prompt forbids a third consecutive identical ending.
- Zero samples/sources is valid. Generation never requires history or starts learning, rule comparison, validation,
  embedding, or judge work.
