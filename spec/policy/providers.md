# Policy — Model providers and the model catalog

Canonical rules that are **currently true** in the code. Source: [plan/04](../plan/04.model-provider-registry.md),
extended by [plan/09](../plan/09.stage-model-experiments-and-leaderboards.md) and
[plan/18](../plan/18.openrouter-model-catalog-and-admin-model.md); built by jobs 06, 15, 23, 37, 46, 47 and 48.
What an account may afford to run is owned by [plans.md](plans.md).

## The port is a boundary

`backend/internal/llm` is the only way a model is called (PRD §6.4; ARCHITECTURE §2.1):

- Nothing above the port learns which vendor answered. No adapter package and no provider SDK is imported anywhere
  except under `internal/llm/…` and in the composition root `cmd/api`, whose job is to inject the adapters. This is
  enforced by `internal/llm/boundary_test.go`, which asks `go list -deps` for the real dependency closure of every
  other package.
- The model is an **input** to every call (`Registry.Complete(ctx, ref, req)`); the port reads no default. The
  observe, write and analyze stages each carry their own `ModelRef` ([I3]).
- Errors are normalized to a small set the callers act on — `ErrModelUnavailable`, `ErrProviderDisabled`,
  `ErrRateLimited`, `ErrUnsupported`, `ErrBadOutput`, `ErrOutputTruncated` — and a `ProviderError` that keeps optional
  provider prose as diagnostic detail while still supporting `errors.Is`. `llm.Failure` is the one stable mapper used
  by jobs and model experiments (`MODEL_UNAVAILABLE`, `MODEL_RATE_LIMITED`, `MODEL_UNSUPPORTED`,
  `MODEL_OUTPUT_INVALID`, `MODEL_OUTPUT_TRUNCATED`, or `UNKNOWN_FAILURE`). Provider text is never primary UI copy or
  an interpolated param. Output ending with `finish_reason: length` that has no usable content, including non-empty
  partial JSON rejected by a caller parser, maps to the truncated-output reason. When the provider reported a
  reasoning token count, the technical detail **names the reasoning/visible split**, because the two causes have
  opposite remedies: a body that filled its budget wants a larger one, while a body the model never wrote because
  it reasoned through the budget wants a lower effort for that purpose, or another model. A provider that reported
  no split keeps the message it always had, and no user-facing string changes.
- `ErrRateLimited` means the provider refused for **rate** reasons — the caller's quota, the account's, or the
  gateway's own upstream pool. It attributes nothing to a tier, and it may arrive as an HTTP 429 or as an upstream
  `code: 429` inside an HTTP 200.
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
- Resolution is the operator's `reasoning_effort` override **for the purpose the call is being made for** →
  request/stage value → nothing sent. The override is a property of a REGISTRATION, not of a model: the policy it
  overrides is per stage, so one value for the whole model erased that distinction and lowering the effort for
  writing silently changed photo observation (change 24). One model may therefore observe at one strength and write
  at another within a single run, and each purpose tab in 모델 관리 shows and edits only its own value. Unknown yaml
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

## The connection is a file, the models are rows registered per purpose

Plan 18 split what used to be one file. **How to reach the provider** is configuration, read once at boot; **which
models this installation offers, and for which purposes** is curated data, read live (change 20 → job 48).

- `backend/config/providers.yaml` declares the provider connection (`id`, `adapter`, `base_url`, `api_key_env`,
  `reasoning_format`) and the versioned `recommendation_sets`. It ships inside the image at
  `/config/providers.yaml` (`PROVIDERS_CONFIG`); a stack may mount its own over it.
- **Exactly one provider may be declared.** The curated catalog carries no provider dimension — every row is served
  by the registered endpoint — so a second entry would attribute models to a vendor nobody chose. Adding a
  genuinely different vendor is a design change with its own plan, not a yaml edit.
- The file is validated at boot with unknown fields rejected. Any problem — unknown adapter, a missing or malformed
  id, an adapter's own check (`openai_compatible` needs an http(s) `base_url`), more or fewer than one provider —
  **stops the process**, the same posture as a failed migration, so the deploy's health gate rolls back. A leftover
  per-provider `models:` list is rejected by the strict parser rather than ignored, so a stack still mounting the
  old shape hears about it instead of quietly serving an empty catalog.
