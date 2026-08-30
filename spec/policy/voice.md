# Policy — Voices and voice profiles

Canonical backend and frontend rules. Source: [plan/03](../plan/03.voice-profile-learning.md), built by jobs 08, 09,
and 16, and [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), built by job 18. The cross-context
decision behind the partition is [multi-voice partitioning](../tech/multi-voice-partitioning.md).

## Voices, ownership, and persistence

- An account owns one or more **voices** (`voices`), the user-facing noun 말투. Each voice owns exactly one
  possibly-empty profile and every row that can change it: `voice_profiles`, `voice_samples`, profile versions,
  manual overrides, learning events, authored sources, contrast rules and evidence, confirmations, sentence feedback,
  rule comparisons, and validations are all keyed by `(user_id, voice_id)`. There is no shared account-level profile,
  inheritance, copying, or fallback between voices; a voice with no evidence is empty even when another voice is
  well trained.
- Every account has at least one active voice and exactly one active default (partial unique index
  `voices_one_default`). Active display names are unique within the account (`voices_active_name`); a tombstone's
  name does not reserve anything.
- Every store query and RPC is scoped by the authenticated user **and** by one voice, named explicitly (`voice_id` on
  profile, sample, version, override, confirmation-list, validation and analyze-experiment requests) or derived from
  an owned aggregate (a rule, an event, a post). No request carries a user id. A foreign voice id is indistinguishable
  from an unknown one (`NotFound`), and a same-account id from another voice is refused — it is never interchangeable
  evidence.
- Migration `0009_independent_voice_profiles.sql` creates one active default `기본 말투` per existing account, assigns
  every existing post and profile/evidence row to it, and preserves bodies, snapshots, revisions, statuses, timestamps
  and legacy styleguide/rules bytes. `adduser` establishes the same default voice through the voice context's
  idempotent bootstrap immediately after creating the account and exits nonzero if it cannot; a rerun repairs a user
  left without a voice without duplicating data. Reads never create a voice or a profile.
- A voice profile is never modified by, and never contains, purpose text: a 용도 is a separate account-owned aggregate
  and is not part of `ProfileForPrompt` (see [purposes](purposes.md)). The prompt appends it after the complete
  profile, so the profile's own bytes are identical with and without one.
- `styleguide` is machine-generated but user-editable. `rules` is user-owned; analysis never changes it.
  `UpdateVoiceProfile` uses optional field presence and atomically updates only supplied columns, so a rules save
  cannot write a stale styleguide over a completed analysis.
- A sample body is private server-side source material. RPCs return only id, label, Unicode character count, and
  creation time; they never return the body.

## Voice lifecycle

- `ListVoices` returns the account's voices including tombstones, active first, the default first among them, then
  by name. `GetVoiceProfile` returns the owning `voice` summary with the profile.
- `CreateVoice` trims the name, requires 1–`VoiceNameMaxChars` (50) Unicode scalar values (`InvalidArgument`
  otherwise), refuses an active-name collision (`AlreadyExists`), and creates an empty isolated profile. It never
  copies the default profile. There is no voice-count limit.
- `RenameVoice` changes the display name of an active or deleted owned voice with the same validation; it rewrites
  no post row and no immutable snapshot. `SetDefaultVoice` accepts only an active owned voice and atomically clears the
  previous default; it answers with the whole directory.
- `DeleteVoice` is a soft delete (`deleted_at`). It refuses the active default and the last active voice, and it
  refuses a voice that still has a queued/running voice-owned job, an undecided rule comparison or validation, or a
  publishable analyze experiment (all `FailedPrecondition`). Posts, samples, versions, rules, sources and completed
  history stay intact; the voice disappears from selectable options at once. Deleting an already deleted voice is a
  no-op.
- `RestoreVoice` clears `deleted_at` without enqueueing anything. It fails with `AlreadyExists` while an active voice
  holds the same name — the tombstone can be renamed first — and never changes the default.
- A deleted voice stays addressable for display: its profile, versions and history are readable, but every profile
  mutation (samples, updates, overrides, restore, analysis, learning, feedback, comparison, validation, experiment
  application) is refused with `FailedPrecondition` before any enqueue or provider call.
- Directory reads, lifecycle mutations, page load, polling, copy and export make no provider call and enqueue no work
  ([I5]).

## Samples and analysis

- A body is trimmed and must contain at least `SampleMinChars` (200) Unicode characters. Rejection includes the
  measured count. An empty label falls back to the first `LabelFallbackChars` (20) characters.
- Before storing a sample, the server requires that the requested model exactly match the account's current enabled
  analyze-stage selection (model selection stays account-scoped). With no usable selection, nothing is stored or
  enqueued.
- Adding a sample stores it under its voice and enqueues `analyze_voice` frozen to that voice. If that **voice**
  already has an active analysis, the sample remains stored and the existing durable job id is returned; two voices
  of one account analyze independently. Deleting a sample re-enqueues while samples remain; deleting the last sample
  leaves the existing styleguide untouched.
