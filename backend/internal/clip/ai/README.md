# Clip observation and planning boundary

`Service.ObserveChunk` makes exactly one `observe` call with one freshly signed
proxy URL. Relative integer times are clamped to that chunk, then offset once.
`clip.MergeAnalyses` requires the complete manifest-ordered chunk sequence and
returns no partial result when an id, range, fingerprint or sequence is invalid.
The caller owns signing, proxy cleanup and persistence; none happens in this adapter.

`Service.Plan` makes exactly one `write` call with the frozen recipe, exact answers,
ratio, target and merged facts. It accepts no source pixels or URLs. Closed schemas
are always included in the system contract and additionally sent as `JSONSchema`
only for a registered structured-output model. Both paths use the existing shared
JSON-object fallback; they never issue a repair call or choose a fallback model.

Caption time is relative to its trimmed cut. The renderer applies that interval;
legacy whole-cut captions with both time fields zero retain their behavior.
Real bundled-font glyph measurement drives placement. Source-space avoidance boxes
are projected through the focal cover crop and can move copy only among the three
approved positions. Manually edited plans do not invoke this automatic placement.

T076 must freeze `Service.Budgets()` into its actual `PlannedCall.CompletionTokens`:
8192 for each probed 60-second chunk and 32768 for the single composition call.
There is no provider call at enqueue time here. Every runtime request passes the
same explicit budget and stage through the metered registry; the worker owns credit
admission, settlement, durable stage progress, previous-result retention and retry.

Official API contracts rechecked on 2026-09-10:

- [Video input](https://openrouter.ai/docs/guides/overview/multimodal/videos):
  `video_url` supports compatible models, but arbitrary URL delivery varies by
  upstream provider; Gemini AI Studio documents YouTube-only URL input and Vertex
  documents no URL input. A `video_input` flag alone is not proof that a particular
  endpoint accepts a private-storage signed URL. Unsupported delivery must fail;
  this adapter never uploads bytes inline or substitutes a different model.
- [Structured output](https://openrouter.ai/docs/guides/features/structured-outputs):
  the existing LLM adapter owns `response_format` and capability handling.
- [FFmpeg timeline editing](https://ffmpeg.org/ffmpeg-filters.html#Timeline-editing):
  overlay enable expressions receive only validated integer caption times.

Provider, malformed output and completion truncation retain their normalized LLM
reason and returned usage through `StageError`; no raw model output is included in
diagnostic text. `CLIP_COPY_TOO_LONG` remains the renderer's distinct failure.
