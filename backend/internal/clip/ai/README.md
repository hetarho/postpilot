# Clip observation and planning boundary

`Service.ObserveChunk` makes exactly one `observe` call with one verified,
file-backed inline/static MP4. Relative integer times are clamped to that chunk,
then offset once. Every request carries the complete admitted T083 price policy.
`clip.MergeAnalyses` requires the complete manifest-ordered chunk sequence and
returns no partial result when an id, range, fingerprint or sequence is invalid.
The caller owns file lifetime and persistence; no cloud proxy is uploaded or signed.

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

The worker freezes `Service.Budgets()` into its actual `PlannedCall.CompletionTokens`:
8192 for each probed 60-second chunk and 32768 for the single composition call.
There is no provider call at enqueue time here. Every runtime request passes the
same explicit budget and stage through the metered registry; the worker owns credit
admission, settlement, durable stage progress and previous-result retention. Old
version-1 jobs and token-only price snapshots are refused. There is no automatic
retry. Preparation checks known recipe/answer/prompt bounds before reservation;
the planner checks complete structured input again without silently truncating it.

The [isolated release harness](../../../build/MEDIA.md#isolated-release-regression)
runs this adapter with real prepared media and a counted loopback HTTP completion
stub through authenticated approval, durable admission and settlement. Stub success
is evidence of the bounded wire/accounting contract, not of a live provider's
acceptance, visual understanding or billing dashboard.

Official API contracts rechecked on 2026-09-10:

- [Video input](https://openrouter.ai/docs/guides/overview/multimodal/videos):
  `video_url` supports compatible models, but arbitrary URL delivery varies by
  upstream provider; Gemini AI Studio documents YouTube-only URL input and Vertex
  documents no URL input. A `video_input` flag alone is not proof that a particular
  endpoint accepts a private-storage signed URL. Clip observation requires the
  adapter-owned inline/static profile; T083 streams the base64 request with bounded
  memory and pins the admitted endpoint, parameters, price limits and no fallback.
  Ordinary post-video URLs remain a separate, independently gated workflow.
- [Structured output](https://openrouter.ai/docs/guides/features/structured-outputs):
  the existing LLM adapter owns `response_format` and capability handling.
- [FFmpeg timeline editing](https://ffmpeg.org/ffmpeg-filters.html#Timeline-editing):
  overlay enable expressions receive only validated integer caption times.

Provider, malformed output and completion truncation retain their normalized LLM
reason and returned usage through `StageError`; no raw model output is included in
diagnostic text. `CLIP_COPY_TOO_LONG` remains the renderer's distinct failure.
