# Policy — Plans, quotas, and the operator tier

Canonical rules that are **currently true** in the code. Source:
[plan/17](../plan/17.plan-based-authorization-and-usage-quota.md), built by job 37. Decision record:
[tech/usage-quota-and-plan-gating](../tech/usage-quota-and-plan-gating.md). A change to any rule here is a change to
shipped behavior — go through `/create-change`.

## The ladder

- Every account carries exactly one plan: `free`, `basic`, `max`, or `master`, ordered `free < basic < max < master`.
  It is a column on `users` with a `CHECK`, so a value off the ladder cannot be stored.
- **`free` is the provisioning default.** `api adduser <id>` creates a free account; `--plan=<tier>` (in either
  argument order) provisions another tier. A command that handed out unlimited spend by omission is the failure this
  ladder exists to prevent.
- Migration `0013` backfilled every account that existed before it to `master`: those are the operator's own
  accounts, and nothing about their authority regressed on that deploy.
- Only `master` changes a plan — `AdminService.SetUserPlan` (master-only) or `api setplan <id> <tier>` on the box.
- **The last master cannot be demoted**, on either path. The guard is part of the UPDATE statement, not a count taken
  before it, so two concurrent demotions cannot both commit and leave the deployment with no operator.
- The acting plan is resolved **once per request**, on the session path, from the row — so a promotion or demotion
  takes effect on the caller's very next call rather than when their session happens to expire.

## What a plan bounds

Three independent numeric axes, plus a model floor and two master-only surfaces. Every number is a **code-owned
product rule** in `backend/internal/plan` — not env config, so two deploys can never disagree — and is published to
the client through `GetMyPlan`. The frontend never hardcodes one.

| Plan     | Daily LLM-job starts | Daily budget | Monthly budget | Models                   | Publishing | Admin |
| -------- | -------------------- | ------------ | -------------- | ------------------------ | ---------- | ----- |
| `free`   | 10                   | $0.10        | $2.00          | `min_plan: free` only    | —          | —     |
| `basic`  | 30                   | $0.50        | $12.00         | `min_plan: free · basic` | —          | —     |
| `max`    | 100                  | $1.00        | $25.00         | all registry models      | —          | —     |
| `master` | unlimited            | unlimited    | unlimited      | all registry models      | ✅         | ✅    |

- Budgets are stored and compared in **micro-USD** (`int64`). Zero on any limit means *unlimited*, never a zero
  allowance — only `master` carries zeros.
- Windows are **calendar day and calendar month in `Asia/Seoul`**, a product constant (a fixed UTC+9 zone, since the
  runtime image ships no zoneinfo and KST has had no DST since 1988). Every refusal names the instant it lifts.
- An unknown plan value gets the **strictest** known tier's limits and satisfies no floor: the ladder fails closed at
  both ends.

## Admission

- Every LLM-consuming job start — `generate`, `revise`, `analyze_voice`, `learn_voice`, `compare_voice_rule`,
  `validate_voice_profile`, `model_experiment` — passes one gate at the shared enqueue seam.
- The gate asks, in order: **may this tier run these models**, then the daily start count, then today's spend, then
  the month's. A refusal writes nothing and **creates no job row**.
- Refusals are `resource_exhausted` with reason `DAILY_COUNT` / `DAILY_COST` / `MONTHLY_COST`, carrying `limit`,
  `used`, and `resets_at` (RFC3339). A model refusal is `permission_denied` with reason `MODEL_LOCKED`, carrying
  `model`, `models`, and `required_plan`. The client renders copy from the reason, never from the message.
- The count axis counts **admissions**, so a refused start consumes nothing, an A/B comparison that fans out to two
  candidates consumes exactly one, and a job that later fails still consumed the start it may have spent tokens on.
- The check and the admission row are **one transaction** (`BEGIN IMMEDIATE`): two concurrent starts that each read
  the same window cannot both be admitted past the limit.
- An admission whose job row then fails to insert is **released**, on a context that outlives the caller's
  cancellation. This is the only deletion the context performs.
- `master` skips the numeric axes but its admissions and events are still recorded — unlimited spend is exactly the
  account whose spend the operator most wants to be able to read.
- Enforcement is at admission only. **Bounded overshoot is accepted**: a window may exceed its budget by the cost of
  the jobs already in flight when it filled. Nothing is killed mid-flight ([I5]).

## The ledger

- Every server-side LLM call writes one `usage_events` row: token counts, `cost_microusd`, and `cost_source`
  resolved **reported → estimated → unavailable**. A failed call is recorded when the provider reported usage
  (those tokens were bought) and skipped when it reported none.
- Recording happens at the **llm boundary**, not at each call site: the model registry every context is given is
  wrapped so a call made anywhere lands on the ledger, attributed to the job the worker stamped on the context.
- The ledger's `stage` is the job kind, except that a generate job's photo-observation call is recorded as
  `observe` — and not even then when both stages ran the same model, which the ref cannot distinguish.
- A/B candidate calls appear in **both** the experiment tables and the ledger; quota math never joins experiment
  internals.
- `usage_events` is append-only from the product's point of view. Rows are kept indefinitely (billing groundwork).

## Model floors

- Access is a property of the **model**, declared once per model in `config/providers.yaml` as
  `min_plan: free|basic|max`. It is **required**: a missing or unknown value stops the API from starting, the same
  posture as a bad migration.
- Current assignment — free: `openrouter/free`, `deepseek/deepseek-v4-flash-0731`, `z-ai/glm-5.3-flash`,
  `qwen/qwen3.8-flash` · basic: `openai/gpt-5.6-luna`, `google/gemini-3.7-flash`, `deepseek/deepseek-v4-pro-0813`,
  `z-ai/glm-5.3` · max: `x-ai/grok-4.6`, `anthropic/claude-sonnet-5`, `openai/gpt-5.6-sol`,
  `anthropic/claude-opus-5`.
- The registry stays **user-ignorant**: it carries the floor, and the comparison happens in callers that already
  know the acting plan.
- The server enforces the floor on **every ref-accepting RPC** — starting a job (including a write comparison's
  observe model, and the stored refs an experiment or validation **retry** re-runs), saving a selection, saving a
  comparison pair, applying a recommendation set. `ListModels.locked` exists only so the UI can disable and label.
- A recommendation set is applied whole, so applying one reports **every** offending selection at once, with the
  highest floor among them as the required tier. The shipped `balanced-2026-08` set names `max` models, so it is
  refused below `max` and the frontend disables its action with that reason.
- A saved selection that a **downgrade** locked is reported to the client as missing but its **row is preserved**,
  and an upgrade restores it with no re-selection. Vanished and capability-unsuitable rows are still deleted.

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

- `GetMyPlan` returns the caller's tier, all three limits (0 = unlimited), live usage, and both reset instants.
- `GetMe` and `Login` both carry the tier, so the app gates master-only surfaces on its first paint without a second
  round-trip.
- The app shell shows a plan badge; opening it fetches the usage and shows the three meters, the reset instants, and
  the statement that plans are operator-assigned. There is no upgrade, purchase, or checkout affordance anywhere
  (PRD §9 — no money moves).
