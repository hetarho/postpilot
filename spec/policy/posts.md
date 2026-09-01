# Policy — Posts and drafts

Canonical rules that are **currently true** in the code. Source: [plan/02](../plan/02.post-drafting-and-list.md),
backend built by job 03; voice assignment from [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md),
built by job 18; language state from [plan/13](../plan/13.multilingual-interface-and-target-langua.md), built by job
32. A change to any rule here is a change to shipped behavior — go through `/create-change`.

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
  `memo` and `updated_at`. A create also requires a concrete supported `target_language`; on update that field is
  presence-aware, so absence preserves the target and presence replaces only the target alongside the current full
  title/memo snapshot. The server does not infer a language. It is the autosave endpoint, called about once a second
  while someone types, so repeated saves are plain idempotent updates.
- **There is no save button.** The editor saves 1 s after the last keystroke, and again on every way out of it
  (the tab being hidden or unloaded, and leaving the editor). A save that fails retries with a capped backoff.
- A failed save keeps retrying after the user leaves the editor, but **never after the session ends** — a retry
  landing under the next account's cookie would file one person's draft in someone else's account. The mechanism
  and its full rule list are in [spec/tech/draft-autosave.md](../tech/draft-autosave.md).
- A response that carries no post is not a confirmation, and the client treats it as a failed save. This matters
  most for the create: trusting it would leave the next edit minting a second post for the same draft.
- A new post starts as `draft`. A successful ordinary generation, AI revision, or applied write-experiment result
  replaces its canonical content and moves it to `review`; see [generation.md](generation.md).
- `target_language` is the language of the next ordinary generation or write A/B run. `content_language` is nullable
  provenance for the current machine-established canonical content. Changing target preserves content, observations,
  revisions, machine baseline, status, finalization, assignments, and content language; it never translates or
  relabels existing prose. A frozen older run may therefore land with target and content language intentionally
  different.
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
  the post's voice at that moment. The same atomic post-owned write stores or preserves `content_language` as required
  by the operation. Finalization learning requires the baseline voice to equal the current voice and content language
  to equal that voice's immutable source language.
- `Post.voice` and `PostSummary.voice` are a transport-only `VoiceRef {id, name, deleted, source_language}` resolved through the
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

## Purpose assignment

- A post has **zero or one** purpose of the same account (`posts.purpose_id`, nullable, composite FK to
  `purposes(id, user_id)`). 없음 is the default and a real answer: the server never picks one. The full rules live in
  [purposes](purposes.md); what the post context owns is below.
- `purpose_id` on a draft save is **presence-aware with three meanings**: absent preserves (ordinary autosaves omit
  it), a present empty string clears it, and a present non-empty value assigns it. An unknown or foreign id is
  `NotFound` — and it is validated before anything else in the request is applied, so such a save changes no title, no
  memo, and mints no post.
- Assignment is allowed in **every** status, including `finalized`, and while a job or an undecided experiment is
  outstanding. It writes `purpose_id` and `updated_at` and nothing else: no content, revision, machine baseline,
  finalization field or learn eligibility moves. This is the deliberate contrast with a voice reassignment — nothing is
  ever learned from a purpose, so assigning one costs the post nothing and needs no confirmation.
- Deleting a purpose detaches it from every post that named it, in that delete's own transaction, and returns the
  count. No post or content is removed.
- `Post.purpose` and `PostSummary.purpose` are a transport-only `PurposeRef {id, name}`, unset when there is none,
  resolved through the purpose context's published directory — the post context stores only the id and never joins the
  `purposes` table.
- Frontend: the editor and `/posts/new` render a `용도` select beside the `말투` picker, defaulting to 없음, with the
  chosen purpose's description under it and a link to `/purposes`. The assignment travels through the draft queue
  ([draft-autosave](../tech/draft-autosave.md)), so a title save in flight cannot revert it; on a create 없음 sends no
  field at all. The select stays enabled while a job runs and says the running job keeps its original purpose. The list
  row names an assigned post's purpose beside its voice and shows nothing for an unassigned one.

## Editor presentation

- The editor presents the post's lifecycle as three steps — 글 생성 · 글 다듬기 · 글 완성 — and the current step is
  **derived from `post.status`** (`draft` → ①, `review` → ②, `finalized` → ③). Nothing new is persisted, so a reload
  and the list badge cannot disagree with the screen. A status transition moves the step; a deliberate selection
  holds until the next transition.
- The step bar is the first thing on the screen, above the post's 가제: the lifecycle is what you navigate before
  you read anything else.
- **The 가제 belongs to ① alone.** It is rendered only there; from ② on the one title on screen is
  `content.title`, edited through the block editor's header. Its value and its autosave still live above the
  panels, so unmounting the field on another step cannot strand a queued save.
- **One options surface holds the writing brief, and 말투 stays outside it.** 관찰 모델 · 작성 모델 · 용도 · 목표 언어 ·
  목표 분량 and the A/B 후보 설정 link live behind a single trigger in ①'s dock, and nowhere else in the editor. The
  trigger is a **settings glyph at the right end of the dock's first row**. **말투 is the exception**: it sits on that
  same row as its own dropdown, on the dock's own surface, because a wrong voice silently ruins a draft and is the one
  thing that must be readable — and changeable — without opening anything. Every control keeps the semantics it had: a voice reassignment still confirms and
  is still blocked while a job runs, the 용도 select stays usable during a job and says the running one keeps its
  enqueued brief, the language select still shows its frozen note, and every assignment still rides the draft
  autosave queue. On a phone the surface is a bottom sheet; from `sm:` up it is a right-aligned popover.
