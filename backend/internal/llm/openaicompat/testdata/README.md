# Bounded clip wire fixtures

Verified against official documentation on 2026-09-10:

- [OpenRouter video inputs](https://openrouter.ai/docs/guides/overview/multimodal/videos): Chat Completions places `processing` **inside `video_url`**, beside `url`; it is not the Responses API shape. Gemini supports inline MP4; AI Studio's URL path is YouTube-only and Vertex does not accept ordinary video URLs.
- [Gemini video understanding](https://ai.google.dev/gemini-api/docs/video-understanding#technical-details-about-videos): explicit static mode uses 1 FPS, up to 258 frame tokens and 32 audio tokens per second at the documented default settings. The 60-second profile reserves 20,000 media units plus at most 8,976 text/schema bytes and 1,024 wrapper units inside the 30,000-input envelope. No agentic processing or resolution/FPS override is enabled.
- [OpenRouter provider routing](https://openrouter.ai/docs/guides/routing/provider-selection): `max_price.prompt/completion` use USD per million tokens, while endpoint catalog prices use USD per token. `request` and `image` have separate maxima. Supported-parameter filtering and disabled fallbacks are mandatory.
- [Endpoint discovery](https://openrouter.ai/docs/api/api-reference/endpoints/list-all-endpoints-for-a-model): check exact model ID, unique leaf tag, parameters and decimal pricing. Freeze a fingerprint and recheck it before each completion. Use both explicit `order` and `only`; account-level provider lists cannot silently broaden the execution order.
- [Current OpenAPI](https://openrouter.ai/docs/openapi/openapi.yaml): follow string-valued routing prices. Audio is USD per input audio token, image is USD per image, request is USD per request; prompt/completion maxima are per million tokens. Conditional tiers are conservatively enveloped, not ignored. Unknown dimensions/conditions refuse.
- [Prompt caching](https://openrouter.ai/docs/guides/best-practices/prompt-caching): Gemini implicit caching has no write/storage charge; explicit cache-control is absent. Anthropic writes need explicit cache control. OpenAI/DeepSeek automatic token-priced writes are covered. Discounted cache reads or other nonuniform rates make aggregate-only final estimates unavailable. Explicit provider order disables cache-sticky routing; fallback remains disabled.

Re-read on 2026-09-12 for vendor-neutral qualification (CLIP-30, T111):

- [Endpoint listing schema](https://openrouter.ai/docs/openapi/openapi.yaml) `PublicEndpoint`: `tag` (the provider slug routing keys use), `model_id`, `status` (0 healthy, negative degraded — read for admission, excluded from the fingerprint), `supported_parameters`, `pricing`, `context_length`, `max_prompt_tokens`, `max_completion_tokens`. The listing carries no per-endpoint modality; the model's `architecture.input_modalities` (the catalog's `video_input`) is the only admission decided before a document is read.
- [Provider routing](https://openrouter.ai/docs/guides/routing/provider-selection): `require_parameters` filters to providers supporting every parameter the request sends; `only` merges with account-wide allowed providers, which is why `order` names the same single leaf; `allow_fallbacks: false` is sent explicitly. `max_price` keys are `prompt`, `completion`, `request`, `image`, `audio` — string-valued.
- [Video inputs](https://openrouter.ai/docs/guides/overview/multimodal/videos): `video_url` takes a base64 data URL on any model with video input; the optional `processing` field (`static` | `agentic`) is documented for Gemini models only, so `static` is sent on that profile and no processing field elsewhere. `agentic` is never sent.
- Pricing contract: a pricing key is present when that unit is charged separately. A leaf listing no `image`/`audio` rate bills an inline clip as prompt tokens, so those envelopes are zero and known (`qwen_priced_endpoints.json`); a present-but-malformed rate, an unknown dimension, or cache-write semantics this code cannot place still refuse.

The four qualification fixtures prove distinct METADATA SHAPES; their ids are not an allowlist and no test asserts live availability:

- `gemini_priced_endpoints.json` — explicit per-unit media prices, two leaves (sanitized public metadata, see below).
- `qwen_priced_endpoints.json` — prompt/completion-only pricing, one leaf (synthetic shape).
- `bytedance_priced_endpoints.json` — conditional `overrides` tiers and a per-image price (synthetic shape).
- `amazon_priced_endpoints.json` — no `response_format`/`structured_outputs`, so only the parser-fallback request qualifies (synthetic shape).

`strict_inline_request.json` is a synthetic serialization fixture, not a recorded user request. Its three payload bytes (`abc`) are not a playable video. Test endpoint prices are deliberately synthetic; green tests do not claim those routes/prices exist live.

`gemini_priced_endpoints.json` is a sanitized subset of the public unauthenticated [Gemini 2.5 Flash endpoint metadata](https://openrouter.ai/api/v1/models/google/gemini-2.5-flash/endpoints), read on 2026-09-10. It retains two real endpoint tags, parameters and prices; performance/health data and unrelated endpoints are omitted. Tests quote and send to a local HTTP stub only. Nonzero media prices are eligible under QUOTA r9: input token classes use a maximum-rate envelope, reasoning is inside output tokens, and per-image/request units are accounted separately. Optional search, explicit cache writes and generated media are never requested. This is not evidence of a successful paid live completion.

Final accounting uses reported cost, including zero. A nonuniform admission envelope cannot price aggregate tokens as measured usage. Unknown final cost stays unavailable, user debit stays below both approval and hold, and service overage is never transferred to the user. No historical usage is changed.

No generic video modality enables ordinary signed object URLs. An additional adapter profile needs its own current official delivery evidence and sanitized fixture; existing photo/text calls and model registration retain their prior behavior.

Run locally (no provider key or paid call): `cd backend && go test -race ./internal/llm/...`.
