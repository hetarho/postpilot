# Tech — Model experiment methodology

The comparison contract behind [plan 09](../plan/09.stage-model-experiments-and-leaderboards.md): how a pair is kept
fair, blind, attributable, rankable and cheap enough for a two-user product. This is a technical decision, not a
claim that a handful of personal votes is statistically significant.

## 1. Isolate one stage

A verdict changes exactly one variable:

- **observe** — identical ordered photos and observation schema; no writing call;
- **analyze** — identical voice-corpus snapshot and analysis prompt; no profile mutation;
- **write** — identical observations, memo, filenames and voice-profile snapshot.

Normal post generation observes once and fans out only at write. Fanning out both observe and write would either need
four drafts (`2 × 2`) or pause a server job for human input. Both make the verdict ambiguous or the UX fragile.

## 2. Freeze and identify the input

Before enqueue, the stage owner produces a canonical snapshot and prompt-version string. Experiment stores SHA-256 of
that canonical representation. Both candidates receive the same bytes/schema/options except for `ModelRef`.

The snapshot is also the retry boundary. A failed candidate may retry only against the original snapshot. If a photo
object no longer exists or the snapshot cannot be reconstructed, the comparison is no longer fair and must restart.
No provider call runs inside a SQLite write transaction.

Experiment start and the shared job queue are separate context writes. Boot recovery therefore reconciles them:
`running` experiments become retryable failures after the job sweep, while a `queued` experiment is failed only when
the job context confirms that no runnable `model_experiment` job names its id. This preserves durable queued work and
repairs the narrow crash window before enqueue/link.

## 3. Blind on the wire

The server assigns `left`/`right` once using cryptographic randomness and stores the assignment. Before a verdict the
RPC omits model refs and labels; React does not merely hide already-delivered identity fields. Reloading or changing
breakpoint never swaps sides. A choice or dismissal reveals both model snapshots and accounting data.

"둘 다 사용하지 않기" is a valid non-verdict. Forcing a winner from two bad/indistinguishable outputs adds noise.
An unpaired survivor may be applied after a sibling failure but contributes no quality match.

Winner application uses persisted `applied_at`, not an empty error string, as its idempotency marker. Until that
marker exists, write experiments stay in the post's pending projection. Stage-owner writes additionally no-op when
the target already equals the selected value, covering a retry between the target write and marker write.

## 4. Ranking

Ratings are private to `(user, stage)`. Every model starts at 1500. Winner-only verdicts replay in decision order with
standard two-player Elo and K=32:

```
expectedA = 1 / (1 + 10 ^ ((ratingB - ratingA) / 400))
newA      = ratingA + 32 * (scoreA - expectedA)
```

`scoreA` is 1 for a win and 0 for a loss. Dismissed, failed and unpaired experiments are excluded. Models below three
matches remain visible as provisional. The leaderboard also shows raw wins/losses/win-rate: Elo orders competitors;
the raw numbers keep it interpretable.

The projection is recomputed from immutable verdict metadata. At two-user scale a materialized leaderboard table is
unnecessary; if one is later added, it must be rebuildable and cannot become the source of truth.

## 5. Usage and cost

Each candidate records prompt/completion tokens, wall-clock provider latency and cost. Reported provider cost is
authoritative because routing, caching, reasoning and promotions can make a static token-rate calculation wrong.
When cost is absent, the provider-registry price snapshot supplies an estimate marked `≈`; absent inputs remain
unknown, never zero.

Because the provider port defines zero token counts as "not reported", an absent provider usage object is
`unavailable`; it is not a valid zero-dollar estimate.

OpenRouter currently includes token and cost accounting in API responses without another request:
[Usage Accounting](https://openrouter.ai/docs/cookbook/administration/usage-accounting). Its model catalog exposes
context/pricing metadata:
[Models API](https://openrouter.ai/docs/api/api-reference/models/get-models). The initial model facts in plan 09 were
checked on 2026-08-29 against the official pages for
[Gemini 3.7 Flash](https://openrouter.ai/google/gemini-3.7-flash),
[Qwen3.8 Flash](https://openrouter.ai/qwen/qwen3.8-flash),
[DeepSeek V4 Flash 0731](https://openrouter.ai/deepseek/deepseek-v4-flash-0731),
[Grok 4.6](https://openrouter.ai/x-ai/grok-4.6),
[OpenAI models](https://openrouter.ai/openai), and
[Anthropic models](https://openrouter.ai/anthropic).

Catalog prices are dated presentation metadata. They do not silently alter experiment history and do not replace a
reported charged amount.

## 6. Content retention

Input snapshots and candidate outputs are necessary while a user reviews or retries. After a terminal
decision/dismissal, both are purged after 30 days. The durable record keeps only user id, stage, snapshotted model
identity/label, verdict, token/cost/latency metadata and timestamps. Source-post deletion purges content immediately;
account deletion cascades all metadata.

Logs contain ids, refs and accounting, never prompts, photos, voice samples or candidate output. The canonical chosen
post/profile belongs to its existing context and is not touched by the experiment-content sweeper.

The shipped post deletion behavior calls the experiment purge in one transaction before deleting the post row. Only
then may `ON DELETE SET NULL` detach retained leaderboard metadata from its source slug.