- Sample insertion/deletion advances the voice's `corpus_version` in the same SQLite transaction. Analysis saves only
  when that version still matches; if the corpus changed during a provider call, the same job repeats against the
  newest full snapshot of that voice. Deleting the last sample during a call makes the job finish without publishing
  the stale result. No other voice's corpus version or profile head moves.
- A non-conflict queue failure is compensated before the RPC returns: a just-added sample is removed, or a
  just-deleted sample is restored.
- Analysis loads the voice's full current corpus, with explicit label separators, and runs through the shared job
  queue. Progress is `analyze 0/1 → 1/1`. The selected model is recorded in the job's `write_model` field.
- The prompt requires nine ordered Korean sections: 종결어미 distribution first; sentence length; sentences per
  paragraph; connectives/adverbs; verbal tics; emoji/interjections/ellipses; metaphors/numerals; expressions the
  author never uses; first-person form. A result whose first section is not 종결어미, or which lacks a “never uses”
  section, fails instead of replacing the profile.
- Successful analysis replaces that voice's `styleguide` wholesale and preserves `rules`. Provider failures flow
  through the job queue's normal user-facing error policy.

## Published behavior

- `PromptProfileForTopic(userID, voiceID, retrievalText, tags)` publishes one voice's projection: typed descriptors
  and legacy manual guidance, then evidence-ranked active rules, bans and ending constraints, then up to
  `ExcerptCount` (3) excerpts of at most `ExcerptChars` (1500) characters, then `rules`. It never falls back to the
  default or to another voice. Generation and revision consumers must inject those parts in exactly that order.
- Its `empty` value means there is neither a styleguide nor a sample nor a finalized source for that voice. Rules
  alone do not make the learned voice profile non-empty.
- `AppendRule(userID, voiceID, line)` trims one line, de-duplicates an exact existing line, appends with a newline,
  and never changes the styleguide. Profile writes and rule appends are serialized in the single API process so
  concurrent revision flows cannot lose a line.
- `GetVoiceProfile.active_job_id` exposes the voice's queued/running analysis so clients can resume polling after
  navigation or reload.

## Analyze model experiments

- An analyze experiment names one explicit active owned `voice_id`; the server never substitutes the default. The
  model lab freezes that voice's complete current corpus once and runs two explicit analyze refs through the same
  nine-section prompt and validator. Candidate completion changes neither profile nor rules.
- A verdict reveals model identities. Applying the winner requires overwrite confirmation, replaces only that same
  still-active voice's `styleguide`, preserves `rules`, and is value-idempotent; a voice deleted or a head moved since
  the freeze refuses the application. Adopting the winner as the account's active analyze model is separate. Model
  Elo and stage selections remain account-scoped.

## Frontend behavior

- 말투 is the directory `/voices`: active voices first (the default badged `기본`, each name a link into its profile,
  a pencil to rename in place, `기본으로 설정` and `삭제` on every non-default voice), a `새 말투` form (name field with
  a `n / 50자` count and the page's one CTA `말투 만들기`), and a `삭제된 말투` section (`삭제됨` badge, rename, `복원`).
  Deletion is confirmed through the sheet, which says what stays; the default offers no delete at all. Server
  refusals are shown under the control in the user's words (duplicate name, deleted default, busy voice, restore
  conflict).
- One voice is `/voices/$voiceId` and its five sibling tabs sharing one tab row: the profile (`프로필`),
  `/versions`, `/import` (analyze-stage model selector, paste form, sample list, analysis progress, legacy manual
  guidance), `/rules`, and `/validations`. The layout names the voice (`h1`), badges default/deleted, and for a
  tombstone shows a notice with `복원`; the import form is blocked for a deleted voice with the reason. An id the
  account does not own reads `없는 말투예요.`. The tab row is links, not a state control; the tab matching the address
  carries `aria-current="page"`. The top-level destinations stay 글 / 말투 / AI 모델.
- The legacy addresses `/voice` and `/voice/<tab>` redirect to the same tab of the account's default voice, read from
  the directory — never created — or to `/voices` when there is none.
- `/voices/$voiceId/rules/$id/compare` and `/voices/$voiceId/validations/$id` keep their addresses and return to
  their owning tab.
- Each tab issues only the queries its own panel renders. The profile query is shared by all five; the version,
  confirmation, and validation lists are fetched by the tab that displays them, and a list still in flight reads as
  loading rather than as an empty voice.
- A profile field is read first: label, provenance badge, and the value as wrapping text with no form control. Its
  pencil (named for the field) opens that one field for editing — a capped growing textarea seeded with the published
  value, 저장, 취소, and 직접 설정 해제 only when the field's source is `manual`. Fields are independent. 취소 discards
  the draft; a rejected save keeps edit mode and the draft, and its message does not survive into the next edit.
- The learn action is disabled below 200 trimmed Unicode characters or without a usable analyze selection; backend
  validation text is shown verbatim when the RPC rejects it. When a non-empty styleguide exists, adding a sample
  requires confirmation that re-analysis will overwrite the current styleguide.
- `active_job_id` resumes polling after navigation or reload. A successful job refreshes that voice's profile query
  so the new styleguide appears automatically; deletion does the same and shows re-analysis progress while samples
  remain.
