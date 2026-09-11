# Synthetic multi-source capture

Captured on 2026-09-11 during the owner's T096 verification approval (strictly
less than USD 0.10 in aggregate). No production user footage or private model
output is included. Source ids, fingerprints, events and captions belong only to
FFmpeg-generated moving test patterns with sine tones.

- Eight sources: requested durations 4290, 3744, 1480, 5010, 5108, 4508, 5428,
  6702 ms; 1440x1920 except source 6 at 1920x1440. Fixture input records probed
  media durations, which can differ slightly due to frame/container rounding.
- Model for observation and composition: `openrouter/google/gemini-3.8-flash`;
  low reasoning, production completion budgets 8192/32768, no fallback or retry.
- Eight actual observations plus one actual general-guidance composition used
  USD 0.008697 reported total. The next composition reused these observations
  with the failing attempt's `일상` / `균등분할` / diary / amber settings; its
  additional reported cost was USD 0.002360. Total: USD 0.011057 (11057 micro-USD).
- Both live compositions selected four source ranges. Both rendered from the
  original synthetic footage into 15-second 1080x1920 H.264/AAC MP4s under the
  nonroot, 2-CPU, 1-GiB runtime; complete decoding and Korean-caption frame
  inspection passed. The eight-cut regression is a separate deterministic case,
  not a claim that the live model selected all eight sources.
- `multi-source.json` contains both unchanged `llm.Response` values and the
  equal-split planning input. Transport pricing policy and media metadata unused
  by planning validation are omitted. The first response was obtained with
  general guidance, then additionally validated against the same style/ratio
  constraints in this fixture; it is not a second equal-split live call.
- The ordinary test replays these responses through the parser and a fake
  caption sizer. Actual glyph measurement, rendering and visual inspection are
  separate live-run evidence, not performed by this unit test.

These successful synthetic captures do not reproduce or establish the cause of
the owner's earlier `MODEL_OUTPUT_INVALID`; its rejected output was not retained.
Temporary run artifacts and the conservative per-call budget ledger are at
`/tmp/postpilot-t096.HRFfRS` on the verification host, not shipped as project assets.
