# Tech — Multi-voice partitioning

The cross-context decision behind [plan 10](../plan/10.independent-voice-profiles-and-post-assi.md), current
implementation truth since job 18. Rules that are user-facing live in [voice](../policy/voice.md) and
[posts](../policy/posts.md); this document owns why the partition is shaped the way it is.

## Why `voice_id` is a partition key

An account is still the privacy boundary, but it is no longer the profile aggregate boundary. Two voices owned by
one account may intentionally contradict each other, so filtering only by `user_id` can return valid private data
that is nevertheless wrong for the selected voice. Every profile-derived root therefore carries both:

- `user_id` for authenticated account isolation and account deletion; and
- `voice_id` for aggregate isolation, versions, uniqueness, jobs, and prompt composition.

SQLite composite foreign keys and store signatures enforce both dimensions: every store method that reads or writes
profile data takes `(userID, voiceID)`, and the tables that were unique per user are unique per voice —
`(voice_id, version)` for profile versions, `(voice_id, canonical_key)` for contrast rules, one active validation per
voice, one active analysis per voice. The service never loads a user-wide corpus and filters it after prompt
assembly. A profile query starts from the owned voice and returns only rows in that partition.

Rule-derived operations (status changes, confirmations, comparisons) name only the rule; the store reads the voice
off the owned row (`GetContrastRuleForUser`), so a same-account caller cannot point a rule at another voice.

## Aggregate and context boundary

`Voice` and its profile/evidence lifecycle live in `backend/internal/voice`. `Post` lives in `backend/internal/post`
and references the voice by id. Post, generation, and experiment contexts ask the voice context's published behavior
through consumer-owned ports — `post.VoiceDirectory` (resolve `VoiceRef`s, check an active voice),
`generation.Profiles`/`Rules` (one voice's projection, rule append), `experiment.VoiceDirectory` (active-voice check)
— and `voice` in turn asks `Jobs` and `Experiments` whether a voice still has publishable work before a delete. None
of them joins or updates another context's tables; `backend/cmd/api` is the only place that adapts these ports, and a
boundary test forbids sibling `store`/`sqlc` imports inside `internal/`.

`voices.deleted_at` is a tombstone, not an ownership transfer. There is no application hard-delete path. This keeps
post history renderable and prevents profile rows from cascading away merely because the user no longer wants the
voice in new-post choices.

The directory order is part of the contract — active before deleted, the default first, then by name and id — and
the frontend re-applies the same order after a cache patch so an inserted or renamed voice lands where a refetch
would put it.

## Frozen ownership for work

Every provider-backed operation freezes a `voice_id` before enqueue:

- generation/revision take it from the owned post;
- finalization learning takes it from the post and stores it on the learning event;
- rule comparison takes it from the owned rule/source;
- profile validation and analyze-model experiments take an explicit active voice; and
- retries take it from the durable owner row, never from the post's current assignment.

The job handler rechecks that the frozen voice belongs to the acting account and is eligible to receive the result.
It never follows a later post reassignment: a generation or revision whose frozen voice no longer matches the post is
refused rather than written. Soft deletion is refused while a voice-owned job, an undecided comparison/validation, or a
publishable analyze experiment could still publish; new work against an already deleted voice fails before enqueue.
`generation_jobs.voice_id` is nullable only for work that neither reads nor publishes voice-owned state (for
example an observe comparison). Generation, revision, write comparisons, and all personalization kinds freeze the
post or explicit target voice. Voice-owned kinds are guarded per `(voice_id, kind)` even when a learning or rule
comparison row also retains its triggering `post_slug`; that row must satisfy both the post guard and the voice guard.
Two different voices can therefore analyze or learn independently, while two posts cannot start overlapping
learning work for the same voice.

## Reassignment is prospective

Changing `posts.voice_id` preserves the canonical post but does not rewrite historical evidence. In the same write it
clears `machine_baseline` and `machine_baseline_voice_id`. A later machine result under the new voice establishes a
new matching baseline. Finalization learning requires:

```
post.voice_id == post.machine_baseline_voice_id == learning_event.voice_id
```

This prevents a user correction against voice A's generated text from being interpreted as evidence about voice B.
Completed sources, rules, versions, and event retries remain in their original partition. Reassignment is refused
while a job targets the post or an undecided write experiment exists, because either could still establish a baseline
for the old voice.

## Frontend partitioning

Every voice cache key carries `(account, voice)`: `['voice-profile', transport, ownerId, voiceId]` and the like, with
the directory itself per account. The post caches carry the `VoiceRef` projection, so a rename, delete or restore marks
the voice's own scope **and** every cached post stale rather than patching names into places the server owns.

The per-post draft queue carries the assignment alongside the text (`assignVoice`): a create always sends the voice,
an ordinary edit never does, and a reassignment is sent immediately and reported on its own promise — so a title save
that left before the reassignment cannot carry the old voice back over it, and a refused reassignment is taken back
instead of poisoning every later autosave. Details in [draft-autosave](draft-autosave.md).

## Migration rule

Migration `0009_independent_voice_profiles.sql` first creates one default voice per account, records the
user-to-voice mapping, then rebuilds every account-keyed voice table and the posts table with `voice_id`. Backfill is
lossless: text bodies, legacy guidance, protojson snapshots, revision numbers, statuses, timestamps, and durable ids
do not change. Per-user uniqueness is re-expressed per voice only after the backfill is verified.

The migration fails rather than guessing if a row cannot map to exactly one account/default voice (a guard table
asserts no unassigned rows before the swap). It runs through the embedded boot migration path and the single SQLite
writer required by [I7]; its Down collapses only an untouched one-voice-per-account database and refuses to lose
multi-voice data.
