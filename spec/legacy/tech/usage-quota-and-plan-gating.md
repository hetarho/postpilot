# Tech — Credit metering and the balance gate

Why LLM work is metered in **credits held at job start and settled against a persisted ledger**, and why model
access is decided by what an account can afford rather than by a floor declared per model. Decision record for
[plan 17](../plan/17.plan-based-authorization-and-usage-quota.md), **as built by job 37 and re-denominated by
[change 19](../changes/archive/19.credit-metering-and-open-model-access.md) (job 47)**. The currently-true rules
are in [policy/plans.md](../policy/plans.md); this document keeps the reasoning behind them.

Owner in code: `backend/internal/plan` (the ladder + grant table + charge formula + windows + typed refusals),
`backend/internal/usage` (lots + hold/settle + ledger + the metering seam), `backend/internal/auth` (plan on the
user/session, master-only procedure set).

## Why credits rather than USD axes

The three USD axes job 37 shipped (daily starts, daily budget, monthly budget) measured **our provider cost** and
showed it to the user as money. That conflates two different numbers. What we pay a provider moves with model
prices, provider fees and the exchange rate; what a user is entitled to should not. A credit is a product-fixed
$0.01 of list value, and the charge formula is what maps one onto the other:

```
credits = ChargeBase + ceil(actual_cost_usd × 100 × ChargeMultiplier)
```

`ChargeBase` (2) recovers the per-request infrastructure a pure cost multiple cannot see — storage, database,
worker — and is also what keeps a near-free model from being effectively unmetered: without it, a model priced at
$0.045 per million input tokens costs a fraction of a credit per call, and the account is bounded by nothing.
`ChargeMultiplier` (3) covers the provider top-up fee, card fees, VAT and gross margin. Both are single named
constants in code rather than env config, for the same reason the grant table is: two deploys must never disagree
about what a request costs.

The arithmetic is integer-only. A credit is exactly 10 000 micro-USD, the unit the ledger already stores, so no
float ever enters the money path — and the division rounds up, so a call too cheap to reach one credit still costs
`ChargeBase + 1` rather than disappearing.

Collapsing three axes into one also removed the thing that made refusals hard to explain. A user who hit the daily
budget had to be told which of three limits filled and when that particular window reopened. A user out of credits
is told one number and one date.

## Why a hold rather than admission-and-overshoot

Cost is only knowable **after** a call completes, so any scheme that charges afterwards can be surprised. Job 37
accepted that as *bounded overshoot*: a window could exceed its budget by the cost of the jobs in flight when it
filled. That bound turned out to be unbounded in a direction the plan had not modelled — nothing capped how many
photos a post could hold, and observation batches four per call, so a single forty-photo generate job on the most
expensive model could overshoot by nearly three dollars.

The hold closes it. At the enqueue seam the gate prices every call the work will make at its worst case — an
assumed large prompt at the model's input price, plus the completion cap at its output price — and deducts that
many credits before the job row exists. The check and the deduction are one `BEGIN IMMEDIATE` transaction, so two
concurrent starts reading the same balance cannot both pass it.

Settlement is what makes a worst-case hold acceptable to a user. When the job reaches a terminal state the hold is
reconciled against what `usage_events` actually recorded for it, and the remainder is returned within the minute.
An eight-photo Opus post reserves far more than it spends and gets the difference back; the worker is
single-consumer, so holds cannot pile up while that happens.

The refund goes back to **the lots the hold came from**, recorded per hold in `credit_hold_lots`, rather than to
whatever the consumption order is at settle time. By the time a job ends a lot may have expired or a new one may
have opened, and refunding into the wrong lot would quietly move credits between expiry dates — turning a bonus
that was about to lapse into one that does not, or the reverse.

**A balance can never go negative.** That is the guarantee the change exists for, and it does not rest on one
layer: the Go path refuses a hold it cannot cover, and `credit_lots` carries `CHECK (remaining >= 0 AND remaining
<= granted)` with both the spend and the refund statements guarded by the amount available. A job whose real cost
exceeds its own hold takes what the lots can still give and stops — the difference is ours, never a debt the
account carries into next month.

An **exempt tier** (`master`) is held, recorded and settled like everyone else but spends no lot. The recorded
hold is the point: unlimited spend is exactly the account whose spend the operator most wants to be able to read.

## Why lots rather than a counter

A balance could have been two integers on the user row — one monthly, one bonus. Lots are one row per grant, and
they buy three things a pair of counters cannot.

First, **expiry becomes data rather than a rule**. A lot carries the instant it lapses (or `NULL`), a balance is
the sum over the unexpired ones, and no scheduled job has to run for that sum to be correct at a boundary.

