# Policy — Plans, credits, and the operator tier

Canonical rules that are **currently true** in the code. Source:
[plan/17](../plan/17.plan-based-authorization-and-usage-quota.md), built by job 37 and re-denominated in credits by
[change 19](../changes/archive/19.credit-metering-and-open-model-access.md) (job 47). Decision record:
[tech/usage-quota-and-plan-gating](../tech/usage-quota-and-plan-gating.md). A change to any rule here is a change to
shipped behavior — go through `/create-change`.

## The ladder

- Every account carries exactly one plan: `free`, `basic`, `pro`, `max`, or `master`. It is a column on `users`
  with a `CHECK`, so a value off the ladder cannot be stored.
- A tier decides exactly **two** things: how many credits it is granted each month, and — for `master` alone —
  access to the master-only publishing and admin procedures. **Which models an account may run is not one of
  them.**
- **`free` is the provisioning default.** `api adduser <id>` creates a free account; `--plan=<tier>` (in either
  argument order) provisions another tier.
- Only `master` changes a plan — `AdminService.SetUserPlan` (master-only) or `api setplan <id> <tier>` on the box.
  A tier change re-sizes the **next** monthly lot; the current one is never edited retroactively.
- **The last master cannot be demoted**, on either path. The guard is part of the UPDATE statement, not a count
  taken before it, so two concurrent demotions cannot both commit and leave the deployment with no operator.
- The acting plan is resolved **once per request**, on the session path, from the row.

## Credits

- A credit is a product-fixed **$0.01 of list value**, stored as an integer. It is a billing unit, not a cost
  measurement: the ledger keeps recording true provider cost in micro-USD underneath, and the two never reconcile.

| Plan     | Monthly credits | Intended monthly price | Master surfaces |
| -------- | --------------- | ---------------------- | --------------- |
| `free`   | 50              | —                      | —               |
| `basic`  | 200             | $2                     | —               |
| `pro`    | 500             | $5                     | —               |
| `max`    | 1000            | $10                    | —               |
| `master` | unlimited       | —                      | ✅              |

- A `free` account is additionally provisioned with a **50-credit signup bonus** that does not expire. Its lot id
  is derived from the account, so re-running `adduser` to repair an account cannot mint a second one.
- The prices are the figures the grants were sized against and are published through `GetMyPlan` so a comparison
  screen can name them. **No money moves** (PRD §9): there is no checkout, no PG, no subscription lifecycle.
- Every number above is a **code-owned product rule** in `backend/internal/plan` — not env config, so two deploys
  can never disagree — and is published to the client. The frontend hardcodes none of them.

## What a request costs

```
credits = ChargeBase + ceil(actual_cost_usd × 100 × ChargeMultiplier)     ChargeBase = 2, ChargeMultiplier = 3
```

- `ChargeBase` recovers the per-request infrastructure a pure cost multiple cannot see and keeps a near-free model
  from being effectively unmetered. `ChargeMultiplier` covers the provider top-up fee, card fees, VAT and margin.
- The arithmetic is **integer-only**: a credit is exactly 10 000 micro-USD, so no float enters the money path. The
  division rounds up, so a call too cheap to reach one credit still costs `ChargeBase + 1`.
- A call the registry has no price for resolves to zero cost, and so to `ChargeBase` alone. Today that is exactly
  one model, and it is free.

## Lots

- A balance is **lots**, not a counter: one `credit_lots` row per grant, carrying what was granted, what remains,
  and when it expires (`NULL` for a grant that does not).
- Consumption walks lots by **`expires_at` ascending, NULLs last**, then by creation. Because a monthly lot always
  expires before a non-expiring bonus, "spend the monthly grant first" follows from that one ordering rather than
  existing as a second rule.
- Expiry is a **read-time predicate**: a lapsed lot contributes nothing to a balance, so no scheduled job has to
  run for a balance to be correct.
- Monthly renewal is **computed on access**. A request that finds the account's monthly lot lapsed opens the
  tier's next one inside the same write transaction. A lapsed lot's remainder does **not** carry over, and the
  lapsed row is kept rather than deleted so the grant history stays readable.
- The `remaining` column carries `CHECK (remaining >= 0 AND remaining <= granted)`, and both the spend and the
  refund statements are guarded by the amount available. The no-negative-balance guarantee does not rest on Go
  alone.

## Holds and settlement

- Every LLM-consuming job start — `generate`, `revise`, `analyze_voice`, `learn_voice`, `compare_voice_rule`,
  `validate_voice_profile`, `model_experiment` — passes one gate at the shared enqueue seam.
