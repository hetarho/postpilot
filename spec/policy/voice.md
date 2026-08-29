# Policy — Voice profiles

Canonical backend and frontend rules. Source: [plan/03](../plan/03.voice-profile-learning.md), built by jobs 08, 09,
and 16.

## Ownership and persistence

- Each account owns at most one `voice_profiles` row and any number of `voice_samples`. Every store query and RPC is
  scoped by the authenticated user; voice requests never accept a user id. A foreign sample id is indistinguishable
  from an unknown one.
- Profile rows are created lazily. `styleguide` is machine-generated but user-editable. `rules` is user-owned;
  analysis never changes it. `UpdateVoiceProfile` uses optional field presence and atomically updates only supplied
  columns, so a rules save cannot write a stale styleguide over a completed analysis.
- A sample body is private server-side source material. RPCs return only id, label, Unicode character count, and
  creation time; they never return the body.

## Samples and analysis

- A body is trimmed and must contain at least `SampleMinChars` (200) Unicode characters. Rejection includes the
  measured count. An empty label falls back to the first `LabelFallbackChars` (20) characters.
- Before storing a sample, the server requires that the requested model exactly match the account's current enabled
  analyze-stage selection. With no usable selection, nothing is stored or enqueued.
- Adding a sample stores it and enqueues `analyze_voice`. If that account already has an active analysis, the sample
  remains stored and the existing durable job id is returned. Deleting a sample re-enqueues while samples remain;
  deleting the last sample leaves the existing styleguide untouched.
- Sample insertion/deletion advances `corpus_version` in the same SQLite transaction. Analysis saves only when that
  version still matches; if the corpus changed during a provider call, the same job repeats against the newest full
  snapshot. Deleting the last sample during a call makes the job finish without publishing the stale result.
- A non-conflict queue failure is compensated before the RPC returns: a just-added sample is removed, or a
  just-deleted sample is restored. Callers therefore do not see an error paired with a silently committed mutation.
- Analysis loads the account's full current corpus, with explicit label separators, and runs through the shared job
  queue. Progress is `analyze 0/1 → 1/1`. The selected model is recorded in the job's `write_model` field.
- The prompt requires nine ordered Korean sections: 종결어미 distribution first; sentence length; sentences per
  paragraph; connectives/adverbs; verbal tics; emoji/interjections/ellipses; metaphors/numerals; expressions the
  author never uses; first-person form. A result whose first section is not 종결어미, or which lacks a “never uses”
  section, fails instead of replacing the profile.
- Successful analysis replaces `styleguide` wholesale and preserves `rules`. Provider failures flow through the job
  queue's normal user-facing error policy.

## Published behavior

- `ProfileForPrompt(userID)` publishes `styleguide`, then up to `ExcerptCount` (3) most-recent sample excerpts of at
  most `ExcerptChars` (1500) characters each, then `rules`. Generation and revision consumers must inject those parts
  in exactly that order, with rules last.
- Its `empty` value means there is neither a styleguide nor a sample. Rules alone do not make the learned voice
  profile non-empty.
- `AppendRule` trims one line, de-duplicates an exact existing line, appends with a newline, and never changes the
  styleguide. Profile writes and rule appends are serialized in the single API process so concurrent revision flows
  cannot lose a line.
- `GetVoiceProfile.active_job_id` exposes the account's queued/running analysis so clients can resume polling after
  navigation or reload.

## Analyze model experiments

- The model lab freezes the complete current corpus once and runs two explicit analyze refs through the same
  nine-section prompt and validator. Candidate completion changes neither profile nor rules.
- A verdict reveals model identities. Applying the winner requires overwrite confirmation, replaces only
  `styleguide`, preserves `rules`, and is value-idempotent. Adopting the winner as active analyze model is separate.

## Frontend behavior

- 말투 is five authenticated sibling routes sharing one tab row: `/voice` (the current typed profile),
  `/voice/versions` (version list and restore), `/voice/import` (analyze-stage model selector, paste form, sample
  list, analysis progress, legacy manual guidance), `/voice/rules` (rule list, pending confirmations, blind
  comparison), `/voice/validations` (validation start and record list). The tab row is links, not a state control, so
  every tab is an address with a working back button; the tab matching the address carries `aria-current="page"`.
  The top-level destinations stay 글 / 말투 / AI 모델 — these five are sub-navigation.
- Each tab issues only the queries its own panel renders. The profile query is shared by all five; the version,
  confirmation, and validation lists are fetched by the tab that displays them, and a list still in flight reads as
  loading rather than as an empty account.
