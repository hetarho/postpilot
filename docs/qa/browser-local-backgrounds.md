# Browser original background measurements

T598 resolves the frozen native glyph bounds before paint and samples original footage only. Caption and information entries use their glyph union; an intro/outro entry uses its complete block text bounds. Each visual reads the native first/middle/last output frame indexes. The existing original decoder applies the frozen rate, focal crop and transition timeline. Compatible frame/ROI requests share decoded resources and region means.

RGB is averaged on every second region pixel before WCAG/Rec.709 luminance is calculated. Mean and population sigma describe the three temporal samples. Dissolves mix the two RGB means; fade-through-black clips each sample through the native limited-YUV calculation. Native CPU-v4 and browser sequence paths both consume sampled ground. Every unplated caption takes its backdrop even when an old style catalog flag omitted it; intrinsic sequence plates keep their own paint and require no ground reads. The sequence raster crop includes its bounded band. Native scrim geometry, accent-to-white and effective stroke/paint contrast determine the existing `composition_contrast` notices. A component draws its scrim immediately before its own ink in native overlay order.

The sampler permits at most 2,400 reads, 8 MiB per region and 24 MiB of live sampled pixels. It never reads an entire export frame on every encoded frame. Region canvases, decoded frame leases, reader/decoder reservations and font/ink services are released on success, cancellation, revision supersession and refusal. Fade-black corrects the producing canvas with one bounded GPU surface/texture; no per-export-frame CPU pixel readback is introduced.

Both sampling and final footage use `native-source-color-v1`. An untagged YUV original receives the native limited-range BT.601 default through the decoder configuration; explicit supported SDR matrices/ranges are retained. This changes the producing frame as well as the sampled frame. Unsupported HDR/gamut/matrix tags refuse with `CLIP_SOURCE_COLOR_UNSUPPORTED`. WebCodecs defines decoder color-space metadata as an override of stream values. [WebCodecs specification](https://w3c.github.io/webcodecs/#videodecoderconfig)

`StartClipBrowserCompositionRender` preserves the existing authoritative owner/revision/source admission and closed qualification gate. It returns the server-authored composition binding and renderer/component/font/asset identities without a media sampling job. Snapshot export accepts `assets: []` and renders local components; server rasters/frame sheets cannot supply its background evidence. The producing verdict carries a finite background version, admission fingerprint, digest, complete flag, three-frame sample count and bounded contrast notices. Missing or foreign evidence fails before PUT. An accepted verdict cannot change its digest on retry; existing transactional revision/cancellation fences govern upload promotion.

These measurements retain browser-client provenance. The server bounds and binds the report; it does not claim to have independently decoded the original or proved client rendering semantics. Copy safety, native original verification and release qualification are separate claims.

## Reproduction

From a clean committed task checkout with the locked dependencies, real Chrome and FFmpeg/ffprobe installed:

```sh
node scripts/browser-background-check.mjs --ffmpeg /opt/homebrew/bin/ffmpeg --ffprobe /opt/homebrew/bin/ffprobe --output /private/tmp/postpilot-browser-background-check
cd backend
POSTPILOT_BROWSER_BACKGROUND_PROOF=/private/tmp/postpilot-browser-background-check/report.json go test ./internal/clip/media -run '^TestBrowserOriginalBackgroundProof$' -count=1
```

The first command generates tiny original videos, serves finite local GET ranges, runs the production browser source adapter and ROI sampler, and records actual native decoded reference frames. The second compares the report through the native ground sampler's timeline, mean/sigma, scrim/accent thresholds and contrast rules. It does not substitute browser math for the native reference.

The current controlled matrix covers vertical/square/horizontal and caption/info/hook/ending roles; dark, bright, temporal-noise and both sides of the 0.6 threshold; dissolve/fade-black cross-cut windows; untagged BT.601, tagged limited/full BT.709; and HDR, expired access and HTTP-200 range refusals. GPU controls cover thirty orientation/color/fade weights. Actual Chrome 154.0.8037.98 and host FFmpeg 9.0.1 matched all sampled native values/decisions/notices within 1e-7. The GPU controls differed by at most one 8-bit channel value. Tagged red also retains a documented one-channel-value native/browser decode rounding difference.

The report remains `qualification: false`: these are bounded regression controls, not a general color/device/runtime qualification or permission for paid provider calls. T599/T600 connect the new admission/snapshot export to product orchestration and refusal presentation; T604 owns final runtime/quality qualification. Legacy/native sampling routes remain available under their existing contracts.

The combined component identity is `native-cds-r33-pop-exposure-v2-ground-v1`; ink asset identity retains the approved T595 hash. Native execution changes to `cpu-v4`, retaining contract version 3 and the distinct analysis-verification operation/profile. Background evidence also carries the client frozen snapshot hash: changing local layout under the same admission cannot reuse another snapshot's evidence or leased components.

Matching CPU-v4 full image and twelve native/background scene PNG comparisons are pending until the clean combined image is built; earlier CPU-v3 evidence is not substituted.
