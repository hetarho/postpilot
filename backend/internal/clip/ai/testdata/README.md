# Synthetic live clip contract regression

Captured on 2026-09-11 KST using `google/gemini-3.8-flash` through OpenRouter, with no user footage, production job or account ledger mutation.

- Input: generated 15-second 320x180 H.264/AAC test bars and a 440 Hz tone; SHA-256 `de8b65ed11437dac76df7dbe0ce4756828a2baefae76cdcdf147e1637688b01e`.
- The original observation `json_schema` returned HTTP 400 / `INVALID_ARGUMENT`. Removing only string-length constraints also returned 400. Removing response_format returned 200, but not bare JSON. Keeping the closed structural schema while omitting size/range bounds returned 200 and valid observation JSON.
- `live-observation.json`: generation `gen-1789059164-JxDNetbGD5fjFCdkxbdf`, 8192 output-token ceiling, reported cost rounded up to 1164 micro-USD.
- `live-plan.json`: generation `gen-1789059391-EqRHgOB6tB1GygNXsEnc`, reported cost rounded up to 725 micro-USD. Reused the captured observation rather than buying another analysis. The approved diagnostic budget required a test-only 16384 output-token ceiling; production remains 32768. The full production prompt and the corrected structural schema were used.
- Production media preparation, analysis parsing/merge, planning validation, caption measurement and original-footage rendering ran inside the nonroot production runtime with 2 CPUs / 1 GiB. Output: 15 seconds, 1920x1080, 30 fps, H.264 yuv420p and stereo AAC 48 kHz, Korean caption `영상 생성 확인` from 1 to 5 seconds; 4,118,052 bytes. This was not a deployed browser/queue smoke or Naver upload.

The [Gemini structured-output contract](https://ai.google.dev/gemini-api/docs/structured-output#limitations) documents a schema subset and rejection of complex schemas. The [OpenRouter structured-output guide](https://openrouter.ai/docs/guides/features/structured-outputs) documents `json_schema` routing. The live experiment isolates rejection of our constrained output schema; it does not identify a single offending keyword or prove every provider has the same limitation.

Production now sends only type, properties, required, items, enum and additionalProperties to the output grammar. The full embedded contracts remain in the prompts, and server parsing still checks counts, ranges, string lengths, source grounding, enums and timing before rendering. Non-clip requests, model choices, pricing, completion ceilings and retry policies are unchanged.

Both recorded responses were MECHANICALLY migrated when T105 moved style and
position out of the model's hands: each segment's keep-out box became the
principal-subject box it was already measuring, `scene` and `readable_text` took
their documented defaults, and each caption lost the position, style and accent
the contract no longer asks for and gained `short_text` and `keyword`. Nothing
else was edited, and no new paid call was made — the fixtures still pin the
parser against real provider output, not against a hand-written response.

These text fixtures can be replayed locally without a credential or a paid call: `cd backend && go test ./internal/clip/ai`. Exploratory live-call code and generated media are not shipped in the repository or run by CI.