- A profile field is read first: label, provenance badge, and the value as wrapping text with no form control. Its
  pencil (named for the field) opens that one field for editing — a capped growing textarea seeded with the published
  value, 저장, 취소, and 직접 설정 해제 only when the field's source is `manual`. Fields are independent. 취소 discards
  the draft; a rejected save keeps edit mode and the draft, and its message does not survive into the next edit.
- `/voice/rules/$id/compare` and `/voice/validations/$id` keep their addresses and return to their owning tab.
- The learn action is disabled below 200 trimmed Unicode characters or without a usable analyze selection; backend
  validation text is shown verbatim when the RPC rejects it.
- When a non-empty styleguide exists, adding a sample requires confirmation that re-analysis will overwrite the
  current styleguide. This confirmation fires for every existing styleguide, not only one known to be hand-edited.
- `active_job_id` resumes polling after navigation or reload. A successful job refreshes the profile query so the new
  styleguide appears automatically; deletion does the same and shows re-analysis progress while samples remain.
- The editor shows the empty-profile warning only when both styleguide and samples are empty and links to `/voice`.
  Generation remains available.
- Profile query caches are partitioned by authenticated account id. Save responses patch only their owned field into
  the newest cache snapshot, and delayed responses clear a draft or sample input only when it still equals the value
  submitted. Account switches and in-flight operations therefore cannot expose or silently discard newer input.
- Styleguide and rules saves share an account-scoped in-flight guard so the two controls report one coherent pending
  state even though the backend updates their columns independently.

## Owned constants

| Constant | Value |
|---|---:|
| `SampleMinChars` | 200 |
| `ExcerptCount` | 3 |
| `ExcerptChars` | 1500 |
| `LabelFallbackChars` | 20 |

## Progressive finalized-post learning

- Zero sources is supported: the typed profile is empty/unknown and generation remains enabled. Historical imports
  are optional; only their paste surface has the 200-character minimum. No screen requests or recommends 30 posts.
- Finalization and learning are separate explicit boundaries. **Finalize** records the post's exact finalized
  revision without a model or voice mutation. **Finalize and learn** performs that same finalization and then starts
  learning with an explicitly selected enabled analyze model; **Learn voice** may do so later while the same revision
  remains finalized. The input-hashed event freezes post-owned baseline/final JSON before `learn_voice` enqueue.
  Repeats/retries cannot duplicate a source, profile version, feedback, or evidence row.
- A learning setup, enqueue, worker, or provider failure leaves the post finalized and exposes a separate retryable
  learning outcome. Retry never repeats finalization or demotes the post.
- A completed event adds the finalized post to the authored-source/few-shot bank. The first source is eligible for
  the next generation even when its topic differs; topic/tag matches rank ahead of stable recent fallback.
- The six axes carry presence. The analysis prompt names the `axes` object and its six keys, and the completion
  attaches a JSON schema requiring them when the resolved model declares `structured_output`; a model without the
  capability keeps the prompt-only path. An axis the model does not answer publishes as unknown and renders as
  알 수 없음 — never as 0 — and an answered value outside `-3..3` still fails the job. Snapshots already storing
  explicit zeros are immutable and keep showing them until the next learn.
- Typed whole-profile snapshots contain lexical, endings, syntax, structure, bounded axes, explicit unknown/source
  values, rules, sources, and feedback. Manual overrides and restore publish new heads while old snapshots remain.
  Legacy styleguide/rules remain byte-preserved manual guidance.
- Imported-sample analysis includes finalized authored sources. Source writes advance `corpus_version`, and typed
  publication also compares the profile head, so stale output cannot overwrite a concurrent source, override, rule
  update, or restore.

## Evidence, feedback, and explicit evaluation

- Deterministic Unicode measurement and LCS alignment reject factual-only edits. A same-kind pattern needs two cited
  edits and one finalization emits at most three rules.
- Independent evidence advances `candidate/1 → candidate/2 → active/3`. Only active rules reach generation.
  Contradictions wait for owner confirmation. Active rules older than 180 days retire only during a later explicit
  generation/revision request; retirement and profile publication are atomic.
- Sentence feedback requires vocabulary, ending, length, or structure and is retry-idempotent against owned final
  text. No-edit satisfaction is auxiliary. Neither alone creates or activates a rule.
- Rule comparison is an explicitly started blind one-rule job; a verdict requires two successful non-empty outputs
  and affects that rule only. Profile validation requires three finalized sources; judge calls occur only when the
  user explicitly enables them. Neither writes model Elo.
- Page load, copy/export, polling, time, and boot never enqueue or call a personalization model. Queued
  personalization rows fail before boot worker drain and require explicit retry.

Full details: [voice personalization tech](../tech/voice-personalization-learning.md).
