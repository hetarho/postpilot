# Browser media baseline

This diagnostic implements T588 under ARCH-60/66/67, CLIP-207 and CDS-104. SSOT remains authoritative. It does not activate a renderer or qualify AI analysis, real-device throughput, caption fidelity or listening quality.

## Reproduce

Use the pinned Node in `.node-version`, `pnpm install --frozen-lockfile`, and `pnpm exec playwright install chromium`. The fixture exporter uses the actual native runtime and only synthetic footage. It connects to no account, AI provider, database or object store.

```sh
docker build --target media-worker-smoke -t postpilot:browser-benchmark-fixtures -f backend/Dockerfile .
mkdir -p tmp/browser-media/native-bold
docker run --rm --network none --cpus 2 --memory 1g --user "$(id -u):$(id -g)" \
  -v "$PWD/tmp/browser-media/native-bold:/out" \
  -e CLIP_BROWSER_BENCHMARK_DIR=/out \
  -e CLIP_BROWSER_BENCHMARK_STYLES=bold \
  -e CLIP_BROWSER_BENCHMARK_RATIOS=vertical \
  --entrypoint /media.test postpilot:browser-benchmark-fixtures \
  -test.run '^TestBrowserBenchmarkFixtureExport$' -test.v -test.timeout 20m
pnpm benchmark:browser --fixtures tmp/browser-media/native-bold --output tmp/browser-media/run-bold --case vertical-bold
```

The output directory for native fixtures must be empty. Without the two selectors, the exporter prepares all 16 approved styles at all three ratios. Its stable 52-case catalog additionally names the 60-second all-style stress, fixed-rate/transitions, narration-only and mixed-audio recipes. Those four recipes remain explicitly unprepared until their owning implementations supply real fixtures. An unprepared case never counts as a successful render. Prepared per-style cases cover entrance, settled, effect-extreme and exit intervals; sequence styles use the actual native sheet frames.

On Linux without browser system libraries, `pnpm benchmark:browser:container` accepts the same flags. It uses `mcr.microsoft.com/playwright:v1.62.1-noble` only for system libraries, the locally locked Chromium cache and the exact host Node executable. This run is network-isolated, 2 CPU/1 GiB/256 MiB shared-memory and explicitly uses SwiftShader. It cannot certify GPU acceleration on customer devices. The normal harness binds only localhost, isolates Vite configuration from the application API proxy, and serves only manifest-owned artifacts with range and path/symlink checks.

The ordinary Vite production build has no benchmark HTML entry or Remotion import. Diagnostics run outside the application route tree. Optional phase metrics in the existing worker are off by default and do not alter the plan or saved result.

## Interpret the evidence

`browser-media-baseline-2026-10-06.json` records a real synthetic 15-second vertical bold-caption export with 450 frames at 1080×1920/30 fps. Cold and warm each run in the same fresh fixture context, so warm includes reuse within that context. The harness probes actual WebCodecs support, browser version and WebGL renderer. A `no-preference` WebCodecs configuration does not prove use of a hardware encoder.

| Observed milliseconds | Cold | Warm |
|---|---:|---:|
| Browser export elapsed | 146,614 | 145,778 |
| Awaiting source frames | 133,407 | 130,094 |
| Drawing command submission | 65 | 36 |
| Encoder flush waiting | 11,183 | 14,743 |

These are software-container observations. The phases may overlap and are not additive GPU/CPU execution times. Repeated source-frame waiting dominates this case; this supports investigating selected-range decode in T592. It does not prove that text effects on real GPUs are cheap. Layout/assets, source reading, video, audio, mux and nested worker spans are separate fields. Unexecuted production sampling/upload, absent audio and inaccessible worker/GPU memory remain `null`, rather than measured zero. Offline native asset preparation is a separate operation and is not added to browser elapsed time.

A successful file is demuxed, its dimensions/duration/frame count checked, three frame positions decoded, and bytes hashed. The current adapter saves the actual MP4 in local ignored output. The report is still `productionQualified: false`: demux/decode is not pixel, speech or semantic approval. This container supports the tested H.264 configuration but has no native AAC encoder. Audio cases must be refused in that environment until an explicitly qualified codec exists; this says nothing about a real Windows/macOS browser's support.

`canvas2d-current` uses the current product worker. `canvas2d-optimized` and `pixi-webgl` are registered adapter identities for T592/T594 onward; until supplied they report `not-implemented`. All adapters receive the same frozen fixture, ratios, duration and output verification contract. A failed, unsupported or missing-fixture outcome retains its reason and never becomes a passed benchmark. Later tasks must compare pixel/effect references and audio before interpreting throughput.