- **A step that cannot start anything offers the way to set it up, not a dead button.** When the only thing standing
  between ① and a run is that the models have never been chosen, the bar drops 생성 and A/B 비교 entirely and renders
  the per-action reason plus the route to the surface each missing piece is chosen on — the writing brief for the
  active 관찰/작성 모델, the AI 모델 page for the A/B pair. Any other blocker (a job already running, a deleted voice, a
  selection still loading) keeps the ordinary disabled buttons, because waiting is the answer to it.
- Each step renders only its own panel: ① the 가제, the memo, photos, the empty-profile warning and the contact
  sheet; ② the draft as prose; ③ `말투 학습`, export and publishing. The memo is the post's own words and the input
  글 생성 works from, so it belongs to that step. Any step is selectable at any time — a step with no work yet
  says what it is waiting for and offers the way to the step that produces it, and is never disabled. Selecting
  a step changes no status, starts no job, and makes no provider call.
- `/posts/new` has no lifecycle and therefore no step bar; it renders step ①'s content alone.
- The steps are **panels of one mounted editor**, not routes: title, memo, the autosave queue, the slug mint and the
  caret handoff live outside them, so a step change can never remount the editor or strand a queued save.
- Both confirming actions carry the user to ③ once the revision is recorded — finishing a step is one gesture,
  not a click plus a tab change — and they are offered while the post is not `finalized`; a content save after
  a finalize returns it to `review`, so they come back on their own. ③'s own action is `말투 학습`, which stays on
  screen once the revision has been learned from and is **disabled with the outcome shown** rather than
  removed. It is likewise disabled, with the reason said in place, while the exact revision on screen is not
  the finalized one, or while a voice gate refuses learning; the voice gate is named ahead of the finalize
  gate, since confirming would not unblock it.
- **① and ② always dock; ③ docks only when it has something to say.** ①'s dock carries the brief trigger,
  A/B 비교 생성 and 생성. ②'s dock carries the step in two rows: **row 1** the AI revision instruction with an
  icon-only send button, **row 2** `확정` beside `확정하고 말투 학습`. Neither the revision nor the finalize section
  is rendered in ②'s panel any more — a draft there is routinely thousands of pixels tall, so an action at the
  end of the flow is an action off the screen. ③ keeps its in-flow `말투 학습`, and its dock exists only for a
  running or failed job or a save in flight, never as an empty card. There is exactly **one** docked bar in the
  page's scroller on every step. Every blocker, validation message and failure renders **above** the control it
  explains, because the software keyboard may hide a control but never the reason it is disabled. A job is
  reported on every step, because a failure the user cannot see is the bug the dock exists to prevent; its
  retry is offered only on the step that owns the job.
- ②'s revision row collapses its secondary controls — the character counter, `규칙으로 저장` and the post-revision
  `지침으로 저장` — while the instruction field is empty and unfocused, and shows them while it is focused, holds
  text, or a revision is running or failed. `규칙으로 저장` itself is unchanged in every other respect.
- Step ② opens as **prose**: `entities/post`'s `BlockList` renders title, summary, tags and every block
  read-only, and each block plus the header carries one edit control built on the shared `Editable` primitive.
  Opening one block does not close another. Edits write through to the content, so autosave keeps running on
  every keystroke; 취소 restores the value the block held when its editor opened, and moving or deleting a block
  closes it. 확정 is docked over that editor on the same step, so it flushes it — and the slug's content queue
  behind it, which outlives the unmounted editor — before naming a revision, and can never finalize one that
  omits a pending edit. The learning run a `확정하고 말투 학습` starts is owned above both panels, so the step change
  the finalize causes cannot strand it: the job, its handoff and its failure are all reported on ③.

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
- **`FinalizePost` also copies the confirmed content's title into `posts.title`**, in the same guarded statement as
  the finalization, so a concurrent content save simply matches zero rows rather than splitting the two. A
  `content.title` that is empty after trimming leaves the 가제 in place. The copy advances no `content_revision`,
  changes no `machine_baseline_*`, starts no job and calls no provider — and **the slug never changes**: a post
  keeps the URL it was minted with. The already-finalized-at-this-revision early return happens before the store
  call, so confirming the same revision twice cannot overwrite a title edited in between. Consequently the list
  shows the 가제 while a post is `draft` or `review` and the confirmed title once it is `finalized`; the existing
  fallback to `content.title` for an empty 가제 is unchanged.
- Only the post context reads these columns. It publishes an ownership-checked baseline/final snapshot to voice only
  while the current revision is exactly finalized; voice never reads or writes post tables directly.
- It also publishes an ownership-checked, deeply detached finalized snapshot to the publishing context. That read
  exposes stable JPEG object identities for server-side copies but mints no view URL, advances no revision, changes no
  finalization/learning state, and calls no provider. See [publishing.md](publishing.md).
- `target_length` is an optional per-post generation setting saved/cleared independently of canonical content. NULL
  means natural length and remains absent in prompts and snapshots; a positive value is frozen by generation,
  revision, and write comparisons. Option changes do not advance `content_revision`, demote finalization, or start
  provider work.