- **Hold.** The gate prices every call the work will make at its worst case (an assumed 30 000-token prompt at the
  model's input price, plus `LLM_MAX_TOKENS_DEFAULT` at its output price), runs the total through the charge
  formula, and deducts it from the lots in consumption order — inside one `BEGIN IMMEDIATE` transaction with the
  admission row. The call count is stated by the enqueuing service, because only it knows that observation batches
  four photos per call or that a validation repeats its stages per sampled post.
- **Settle.** When the job reaches a terminal state, the hold is reconciled against what `usage_events` actually
  recorded for it and the difference is returned **to the lots it came from** — not to the current consumption
  order, which may have changed. Settlement is idempotent on `settled_at IS NULL`.
- A job whose actual cost **exceeds** its hold takes what the lots can still give and stops. The balance floor is
  absolute: the difference is ours, never a debt the account carries forward.
- An **exempt tier** (`master`) is held, recorded and settled like everyone else, but spends no lot and is never
  refused. Its recorded hold is what keeps the operator's own spend readable.
- A hold whose job row then failed to insert is **released** in full, on a context that outlives the caller's
  cancellation. A boot sweep settles any hold left open behind an already-terminal job.
- Refusals are `resource_exhausted` with reason `INSUFFICIENT_CREDITS`, carrying `required`, `balance` and
  `renews_at` (RFC3339). A refusal writes nothing and **creates no job row**. The client renders copy from the
  reason, never from the message.

## Model access

- **There is no plan floor.** `min_plan` does not exist in the registry, in `config/providers.yaml`, in boot
  validation, or on any RPC. Every account may select, save and run every registered model.
- The only access rule is affordability, and it is the hold itself. `ListModels` reports, per model and for the
  calling account, the credits one call of it would require and whether the balance covers it, so a picker can
  disable it and name the number. That listing is display only — the server refuses at the gate whatever the
  client rendered.
- An unaffordable model is **temporary state**, not a broken pointer: a saved selection is never invalidated,
  cleared, or reported missing because of a balance. Only a model that has actually vanished or become unsuitable
  for its stage is cleared.

## The ledger

- Every server-side LLM call writes one `usage_events` row: token counts, `cost_microusd`, and `cost_source`
  resolved **reported → estimated → unavailable**. A failed call is recorded when the provider reported usage
  (those tokens were bought) and skipped when it reported none.
- Recording happens at the **llm boundary**, not at each call site: the model registry every context is given is
  wrapped so a call made anywhere lands on the ledger, attributed to the job the worker stamped on the context.
- The ledger's `stage` is the job kind, except that a generate job's photo-observation call is recorded as
  `observe` — and not even then when both stages ran the same model, which the ref cannot distinguish.
- A/B candidate calls appear in **both** the experiment tables and the ledger; credit math never joins experiment
  internals.
- `usage_events` is append-only from the product's point of view. Rows are kept indefinitely (billing groundwork).

## Photo ceiling

- A post holds at most **30 photos** (`UPLOAD_MAX_PHOTOS_PER_POST`, in both config owners). The server refuses the
  upload that would cross it, and the browser's selection gate reports the excess as skipped before any file is
  decoded ([I6]).
- It exists for the hold, not for storage: observation batches photos, so the calls a generate job makes — and the
  credits it must reserve before starting — grow with the photo count. Without a ceiling the worst case a hold has
  to price is unbounded.

## Master-only surfaces

- The nine human `PublishingService` procedures and the two `AdminService` procedures are a closed set in the auth
  interceptor (`masterProcedures`), refused `permission_denied` with reason `MASTER_ONLY`. A new master-only
  procedure must be added to that set in the same change that adds it to the proto.
- The `PublishingAgentService` bearer path is untouched: a device capability is not a human session
  ([auth.md](auth.md)).
- The frontend hides what the server would refuse — the publishing nav entry, the publish panel, and `/admin` — and
  redirects a non-master away from `/publishing-agents` and `/admin`. That is UX only; the server remains
  authoritative.

## Self-visibility

- `GetMyPlan` returns the caller's tier, the balance, the individual lots with their expiries, the renewal instant,
  the tier's monthly grant, and every rung on offer with its grant and intended price. Reading a balance also
  renews it, so a client polling at a month boundary sees the new grant rather than a lapsed one.
- `GetMe` and `Login` both carry the tier, so the app gates master-only surfaces on its first paint without a
  second round-trip.
- The app shell shows a plan badge; opening it fetches the balance and shows one meter, the renewal instant, the
  lots behind the total, and the statement that plans are operator-assigned.
- `/plans` lists the four offered tiers with their grants and intended prices. Each tier's action is the seam a
  checkout will later attach to; today it states that plans are operator-assigned. **There is no upgrade, purchase,
  or checkout affordance anywhere** (PRD §9).
- A zero balance blocks every LLM-consuming action and nothing else: writing, editing, exporting and reading posts
  keep working.