- `api_key_env` names an environment variable; the key is read from the process environment at boot and never
  written to the file. **An unset key is not a boot failure**: every model is listed `disabled` with the reason
  `API key not configured` and cannot be selected (`SaveSelection` refuses it too). The entry is still validated, so
  a bad `base_url` cannot hide behind a missing key. `api_key_env` is **optional**: an entry without one is a
  keyless endpoint (a local Ollama, vLLM, LM Studio) — enabled as is, and the adapter sends no `Authorization`
  header.
- The registry reads its models through an injected `llm.ModelSource` rather than from the parsed file, and it reads
  it on **every** request: curating a model takes effect for the next call, not the next deploy. An **empty catalog
  is a valid state** — a fresh install has curated nothing, and the right answer is an empty picker and a trip to
  `/admin/models`, not a refused boot. Boot never contacts the provider's catalog.
- `vision`, `structured_output`, `image_output` and `video_output` are per-model row values snapshotted from the
  provider's catalog. They gate what a model may be REGISTERED for; the stage-side check is pure membership.
- `reasoning_effort` is an optional value on a **purpose registration** (`catalog_model_purposes`), curated by the
  operator and validated on write. Setting one for a purpose the model is not registered to is refused server-side,
  not merely hidden by the UI. Migration 0021 moved the column off `catalog_models` and **cleared** every existing
  value rather than copying it into five purposes, so all curated models resolve to the code-owned stage policy.
- Structured output is requested whenever the model declares it (`response_format: json_schema`, without `strict` —
  the schemas belong to the callers). Callers keep a parser fallback for models that do not.

## The catalog is curated rows, registered per purpose

`catalog_models` is the curated-model list and `catalog_model_purposes` holds its registrations across five
purposes — `photo-analysis` · `style-analysis` · `writing` · `image-generation` · `video-generation` — curated on
the five tabs of the 모델 관리 screen. Both are global rather than per-account: what an installation offers is an
operator decision. Only the master tier may read or change them (`ModelCatalogService`, in the interceptor's
master-only set).

- **Registration replaced the global `enabled` flag** (migration `0020` dropped the column). A model is visible to
  a user-facing stage exactly when it is registered to that stage's purpose: `photo-analysis` → observe,
  `style-analysis` → analyze, `writing` → write. The generation purposes feed **no stage yet** — they persist and
  round-trip in admin, and nothing else reads them (future plans attach the features).
- **Each purpose enforces a capability gate at registration, server-side**: `photo-analysis` requires `vision`;
  `image-generation`/`video-generation` require the matching `image_output`/`video_output`; the text purposes take
  any model. The admin tab force-filters its candidate list to the same gate, so the checkbox never offers what
  the server would refuse (`MODEL_PURPOSE_INELIGIBLE`).
- A row with **zero registrations** is the state `enabled = 0` used to be: kept —
  but served to nobody, absent from `ListModels` and unavailable to `Registry.Complete`. A model registered only
  to a generation purpose is likewise never sent over the user-facing wire.
- **Capability drift stops the stage, not the registration.** A refresh re-snapshots the capability flags; a
  registered model that LOST the capability its purpose was gated on stops being served to that stage at once
  (`stagesOf` re-checks the gate), while the registration row is kept and stays visible on its admin tab — flagged
  for the operator to uncheck, never auto-retired.
- **Stage membership is enforced at every execution boundary**, not only in the picker: `SaveSelection`/pairs
  (provider), `StartGeneration`/`StartRevision` (generation), experiment starts and winner adoption (experiment),
  and the voice validation/rule-comparison model checks all refuse a client-supplied ref that is not registered to
  the stage they run it for (`llm.ModelInfo.ServesStage`).
- The cutover **seeded no registrations** (the interview decision recorded in
  [change 20](../changes/archive/20.purpose-scoped-model-enablement-with-adm.md)): after migration `0020`, every
  purpose tab and every stage dropdown starts empty, and pre-cutover saved selections flow through the existing
  vanished-selection machinery on their next read.

