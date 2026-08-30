# Policy — AI revision

Canonical rules that are **currently true** in the code. Source: [plan/07](../plan/07.ai-revision.md), built by job
12; voice scoping from [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), job 18. The shared content and queue rules remain canonical in [generation](generation.md); voice profile ownership and
rule behavior remain canonical in [voice](voice.md).

## Request and queue contract

- Revision is a durable `revise` job. `StartRevision` validates the authenticated user's post, existing content, a
  trimmed non-empty instruction of at most `500` Unicode characters, and an enabled explicit write model before it
  enqueues and returns the job id.
- The one-active-job-per-post queue constraint is authoritative. A collision is `FailedPrecondition` and carries the
  active job id; revision uses the same polling, progress, restart, and boot recovery mechanics as generation.
- The post's voice must be active; a deleted voice is `FailedPrecondition` before enqueue, rule append, or any
  provider call. Start freezes the post's exact `voice_id` on the job.
- When “save as rule” is checked, the trimmed instruction is appended to the **post's current voice** before
  enqueue — never to the default or another voice. That preference remains even if enqueue or the provider call later
  fails. Exact trimmed duplicate lines are not appended twice.

## Prompt and output

- Every revision reloads and injects the complete current profile of the frozen voice in canonical order:
  styleguide, recent excerpts, then rules. No profile or prompt state from an earlier revision is reused, and no other
  voice's data is consulted.
- The user prompt contains the current full `PostContent`, the attached filenames, and the instruction. It requires
  the smallest requested change, verbatim preservation of unrelated sentences, unchanged title/summary/tags unless
  requested, immutable attached filenames, and a complete replacement `PostContent` rather than a diff.
- Provider output uses the generation parser and the same block validator. IMAGE blocks are then filtered with exact,
  case-sensitive filenames from a fresh attachment snapshot taken after the provider call, so deletion during an
  in-flight revision cannot leave a dangling image reference. Reordering attached IMAGE blocks is preserved.
- Successful revision replaces the whole canonical content through `post.SetGeneratedContent`; status remains
  `review`. There is no revision history or partial-range update.

## Frontend behavior

- The revision form is shown only when canonical content exists. It uses the account's explicit write-stage selection
  and is disabled for an empty instruction, missing or pending selection, an unresolved start, another active job, or
  a deleted voice — the last with the shared deleted-voice message, before every other reason; the sentence-feedback
  control is not offered on such a post at all.
- The form can optionally save the request as a rule and refreshes the voice-profile cache after acceptance. Progress
  and completion use the shared generation-job polling path, including resume after navigation or reload.
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
