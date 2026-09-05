# Tech — The OpenRouter model-catalog contract

How the app learns which models exist: the endpoint, the fields we consume, the mapping into our own
`catalog_models` shape, and the cache/availability semantics. Owned by
[plan 18](../plan/18.openrouter-model-catalog-and-admin-model.md) and built by jobs 46 and 48. The OpenRouter
response facts below were verified against the live API and its published OpenAPI spec on 2026-09-03.

Owner in code: `backend/internal/modelcatalog/openrouter` (the HTTP client + TTL cache) and
`backend/internal/modelcatalog` (mapping + availability bookkeeping).

## The endpoint

- `GET {base_url}/models?output_modalities=text,image,video` — with the shipped provider entry that is
  `https://openrouter.ai/api/v1/models`. The URL is derived from the provider's existing `base_url`; no separate
  catalog URL is configured.
- **The query is mandatory, not a filter for convenience.** OpenRouter's spec for this parameter reads: "Accepts a
  comma-separated list of modalities (text, image, embeddings, audio, video, rerank, speech, transcription) or
  `all` to include all models. **Defaults to `text`.**" A request without it therefore returns *text-output models
  only*: 424 of the 572 models the catalog actually holds, with **zero** video models and only the 11 image models
  that also answer in text. The three requested values are exactly what the five curated purposes need — `text` for
  photo analysis, style analysis and writing; `image` and `video` for the two generation purposes. `all` is
  deliberately not requested: embeddings, rerank, speech and transcription models serve no purpose this product
  curates.
- Measured on 2026-09-03: `text` 424 · `image` 50 (39 image-only + 11 image+text) · `video` 28 · `all` 572.
- **No authentication is required** for the list; the client sends no `Authorization` header, so a missing
  `OPENROUTER_API_KEY` disables model *calls* (plan 04 rule) but never blocks *browsing* the catalog.
- The response is one JSON document, `{"data": [<model>, …]}`, ~490 entries for the three requested modalities
  (~1.7 MB). OpenRouter serves it
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
  "pricing": { "prompt": "0.00000125", "completion": "0.00000425",
               "image_token": "0.0000024", "image_output": "0.00003" },
  "supported_parameters": ["structured_outputs", "tools", "reasoning", "reasoning_effort", "…"],
  "reasoning": { "mandatory": false, "supported_efforts": ["max", "high", "low"],
                 "default_effort": "high", "default_enabled": true,
                 "supports_max_tokens": true }
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
| `"image" ∈ architecture.output_modalities` | `image_output` | gates image-generation registration (change 20); no feature consumes the registration yet |
| `"video" ∈ architecture.output_modalities` | `video_output` | gates video-generation registration (change 20); 28 such models exist, reachable only through the `output_modalities` query above |
| `"structured_outputs" ∈ supported_parameters` | `structured_output` | whether the adapter may send `response_format: json_schema` |
| `reasoning` (the object is present) | `reasons` | the model reasons at all. 300 of 427 entries carry it (measured 2026-09-05); **absence is recorded, not skipped** — such a model gets no effort control |
| `reasoning.supported_efforts` | `reasoning_efforts` | **verbatim and in the source's descending order**, which is the order a selector offers. 154 entries publish it. Stored as a JSON array in one column — the column's whole claim is verbatim, which a delimiter cannot back |
| `reasoning.default_effort` | `reasoning_default_effort` | what the model uses when reasoning is on and no effort is sent. It is what our `unset` **means**, so the admin labels `unset` with it |
| `reasoning.mandatory` | `reasoning_mandatory` | 97 entries are `true`. `none` is then never offered and never sent |
| `reasoning.supports_max_tokens` | `reasoning_max_tokens` | 10 entries publish it, always `true`. Recorded and displayed; nothing consumes it |
| `reasoning.default_enabled` | — | read but **not persisted**: display context change 27 does not scope |
| `"reasoning_effort" ∈ supported_parameters` | `reasoning_native_effort` | whether the provider receives the effort **string** itself rather than a token budget OpenRouter derived from it. 157 entries. Nothing consumes it yet; [change 29](../changes/29.give-the-write-budget-reasoning-headroom.md) needs it to size a budget safely |
| `pricing.*` | `input_usd_per_million` / `output_usd_per_million` | decimal **strings in USD per token**, multiplied by 10⁶ with decimal string arithmetic (no float round-trip) to match plan 09's $/1M display convention; `pricing_checked_at` := fetch time. Which keys apply depends on the output modality — see below |

### Which pricing keys apply (change 20)

The source publishes several price axes and a model is billed on only some of them, so the pair we store is chosen
by what the model answers in:

| Model answers in | `input_usd_per_million` | `output_usd_per_million` |
|---|---|---|
| text (incl. image+text) | `pricing.prompt` — **a zero is a real price** (a free model) | `pricing.completion` — likewise |
| image only | `pricing.prompt` if non-zero, else `pricing.image_token` | `pricing.completion` if non-zero, else `pricing.image_output` |
| video | unknown (empty) | unknown (empty) |

- A model that answers only in images is **not billed on text tokens at all**, so a zero `prompt`/`completion`
  there is the *absence* of a price rather than "free", and the image-token pair is what it charges.
- Every one of the 28 video models reports `prompt` and `completion` as `"0"` and publishes no other price key.
  That is an absence, not free, so both columns stay empty and the operator screen says 토큰 단가 미공개. The
  per-second prices shown on OpenRouter's own model pages (for example Veo 3.1 Fast at $0.10/s) are **not in this
  API response** and are therefore not something the product can display or verify.
- ⚠️ **`image_output`'s documented wording is wrong.** The spec calls it "Price in USD per output image", but the
  values are per output image **token**: ×10⁶ reproduces the vendors' own published $/1M figures exactly — Gemini
  2.5 Flash Image `0.00003` → $30, Gemini 3 Pro Image `0.00012` → $120, GPT-5 Image `0.00004` → $40. It therefore
  shares the $/1M column with the text prices instead of needing a unit of its own. (`pricing.image`, by contrast,
  really is per *input* image and is not consumed.)

### Reasoning capability, and why absence is not "no" (change 27)

Until change 27 this document declared every field but `structured_outputs` ignored, and the reasoning override an
operator set beside it was therefore chosen with **no knowledge of what the model accepts** — the admin offered the
same eight values for every model. The correction is narrow: the response's top-level `reasoning` object names the
accepted values exactly, and it was never in the table above. (Change 24's note that `supported_parameters` "never
says which VALUES a model honors" was true of the field it was reading, and is superseded by these rows.)

Every one of the six fields follows the same rule the pricing keys already follow: **a falsy value is UNKNOWN, not
"supports nothing".** An empty `reasoning_efforts` is a model whose accepted values the source does not publish —
273 of 427 — and the answer there is to offer all eight, never none.

That rule leaves one thing the six fields cannot say about themselves: `reasons: false` with no list is *both* "the
source publishes no reasoning object" and "nothing has ever asked". Migration `0024` leaves every existing row in
exactly that shape, and so does any row served while the fetch is failing. So the browse response carries a seventh,
**unstored** flag — `reasoning_known` — saying whether the capability in *this* response came from a read that
actually looked. A live candidate is known by construction; a stored row is known only if it says something.
Everything downstream keys off it: an unknown capability offers the full vocabulary and accepts any effort, while a
*known* non-reasoning model offers no control and accepts none.

Unknown fields are ignored (the reverse of the yaml's strict parsing: this document is OpenRouter's schema, not
ours, and it grows without notice) — including an unknown key inside the `reasoning` object. A model entry missing
`id` or `name` is skipped with a warning, never a boot or request failure.

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
