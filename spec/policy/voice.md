# Policy — Voices and voice profiles

Canonical backend and frontend rules. Source: [plan/03](../plan/03.voice-profile-learning.md), built by jobs 08, 09,
and 16, and [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), built by job 18. The cross-context
decision behind the partition is [multi-voice partitioning](../tech/multi-voice-partitioning.md). Immutable source
language, bilingual analysis, portable projection, and language-safe learning are from
[plan/13](../plan/13.multilingual-interface-and-target-langua.md), job 32.

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
- The same holds for a 작문 지침: writing guidelines are not voice data, never enter `ProfileForPrompt`, and leave the
  profile's bytes unchanged (see [guidelines](guidelines.md)). Register stays voice-owned — a guideline's precedence
  sentence says so explicitly — and `규칙으로 저장` remains the only revision path that writes voice state.
- The profile is the **structured profile**, its earned rules and its sample excerpts. There is no user-editable
  free-text styleguide: migration 0017 dropped the column (change 16). `rules` survives as the 규칙으로 저장 text
  alone — no editor, no RPC, and analysis never changes it. A voice whose analysis existed ONLY as that free text and
  had never published a structured version is rescued by 0017 itself, which publishes it as that voice's version 1
  with the text as an `analyzed` lexical description.
- A sample body is private server-side source material. RPCs return only id, label, Unicode character count, and
  creation time; they never return the body.

## Voice lifecycle

- `ListVoices` returns the account's voices including tombstones, active first, the default first among them, then
  by name. `GetVoiceProfile` returns the owning `voice` summary with the profile.
- `CreateVoice` trims the name, requires 1–`VoiceNameMaxChars` (50) Unicode scalar values and an explicitly present
  supported immutable `source_language` (`InvalidArgument` otherwise), refuses an active-name collision
  (`AlreadyExists`), and creates an empty isolated profile. It never copies the default profile. Existing/bootstrap
  voices are Korean; no RPC changes source language after creation. There is no voice-count limit.
- `CreateVoice` also takes an OPTIONAL trimmed `description` of at most `VoiceDescriptionMaxChars` (500) Unicode
  scalar values (`VOICE_DESCRIPTION_TOO_LONG`, `InvalidArgument`) plus the account's current analyze-stage `ModelRef`.
  With one, it creates the voice and enqueues exactly one `seed_voice` job that writes the voice's first profile from
  that description, and answers with the job id; without one it behaves exactly as before and answers with an empty
  job id, requiring no model. The description length and the analyze model are both checked BEFORE the insert, so a
  refusal leaves no voice behind. The reverse is deliberate and documented: creation and seeding are separable
  outcomes, so a seed the queue refuses to start (too few credits, say) still leaves a real, usable, listed voice.
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
  ([I5]). A described creation is the one exception, and it is an explicit user action: it enqueues, it never calls a
  provider in the request.

### Seeding a voice from a description

- The `seed_voice` job is voice-owned like every other personalization kind: it serializes against that voice's other
  work, is refused for a deleted voice, blocks that voice's deletion while queued or running, and fails before the
  boot worker drain.
- The handler makes exactly ONE completion against the account's analyze-stage selection — the same stage that writes
  every other voice profile — in the voice's declared `source_language`, in the same nine-section shape the analysis
  produces, and publishes it as profile version 1 with origin `seed`.
- A description is an instruction, never evidence. It becomes no `voice_samples` row, no source count, no few-shot
  entry, no embedding, and no `corpus_version` bump, and `can_validate` is unchanged. The published version carries
  the analysis as an `analyzed` lexical description and leaves every measured quantity and axis unset; the prompt
  projection renders an unset measurement as `unknown`, never as `0`.
- A seed never overwrites real work. It publishes only while the profile is still empty and unversioned, and only
  while the corpus version it read still holds; a seed that resolves after a sample analysis or a finalization has
  published completes successfully without writing.
- Seeding happens once, at creation. There is no re-seed RPC: a failed or superseded seed leaves an ordinary empty
  voice whose `말투` tab shows the failure, and the recovery path is the existing 기존 글 가져오기.

## Samples and analysis

- A body is trimmed and must contain at least `SampleMinChars` (200) Unicode characters. Rejection includes the
  measured count. An empty label falls back to the first `LabelFallbackChars` (20) characters.
- Before storing a sample, the server requires that the requested model exactly match the account's current enabled
  analyze-stage selection (model selection stays account-scoped). With no usable selection, nothing is stored or
  enqueued.
- Adding a sample stores it under its voice and enqueues `analyze_voice` frozen to that voice. If that **voice**
  already has an active analysis, the sample remains stored and the existing durable job id is returned; two voices
  of one account analyze independently. Deleting a sample re-enqueues while samples remain; deleting the last sample
  leaves the existing published profile untouched.
