# Policy — Stage model experiments

Canonical rules implemented by job 15. Source: [plan 09](../plan/09.stage-model-experiments-and-leaderboards.md)
and [tech/model-experiment-methodology](../tech/model-experiment-methodology.md).

## Fair and blind comparisons

- One experiment varies exactly one stage: `observe`, `analyze`, or `write`. Both candidates receive the same
  immutable snapshot, prompt version, schema and validation path; only the explicit `ModelRef` differs.
- Normal generation observes once and fans out only the two write candidates. Observe/analyze comparisons run only
  from the explicit model lab and do not mutate their source while candidates run.
- The server randomizes and persists left/right once. Before a verdict, RPC responses contain opaque candidate ids,
  sides and outputs but omit model refs, labels, accounting and provider errors. Removed-model retry errors are also
  identity-free. A verdict or dismissal reveals the snapshotted identities and accounting.
- Candidate calls run concurrently with pair width two. Progress is one monotonic `compare_<stage>` counter; provider
  work never runs inside a SQLite transaction.

## Lifecycle, retry and application

- `queued → running → review | partial | failed`; `review/partial/failed → decided | dismissed`. Only a successful
  paired verdict has `outcome=winner`; `unpaired` and `skipped` never affect ratings.
- Start persists the experiment and queue job before returning both ids and before any provider call. One unresolved
  or not-yet-applied write experiment may exist per post. The post projection publishes its id.
- A partial/failed retry resets and invokes only failed candidates against the original snapshot. Enqueue failure
  restores their prior failure state. A missing model or unusable snapshot requires a new comparison without
  exposing the hidden ref.
- Boot recovery fails interrupted `running` experiments and only those `queued` experiments that have no runnable
  queue record. Surviving queued jobs continue normally; recovered candidates become retryable.
- Candidate completion never changes canonical post/profile state. Choosing records the verdict, then explicit
  stage-owner behavior applies the winner. `applied_at` is the durable idempotency marker; an unapplied or
  apply-failed write remains in `pending_experiment_id` with a retry action. Stage-owner writes are value-idempotent
  as an additional cross-context retry safeguard.
- Write apply replaces validated `PostContent` and moves the post to `review`. Observe apply replaces observations.
  Analyze apply requires confirmation, replaces `styleguide`, and never changes user-owned `rules`. Adopting the
  winner as the active model is a separate action.

## Ranking and accounting

- Leaderboards are private to `(account, stage)`. Winner verdicts replay in decision order from Elo 1500 with K=32.
  Dismissed, failed and unpaired outcomes are excluded. Entries below three matches are provisional.
- Each candidate retains prompt/completion tokens, latency and cost quality. Provider-reported cost is authoritative;
  otherwise catalog prices estimate cost only when usage tokens are present. Missing usage or pricing is
  `unavailable`, never estimated zero. Aggregates preserve `reported`, `estimated`, `mixed`, and `unavailable`.
- Model refs, labels and usage remain legible after catalog removal. Removed/disabled models cannot start, retry or
  become active, and capability checks happen before a network call.

## Ownership, retention and deletion

- Every experiment/read/action and leaderboard is scoped from the authenticated account; no request accepts an
  account id. Foreign experiment access is denied.
- Verdict/dismissal starts a 30-day content-retention clock. The sweeper clears input snapshots and candidate output,
  retaining verdict, model snapshot, usage and timing for leaderboard replay.
- `DeletePost` calls the experiment purge behavior transactionally before deleting the post, so the FK may safely
  detach retained metadata only after private payload/output is gone. Account deletion cascades all experiment rows.
- Logs may contain ids, stage and accounting, but never snapshots, photos, voice samples or candidate output. Apply
  failures store fixed user-facing copy; internal errors stay in logs.

## Configuration

| Value | Owner | Value |
|---|---|---|
| candidate concurrency | BE `platform/config` | `2` |
| `EXPERIMENT_CONTENT_RETENTION` | env | default `720h`, positive |
| `EXPERIMENT_SWEEP_INTERVAL` | env | default `24h`, positive |
| Elo initial/K/minimum matches | BE experiment domain | `1500` / `32` / `3` |
| `LEADERBOARD_MIN_MATCHES` | FE `shared/config` | `3` |
| input hash | BE experiment domain | SHA-256 of frozen bytes |
