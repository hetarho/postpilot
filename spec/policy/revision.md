# Policy — AI revision

Canonical rules that are **currently true** in the code. Source: [plan/07](../plan/07.ai-revision.md), built by job
12; voice scoping from [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), job 18; and the Korean
naturalness floor from [plan/06](../plan/06.two-stage-generation-and-contact-sheet.md), job 36. The shared content and queue rules remain canonical in [generation](generation.md); voice profile ownership and
rule behavior remain canonical in [voice](voice.md). Content-language preservation is from
[plan/13](../plan/13.multilingual-interface-and-target-langua.md), job 32.

## Request and queue contract

- Revision is a durable `revise` job. `StartRevision` validates the authenticated user's post, existing content, a
  concrete `content_language`, a trimmed non-empty instruction of at most `500` Unicode characters, and an enabled
  explicit write model before it enqueues and returns the job id. It freezes content language and never substitutes
  the post's newer target language.
- The one-active-job-per-post queue constraint is authoritative. A collision is `FailedPrecondition` and carries the
  active job id; revision uses the same polling, progress, restart, and boot recovery mechanics as generation.
- The post's voice must be active; a deleted voice is `FailedPrecondition` before enqueue, rule append, or any
  provider call. Start freezes the post's exact `voice_id` on the job.
- When “save as rule” is checked, content language must equal the current voice's immutable source language. A
  mismatch is refused before any rule or job is created. When eligible, the trimmed instruction is appended to the
  **post's current voice** before enqueue — never to the default or another voice. That preference remains even if
  enqueue or the provider call later fails. Exact trimmed duplicate lines are not appended twice.

## Prompt and output

- Every revision reloads the current profile of the frozen voice and projects it relative to frozen content language.
  Equal languages receive the complete profile in canonical order; cross-language revision receives only the portable
  allowlist. No profile or prompt state from an earlier revision is reused, and no other voice's data is consulted.
- Before that profile, every Korean revision injects the same fixed `[한국어 자연 문체 기준선]` bytes used by a fresh
  write. The section itself limits the floor to requested changes in `TEXT` prose and excludes titles, summaries,
  `HEADING`/`LIST` content, and untouched text. The profile, active contrast rules, and user rules override it on
  conflict.
- The post's purpose brief is frozen into the revision payload at `StartRevision` and injected at the same relative
  position as in the write prompt — after the whole profile, before the current content (see [purposes](purposes.md)).
  Revision of a post without a purpose adds no purpose bytes and is byte-identical to the current fixed no-purpose
  golden. Revision never learns, saves or changes any purpose state; "save as rule" continues to write a voice rule
  only.
- The account's applicable writing-guideline texts are frozen into the same payload at `StartRevision` and injected as
  one `[작문 지침]` section at the same relative position as in the write prompt — after the purpose section when the
  post has one, otherwise directly after `[종결어미 제약]`, always before the current content (see
  [guidelines](guidelines.md)). A revision of a post with no applicable guidelines adds no guideline bytes and stays
  byte-identical to the baseline. The fixed revise prompt text also carries the universal grounding constraint, scoped
  to the sentences the request writes or touches: the revise pass receives neither the memo nor the observations, so an
  unscoped "omit what you cannot confirm" would license stripping real facts out of untouched blocks.
- After a completed revision, `지침으로 저장` saves the user-edited instruction as a guideline through the standard
  guideline RPC. It is an explicit user save of user-authored text: nothing about a guideline is learned, and no model
  or job can write one. `AlreadyExists` is surfaced as already-saved information, not a failure. This is beside — not
  instead of — `규칙으로 저장`, which stays a pre-flight checkbox because the voice learns from the run itself.
- The user prompt contains the current full `PostContent`, the attached filenames, and the instruction. It requires
  the smallest requested change, verbatim preservation of unrelated sentences, unchanged title/summary/tags unless
  requested, immutable attached filenames, preservation of the frozen content language, no translation semantics,
  and a complete replacement `PostContent` rather than a diff.
- Provider output uses the generation parser and the same block validator. IMAGE blocks are then filtered with exact,
  case-sensitive filenames from a fresh attachment snapshot taken after the provider call, so deletion during an
  in-flight revision cannot leave a dangling image reference. Reordering attached IMAGE blocks is preserved.
- Successful revision replaces the whole canonical content and machine baseline atomically while preserving the
  frozen `content_language`; status remains `review`. There is no revision history or partial-range update.

## Frontend behavior

- The revision form is shown only when canonical content exists. It is the **first row of 글 다듬기's dock** — the
  instruction field beside an icon-only send button — rather than a section at the end of a draft that is routinely
  thousands of pixels tall ([posts.md](posts.md) *Editor presentation*). It uses the account's explicit write-stage
  selection and is disabled for an empty instruction, missing or pending selection, an unresolved start, another
  active job, or a deleted voice — the last with the shared deleted-voice message, before every other reason; the
  sentence-feedback control is not offered on such a post at all. Every blocker, validation message and failure
  renders above the row.
- Its secondary controls — the character counter, the `규칙으로 저장` checkbox with its language-mismatch note, and the
  post-revision `지침으로 저장` button — **collapse** while the instruction field is empty and unfocused, and are shown
  while it is focused, holds text, or a revision is running or failed. Focus moving between controls inside the form
  does not collapse it.
- The form can optionally save the request as a rule and refreshes the voice-profile cache after acceptance. That
  behavior is unchanged in every respect by the move into the dock — the pre-flight checkbox, the append at request
  time, the duplicate guard and the content/voice language gate all stand; only where the checkbox sits has moved.
  Progress and completion use the shared generation-job polling path, including resume after navigation or reload.
- A failed revision started in the current editor session can retry with its retained instruction. A revision resumed
  from an earlier session cannot reconstruct private job payload in the browser, so it shows the failure and enables
  a new instruction instead of presenting a no-op retry button.

## Configuration

| Value | Owner | Value |
|---|---|---:|
| `RevisionInstructionMaxChars` | BE `internal/generation` | `500` Unicode characters |
| `REVISION_INSTRUCTION_MAX_CHARS` | FE `shared/config` | `500` input characters |
| `LLMStageTimeout` | BE `internal/platform/config` | `5m` per provider call |

## Baseline and progressive voice

- The frontend flushes block-content autosave before `StartRevision`; a save conflict stops the action.
- Every revision reloads the current topic-aware structured voice projection and optional target length while
  preserving the minimal-change and attachment rules. When the setting is absent, the prompt contains no numeric
  target or hidden 1,200-character fallback.
- Successful output atomically establishes a new machine baseline carrying the frozen voice id and `review` state,
  clearing any prior finalized revision; a frozen voice that no longer matches the post is refused rather than
  written. Failure leaves canonical content and the prior baseline unchanged. Revision never implicitly finalizes
  or learns.
