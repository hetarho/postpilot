# Whole-frame Pixi WebGL comparison (T749)

At source commit `1a603269363ce06cf7896188e29af072913d36f6`, the single-case smoke completed all six runs,
then the four-case matrix completed all 24 cold/warm runs and 16 control/candidate
comparisons. All producing-side output verdicts, frozen snapshot/source clock/
component/background contracts and normalized PCM comparisons passed. All owned
frame/encoder/output/Worker cleanup checks passed; OPFS temporary names were empty.
Source, adapter, dependency metadata and fixture bindings remained unchanged.

The full observations and phase/resource fields are appended under
`wholeFramePixiComparisons` in [the retained report](../design/browser-render-comparison-current.json).
Its older 16 runs, eight comparisons, source hashes and portable/history records
are preserved. The preliminary single case is retained separately from the matrix.

## Identified device and implementation

- MacBookPro18,2; Apple M1 Max; 34359738368 bytes RAM; macOS 26.5.2, build 25F84.
- Playwright Chromium 153.0.8010.12, headless, default arguments plus the existing precise-memory flag.
- Actual Worker WebGL2 renderer: `ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Max, Unspecified Version)`; unmasked=true, softwareRasterizer=false.
- This is an identified high-resource Mac observation. It supplies no Windows integrated-GPU or lower-resource result, hardware-codec proof or physical GPU/process peak.

The diagnostic arm uses a full-frame Pixi RenderTexture on the Worker's
OffscreenCanvas. A small paint port lets the existing frame evaluator and
measured region/scrim/ink builders submit their operations to WebGL. Decoded
VideoFrame textures are drawn and destroyed before their presentation leases
close. Captions use the existing Pixi scene/filter/ember builders. The existing
native fadeblack helper remains a GPU subpass, shared with the control; no
whole-frame Canvas2D composition enters the new arm. Font/WASM/layout/background
preparation, decode/audio/packet/OPFS/mux/verdict contracts stay shared.
The production `frontend/src/entities/clip-preview/lib/video.worker.ts` and all
other production frontend code are unchanged.

