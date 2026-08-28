# Policy — Durable generation jobs

Canonical rules that are **currently true** in the code. Source: [plan/05](../plan/05.generation-job-queue.md),
built by job 07. The concrete `analyze_voice`, `generate`, and `revise` handlers are registered by their owning
feature jobs; this document owns the shared record, worker, query, and polling contract.

## Record and lifecycle

- Long model work is represented by a durable `generation_jobs` row. Enqueue writes `queued` and returns its id
  without running the handler in the request; the API process's worker later moves it through
  `queued → running → done | failed` ([I5]). Terminal rows are never reopened; retrying creates a new row.
- The row records its owner, optional post target, kind, stage, exact progress, user-facing error, selected observe
  and write models, kind-specific JSON payload, and created/updated/started/finished timestamps.
- `post_slug` is nullable because `analyze_voice` belongs to an account rather than a post. Model references are
  stored as `provider_id/model_id`; payload is stored as text.
- One non-terminal job may target a post, regardless of kind. One non-terminal account-scoped job may exist for a
  `(user_id, kind)` pair. Partial unique indexes enforce both rules at the database boundary, closing concurrent
  enqueue races; `ErrAlreadyInProgress` carries the durable active id the caller should attach to.
- A post-targeted row has a composite foreign key to `(posts.slug, posts.user_id)`, and enqueue's guard lookup is
  owner-scoped. A job therefore cannot attach one account's work or status to another account's post, even if a
  future caller forgets its own ownership precheck.

## Worker and restart behavior

- The worker runs inside the API process with concurrency `1`. Enqueue sends a best-effort in-process wake signal;
  a 1 s fallback poll guarantees a missed signal does not strand a row. The oldest queued row is picked first.
- Picking is one `UPDATE … RETURNING` write that marks the row `running`. Every progress callback is a separate
  short write through the serialized SQLite writer. No job-layer transaction spans a handler or provider call.
- Handler success becomes `done`. A returned error becomes `failed` with a non-empty user-facing message. A
  normalized `llm.ProviderError` preserves the provider's own message verbatim. A panic is recovered into the fixed
  generic Korean reason, and the worker continues to the next row.
- Graceful shutdown stops the worker before HTTP shutdown. A handler that returns cancellation leaves its row
  `running`; a handler that already produced success or an ordinary failure gets one bounded terminal write even if
  shutdown raced its return. The next boot sweep changes every still-running row to `failed` with
  `서버가 재시작되어 작업이 중단됐어요. 다시 시도해 주세요.` before a new worker starts.
- There is no automatic retry, cancellation, separate worker process, external queue, job-history RPC, or partial
  text streaming.

## Ownership and published read behavior

- `GetGeneration(id)` is authenticated and takes the acting user only from the session context. An existing job
  owned by another account is `PermissionDenied` (403); an unknown id is `NotFound` (404).
- `Post.active_job` and `PostSummary.active_job` contain the post's queued/running job, if any. The post context asks
  the job context through its consumer-owned `ActiveJobFinder` port; it never reads `generation_jobs` itself.
- The post list renders an active row as `생성 중`. Reopening a post recovers the durable id from `active_job`, so
  polling does not depend on component memory.

## Browser polling and feedback

- `useJob` asks `GetGeneration` every `POLL_INTERVAL_MS` (2 s) while status is `queued` or `running`, and stops after
  `done` or `failed`. On `done`, it invalidates owner query keys supplied by the caller so completed content reloads.
- Stage copy is fixed: `observe` → `사진 {done}/{total} 관찰됨`, `write` → `작성 중`, `analyze` → `문체 분석 중`.
  A failed row exposes its stored error. `FailureNotice` delegates retry behavior to an `onRetry` callback supplied
  by the owning feature; the shared queue does not invent retries.
- Progress and error feedback use live-region semantics. Generic controls come from `shared/ui`, and the feedback
  surfaces use the design system's semantic notice roles.

## Configuration

| Value | Owner | Value |
|---|---|---|
| `WorkerConcurrency` | BE `internal/platform/config` | `1` |
| `WorkerPollInterval` | BE `internal/platform/config` | `1s` |
| `POLL_INTERVAL_MS` | FE `shared/config` | `2000` |
| restart and panic copy | BE `internal/job/messages.go` | fixed Korean user-facing strings |
