# Policy — Voice profiles

Canonical backend and frontend rules. Source: [plan/03](../plan/03.voice-profile-learning.md), built by jobs 08 and
09.

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

- `/voice` is authenticated and composes the analyze-stage model selector, sample learning form/list, durable-job
  progress, and separate styleguide/rules editors. The learn action is disabled below 200 trimmed Unicode characters
  or without a usable analyze selection; backend validation text is shown verbatim when the RPC rejects it.
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
