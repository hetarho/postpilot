# Policy — Model providers and the model catalog

Canonical rules that are **currently true** in the code. Source: [plan/04](../plan/04.model-provider-registry.md),
extended by [plan/09](../plan/09.stage-model-experiments-and-leaderboards.md); built by jobs 06, 15, and 23.

## The port is a boundary

`backend/internal/llm` is the only way a model is called (PRD §6.4; ARCHITECTURE §2.1):

- Nothing above the port learns which vendor answered. No adapter package and no provider SDK is imported anywhere
  except under `internal/llm/…` and in the composition root `cmd/api`, whose job is to inject the adapters. This is
  enforced by `internal/llm/boundary_test.go`, which asks `go list -deps` for the real dependency closure of every
  other package.
- The model is an **input** to every call (`Registry.Complete(ctx, ref, req)`); the port reads no default. The
  observe, write and analyze stages each carry their own `ModelRef` ([I3]).
- Errors are normalized to a small set the callers act on — `ErrModelUnavailable`, `ErrProviderDisabled`,
  `ErrRateLimited`, `ErrUnsupported`, `ErrBadOutput`, `ErrOutputTruncated` — and a `ProviderError` that keeps the provider's own message
  and wraps the sentinel, so the user is told the cause verbatim (PRD §7) while the code still branches on
  `errors.Is`. `llm.UserMessage` is the one mapper used by jobs and model experiments. Output ending with
  `finish_reason: length` that has no usable content, including non-empty partial JSON rejected by a caller parser,
  gets actionable budget-exhaustion copy rather than the generic bad-output error.
- Capability checks run **before any network call**: an image part on a model without `vision`, or a JSON schema on
  a model without `structured_output`, is `ErrUnsupported` immediately. A caller that wants a plain-text fallback
  checks the model's flag first and omits the schema.
- Every call runs under `LLM_STAGE_TIMEOUT` (5 min). The `openai_compatible` adapter requests a stream and joins it
  server-side so a long draft does not idle out an intermediary; the stream never leaves the process (PRD §6.6). A
  stream that ends without `[DONE]` or a `finish_reason` is a truncated answer and fails as `ErrBadOutput` rather
  than being passed on. A 404 means "model gone" only when the body says so — a wrong `base_url` is a generic error.
- A provider response may carry `Usage` and `FinishReason` together with an error. The adapter preserves those fields
  once reported, so failed model-experiment candidates retain billable token/cost evidence.

## Reasoning policy

