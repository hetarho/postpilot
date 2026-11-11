# Browser render comparison diagnostic

This diagnostic compares three arms: `canvas2d-shared`, `pixi-scene-canvas2d-hybrid` and `pixi-webgl-compositor`. The hybrid copies Pixi caption scenes into Canvas2D footage/regions. The whole-frame arm submits footage, region ink/scrims and the existing Pixi caption builders to one full-frame Pixi WebGL render target on the Worker's OffscreenCanvas. The production video.worker path and default stay unchanged.

All arms use the same strict frozen plan, frame evaluator, source decoder, background/component builders, audio processor, finite packet sink, bounded OPFS MP4 and producing-side verdict. The whole-frame paint port uploads and destroys each transient source texture before its decoded-frame lease closes. It reuses the existing native fadeblack GPU helper; it does not composite a full frame on Canvas2D. WebGL2 absence records the arm as unexecuted, with no Canvas fallback.

Every arm pays full final-image diagnostic readback. The WebGL arm additionally reports row-normalization's second logical buffer. Timings include these costs and cannot establish ordinary production throughput or hardware acceleration. The actual Worker WebGL renderer string and software classification are retained; a masked renderer remains unknown. Warm retains font/WASM/layout/ink/Pixi caches while source decoders, audio work, encoders and spools restart.

Four checked-in authorized synthetic fixtures cover blur-in, neon, glitch and ember across all three ratios with three cut rates, dissolve/fadeblack, source-only audio and one synthetic mixed tone. They cover steady captions only. Windows/Edge, lower-resource devices, human listening/appearance review, the complete style/pace matrix and original WASM Rust provenance remain open. Upload is not executed and remains null. Managed allocation counts omit physical driver/process/private codec/DSP peaks. No raw PCM is retained; a diagnostic hash binds normalized pre-encode stereo samples. No human pixel threshold is invented.

Install the pinned workspace dependencies, use the pinned Node and installed Playwright Chromium (the default), then bind a clean exact commit. `--chrome PATH` explicitly selects another installed browser and is recorded in each result:

```sh
node scripts/browser-render-comparison/static-check.mjs
node scripts/browser-render-comparison/runner.mjs --prepare --expected-head COMMIT
node scripts/browser-render-comparison/runner.mjs --execute --serial-slot-confirmed --expected-head COMMIT --case vertical-blur-in-source --output tmp/browser-render-comparison/single
```

Only after the single case succeeds, omit --case to execute the bounded four-case matrix. Run it with other expensive project checks stopped and record competing work when present. Every run preserves source/lock/fixture/adapter hashes, partial failures and unexecuted arms, output metadata/clock/audio/contract identity, sampled PNGs, named phases, logical peaks and cleanup. MP4 bytes are streamed to Node for SHA and discarded by default. --keep-mp4 retains them explicitly within the bounded diagnostic output.

Historical external execution at 26fce813 is separate from this portable recipe. Its four synthetic cases completed sixteen cold/warm runs; packaging changes to this runner are not falsely labeled as those already executed bytes. See the retained design report and source-binding evidence before making a new comparison. All activation/distribution/analysis/voice/hardware/human qualification gates remain false.
