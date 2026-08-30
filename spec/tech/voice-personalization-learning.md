# Tech — Progressive voice personalization

This document is the technical SSOT for the progressive learning system introduced by change 02 and job 16, and
partitioned per voice by job 18 ([multi-voice partitioning](multi-voice-partitioning.md)). It extends the original
optional pasted-sample analysis without making historical writing an onboarding requirement. Wherever this document
says "account", read "one voice of the account": every profile, source, rule, event, comparison and validation row
carries `(user_id, voice_id)`, and no step reads across voices.

## Product boundary

- An account with no samples and no finalized posts has an empty typed profile and can generate immediately.
- `확정` flushes the content queue and records the exact post revision without voice work or an analyze model.
  A finalized post becomes learning input only after the user separately presses **Finalize and learn** or the later
  **Learn voice** action with an explicit analyze model. Page load, navigation, export, copy, time, and boot never
  initiate personalization provider work.
- Pasted historical posts remain an optional import path with the existing 200-character minimum. There is no
  minimum or recommendation of 30 posts. Three finalized posts are required only for the separately initiated
  profile-validation tool.
- There is no scheduled A/B test, weekly/monthly job, automatic embedding, or background judge. Rule comparison and
  profile validation each require an explicit start action and explicit model references.

## Dependency decision

Job 16 adds no third-party sentence, Korean morphology, diff, vector, embedding, SDK, Python, CGO, sidecar, or model
dependency. Sentence segmentation, Unicode character counts, Korean ending buckets, LCS alignment, canonical rule
keys, and tag retrieval use the Go standard library. This keeps the existing static-SPA + distroless-Go deployment
unchanged and makes unit tests deterministic.

The optional `embedding_ref` column is only a future-compatible seam. Even after the 50-source threshold, the
current implementation has no embedding adapter and therefore keeps using deterministic tag/title retrieval. It
never selects a provider or model implicitly.

## Canonical post and immutable baseline

`posts.content` remains the validated `PostContent` block array. `content_revision` is the optimistic revision of
that mutable canonical value. Every machine winner or successful AI revision atomically writes both canonical
content and `machine_baseline`, then sets `machine_baseline_revision` to the new content revision. Manual saves
increment only `content_revision`; they cannot change the baseline. Consequently equality of the two revisions means
the current text still equals the latest machine baseline.

`SavePostContent` rejects stale revisions and any invalid block shape or IMAGE filename not attached to the owned
post. A changed save after finalization returns it to `review`; an identical save preserves `finalized`. The voice
context obtains a frozen ownership-checked baseline/final pair only when `LearningSnapshot` verifies that the current
revision is exactly the recorded finalized revision.

The browser uses a per-post content queue separate from title/memo autosave. It allows one request in flight, keeps
only the newest pending snapshot, advances the expected revision from each response, applies bounded exponential
retry, stops on an optimistic conflict, discards all pending retry state on logout, and performs a best-effort flush
on page hide. AI revision and finalization await the same explicit `flush()` boundary.

## Explicit finalization and idempotency

Post finalization and voice learning are separate boundaries. `FinalizePost` records status/revision/time without a
voice event, durable job, or provider call. An explicit learning action then derives the post's voice — the caller
cannot nominate one — requires `post.voice_id == machine_baseline_voice_id`, hashes the baseline revision plus
canonical baseline/final JSON and stores a `voice_learning_events` row carrying that frozen `voice_id` before queue
wake-up. A uniqueness constraint on the account, voice, post, baseline revision and input hash makes a repeated action
return the existing event; retries follow the event's voice even after the post is reassigned. A failed
enqueue leaves the post finalized and that event retryable; an explicit retry creates a new durable job without
duplicating the event. When boot has deliberately failed an old queued/running personalization job, repeating Learn
or Retry checks that durable job and re-enqueues the same immutable event; the restart itself performs no provider
call.

Successful learning writes the authored source, excerpt, extracted evidence/rules, whole profile version, and event
completion in one short SQLite transaction after provider calls finish. A completed event is a no-op on retry.
Source and evidence uniqueness constraints are the final concurrency arbiters.

## Typed profile and versions

The effective profile of one voice is a whole immutable snapshot containing:

- lexical description, preferred/banned words and patterns;
- endings as a primary layer: base register, measured distribution, bans/signatures, and constraints;
- syntax measurements and descriptors;
- opening/closing and paragraph/heading/list/emoji structure;
- six integer axes bounded to `-3..3`, each carrying presence — an axis the analysis did not answer is absent
  (published as unknown), not a neutral 0;
- contrast-rule metadata, authored-source/few-shot references, feedback references, version, timestamps, and source
  count.

The analysis call names every key it expects, including the `axes` object and its six keys, and joins the
structured-output path: it attaches an embedded JSON schema (`internal/voice/schemas/voice_analysis.schema.json`)
when the resolved model declares `structured_output`, exactly as write, revise, and observe do, and falls back to the
prompt alone otherwise. Weak or absent evidence is represented explicitly as `unknown`. Deterministic measurements override conflicting
model estimates for ending distribution and average sentence length. A manual field override is stored separately,
replayed after analysis, and published as a new whole-profile version. Clearing it replays the newest analysis
snapshot plus remaining overrides. Restore copies an old immutable snapshot into a new head; it never moves or
deletes history.

Optional imported-sample analysis uses both current imports and finalized authored sources. Finalized-source writes
advance the same corpus generation used by analysis CAS, and typed publication additionally checks the profile head,
so a concurrent source, manual override, or rule version cannot be overwritten by stale provider output.

Legacy `voice_profiles.styleguide` and `rules` are retained byte-for-byte by migration 0007 and remain prompt input
as manual guidance until the account changes them.