- Sample insertion/deletion advances the voice's `corpus_version` in the same SQLite transaction. Analysis saves only
  when that version still matches; if the corpus changed during a provider call, the same job repeats against the
  newest full snapshot of that voice. Deleting the last sample during a call makes the job finish without publishing
  the stale result. No other voice's corpus version or profile head moves.
- A non-conflict queue failure is compensated before the RPC returns: a just-added sample is removed, or a
  just-deleted sample is restored.
- Analysis loads the voice's full current corpus, with explicit label separators, and runs through the shared job
  queue. Progress is `analyze 0/1 → 1/1`. The selected model is recorded in the job's `write_model` field.
- Analysis selects a source-language-specific prompt and schema. Korean retains the nine ordered sections and
  deterministic ending distribution. English measures word length, register/contractions, connectives, passive and
  nominal tendencies, terminal cadence, structure, lexical habits, and the same six axes without projecting those
  values into Korean ending categories. Each corpus contains only evidence declared for that immutable source.
- Successful analysis publishes a new structured profile version whose lexical description IS the analysis, and
  preserves the 규칙으로 저장 text. The `corpus_version` guard it has to win first no longer writes any text — its
  whole job is to say whether this analysis still describes the newest corpus (change 16). Provider failures flow
  through the job queue's normal user-facing error policy.

## Published behavior

- `PromptProfileForTopic(userID, voiceID, retrievalText, tags, targetLanguage)` publishes one voice's language-aware
  projection. Equal source/target languages contain the complete typed descriptors — injected **once**, with no
  legacy free-text guidance beside them (change 16) — ranked active rules, the 규칙으로 저장 lines, and up to
  `ExcerptCount` (3) excerpts; cross-language projection contains only the exact portable
  structure/axes allowlist from [languages](languages.md). It never translates or leaks excluded fields, falls back
  to another voice, or consults another language's evidence.
- Its `empty` value means there is neither a published profile version nor a sample nor a finalized source for that
  voice. Rules alone do not make the learned voice profile non-empty. The free-text styleguide that used to count as
  content no longer exists (change 16).
- `AppendRule(userID, voiceID, line)` trims one line, de-duplicates an exact existing line, and appends with a
  newline. It is the only writer of that text, and it has no editor and no RPC. Rule appends are serialized in the
  single API process so concurrent revision flows cannot lose a line.
- **Every profile version may carry one generation snapshot** — a copy of the raw AI output of the last post
  generated under it — and `GetVoiceProfileVersionSample(voiceID, version)` returns it. Both the voice and the
  account are named, so no cross-voice or cross-account read is expressible; a soft-deleted voice's snapshots stay
  readable with the rest of its profile and are never reachable from another voice ([I4]). A version that never
  produced a post answers with no sample, which is an ordinary state rather than a failure. A later generation under
  the same head REPLACES the snapshot. Because it is a copy, deleting, regenerating, editing or reassigning the
  source post leaves it unchanged.
- **`UpdateVoiceProfile` no longer exists.** No RPC accepts or returns a free-text styleguide or rules string; the
  profile is changed by an analysis, a seed, a manual override, an adopted version, or an earned rule.
- `GetVoiceProfile.active_job_id` exposes the voice's queued/running analysis so clients can resume polling after
  navigation or reload.

## Analyze model experiments

- **Applying a winner publishes a structured profile version**, origin `analysis`, whose lexical description is the
  winning analysis and whose measured half is measured from the voice's current corpus exactly as an analysis run
  measures it, with the account's manual overrides and earned rules carried onto it (change 16). It still requires
  its confirmation. It publishes only while it is still the head: an analysis or a rule that published while the
  operator was confirming is newer evidence, so a lost race is reported as a failure to apply rather than recorded
  as a success.
- An analyze experiment names one explicit active owned `voice_id`; the server never substitutes the default. The
  model lab freezes that voice's complete current corpus once and runs two explicit analyze refs through the same
  nine-section prompt and validator. Candidate completion changes neither profile nor rules.
- A verdict reveals model identities. Applying the winner requires overwrite confirmation, affects only that same
  still-active voice, and preserves its 규칙으로 저장 text; a voice deleted or a head moved since the freeze refuses
  the application. Adopting the winner as the account's active analyze model is separate. Model
  Elo and stage selections remain account-scoped.

## Frontend behavior

- **버전 기록 is a list of OPENABLE rows.** A row names the version's number, origin and whether it was restored
  from another; opening it shows that version's date and its **generation snapshot** — the title and body of the last
  post it wrote — and carries `이 버전으로 변경`. The preview IS the confirmation, so there is no separate confirm
  dialog: adopting still publishes a new head and destroys no history. A version that never produced a post says so
  plainly and shows no empty preview. The current head is openable and offers no way to adopt itself, and a deleted
  voice's versions are read-only. The preview shows the snapshot bounded by `VOICE_VERSION_PREVIEW_CHARS` with a way
  to read the rest; a snapshot is read as text rather than re-rendered as an article, because it has no image rows
  behind its file references.
