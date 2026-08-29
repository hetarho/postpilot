# Multi-voice partitioning

> Planned technical decision for [plan 10](../plan/10.independent-voice-profiles-and-post-assi.md). It becomes
> current implementation truth only when that plan's job lands.

## Why `voice_id` is a partition key

An account is still the privacy boundary, but it is no longer the profile aggregate boundary. Two voices owned by
one account may intentionally contradict each other, so filtering only by `user_id` can return valid private data
that is nevertheless wrong for the selected voice. Every profile-derived root therefore carries both:

- `user_id` for authenticated account isolation and account deletion; and
- `voice_id` for aggregate isolation, versions, uniqueness, jobs, and prompt composition.

SQLite composite foreign keys and store signatures enforce both dimensions. The service never loads a user-wide
corpus and filters it after prompt assembly. A profile query starts from the owned voice and returns only rows in
that partition.

## Aggregate and context boundary

`Voice` and its profile/evidence lifecycle remain in `backend/internal/voice`. `Post` remains in
`backend/internal/post` and references the voice by id. Post, generation, and experiment contexts ask the voice
context's published behavior to validate or project a voice; they never join or update voice tables. The composition
root is the only place that adapts these consumer-owned ports.

`voices.deleted_at` is a tombstone, not an ownership transfer. There is no application hard-delete path. This keeps
post history renderable and prevents profile rows from cascading away merely because the user no longer wants the
voice in new-post choices.

## Frozen ownership for work

Every provider-backed operation freezes a `voice_id` before enqueue:

- generation/revision take it from the owned post;
- finalization learning takes it from the post and stores it on the learning event;
- rule comparison takes it from the owned rule/source;
- profile validation and analyze-model experiments take an explicit active voice; and
- retries take it from the durable owner row, never from the post's current assignment.

The job handler rechecks that the frozen voice belongs to the acting account and is eligible to receive the result.
It never follows a later post reassignment. Soft deletion is refused while work or an undecided experiment could
still publish; new work against an already deleted voice fails before enqueue.

## Reassignment is prospective

Changing `posts.voice_id` preserves the canonical post but does not rewrite historical evidence. In the same write it
clears `machine_baseline` and `machine_baseline_voice_id`. A later machine result under the new voice establishes a
new matching baseline. Finalization learning requires:

```
post.voice_id == post.machine_baseline_voice_id == learning_event.voice_id
```

This prevents a user correction against voice A's generated text from being interpreted as evidence about voice B.
Completed sources, rules, versions, and event retries remain in their original partition.

## Migration rule

The migration first creates one default voice per account, records the user-to-voice mapping, then rebuilds every
account-keyed voice table and the posts table with `voice_id`. Backfill is lossless: text bodies, legacy guidance,
protojson snapshots, revision numbers, statuses, timestamps, and durable ids do not change. Per-user uniqueness is
re-expressed per voice only after the backfill is verified.

The migration fails rather than guessing if a row cannot map to exactly one account/default voice. It runs through
the embedded boot migration path and single SQLite writer required by [I7].