- **Candidates** come live from the provider's own public catalog —
  `GET {base_url}/models?output_modalities=text,image,video`, unauthenticated, read server-side and cached for
  `OPENROUTER_CATALOG_TTL` ([tech/openrouter-catalog](../tech/openrouter-catalog.md)). The modality query is
  **mandatory**: the endpoint serves text-output models only by default, so omitting it hides every video model and
  half the image ones. Only the operator path ever triggers that read, so a provider outage cannot change what
  users see.
- **Usable models** are the rows with at least one registration, and nothing else feeds `ListModels`,
  `SaveSelection`, or `Registry.Complete`. A row survives full deregistration, so the curation an
  operator set comes back intact when the model is re-registered.
- Registering never selects ([I3]): a registered model appears in its purpose's picker, and every user still
  chooses their own.
- Availability bookkeeping (`listed`, `last_seen_at`, and the label/context/pricing snapshot) is written **only by a
  successful** live read. A failed one writes nothing and is reported to the operator instead — treating an outage
  as evidence would retire the whole catalog the first time the network hiccuped.
- A model the provider has stopped offering is **flagged, never retired automatically**: `listed = 0` badges it 제공
  종료 on the operator screen and serves it to users as disabled with a reason, which routes it through the existing
  vanished-selection machinery. Removing it stays an operator decision.
- Free-model advice (a router entry such as `openrouter/free`; observe on free, write on paid — PRD §6.5) is catalog
  content, not UI logic.

## Selection memory

- `model_selections (user_id, stage, slot, provider_id, model_id, updated_at)` stores `active`, `candidate_a`, and
  `candidate_b` per account/stage. Existing selection RPCs retain active-slot behavior; pair and preset writes are
  atomic. Start RPCs still receive refs explicitly and never infer them from this table.
- The app writes **no default**. A fresh account has no rows; every stage renders "모델을 선택하세요" and reports
  `selected = null`, which is what the generation and analysis actions block on ([I3]).
- `GetSelections` reports a saved ref that is no longer registered — or that was deregistered from the stage's
  purpose — as `missing` and **clears the row in the same call** (PRD §7: 마지막 선택 초기화). The clear is conditional on the row still holding that ref, so a choice the user makes between the read
  and the clear survives. The client shows the old entry greyed with "등록된 모델 목록에서 사라졌어요" once; the next
  answer no longer has it, and the user must choose again. A saved ref whose provider has since lost its key is
  likewise unusable: greyed with the key reason, `selected = null`. While the catalog is loading or failed to load,
  nothing is judged — a valid choice is never called vanished because its list is not here.
- `SaveSelection` accepts only a known stage and a model registered to that stage's purpose whose provider is not
  disabled — the same rules the dropdown shows, enforced where they can be trusted (`InvalidArgument` /
  `NotFound` / `FailedPrecondition`). Observe's vision requirement is subsumed: the photo-analysis gate already
  refused a text model at registration. **No tier is consulted**: any account may save any registered model.
- A model the caller cannot currently **afford** is never reported `missing` and its row is never touched. A
  balance is temporary state the next renewal clears, so it invalidates nothing; the picker greys the entry with
  what it would cost ([plans.md](plans.md)).
- A pair must contain two distinct refs registered to the stage's purpose. `ApplyRecommendationSet` validates the complete
  three-stage, nine-slot set before one transaction; no partial rows survive rejection. A recommendation is never
  applied on mount, login, or account creation.

## Catalog metadata and recommendations

- Models carry context tokens, dated input/output USD-per-million snapshots, and labels, refreshed from the
  provider's catalog on an operator refresh. Prices are display/estimate metadata only; reported provider cost wins.
- **Which price axis applies depends on the output modality.** A model answering in text keeps the text token pair,
  where a zero is a genuine price (a free model). One answering only in images is not billed on text tokens, so its
  zeros are the absence of a price and the image-token pair is stored instead. A video model publishes no token
  price at all — both columns stay empty and the screen says 토큰 단가 미공개 rather than showing $0.
- Recommendation-set refs are validated **at apply time**, against the catalog as it is then. Boot can no longer
  settle whether a referenced model exists — the catalog is curated data that changes while the process runs — so
  boot keeps shape validation only (three distinct stages, complete refs, candidates that differ).
