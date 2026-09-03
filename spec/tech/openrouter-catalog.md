# Tech — The OpenRouter model-catalog contract

How the app learns which models exist: the endpoint, the fields we consume, the mapping into our own
`catalog_models` shape, and the cache/availability semantics. Owned by
[plan 18](../plan/18.openrouter-model-catalog-and-admin-model.md) and built by job 46. The OpenRouter response
facts below were verified against the live API on 2026-09-03.

Owner in code: `backend/internal/modelcatalog/openrouter` (the HTTP client + TTL cache) and
`backend/internal/modelcatalog` (mapping + availability bookkeeping).

## The endpoint

- `GET {base_url}/models` — with the shipped provider entry that is `https://openrouter.ai/api/v1/models`.
  The URL is derived from the provider's existing `base_url`; no separate catalog URL is configured.
- **No authentication is required** for the list; the client sends no `Authorization` header, so a missing
  `OPENROUTER_API_KEY` disables model *calls* (plan 04 rule) but never blocks *browsing* the catalog.
- The response is one JSON document, `{"data": [<model>, …]}`, ~420 entries (~1.5 MB). OpenRouter serves it
  edge-cached; optional `offset`/`limit` pagination exists but a single unpaginated fetch is the contract here —
  the merge needs the whole list anyway to judge availability.
- Plain `net/http` under `CatalogFetchTimeout`; no SDK, keeping the `internal/llm` boundary test's
  import rules trivially satisfied (the client lives outside `internal/llm` and talks to a public data endpoint,
  not to a model).

## The fields we consume

One model object (fields we ignore elided):

```json
{
  "id": "openai/gpt-5.6-sol",
  "canonical_slug": "openai/gpt-5.6-sol-20260815",
  "name": "OpenAI: GPT-5.6 Sol",
  "created": 1788362056,
  "description": "…",
  "context_length": 1048576,
  "architecture": { "input_modalities": ["text", "image"], "output_modalities": ["text"] },
  "pricing": { "prompt": "0.00000125", "completion": "0.00000425" },
  "supported_parameters": ["structured_outputs", "tools", "reasoning", "…"]
}
```

| OpenRouter field | Our field | Rule |
|---|---|---|
| `id` | `model_id` | verbatim; it is already the `provider-slug/model-slug` string our `ModelRef.model_id` has always held, so selections and job records need no rewrite. Variant suffixes (`:free`) are part of the id and kept |
| `id` before the first `/` | `provider_slug` | grouping/filter key only — `ModelRef.provider_id` stays `openrouter` |
| `name` | `label` | verbatim |
| `created` | `source_created_at` | epoch seconds; drives "newest first" within a provider group |
| `description` | admin display only | not persisted |
| `context_length` | `context_tokens` | verbatim; nullable when absent |
| `"image" ∈ architecture.input_modalities` | `vision` | gates photo-analysis registration (observe consumes images as input) |
| `"image" ∈ architecture.output_modalities` | `image_output` | gates image-generation registration (change 20); no feature consumes it yet |
| `"video" ∈ architecture.output_modalities` | `video_output` | gates video-generation registration (change 20); OpenRouter lists few or no such models today, so the tab may be empty |
| `"structured_outputs" ∈ supported_parameters` | `structured_output` | whether the adapter may send `response_format: json_schema` |
| `pricing.prompt` / `pricing.completion` | `input_usd_per_million` / `output_usd_per_million` | decimal **strings in USD per token**; multiply by 10⁶ with decimal string arithmetic (no float round-trip) to match plan 09's $/1M display convention; `pricing_checked_at` := fetch time |

Unknown fields are ignored (the reverse of the yaml's strict parsing: this document is OpenRouter's schema, not
ours, and it grows without notice). A model entry missing `id` or `name` is skipped with a warning, never a boot
or request failure.

## Cache and availability semantics

- The parsed list is cached in process memory for `CatalogTTL`; `ListCatalog(refresh: true)` (the
  admin's 새로고침) bypasses and replaces it. There is no background/scheduled fetch.
- Only the **admin path** ever triggers a fetch. The user-facing catalog (`ListModels`, `SaveSelection`, admission,
  `Complete`) reads `catalog_models` rows exclusively, so an OpenRouter outage cannot change what users see.
- Availability bookkeeping is written only by a **successful** fetch: enabled rows present in the list get
  `listed = 1`, `last_seen_at`, and refreshed label/context/pricing snapshots; enabled rows absent from it get
  `listed = 0`. A failed fetch writes nothing and is reported to the admin (`fetch_error`), degrading the browse
  list to DB rows only.
- `listed = 0` is a flag, not an action: the model serves disabled-with-reason (제공 종료) to users and waits for
  an admin to disable it (plan 18's interview decision — no automatic cleanup).
