# Policy — Posts and drafts

Canonical rules that are **currently true** in the code. Source: [plan/02](../plan/02.post-drafting-and-list.md),
backend built by job 03; voice assignment from [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md),
built by job 18. A change to any rule here is a change to shipped behavior — go through `/create-change`.

Photo upload has its own document: [uploads.md](uploads.md).

## Identity

- A post is identified by its **slug**, minted once on the first save and **never changed**. It is the primary key
  *and* part of every object key for the post's photos, so renaming it would orphan the photos.
- Shape: `YYYYMMDD-<sanitized title>`, with a serial suffix `-2`, `-3`, … when that name is taken (PRD §7).
  An empty or fully-stripped title gives `YYYYMMDD-untitled` — never a bare date, which would collide with every
  other untitled post that day.
- Sanitizing: trimmed, lowercased, runs of whitespace and separators collapsed to one `-`, path- and URL-unsafe
  characters dropped (`/ \ ? # % : * " ' < > | & + .` and control characters), truncated to 60 runes, never ending in
  a separator. **Korean and any other letter is kept** — a slug is not required to be ASCII, and stripping Hangul
  would make almost every real title "untitled".
- Slugs are globally unique, not per user: two accounts cannot hold the same slug.
- The mint is a check-then-insert, so the **insert decides**. A slug taken between the two makes the create retry
  with the next serial rather than fail.

## Ownership

- Every read and write is scoped to the acting user, taken from the session by the auth interceptor. No request
  carries a user id.
- A slug that exists but belongs to someone else is **`PermissionDenied` (403)**, not a 404 (PRD §7). At two users
  there is nothing to enumerate.
- A slug that does not exist is `NotFound` (404).
- `ListPosts` returns only the caller's posts, newest first by `updated_at`.

## Drafts

- `SavePostDraft` is create-or-update: an empty slug creates and returns the minted slug; a slug updates `title`,
  `memo` and `updated_at`. It is the autosave endpoint, called about once a second while someone types, so repeated
  saves are plain idempotent updates.
- **There is no save button.** The editor saves 1 s after the last keystroke, and again on every way out of it
  (the tab being hidden or unloaded, and leaving the editor). A save that fails retries with a capped backoff.
- A failed save keeps retrying after the user leaves the editor, but **never after the session ends** — a retry
  landing under the next account's cookie would file one person's draft in someone else's account. The mechanism
  and its full rule list are in [spec/tech/draft-autosave.md](../tech/draft-autosave.md).
- A response that carries no post is not a confirmation, and the client treats it as a failed save. This matters
  most for the create: trusting it would leave the next edit minting a second post for the same draft.
- A new post starts as `draft`. A successful ordinary generation, AI revision, or applied write-experiment result
  replaces its canonical content and moves it to `review`; see [generation.md](generation.md).
- `observations` and `content` are owned by the post aggregate but may be changed by generation only through the
  post context's ownership-checked `SetObservations` and `SetGeneratedContent` behaviors. `GetPost` returns both;
  no other context reads or writes the `posts` table directly.
- **There is no post deletion.** The PRD defines photo deletion but not post deletion; flagged as a PRD gap, not an
  oversight.

## Voice assignment

- Every post is written in exactly one **voice** of the same account (`posts.voice_id`, composite FK to
  `voices(id, user_id)`). A create must name an active owned voice: `SavePostDraft` with an empty slug and no
  `voice_id` is `InvalidArgument`, an unknown or foreign id is `NotFound`, a deleted id is `FailedPrecondition`. The
  server never substitutes the default for an invalid request.
- `voice_id` on a draft save is **presence-aware**. Absent preserves the assignment — ordinary title/memo autosaves
  omit it. A present value equal to the current voice also preserves it. A different present value is a
  **reassignment**: refused with `FailedPrecondition` while a job targets the post or an undecided write experiment
  could still apply output; otherwise, in one post-owned transaction, it changes `voice_id`, clears
  `machine_baseline` and `machine_baseline_voice_id`, and makes voice learning ineligible until a new machine result.
  Finalize-only remains available for preserved canonical content. Slug, title, memo, photos,
  canonical content, `content_revision`, status, finalized revision and exportability are untouched, and no learning
  event, source, rule or version already published under the previous voice moves.
- Every machine-written baseline (generation, revision, applied A/B winner) stores `machine_baseline_voice_id`,
  the post's voice at that moment. Finalization learning requires it to equal the post's current voice.
- `Post.voice` and `PostSummary.voice` are a transport-only `VoiceRef {id, name, deleted}` resolved through the
  voice context's published directory — the post context stores only the id and never joins voice tables. A deleted
  voice still names itself, so the post stays readable, manually editable, copyable and exportable; generation, AI
  revision, save-as-rule, sentence feedback and finalization learning are refused before any enqueue or provider
  call until the voice is restored or the post is reassigned.