- **기존 글 가져오기's first field is 제목 (선택).** It is the imported piece's name, it is still optional, and a
  blank one still takes the server's body-derived fallback. `SampleMinChars` is unchanged at 200.
- **The 이전 수동 안내 section is gone**, with both of its editors. No voice screen offers a free-text styleguide or
  rules field (change 16).
- **문장 의견 is offered on 글 완성, and only for a post whose voice-learning run has completed** — the exact
  condition the server enforces. It is not rendered on 글 다듬기, where a post is in `review` and the server refused
  it every time. Its surface states its PURPOSE: this teaches the voice and does not change this post, with AI 수정
  named as the way to change the post. The sentence is chosen from a vertical `radiogroup` of full-width rows that
  wrap — the whole-sentence `Listbox` trigger was `whitespace-nowrap`, which gave the sheet a horizontal scroll —
  keeping select-one semantics and one tab stop. The four reasons, the single retry-idempotent record and the
  absence of any free-text field are unchanged.
- 말투 is the directory `/voices`, and it is a LIST. Each active voice is ONE row and one target: a stretched link
  into that voice (no underline — the row is the link, and nothing interactive is nested inside the anchor), its
  `기본`/language badges, and, for a non-default voice, `기본으로 설정` and `삭제` on that same row, layered above the
  link so pressing one acts without navigating. Tombstones live in a `삭제된 말투 N개` disclosure that is rendered only
  when one exists and is closed on load; its rows keep `복원`. The page's one CTA is a docked `새 말투 만들기`.
  Renaming is NOT on the directory: the row leads to the voice, and the name is edited there.
- `새 말투 만들기` opens the shared `Sheet` (a bottom sheet below `md:`) carrying the name field with its `n / 50자`
  count, the source-language listbox, and an OPTIONAL `말투 설명` textarea with its own count. Its commit action sits
  in flow after the last field rather than in the sheet's pinned footer, because the panel is anchored to the layout
  viewport the software keyboard does not resize (design-language §8.3). With no usable analyze selection the
  description's message slot says so and creating without a description still works. A successful create closes the
  sheet and navigates to the new voice, where a seeding run reports its progress and its failure through the same
  surface the analysis uses.
  Deletion is confirmed through the sheet, which says what stays; the default offers no delete at all. Server
  refusals are shown under the control in the user's words (duplicate name, over-long description, deleted default,
  busy voice, restore conflict).
- One voice is `/voices/$voiceId` and its five sibling tabs sharing one tab row: the profile (`프로필`),
  `/versions`, `/import` (analyze-stage model selector, paste form, sample list, analysis progress, legacy manual
  guidance), `/rules`, and `/validations`. The layout names the voice (`h1`), badges default/deleted, and for a
  tombstone shows a notice with `복원`; the import form is blocked for a deleted voice with the reason. That `h1` is
  where the voice is renamed, read-first behind a pencil named for the voice, and a tombstone stays renameable so a
  restore conflict can be resolved. An id the
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
- The learn action is disabled below 200 trimmed Unicode characters or without a usable analyze selection; an RPC
  rejection is rendered from stable `AppFailure` reason/params in the active locale, never raw transport prose. When
  the voice has already published a profile version, adding a sample requires confirmation that re-analysis will
  replace the current analysis.
- `active_job_id` resumes polling after navigation or reload. A successful job refreshes that voice's profile query
  so the new analysis appears automatically; deletion does the same and shows re-analysis progress while samples
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
| `VoiceDescriptionMaxChars` / `VOICE_DESCRIPTION_MAX_CHARS` | BE `internal/voice` authoritative; FE `shared/config` mirror | 500 |
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
  result must be regenerated or revised under the new voice first — plus concrete frozen content/source languages
  equal to the active voice's source language. A mismatch is refused before event/job/provider creation; a deleted
  voice cannot receive evidence.
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
  values, rules, sources, and feedback. Manual overrides and adopting an older version publish new heads while old
  snapshots remain.
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
  final text. Sentence feedback, revision save-as-rule, post-backed comparison, and profile validation all repeat the
  same content/source-language equality gate before mutation or provider work. No-edit satisfaction is auxiliary.
  Neither alone creates or activates a rule.
- Rule comparison is an explicitly started blind one-rule job frozen to the rule's voice; a verdict requires two
  successful non-empty outputs and affects that rule only. Profile validation starts with an explicit active voice,
  requires three finalized sources of that voice, and calls a judge only when the user explicitly enables it. Neither
  writes model Elo.
- Page load, copy/export, polling, time, and boot never enqueue or call a personalization model. Queued
  personalization rows — `learn_voice`, `compare_voice_rule`, `validate_voice_profile` and `seed_voice` — fail before
  boot worker drain and require explicit retry, or, for a seed, leave an ordinary empty voice.

Full details: [voice personalization tech](../tech/voice-personalization-learning.md).