The implementation follows the locked Pixi 8.22.0
[ImageSource API](https://pixijs.download/v8.22.0/docs/rendering.ImageSource.html)
for VideoFrame uploads and
[RenderTexture API](https://pixijs.download/v8.22.0/docs/rendering.RenderTexture.html)
for the frame target. A browser without WebGL2 records the arm as unexecuted;
there is no Canvas fallback. The actual renderer string is recorded separately
from API availability; a masked renderer is classified as unknown.

## Observed times

The matrix's whole-frame arm used 13.2–24.5% less observed preparation-plus-assembly
time than Canvas2D in these eight measurements. The primary metric is
`sequentialPreparationPlusWorkerAssemblyMs`: main preparation plus worker
assembly through mux finalization. It includes full-frame readback, selected
pre-encode PNGs and in-loop contract diagnostics; later decoded-output checks and
Node artifact transfer are excluded. Every named phase remains in the raw report.

| Case | Canvas cold/warm (s) | Hybrid cold/warm (s) | Whole WebGL cold/warm (s) | Whole WebGL shorter cold/warm |
| --- | --- | --- | --- | --- |
| vertical-blur-in-source | 8.720 / 8.647 | 8.931 / 8.739 | 6.789 / 6.707 | 22.1% / 22.4% |
| horizontal-neon-source | 9.350 / 8.868 | 9.573 / 9.417 | 7.130 / 7.007 | 23.8% / 21.0% |
| square-glitch-source | 7.109 / 6.240 | 6.819 / 6.601 | 5.765 / 5.416 | 18.9% / 13.2% |
| vertical-ember-mixed-synthetic | 9.957 / 9.092 | 9.822 / 9.229 | 7.517 / 7.186 | 24.5% / 21.0% |

Each fixture has one cold/warm pair, in fixed arm order. Cold means a fresh
browser/Worker realm, not flushed OS/driver caches; warm retains the declared
font/WASM/layout/ink/Pixi runtime, restarting decoders/audio/encoders/spools.
There is no statistical or uninstrumented-production throughput claim.

## Pixel differences and output checks

| Case | Max pre-encode mean error | Max decoded mean error | Max individual channel error |
| --- | --- | --- | --- |
| vertical-blur-in-source | 0.367219 | 0.660449 | 51 |
| horizontal-neon-source | 0.343020 | 0.738510 | 86 |
| square-glitch-source | 0.347333 | 0.932307 | 255 |
| vertical-ember-mixed-synthetic | 0.367219 | 0.660449 | 47 |

These are maxima over sampled frames, with mean error over RGBA channels on the
0–255 scale. Cold/warm sampled residuals were identical. Output metadata, clock,
H.264/AAC and loudness verdicts pass, but pixel equality is not asserted. No human
acceptance threshold is defined or met by assumption: `pixelAcceptanceThreshold`
is null and human review/qualification remain false. The fixtures contain steady
captions and shared scrim/ink drawing; the full region/style/pace/extrema matrix
and appearance acceptance remain T604 work.

## Resource observations

| Logical peak across four cases/temperatures (bytes) | Canvas | Hybrid | Whole WebGL |
| --- | --- | --- | --- |
| cpuRasterCacheBytes | 3270960 | 3270960 | 3270960 |
| cpuCanvasScratchBytes | 18282240 | 0 | 0 |
| gpuManagedTextureBytes | 0 | 18175332 | 26469736 |
| finalCanvasLogicalBytes | 8294400 | 8294400 | 8294400 |
| diagnosticReadbackBytes | 8294400 | 8294400 | 16588800 |
| decoderReservedBytes | 22118400 | 22118400 | 22118400 |
| decodedFrameBytes | 460800 | 460800 | 460800 |
| nativeFadeBlackGpuTextureLowerBoundBytes | 8294400 | 8294400 | 8294400 |

The whole-frame arm additionally records its full-frame target, transient-texture
peak and renderer-managed peak in `managedResources.wholeFramePixi`. Its maximum
transient upload includes the shared fadeblack canvas; it is not all decoded
footage retained at once. These overlapping logical peaks must not be summed as
a physical/process peak. The WebGL readback reports both the raw and row-normalized
buffers; the Canvas control reports one. The control also uses the shared native
fadeblack GPU helper, represented separately above. Physical GPU/driver/MSAA,
Worker/font/WASM/private codec and DSP peaks remain unknown/null.

The source is the checked-in, authorized 320×180/30-second synthetic fixture;
the outputs are 15 seconds/450 frames at the three delivery ratios, with source
sound and one synthetic mixed tone. These results do not represent 4K owner
footage, real narration, upload or a live storage/deployment test. No raw PCM or
owner footage is retained. Production code keeps its current default. Distribution approval and
hardware/human qualification remain false.

## Reproduce

Use the pinned Node and installed locked dependencies. Replace COMMIT with a
clean exact revision; the recorded run used `1a603269363ce06cf7896188e29af072913d36f6`. Stop other expensive
local project checks first. The original artifacts are retained in this session
under `.git/haeram/v1/launchers/cr11/T749-single` and `T749-matrix`; future reruns
need their own source binding and observations.

```sh
node scripts/browser-render-comparison/static-check.mjs
node scripts/browser-render-comparison/runner.mjs --prepare --expected-head COMMIT --binding .git/haeram/v1/launchers/cr11/T749-binding.json
node scripts/browser-render-comparison/runner.mjs --execute --serial-slot-confirmed --expected-head COMMIT --binding .git/haeram/v1/launchers/cr11/T749-binding.json --case vertical-blur-in-source --output .git/haeram/v1/launchers/cr11/T749-single
node scripts/browser-render-comparison/runner.mjs --execute --serial-slot-confirmed --expected-head COMMIT --binding .git/haeram/v1/launchers/cr11/T749-binding.json --output .git/haeram/v1/launchers/cr11/T749-matrix
pnpm run lint
pnpm run format:check
```

The diagnostic's own TypeScript/syntax diagnostics and all shared-arm/fixture/
pipeline guards pass. The retained checker reports imported-source diagnostics
outside its app-project scope separately; the real project's type/build and
full CI checks are the independent publication gate. Logs: single
`check-999d4892-eb83-4b10-9f14-7c6a78f6d3ad.log` (78060ms), matrix
`check-fd593d9c-a424-4c0c-9ff1-0e04f590cd2a.log` (265202ms), static
`check-2a96452c-d8eb-4682-a8cf-7ba149565c6e.log` and lint/format
`check-2a82dba0-5d80-4d02-a883-c56061f13f79.log`.