- Frontend: `/posts/new` reads the directory before the editor mounts and renders a required `말투` picker initialized
  to the default voice; the first save always carries that concrete id, and choosing a voice by itself creates
  nothing. On an existing post the picker reassigns after a confirmation sheet that says what stays and that prior
  learning remains with the old voice; it is disabled with a reason while a job runs or an A/B result waits, and a
  refusal is shown under the field with the old voice kept. The assignment travels through the draft queue
  ([draft-autosave](../tech/draft-autosave.md)), so a title save in flight cannot revert it. The list row names each
  post's voice; a tombstone renders `삭제된 말투 · {name}` on the row, in the picker, and in the editor's warning,
  which offers `복원` beside the picker's reassignment. Every AI control on such a post is disabled with one fixed
  reason (`삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.`); the server enforces the rule regardless.

## Editor presentation

- The editor presents the post's lifecycle as three steps — 글 생성 · 글 다듬기 · 글 완성 — and the current step is
  **derived from `post.status`** (`draft` → ①, `review` → ②, `finalized` → ③). Nothing new is persisted, so a reload
  and the list badge cannot disagree with the screen. A status transition moves the step; a deliberate selection
  holds until the next transition.
- The step bar is the first thing on the screen, above the post's title: the lifecycle is what you navigate before
  you read anything else. The voice picker sits with the title, outside the step panels: the voice is the post's
  identity, and a reassignment must survive a step change exactly as a title edit does.
- Each step renders only its own panel: ① the memo, photos, the empty-profile warning, the stage-model selects, the
  A/B link, the contact sheet and the generation actions; ② the draft and AI revision; ③ finalize,
  finalize-and-learn and export. The memo is the post's own words and the input 글 생성 works from, so it belongs to
  that step; its value and its autosave stay above the panels, so leaving the step cannot strand a queued save. Any step is selectable at any time — a step with no work yet says what it is waiting for and offers the
  way to the step that produces it, and is never disabled. Selecting a step changes no status, starts no job, and
  makes no provider call.
- `/posts/new` has no lifecycle and therefore no step bar; it renders step ①'s content alone.
- The steps are **panels of one mounted editor**, not routes: title, memo, the autosave queue, the slug mint and the
  caret handoff live outside them, so a step change can never remount the editor or strand a queued save.
- The dock carries at most one committing action — 생성 on ①, 확정 on ③, and none on ②, which commits continuously
  through content autosave — and it exists only when it has something to say: the current step's action, a running
  or failed job, or a save that is in flight or has failed. On a quiet 글 다듬기 / 글 완성 it is absent rather than
  an empty card. A job is reported on every step, because a failure the user cannot see is the bug the dock exists
  to prevent; its retry is offered only on the step that owns the job.
- Step ② opens as **prose**: `entities/post`'s `BlockList` renders title, summary, tags and every block read-only,
  and each block plus the header carries one edit control built on the shared `Editable` primitive. Opening one
  block does not close another. Edits write through to the content, so autosave keeps running on every keystroke;
  취소 restores the value the block held when its editor opened, and moving or deleting a block closes it. 확정 on
  step ③ waits on the post's content queue — which outlives the unmounted editor — so it can never finalize a
  revision that omits a pending edit.

## Storage of time

- Timestamps are stored as **fixed-width RFC3339 in UTC**. The width matters: `ORDER BY updated_at` and
  `expires_at < ?` are plain string comparisons in SQL, and a trimmed fraction (`…08.5Z`) sorts after a longer one
  (`…08.513110616Z`).
- On the wire, timestamps are RFC3339 strings — the client renders one without needing to know a unit.

## Canonical content editing and learning baseline

- `PostContent` remains the only canonical generated value. Direct editing supports title, summary, tags, and
  TEXT/HEADING/QUOTE/LIST/IMAGE blocks; backend validation rejects invalid shapes and unattached IMAGE filenames.
- `content_revision` is optimistic concurrency state. `SavePostContent` is owner-scoped, requires the expected
  revision, increments it once for changed content, and returns `Aborted` on a stale tab. It never changes
  `machine_baseline`. An identical save is a no-op, including for a finalized revision.
- A selected generation winner or successful AI revision atomically writes canonical content plus an immutable
  machine baseline. `machine_baseline_revision` is set to that new content revision and `machine_baseline_voice_id`
  to the post's voice. A later manual edit makes the two revisions differ until another machine result establishes a
  new baseline; a reassignment clears the baseline outright (see *Voice assignment*).
- `draft`, `review`, and `finalized` are durable states. `FinalizePost` requires valid canonical content and the exact
  expected content revision; it deliberately does not require a learning baseline, so reassignment and a deleted
  voice cannot prevent a content-only finalization. It records `finalized_revision`/`finalized_at` without creating
  a job or calling a provider. The first changed-content save or machine result clears that boundary and returns the
  post to `review`; title/memo and generation-option saves do not.
- Only the post context reads these columns. It publishes an ownership-checked baseline/final snapshot to voice only
  while the current revision is exactly finalized; voice never reads or writes post tables directly.
- `target_length` is an optional per-post generation setting saved/cleared independently of canonical content. NULL
  means natural length and remains absent in prompts and snapshots; a positive value is frozen by generation,
  revision, and write comparisons. Option changes do not advance `content_revision`, demote finalization, or start
  provider work.
