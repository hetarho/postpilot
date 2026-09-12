# Clip observation and planning boundary

`Service.ObserveChunk` makes exactly one `observe` call with one verified,
file-backed inline/static MP4. Relative integer times are clamped to that chunk,
then offset once. Every request carries the complete admitted T083 price policy.
`clip.MergeAnalyses` requires the complete manifest-ordered chunk sequence and
returns no partial result when an id, range, fingerprint or sequence is invalid.
The caller owns file lifetime and persistence; no cloud proxy is uploaded or signed.

`Service.Plan` makes exactly one `write` call with the frozen recipe, exact answers,
ratio, target and merged facts. It accepts no source pixels or URLs. Closed schemas
are checked locally and supplied through the prompt or `JSONSchema` for a registered
structured-output model. Both paths use the existing shared
JSON-object fallback; they never issue a repair call or choose a fallback model.

Native compositions use `composition-plan.schema.json` in that same writer stage.
The frozen XML defines section order, viewpoint, repeated item groups and every
visible element. The server resolves fixed/answer-bound text exactly and accepts
generated entries only for declared AI elements. It preserves verified shorter
alternatives from that response for the renderer; it never requests semantic repair.
The plain native prompt includes the compact full contract. Structured requests
carry its closed object grammar once in `JSONSchema` and retain every domain bound
in the prompt. This preserves all observations at the 20-source/49-chunk ceiling
without increasing the reserved input limit or paying for a summarization call.

Every cut retains overlapping observation references. Item identity is established
by an owner range association or a unique supplied name/alias in every overlapping
observation, with identity checked again after timeline resizing. Optional field
IDs `name`, `alias` and `aliases` supply automatic identity hints; arbitrary fields
remain valid and can use owner associations. Filenames, shared digits, generic
subjects, model-proposed item IDs and uncertain observations do not establish a
match. Unknown item bindings omit dependent elements and retain a typed reason.

Generated numeric claims compare complete number/unit/currency/basis tokens against
referenced facts in the bound item; global facts require a declared context section.
Experiential phrases require exact owner-supplied support, so paraphrases can be
conservatively omitted. These finite checks and traceable references support the
scene-by-scene semantic QA in T132; they do not prove arbitrary prose true. No
sentence is admitted by concatenating digits across answers. Blank optional values
omit dependent text, and repeated generated captions are removed without rewriting
fixed content. Native plans retain version-5 geometry, authored declarations, text,
source ranges, scoped facts and omission reasons. Placement belongs to the renderer.

Every retained composition, including an explicitly converted legacy template,
uses this contract. The old writer remains only for queued payloads without a
composition snapshot. Planner and renderer both advertise version 5; the worker
renders and persists the owned plan without injecting live preset furniture.

The renderer applies output text after source transitions, using bounded overlay
batches and lossless intermediates before one final lossy video encode. Its shared
manifest reports element identity, authority, effective style/placement, phrase
windows, measured bounds and omission reasons. Completed automatic choices are
retained with the plan, so rerendering cannot silently rewrite an accepted sentence.

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

## Private failure diagnostics

Failed clip jobs log `job`, `kind`, `reason`, and the known clip `stage`. Strict
provider failures additionally carry `operation` (preflight, metadata, body,
transport or response), a code-owned `error_class`, and available `http_status`,
numeric `upstream_code` and validated `request_id`. In-stream errors can have
HTTP 200 with a different upstream status. IDs come only from the response's
`X-Request-ID`, `Request-ID` or `CF-Ray`; unknown formats and reflected credentials
are omitted. No ID is invented when none was received.

No provider prose, raw errors, request/response bodies, prompts, media paths, data
URLs, authorization headers or arbitrary header values are logged. These fields
are server-only and do not change public error messages, usage evidence, credit
settlement or retry behavior. Use the job ID to join the existing stage/accounting
records; a missing usage record is not proof that the provider billed nothing.
