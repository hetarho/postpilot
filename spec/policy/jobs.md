# Policy — Durable generation jobs

Canonical rules that are **currently true** in the code. Source: [plan/05](../plan/05.generation-job-queue.md),
built by job 07; per-voice ownership from [plan/10](../plan/10.independent-voice-profiles-and-post-assi.md), job 18.
The concrete `analyze_voice`, `generate`, `revise`, and `model_experiment` handlers are registered by their owning
feature jobs; job 32 adds frozen target and structured failure fields. This document owns the shared record, worker,
query, and polling contract.

## Record and lifecycle

- Long model work is represented by a durable `generation_jobs` row. Enqueue writes `queued` and returns its id
  without running the handler in the request; the API process's worker later moves it through
  `queued → running → done | failed` ([I5]). Terminal rows are never reopened; retrying creates a new row.
- The row records its owner, optional post target, kind, stage, exact progress, optional frozen target language,
  structured `reason + params + technical_detail` failure, selected observe and write models, kind-specific JSON
  payload, and created/updated/started/finished timestamps. Deprecated raw error text is read only for legacy rows.
- `model_experiment` payload is the experiment id. Its progress stages are `observe`, `compare_write`,
  `compare_observe`, and `compare_analyze`; candidate completion writes one monotonic compare counter.
- `post_slug` is nullable because some voice-owned work has no post target. Learning and rule-comparison rows may
  retain the post that supplied their source while still belonging to a voice. `voice_id` is nullable only for work
  that neither reads nor publishes voice-owned state (for example an observe comparison); generation, revision,
  write comparison, and personalization rows freeze the relevant post or explicit target voice. Model references
  are stored as `provider_id/model_id`; payload is stored as text.
- One non-terminal job may target a post, regardless of kind. One non-terminal **voice-owned** job may exist for a
  `(voice_id, kind)` pair, including learning/comparison rows that also target a post, so those rows satisfy both
  guards. Two voices of one account can analyze or learn independently; a row with neither post nor voice keeps the
  older `(user_id, kind)` guard. Partial unique indexes enforce the post/account guards. A `BEFORE INSERT` trigger
  enforces the voice guard so a lossless upgrade can retain pre-0009 active learning rows for different posts without
  rewriting their durable statuses; SQLite's single writer closes the same concurrent-enqueue race. New work sees
  every retained row through the guard query and trigger. `ErrAlreadyInProgress` carries the durable active id the
  caller should attach to. The voice context asks `HasActiveForVoice` before a soft delete; the job context never
  reads voice tables.
- A post-targeted row has a composite foreign key to `(posts.slug, posts.user_id)`, and enqueue's guard lookup is
  owner-scoped. A job therefore cannot attach one account's work or status to another account's post, even if a
  future caller forgets its own ownership precheck.

## Worker and restart behavior

- The worker runs inside the API process with concurrency `1`. Enqueue sends a best-effort in-process wake signal;
  a 1 s fallback poll guarantees a missed signal does not strand a row. The oldest queued row is picked first.
- Picking is one `UPDATE … RETURNING` write that marks the row `running`. Every progress callback is a separate
  short write through the serialized SQLite writer. No job-layer transaction spans a handler or provider call.
- Handler success becomes `done`. A returned error becomes `failed` with a stable owned reason and allowlisted params.
  A normalized `llm.ProviderError` maps to a model reason while optional provider prose remains technical detail. A
  panic becomes `JOB_PANICKED`, and the worker continues to the next row.
- Graceful shutdown stops the worker before HTTP shutdown. A handler that returns cancellation leaves its row
  `running`; a handler that already produced success or an ordinary failure gets one bounded terminal write even if
  shutdown raced its return. The next boot sweep changes every still-running row to `failed` with
  `JOB_INTERRUPTED` before a new worker starts.
- After the job sweep, experiment boot recovery reconciles its own lifecycle: interrupted running work becomes
  partial/failed, and an orphan queued experiment is failed only if no runnable experiment job remains.
- There is no automatic retry, cancellation, separate worker process, external queue, job-history RPC, or partial
  text streaming.

### Personalization restart exception

- `learn_voice`, `compare_voice_rule`, and `validate_voice_profile` come only from explicit authenticated actions
  carrying model refs. At boot, queued rows of these kinds are marked `failed` before the worker starts; boot cannot
  turn old user intent into a new provider call. Running rows use the ordinary restart sweep.
- Their owning aggregate exposes failure and explicit retry. A failed aggregate/job link can compensate by failing
  that id for that owner only while it is queued. If the worker already owns it, compensation cannot cancel it and
  the original durable ids remain the recovery path.
- No timer, scheduler, profile age, read RPC, export, copy, or polling operation creates these kinds.

## Ownership and published read behavior

- `GetGeneration(id)` is authenticated and takes the acting user only from the session context. An existing job
  owned by another account is `PermissionDenied` (403); an unknown id is `NotFound` (404).
- `Post.active_job` and `PostSummary.active_job` contain the post's queued/running job, if any. The post context asks
  the job context through its consumer-owned `ActiveJobFinder` port; it never reads `generation_jobs` itself. The same
  port refuses a voice reassignment while a job targets the post.
- The post list renders an active row as `생성 중`. Reopening a post recovers the durable id from `active_job`, so
  polling does not depend on component memory.
- Post lists poll only while at least one returned post has an active job. They stop after the server projection
  switches to a pending experiment review id.

## Browser polling and feedback

- `useJob` asks `GetGeneration` every `POLL_INTERVAL_MS` (2 s) while status is `queued` or `running`, and stops after
  `done` or `failed`. On `done`, it invalidates owner query keys supplied by the caller so completed content reloads.
- Stage labels and progress templates are catalog keys rendered in the active UI locale. A failed row exposes its
  structured failure; an unknown/malformed/legacy value becomes localized `UNKNOWN_FAILURE` and never raw text.
  `FailureNotice` delegates retry behavior to an `onRetry` callback supplied
  by the owning feature; the shared queue does not invent retries.
- Progress and error feedback use live-region semantics. Generic controls come from `shared/ui`, and the feedback
  surfaces use the design system's semantic notice roles.

## Configuration

| Value | Owner | Value |
|---|---|---|
| `WorkerConcurrency` | BE `internal/platform/config` | `1` |
| `WorkerPollInterval` | BE `internal/platform/config` | `1s` |
| `POLL_INTERVAL_MS` | FE `shared/config` | `2000` |
| restart/panic reasons | BE `internal/job` | `JOB_INTERRUPTED` / `JOB_PANICKED` |