- Every voice query cache is partitioned by `(account, voice)` — `voices`, `voice-profile`, `voice-versions`,
  `voice-confirmations`, `voice-validations` — so two voices of one account and two accounts on one device never
  read each other's entry. Directory mutations patch the cached list in the server's order (create/rename/
  delete/restore upsert one voice; set-default installs the returned list); rename, delete and restore also mark the
  voice's own scope and every cached post stale, because those display the name and the tombstone.
- Save responses patch only their owned field into the newest cache snapshot, and delayed responses clear a draft or
  sample input only when it still equals the value submitted. Styleguide and rules saves share a per-voice in-flight
  guard so the two controls report one coherent pending state.
- What a post shows about its voice, and how it is assigned or moved, is in [posts](posts.md).

## Owned constants

| Constant | Owner | Value |
|---|---|---:|
| `DefaultVoiceName` | BE `internal/voice`; FE renders the server value | `기본 말투` |
| `VoiceNameMaxChars` / `VOICE_NAME_MAX_CHARS` | BE `internal/voice` authoritative; FE `shared/config` mirror | 50 |
| `SampleMinChars` | BE `internal/voice` | 200 |
| `ExcerptCount` | BE `internal/voice` | 3 |
| `ExcerptChars` | BE `internal/voice` | 1500 |
| `LabelFallbackChars` | BE `internal/voice` | 20 |

## Progressive finalized-post learning

- Zero sources is supported: the typed profile is empty/unknown and generation remains enabled. Historical imports
  are optional; only their paste surface has the 200-character minimum. No screen requests or recommends 30 posts.
- Finalization and learning are separate explicit boundaries. **Finalize** records the post's exact finalized
  revision without a model or voice mutation. **Finalize and learn** performs that same finalization and then starts
  learning with an explicitly selected enabled analyze model; **Learn voice** may do so later while the same revision
  remains finalized. The input-hashed event freezes post-owned baseline/final JSON before `learn_voice` enqueue.
  Repeats/retries cannot duplicate a source, profile version, feedback, or evidence row.
- Learning derives its voice from the post and freezes it on the event; the caller cannot nominate another. It
  requires `post.voice_id == machine_baseline_voice_id == event.voice_id` — a post reassigned since its machine
  result must be regenerated or revised under the new voice first — and a deleted voice cannot receive evidence.
  Retries follow the event's frozen voice even after the post is reassigned; completed sources, rules and versions
  stay in their original voice.
- A learning setup, enqueue, worker, or provider failure leaves the post finalized and exposes a separate retryable
  learning outcome. Retry never repeats finalization or demotes the post.
- A completed event adds the finalized post to that voice's authored-source/few-shot bank. The first source is
  eligible for the next generation even when its topic differs; topic/tag matches rank ahead of stable recent
  fallback.
- The six axes carry presence. The analysis prompt names the `axes` object and its six keys, and the completion
  attaches a JSON schema requiring them when the resolved model declares `structured_output`; a model without the
  capability keeps the prompt-only path. An axis the model does not answer publishes as unknown and renders as
  알 수 없음 — never as 0 — and an answered value outside `-3..3` still fails the job. Snapshots already storing
  explicit zeros are immutable and keep showing them until the next learn.
- Typed whole-profile snapshots contain lexical, endings, syntax, structure, bounded axes, explicit unknown/source
  values, rules, sources, and feedback. Manual overrides and restore publish new heads while old snapshots remain.
  Legacy styleguide/rules remain byte-preserved manual guidance.
- Imported-sample analysis includes the voice's finalized authored sources. Source writes advance `corpus_version`,
  and typed publication also compares the profile head, so stale output cannot overwrite a concurrent source,
  override, rule update, or restore.

## Evidence, feedback, and explicit evaluation

- Deterministic Unicode measurement and LCS alignment reject factual-only edits. A same-kind pattern needs two cited
  edits and one finalization emits at most three rules.
- Independent evidence advances `candidate/1 → candidate/2 → active/3` within one voice. Only active rules reach
  generation. Contradictions wait for owner confirmation. Active rules older than 180 days retire only during a later
  explicit generation/revision request; retirement and profile publication are atomic.
- Rule status changes, confirmations and comparisons name only the rule; the server reads the voice off the owned row,
  so a same-account caller cannot point a rule at another voice. Sentence feedback derives its voice from the post and
  its finalization source, requires vocabulary, ending, length, or structure, and is retry-idempotent against owned
  final text. No-edit satisfaction is auxiliary. Neither alone creates or activates a rule.
- Rule comparison is an explicitly started blind one-rule job frozen to the rule's voice; a verdict requires two
  successful non-empty outputs and affects that rule only. Profile validation starts with an explicit active voice,
  requires three finalized sources of that voice, and calls a judge only when the user explicitly enables it. Neither
  writes model Elo.
- Page load, copy/export, polling, time, and boot never enqueue or call a personalization model. Queued
  personalization rows fail before boot worker drain and require explicit retry.

Full details: [voice personalization tech](../tech/voice-personalization-learning.md).