Second, **the consumption order falls out of one comparison**. Lots are walked by `expires_at` ascending with
`NULL` last. A monthly lot always carries the earlier expiry, so "spend the monthly grant before the bonus" is not
a second rule anyone has to remember — and a promotional grant with its own deadline needs no new mechanism at
all.

Third, **the grant history stays readable**. A lapsed lot is not deleted; it simply stops counting.

Monthly renewal is computed **on access** rather than by a ticker. The first request after the boundary closes the
stale lot and opens the next inside the same write transaction. A server that was down at midnight, and accounts
whose renewal instants differ, are both correct without a sweeper that has to have run.

## Why the model floor was redundant

`min_plan` declared, per model, the lowest tier allowed to run it — and it was enforced on every ref-accepting RPC
across roughly fifty sites. Once charge is proportional to true cost, it answers a question the balance already
answers, and answers it worse. A `free` account cannot run Opus because 79 credits do not fit in 50; it does not
need a table to say so.

The distinction that mattered for the code around it: a floor refusal was **permanent until upgrade**, which is
why a downgrade-locked selection had to be preserved-not-deleted and reported as missing-but-restorable. A balance
refusal is **temporary until renewal**, so a saved selection is never invalidated by one and needs no preservation
rule at all. Removing the floor removed that whole branch rather than reimplementing it in a new currency.

What replaces it is the hold itself, surfaced for display: `ListModels` reports per model what one call would
require and whether the balance covers it, so a picker can disable and label. That is display only — the server
refuses at the gate whatever the client rendered — and it is computed by the **same estimator** the gate applies,
so what a user is shown and what they are charged cannot be derived two different ways.

## The shape

```
StartGeneration / revise / analyze / A-B          worker goroutine
        │                                              │  ctx carries usage.Work{user, kind, job}
   job.Enqueue ─▶ usage.Hold(user, plan, kind, calls)  ├─▶ metered registry ─▶ llm call
        │            │ BEGIN IMMEDIATE                 │            │
        │            │ renew the monthly lot           │      usage.RecordCall(ctx, ref, usage)
        │            │ price every planned call        │            │
        │            │ spend lots in expiry order      └──────────▶ usage_events row
        │            ▼                                              │
        │     ok → admission + credit_hold_lots, then the job row   │
        │     no → resource_exhausted{INSUFFICIENT_CREDITS}         ▼
        │          required, balance, renews_at — no job row   terminal state
        │                                                           │
        └───────────────────────── usage.Settle(job) ◀──────────────┘
                    Σ usage_events for the job → Charge → refund the remainder
                    to the SAME lots; idempotent on settled_at IS NULL
```

## Why admission-time, not mid-flight

Cost is only known **after** a call completes (the provider reports it, or we estimate from token counts), so a
budget can only ever be checked against what has already been spent. Enforcing at admission gives a simple, honest
guarantee: *no new LLM work starts once a window's budget or start count is exhausted.* The check and the admission
row are one `BEGIN IMMEDIATE` transaction, so the guarantee survives concurrency: two starts that each read the same
window cannot both pass it. The alternative — killing a
running job when the ledger crosses the cap — buys almost nothing (the tokens are already bought) and breaks the
job-queue contract ([I5]) for it. The accepted consequence is **bounded overshoot**: a window can exceed its budget
by at most the cost of the jobs that were already in flight when it filled. With free-tier models at ≤$0.25 per
million output tokens, that bound is far below a cent-scale cap's own granularity of harm.

## Why a ledger, not counters

A per-window counter (e.g. `spent_today` on the user row) is smaller but lies twice: it cannot be re-derived when a
bug miscounts, and it destroys the per-call record that a future paid plan needs for statements and disputes. The
ledger (`usage_events`, one row per LLM call) is the single quota source; windows are `SUM(cost_microusd)` /
`COUNT` over `(user_id, created_at)` — indexed, and at this product's call volume (tens of calls per user-day)
never a hot query. The A/B experiment tables keep their own usage columns for leaderboard analytics; the ledger
row is written **as well**, so quota math never joins experiment internals.

Two tables, two axes, deliberately:

- `usage_admissions` — one row per **admitted job start**, written in the same transaction that evaluates the
  axes. The daily-count axis counts admissions, so a refused start consumes nothing, a fan-out A/B comparison
  consumes exactly one, and a job that later fails still consumed its start (it may have spent tokens). It is also
  the one row this context ever deletes: an admission whose job row then failed to insert is released, on a context
  that outlives the caller's cancellation, so a start that never happened is not charged.
- `usage_events` — one row per **LLM call**, written at the **llm boundary** rather than at each call site. The
  registry every context above the port is given is wrapped once in the composition root, and the worker stamps the
  job's identity on the handler's context, so "every server-side LLM call is on the ledger" is a structural fact
  instead of a rule twelve call sites have to keep remembering — and a new call site is metered the day it is
  written. Failed calls whose usage the provider reported are included (job 23 preserves those). The budget axes sum
  these. The row also records the provider-reported **reasoning token count** (0 means not reported, like the other
  usage fields), so "wrote 8,192 tokens of post" and "spent 8,192 tokens thinking and wrote nothing" stop being the
  same row — the 2026-09-03 failures had to be inferred from an empty body. It is recorded for diagnosis and is
  deliberately not re-priced.