- A refused set names **every** offending ref at once, grouped by cause (unregistered · disabled · not registered
  to its stage's purpose), because a set is applied whole and discovering its problems one attempt at a time is
  nine round trips. With the purposes starting empty after the cutover, the shipped set refuses cleanly until the
  operator registers its models.
- The shipped opt-in `balanced-2026-08` set pins six model ids and one active/A-B selection per stage. Removed models
  stay readable from experiment snapshots but cannot be newly selected. Any tier may apply it: a set is refused only
  when a ref it names is unregistered, disabled, or unsuitable for its stage — never for the tier applying it. A CI
  test asserts every shipped ref names a model the catalog's seed migration inserts, which is what replaced the
  boot-time existence check.

## What crosses to the browser

Ids, labels, capability flags, and display metadata. No key, no SDK payload, no base URL — the proto has no field
for them (plan 04 AC6), and that holds for the operator's catalog surface too, which adds only public catalog data
(descriptions and prices) on top.

## Configuration

| Value | Where | Note |
|---|---|---|
| `PROVIDERS_CONFIG` | env | default `config/providers.yaml` (relative to the working directory); the image sets `/config/providers.yaml` |
| `<api_key_env>` per provider (`OPENROUTER_API_KEY` in the shipped file) | env, named by the yaml | unset ⇒ that provider's models are disabled with the reason |
| `LLM_STAGE_TIMEOUT` | constant | 5 min per provider call (PRD §6.6) |
| `LLM_MAX_TOKENS_DEFAULT` | env, default 8192 | the registry's fallback when a caller sets no budget, the writing stage's floor, and the base of its ceiling; reasoning and visible output share it |
| per-stage completion budget | typed constants | each stage asks for what its work needs instead of sharing one cap: observation scales with `OBSERVE_BATCH_SIZE` (one structured entry per photo in the call), writing derives from the post's requested length, and a revision from the larger of that and the content it must re-emit — floored at the fallback so nothing regresses, capped at a multiple of it so a mistyped target cannot ask for an unbounded completion |
| reasoning spend | recorded, not configured | `usage_events.reasoning_tokens` holds the provider-reported split; the curation surface shows it per model and per purpose, because `supported_parameters` says a model ACCEPTS `reasoning_effort` and never which values it honors — an unhonored effort behaves like sending none and reasoning runs to the cap |
| stage reasoning policy | typed constants | observe `low` · write/revise `low` · analyze has **no field**: a request with no stage value already sends nothing |
| `reasoning_format` | `config/providers.yaml` | optional provider dialect; shipped OpenRouter entry opts in |
| `reasoning_effort` | `catalog_model_purposes` row | optional override for one (model, purpose), curated by the operator; `unset` omits the wire key |
| `OPENROUTER_CATALOG_TTL` | BE constant | 5 min — how long one live read of the provider's catalog is served from memory |
| `OPENROUTER_CATALOG_FETCH_TIMEOUT` | BE constant | 15 s — the catalog read's own timeout; the screen degrades past it |
| `MODEL_CATALOG_STALE_MS` | FE `shared/config` | 5 min — how long the user-facing catalog is trusted before a refetch |
| `FEATURED_MODEL_PROVIDERS` | FE `shared/config` | ordered vendor slugs lifted to the top of the operator's list; display only |
| `CATALOG_ROW_ESTIMATE_PX` · `CATALOG_ROW_OVERSCAN` | FE `shared/config` | the virtualized operator list's unmeasured-row height and its overscan |
| the usable-model list | `catalog_models` + `catalog_model_purposes` rows | curated per purpose through `ModelCatalogService`; the 0018 seed carries the twelve ex-yaml models, the 0020 migration seeds **no** registrations |
| `MODEL_PURPOSES` | FE `shared/config` | the five purpose slugs in tab order; labels live in the i18n resources |
| recommendation sets | `config/providers.yaml` | shape-validated at boot, ref-validated at apply time; no automatic apply |

## Model access is not a registry concern

`min_plan` was removed by [change 19](../changes/archive/19.credit-metering-and-open-model-access.md). A model
carries no tier: every account may select and run every registered model, and the only thing that can refuse one
is whether the account's credit balance covers what the work would hold ([plans.md](plans.md)). The registry
therefore stays entirely user-ignorant — it publishes prices, and the pricing of a caller's choice happens in the
contexts that know the caller.