## Isolated Remotion control

The checked-in experiment supports the same prepared single-cut/static-ink fixture, output size/fps/bitrate, muted source and native static caption motion. It deliberately refuses sequence-sheet captions, audio and multiple cuts. It uses the official stable [web-renderer API](https://www.remotion.dev/docs/web-renderer). Keep its installation outside this workspace:

```sh
mkdir -p /tmp/postpilot-remotion-benchmark
cd /tmp/postpilot-remotion-benchmark
npm init -y
npm install --save-exact remotion@4.0.533 @remotion/web-renderer@4.0.533 @remotion/media@4.0.533 react@19 react-dom@19 vite@8
cd /path/to/postpilot
node scripts/build-remotion-experiment.mjs /tmp/postpilot-remotion-benchmark
pnpm benchmark:browser:container --fixtures tmp/browser-media/native-bold --output tmp/browser-media/run-remotion --case vertical-bold --renderer remotion-web-experiment --adapter-bundle tmp/browser-media/remotion-experiment/adapter.mjs
```

The build records exact SDK/transitive versions, lockfile hash and packaged license hashes. Those packages and optional codec binaries do not enter the product lockfile or production bundle. The experimental AAC/MP3/FLAC wrappers declare MPL-2.0 but embed FFmpeg/LAME/libFLAC respectively; their binary license/build/source obligations are separately unverified. Wrapper licensing alone cannot approve these binaries for a future browser distribution. Remotion's own Mediabunny version differs from the current product version; final codec profiles and pixel fidelity must be reviewed before calling the outputs quality-equivalent. Run throughput comparisons without overlapping builds/tests or competing render work; report `--competing-work` whenever that condition is not met.

Remotion has a [custom license](https://github.com/remotion-dev/remotion/blob/v4.0.533/LICENSE.md). The current individual/evaluation case is eligible under the stated free terms; a commercial organization larger than three employees requires company terms. Future organization size/use and license versions must be rechecked. Its [browser telemetry](https://www.remotion.dev/docs/telemetry) attempts to send render/licensing/domain/IP events even in development. The diagnostic container denies external networking; no telemetry receipt is claimed. A production adoption needs commercial-term and privacy review and cannot reuse an evaluation decision as a permanent exemption. No purchased SDK or AI calls run here.

The actual isolated 4.0.533 run in `browser-media-remotion-2026-10-06.json` is rejected in both cold and warm runs: its last requested frame time is unavailable to the common product-version decoder. Native ffprobe independently sees 450 H.264 High frames at 1080×1920, but the track duration is 14.966667 seconds instead of 15, with average frame rate 13500/449. The current file independently reports exactly 15 seconds, 450 frames and 30/1. This is one observed synthetic case, not a universal claim about the SDK. Competing frontend checks are explicitly recorded, so these experiment times are not a controlled speed comparison. No successful/pixel-equivalent Remotion delivery is claimed. The harness now disables file watching to prevent edits from restarting an active page and has a four-minute per-attempt deadline.

## License and source evidence

`pnpm licenses:browser-media --verify-upstream --output docs/design/browser-media-licenses.json` records actual locked packages, notice/source hashes and font license paths. The script's tests cover missing/unverified evidence. Exact top-level versions are Mediabunny 1.58.0, PixiJS 8.22.0, Pixi Filters 6.1.5, resvg-wasm 2.6.2 and SoundTouchJS core 2.1.1. Inspect the JSON after any dependency change.

Mediabunny, resvg-wasm and this SoundTouchJS core version declare MPL-2.0; Pixi and Filters declare MIT; bundled fonts have OFL-1.1 notices. Preserve the actual notice texts and provide access to the corresponding covered source and modifications for delivered minified JS/WASM. MPL obligations apply at the covered-file level; see the [Mozilla FAQ](https://www.mozilla.org/en-US/MPL/2.0/FAQ/). Separate proprietary application files are not automatically covered. The inventory is evidence for release packaging, not the delivered notices/source-access surface itself. T604 must verify that surface.

The resvg-wasm tag declares patched Rust resvg **0.34.0**, revision **3495d870**, whereas the native reference is **0.48.1**. The published WASM binary's complete Rust dependency graph/build provenance remains unverified and is recorded as such. Do not treat the packages as pixel-equivalent. T594 must select a compatible qualified build or demonstrate the intended typography/geometry; T604 must complete binary license/source review before activation. Native FFmpeg/libx264 licensing is a separate server-image obligation and is not transferred into this browser dependency inventory.