## Measurement, diff, and lifecycle

The deterministic segmenter recognizes Korean/Latin terminal punctuation and newlines. It counts Unicode code
points, classifies `다`, polite `해요` variants, formal `니다/습니다`, and other endings, and measures sentence and
paragraph rhythm.

Finalization diff uses an LCS to retain stable equal sentences and pairs unmatched runs in order. Inside that same
explicit learning job, `voice-diff-rules-v1` asks the selected analysis model to discard factual/topic changes and
return strict JSON containing at most three generalized `LLM does …, but I do …` rules. The server rejects unknown
layers, malformed statements, duplicate/out-of-range citations, fewer than two independent citations, and
inconsistent measured ending/length citations. It persists citation references rather than copying source prose into
rule evidence.

Canonical equality and direct negation checks run before any additional call. Only when those checks cannot decide,
`voice-semantic-rule-v1` asks the same explicit analysis model for strict `same`, `contradicts`, or `distinct` JSON
against same-layer rules. One independent event can add at most one evidence row for a matched rule. Evidence counts
move `candidate/1 → candidate/2 → active/3`. Candidate, retired, and rejected rules never enter generation. A
semantic contradiction against an active rule stores the authored source and a pending confirmation but leaves all
rule state and the current profile head unchanged until the owner resolves it.
Resolution reloads the persisted rule set inside the same transaction after keep/replace, so the newly published
whole-profile version exactly matches the rule row, including reset evidence count and candidate status.
An active rule older than 180 days is retired only while servicing a later explicit generation/revision request; no
clock-driven process exists. Retirement and its new profile version commit together, and a failed retirement blocks
prompt construction rather than using stale rules.

Sentence feedback is one of vocabulary, ending, length, or structure. It is ownership-checked against finalized
text and retry-idempotent. A no-edit satisfaction event is auxiliary. Neither form alone creates or activates a
rule.

## Retrieval and prompt contract

Generation and revision receive, in order:

1. typed structured descriptors and legacy manual guidance;
2. evidence-ranked active rules, bans, and ending constraints;
3. up to three unique authored/import excerpts, with topic/tag matches first and stable recent fallback so one
   finalized source is still useful for an unrelated next topic;
4. the post's frozen observations, title/memo/topic, attachment names, and optional target length when configured.

The prompt tells the model not to copy source phrases or facts, follows the measured ending distribution, and
forbids a third consecutive identical ending. Candidate/retired rules are excluded. Zero excerpts is valid.

## On-demand comparison and validation

Rule comparison freezes one candidate rule, source, profile version, optional target length, examples, and explicit write
model. Its two calls share the same input and differ only by the selected rule. The server does not reveal the
rule-on side before a decision and accepts a decision only when exactly two non-empty candidates succeeded. A
rule-on verdict adds evidence only to that rule; a rule-off verdict rejects it. Model Elo is untouched.

Profile validation requires three finalized authored sources. It freezes the profile and explicit analyze/write
models, selects three sources deterministically from a random id, creates topic-only neutral summaries, then writes
new prose from those summaries. The judge call exists only when the user explicitly enables it and records Y/N for
endings, sentence rhythm, opening/closing, vocabulary, and addressee behavior against the frozen version.

Both flows are durable jobs with partial retry. If enqueue fails, the aggregate becomes failed and stops blocking a
new start. If linking a queued job fails, the queue compensates by failing that owner-scoped queued row; if a worker
already owns it, the durable ids are returned and the worker completes the existing aggregate.

## Boot and provider-call safety

At boot, queued personalization jobs are marked failed before the worker starts, and running jobs use the ordinary
restart sweep. Boot never drains old personalization rows into provider calls. Read RPCs, polling, profile age,
copy/export, and UI mount do not enqueue. A failed job remains visible/retryable and the last valid canonical post and
profile head remain unchanged.

## Retention and deletion

All learning prose, diffs, feedback, comparisons, validations, and versions are private account data. Rows are
account/voice/post scoped, and deletion cascades from the owning account or post as declared by migrations 0007 and
0009; a soft-deleted voice keeps every row and only stops accepting new evidence. No private prose is logged. There is
no cross-account — or cross-voice — retrieval, comparison, validation, or prompt input. The SPA removes every
account-scoped query cache on successful logout and on a mid-session authentication failure before another account
can render.

## Configuration

| Backend value | Default | Meaning |
|---|---:|---|
| `VOICE_FEW_SHOT_TARGET_COUNT` / `VOICE_FEW_SHOT_MAX` | `2` / `3` | soft target and hard excerpt cap |
| `VOICE_FEW_SHOT_EXCERPT_TARGET_CHARS` / `VOICE_FEW_SHOT_EXCERPT_MAX_CHARS` | `500` / `800` | excerpt target and cap |
| `VOICE_EMBEDDING_SWITCH_POSTS` | `50` | capability threshold, never onboarding minimum |
| `VOICE_DIFF_MAX_RULES` / `VOICE_DIFF_MIN_PATTERN_EDITS` | `3` / `2` | per-event rule cap and evidence floor |
| `VOICE_RULE_ACTIVATION_EVIDENCE` | `3` | activation threshold |
| `VOICE_RULE_RETIRE_AFTER` | `180d` | explicit-request retirement age |
| `VOICE_VALIDATION_POST_COUNT` | `3` | explicit validation eligibility |
| `VOICE_ENDING_MAX_CONSECUTIVE` | `2` | prompt ending-run limit |

The frontend mirrors only visible input/display boundaries. There is deliberately no personalization schedule,
interval, or cron configuration.