- `ReasoningEffort` accepts `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, and `max`. An empty value means no
  decision has been made; `unset` is an internal/yaml sentinel that deliberately omits the whole wire key.
- Resolution is strict per-model `reasoning_effort` override → request/stage value → nothing sent. Unknown yaml
  values stop boot. `anthropic/claude-sonnet-5` ships with `unset`, so the generation stage's `low` default does not
  replace that model's provider-controlled adaptive behavior.
- The nested `reasoning: {effort}` wire object is an OpenRouter dialect, enabled only when a provider declares
  `reasoning_format: openrouter`. Other OpenAI-compatible endpoints omit it even when the stage supplied an effort,
  retaining their previous request shape; an unknown format stops boot through adapter validation.
- Observation and writing/revision request `low`; analyze requests no effort. With neither a stage value nor model
  override, the OpenAI-compatible JSON body remains unchanged and contains no `reasoning` key. `none` is different:
  it is sent explicitly.
- `reasoning.exclude` is forbidden: excluding the returned trace does not stop generation or billing of reasoning
  tokens. Reasoning and visible output share the same 8,192-token completion cap.

## The registry is a file, the keys are env

- Providers and models are declared **only** in `backend/config/providers.yaml`. Adding one is an edit plus a
  restart. The file ships inside the image at `/config/providers.yaml` (`PROVIDERS_CONFIG`); a stack may mount its
  own over it.
- The file is validated at boot with unknown fields rejected. Any problem — unknown adapter, duplicate provider or
  model id, missing `api_key_env`, an adapter's own check (`openai_compatible` needs an http(s) `base_url`), no
  models — **stops the process**, the same posture as a failed migration, so the deploy's health gate rolls back.
- `api_key_env` names an environment variable; the key is read from the process environment at boot and never
  written to the file. **An unset key is not a boot failure**: every model of that provider is listed `disabled` with
  the reason `API key not configured` and cannot be selected (`SaveSelection` refuses it too). The entry is still
  validated, so a bad `base_url` cannot hide behind a missing key. `api_key_env` is **optional**: an entry without
  one is a keyless endpoint (a local Ollama, vLLM, LM Studio) — enabled as is, and the adapter sends no
  `Authorization` header.
- `vision` and `structured_output` are declared per model. The observe stage lists vision models only.
- `reasoning_effort` is optional per model and is validated even when that provider's key is missing.
- Structured output is requested whenever the model declares it (`response_format: json_schema`, without `strict` —
  the schemas belong to the callers). Callers keep a parser fallback for models that do not.
- Free-model advice (a router entry such as `openrouter/free`; observe on free, write on paid — PRD §6.5) is yaml
  content, not UI logic.

## Selection memory

- `model_selections (user_id, stage, slot, provider_id, model_id, updated_at)` stores `active`, `candidate_a`, and
  `candidate_b` per account/stage. Existing selection RPCs retain active-slot behavior; pair and preset writes are
  atomic. Start RPCs still receive refs explicitly and never infer them from this table.
- The app writes **no default**. A fresh account has no rows; every stage renders "모델을 선택하세요" and reports
  `selected = null`, which is what the generation and analysis actions block on ([I3]).
- `GetSelections` reports a saved ref that is no longer registered — or that the stage can no longer use (a model
  saved for observe that lost `vision`) — as `missing` and **clears the row in the same call** (PRD §7: 마지막 선택
  초기화). The clear is conditional on the row still holding that ref, so a choice the user makes between the read
  and the clear survives. The client shows the old entry greyed with "등록된 모델 목록에서 사라졌어요" once; the next
  answer no longer has it, and the user must choose again. A saved ref whose provider has since lost its key is
  likewise unusable: greyed with the key reason, `selected = null`. While the catalog is loading or failed to load,
  nothing is judged — a valid choice is never called vanished because its list is not here.
- `SaveSelection` accepts only a known stage and a registered, enabled model that suits the stage (observe needs
  `vision`) — the same rules the dropdown shows, enforced where they can be trusted (`InvalidArgument` /
  `NotFound` / `FailedPrecondition`).
- A pair must contain two distinct enabled stage-suitable refs. `ApplyRecommendationSet` validates the complete
  three-stage, nine-slot set before one transaction; no partial rows survive rejection. A recommendation is never
  applied on mount, login, or account creation.

## Catalog metadata and recommendations

- Models may declare context tokens, dated input/output USD-per-million snapshots, and labels. Unknown/bad metadata
  or a broken recommendation ref stops boot. Prices are display/estimate metadata only; reported provider cost wins.
- The shipped opt-in `balanced-2026-08` set pins six model ids and one active/A-B selection per stage. Removed models
  stay readable from experiment snapshots but cannot be newly selected.

## What crosses to the browser

Ids, labels and flags (`ModelInfo`), and per-stage `Selection`s. No key, no SDK payload, no base URL — the proto
has no field for them (plan 04 AC6).

## Configuration

| Value | Where | Note |
|---|---|---|
| `PROVIDERS_CONFIG` | env | default `config/providers.yaml` (relative to the working directory); the image sets `/config/providers.yaml` |
| `<api_key_env>` per provider (`OPENROUTER_API_KEY` in the shipped file) | env, named by the yaml | unset ⇒ that provider's models are disabled with the reason |
| `LLM_STAGE_TIMEOUT` | constant | 5 min per provider call (PRD §6.6) |
| `LLM_MAX_TOKENS_DEFAULT` | constant | 8192 — shared completion cap for reasoning plus visible output |
| stage reasoning policy | typed constants | observe `low` · write/revise `low` · analyze omitted |
| `reasoning_format` | `config/providers.yaml` | optional provider dialect; shipped OpenRouter entry opts in |
| `reasoning_effort` | `config/providers.yaml` | optional strict model override; `unset` omits the wire key |
| `MODEL_CATALOG_STALE_MS` | FE `shared/config` | 5 min — how long the catalog is trusted before a refetch |
| recommendation/model price metadata | `config/providers.yaml` | strict, dated catalog content; no automatic apply |
