# Policy — Post generation

Canonical rules that are **currently true** in the code. Source: [plan/06](../plan/06.two-stage-generation-and-contact-sheet.md),
built by jobs 10 and 11.

## Canonical content

- A generated post is a `PostContent` block array, not HTML. Its flat `Block` types are `TEXT`, `HEADING`, `IMAGE`,
  `QUOTE`, and `LIST`; those exact enum names are the model-facing protojson contract.
- Every model-produced block passes one validation function immediately after parsing. Missing required fields,
  fields forbidden for that block type, empty list items, and unknown types drop that block and log its type and
  offending field. An invalid heading level is clamped to `2`.
- After validation, every `IMAGE.file` is checked against the post's attached filenames with exact,
  case-sensitive matching. Invented or differently-cased filenames are dropped and logged.
- A chosen write winner replaces `posts.content` wholesale and sets `status = review`. Candidate job completion alone
  never mutates canonical content. There is no generation version history or partial content update.

## Pipeline

- Generation is a durable model-experiment job. `StartGeneration` validates ownership and model capabilities,
  freezes the source, creates two distinct write candidates, persists queued work, and returns `job_id` plus
  `experiment_id` without making a provider call.
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
- Observation runs once. Both writers receive byte-identical prepared snapshots/schema/options except for model ref,
  run concurrently, and store validated candidate output under the experiment. Applying the selected value is
  idempotent and is the only path that changes the post.
- A model declaring structured-output support receives the relevant JSON schema. Other models use the same parser,
  which accepts direct JSON, fenced JSON, or the first complete JSON object. Unparseable output fails with
  `모델이 JSON 대신 다른 답을 돌려줬어요: ` plus at most 200 characters of the raw response.
- Every provider call is bounded by the registry's five-minute stage timeout. No database transaction spans a
  provider call; observations, progress, and final content are separate short writes.

## Start preconditions and ownership

- The acting user comes only from the authenticated session. A foreign post is `PermissionDenied`; an unknown post
  is `NotFound`.
- Two distinct enabled write models are always required. With photos, an enabled vision-capable observe model is also required.
  With no photos, the observe model is ignored and stored empty on the job.
- The job queue's one-active-job-per-post constraint is authoritative under concurrency. A collision is
  `FailedPrecondition` and includes the active job id so the client can attach to it.

## Contact sheet and reading view

- The contact sheet pairs each attached image with its persisted observation by exact filename and displays
  `scene`, `mood`, `visible_text`, and `objects`. During observation, completed entries appear immediately and the
  remainder say `관찰 대기`; each non-terminal job snapshot refreshes the post read model.
- Contact-sheet thumbnails use only the presigned GET `view_url` returned by `GetPost`. A temporary browser `blob:`
  upload preview is never treated as a server-read capability.
- The generated reading view renders the canonical `PostContent` block array directly. It shows title, summary,
  tags, every canonical block type, and resolves IMAGE blocks against attached filenames; it does not store or
  render canonical HTML.
- Generation remains disabled until the required explicit active-observe/write-pair selections are durable, the
  latest title and memo have been saved, and neither an active job nor a pending write experiment exists. A 0-photo
  post does not require observe but still requires the write pair.

## Configuration

| Value | Owner | Value |
|---|---|---|
| `OBSERVE_BATCH_SIZE` | BE `internal/platform/config` | env, default `4`, positive integer |
| `LLMStageTimeout` | BE `internal/platform/config` | `5m` per provider call |
| `TagsMin` / `TagsMax` | BE `internal/generation` | `3` / `6` |
| `BadOutputErrorHeadChars` | BE `internal/generation` | `200` runes |
