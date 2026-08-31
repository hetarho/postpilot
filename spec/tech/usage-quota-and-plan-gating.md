# Tech — Usage quotas and plan gating

Why plan limits are enforced at **job admission** against a persisted **usage ledger**, and why model access is a
declared floor in the registry rather than a list kept beside the user. Decision record for
[plan 17](../plan/17.plan-based-authorization-and-usage-quota.md) (status: planning — this document describes the
design the plan commits to, and becomes as-built when its job lands).

Owner in code (once built): `backend/internal/plan` (the ladder + limits table), `backend/internal/usage`
(ledger + admission), `backend/internal/auth` (plan on the user/session, master-only procedure set).

## The shape

```
StartGeneration / revise / analyze / A-B          worker goroutine
        │                                              │
   enqueue tx ──▶ usage.Admit(user, plan, kind)        ├─▶ llm call ──▶ llm.Usage
        │            │ count axis: usage_admissions    │                  │
        │            │ cost axes:  Σ usage_events      │        usage.Record(event)
        │            ▼                                 │                  │
        │     ok → admission row + job row             └──────────▶ usage_events row
        │     no → resource_exhausted{reason, limit, used, resets_at}, no job row
```

## Why admission-time, not mid-flight

Cost is only known **after** a call completes (the provider reports it, or we estimate from token counts), so a
budget can only ever be checked against what has already been spent. Enforcing at admission gives a simple, honest
guarantee: *no new LLM work starts once a window's budget or start count is exhausted.* The alternative — killing a
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

- `usage_admissions` — one row per **admitted job start**, written in the enqueue transaction. The daily-count
  axis counts admissions, so a refused start consumes nothing, a fan-out A/B comparison consumes exactly one, and
  a job that later fails still consumed its start (it may have spent tokens).
- `usage_events` — one row per **LLM call**, written where `llm.Usage` surfaces (including failed calls whose
  usage the provider reported — job 23 preserves those). The budget axes sum these.

Cost resolution reuses the experiment precedence: provider-**reported** cost wins; otherwise **estimated** from
the registry's per-million prices; otherwise **unavailable** (recorded as 0 cost — today only `openrouter/free`
is unpriced, and it is free). The resolver moves to a place both `experiment` and `usage` can share.

## Why windows are calendar Asia/Seoul

Rolling windows ("last 24h") make refusals unexplainable — the reset instant is a moving target that depends on
per-call history. Calendar windows in the product's home timezone give every refusal a fixed, printable reset
("자정에 초기화", "9월 1일에 초기화") and make the daily limit mean what a user thinks it means. The zone is a
product constant, not env config: two deploys must never disagree about when a day ends.

## Why `min_plan` lives in the registry

Model access is a property of the **model** (its price class), not of the user, so it is declared once per model
in `config/providers.yaml` as an ordered floor (`min_plan: free|basic|max`) and validated at boot like every other
registry field. The alternatives fail predictably: an allowlist per plan in code drifts from the YAML when models
are added; an allowlist per user reintroduces the merged-state problem the ladder exists to avoid. The registry
stays user-ignorant — comparison happens in callers that already know the acting plan. Enforcement is server-side
on **every ref-accepting RPC** (job start, save selection, save comparison pair, apply recommendation set);
`ListModels.locked` exists only so the UI can disable and label, never as the gate.

A plan-locked **saved selection** is reported missing/locked but its row is preserved (unlike vanished or
capability-unsuitable refs, which are deleted): a downgrade is reversible state, not a broken pointer, and an
upgrade must restore the user's choices untouched.

## Why master is a plan, not a flag

`master` rides the same ladder as the paying tiers, so every gate answers one question ("is the acting plan ≥ the
floor?") and the authorization surface stays one mechanism instead of two (tier + role). Master skips the numeric
axes but still writes admissions and events — unlimited spend is exactly the account whose spend the operator most
wants visible. The master-only procedures (the nine human `PublishingService` RPCs, `ListUsers`, `SetUserPlan`)
are a closed-by-default set in the auth interceptor, the same pattern as `publicProcedures` and `agentProcedures`;
the last-master guard makes the ladder unable to lock everyone out.

## Rules

- The gate never chooses or substitutes a model ([I3]); it only refuses an explicit choice below its floor.
- Quota refusals are `resource_exhausted`, gate refusals `permission_denied`, always with a typed detail
  (`reason`, `limit`, `used`, `resets_at`, offending ref) — the FE renders copy from the detail, never from the
  message string.
- The ledger is append-only from the product's point of view; nothing edits or deletes usage rows.
- Plan limits are a code-owned table surfaced through `GetMyPlan`; the FE never hardcodes a number from it.