- `stage` is now the stage the CALL named for itself, in the llm boundary's stable form
  (`observe`/`write`/`analyze`), falling back to the old ref-comparison inference for a call that names none. It used
  to be as specific as the refs distinguished — the job kind, except for a generate job's observation call, and not
  even then when both stages ran the same model — which is exactly the ambiguity a per-(model, stage) aggregate
  cannot tolerate. Nothing read the column back before, so widening its meaning changed no reader. That aggregate
  is what tells an operator a model is ignoring its reasoning effort, per purpose, before it fails a user's job.

Cost resolution reuses the experiment precedence: provider-**reported** cost wins; otherwise **estimated** from
the registry's per-million prices; otherwise **unavailable** (recorded as 0 cost — today only `openrouter/free`
is unpriced, and it is free). The resolver now lives in `internal/llm`, which owns the prices; `experiment` maps its
own types onto it, so the leaderboard and the ledger can never price the same call differently.

## Why windows are calendar Asia/Seoul

Rolling windows ("last 24h") make refusals unexplainable — the reset instant is a moving target that depends on
per-call history. Calendar windows in the product's home timezone give every refusal a fixed, printable reset
("자정에 초기화", "9월 1일에 초기화") and make the daily limit mean what a user thinks it means. The zone is a
product constant, not env config: two deploys must never disagree about when a day ends. It is a fixed UTC+9 zone
rather than a tzdata lookup: the runtime image is distroless/static and ships no zoneinfo, and KST has had no DST
since 1988, so the two agree on every instant this product will ever see.

## Why `min_plan` lives in the registry

Model access is a property of the **model** (its price class), not of the user, so it is declared once per model
in `config/providers.yaml` as an ordered floor (`min_plan: free|basic|max`) and validated at boot like every other
registry field. The alternatives fail predictably: an allowlist per plan in code drifts from the YAML when models
are added; an allowlist per user reintroduces the merged-state problem the ladder exists to avoid. The registry
stays user-ignorant — comparison happens in callers that already know the acting plan. Enforcement is server-side
on **every ref-accepting RPC** (job start, save selection, save comparison pair, apply recommendation set);
`ListModels.locked` exists only so the UI can disable and label, never as the gate. "Job start" means every ref the
work will actually run, not only the ones the job row records: a write comparison's observe model, and the stored
refs an experiment or profile-validation **retry** re-runs, ride the same gate — otherwise a downgrade would be
escapable by retrying yesterday's work.

A plan-locked **saved selection** is reported missing/locked but its row is preserved (unlike vanished or
capability-unsuitable refs, which are deleted): a downgrade is reversible state, not a broken pointer, and an
upgrade must restore the user's choices untouched.

## Why master is a plan, not a flag

`master` rides the same ladder as the paying tiers, so every gate answers one question ("is the acting plan ≥ the
floor?") and the authorization surface stays one mechanism instead of two (tier + role). Master skips the numeric
axes but still writes admissions and events — unlimited spend is exactly the account whose spend the operator most
wants visible. The master-only procedures (the nine human `PublishingService` RPCs, `ListUsers`, `SetUserPlan`)
are a closed-by-default set in the auth interceptor, the same pattern as `publicProcedures` and `agentProcedures`;
the last-master guard makes the ladder unable to lock everyone out. That guard lives inside the UPDATE statement
rather than in a count taken before it: two concurrent demotions would each see two masters and both commit.

## Rules

- The gate never chooses or substitutes a model ([I3]); it only refuses an explicit choice below its floor.
- Quota refusals are `resource_exhausted`, gate refusals `permission_denied`, always with a typed detail
  (`reason`, `limit`, `used`, `resets_at`, offending ref) — the FE renders copy from the detail, never from the
  message string. The reasons follow the product's existing UPPER_SNAKE spelling (`DAILY_COUNT`, `DAILY_COST`,
  `MONTHLY_COST`, `MODEL_LOCKED`, `MASTER_ONLY`); a mixed-case reason namespace would be a second convention in one
  allowlist.
- The detail carries **machine values** — micro-USD integers, RFC3339 instants, the stored tier name — and the
  browser turns them into the reader's own notation through i18next formatters. A server that formatted money or a
  timestamp would be guessing at a locale it cannot see.
- The ledger is append-only from the product's point of view; nothing edits or deletes usage rows.
- Plan limits are a code-owned table surfaced through `GetMyPlan`; the FE never hardcodes a number from it.
