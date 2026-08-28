# Policy — Model providers and the model catalog

Canonical rules that are **currently true** in the code. Source: [plan/04](../plan/04.model-provider-registry.md),
built by job 06.

## The port is a boundary

`backend/internal/llm` is the only way a model is called (PRD §6.4; ARCHITECTURE §2.1):

- Nothing above the port learns which vendor answered. No adapter package and no provider SDK is imported anywhere
  except under `internal/llm/…` and in the composition root `cmd/api`, whose job is to inject the adapters. This is
  enforced by `internal/llm/boundary_test.go`, which asks `go list -deps` for the real dependency closure of every
  other package.
- The model is an **input** to every call (`Registry.Complete(ctx, ref, req)`); the port reads no default. The
  observe, write and analyze stages each carry their own `ModelRef` ([I3]).
- Errors are normalized to a small set the callers act on — `ErrModelUnavailable`, `ErrProviderDisabled`,
  `ErrRateLimited`, `ErrUnsupported`, `ErrBadOutput` — and a `ProviderError` that keeps the provider's own message
  and wraps the sentinel, so the user is told the cause verbatim (PRD §7) while the code still branches on
  `errors.Is`.
- Capability checks run **before any network call**: an image part on a model without `vision`, or a JSON schema on
  a model without `structured_output`, is `ErrUnsupported` immediately. A caller that wants a plain-text fallback
  checks the model's flag first and omits the schema.
- Every call runs under `LLM_STAGE_TIMEOUT` (5 min). The `openai_compatible` adapter requests a stream and joins it
  server-side so a long draft does not idle out an intermediary; the stream never leaves the process (PRD §6.6). A
  stream that ends without `[DONE]` or a `finish_reason` is a truncated answer and fails as `ErrBadOutput` rather
  than being passed on. A 404 means "model gone" only when the body says so — a wrong `base_url` is a generic error.

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
- Structured output is requested whenever the model declares it (`response_format: json_schema`, without `strict` —
  the schemas belong to the callers). Callers keep a parser fallback for models that do not.
- Free-model advice (a router entry such as `openrouter/free`; observe on free, write on paid — PRD §6.5) is yaml
  content, not UI logic.

## Selection memory

- `model_selections (user_id, stage, provider_id, model_id, updated_at)` — one row per account and stage, owned by
  `internal/provider`. It exists only to refill the dropdowns: the `Start*` RPCs receive their refs explicitly and
  record them on the job row, and never read this table.
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

## What crosses to the browser

Ids, labels and flags (`ModelInfo`), and per-stage `Selection`s. No key, no SDK payload, no base URL — the proto
has no field for them (plan 04 AC6).

## Configuration

| Value | Where | Note |
|---|---|---|
| `PROVIDERS_CONFIG` | env | default `config/providers.yaml` (relative to the working directory); the image sets `/config/providers.yaml` |
| `<api_key_env>` per provider (`OPENROUTER_API_KEY` in the shipped file) | env, named by the yaml | unset ⇒ that provider's models are disabled with the reason |
| `LLM_STAGE_TIMEOUT` | constant | 5 min per provider call (PRD §6.6) |
| `LLM_MAX_TOKENS_DEFAULT` | constant | 4096 — the completion cap when a caller sets none |
| `MODEL_CATALOG_STALE_MS` | FE `shared/config` | 5 min — how long the catalog is trusted before a refetch |
