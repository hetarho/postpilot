# Browser render comparison diagnostic

This retained runner compares the shared Canvas2D composition with Pixi caption scenes copied into the same Canvas2D footage/region composition. It is a hybrid diagnostic, not a whole-GPU compositor or the production video.worker path. Neither arm changes the production default.

Both arms use the same strict frozen plan, source decoder, audio processor, finite packet sink, bounded OPFS MP4 and producing-side verdict. Equal full final-Canvas readback is a separate diagnostic completion cost. Timings include that instrumentation and cannot establish ordinary production throughput or hardware acceleration. Warm retains font/WASM/layout/ink/Pixi caches while source decoders, audio work, encoders and spools restart.

Four checked-in authorized synthetic fixtures cover blur-in, neon, glitch and ember across all three ratios with three cut rates, dissolve/fadeblack, source-only audio and one synthetic mixed tone. They cover steady captions only. Windows/Edge, lower-resource devices, human listening/appearance review, the complete style/pace matrix and original WASM Rust provenance remain open. Upload is not executed and remains null. Managed allocation counts omit physical driver/process/private codec/DSP peaks. No raw PCM is retained; a diagnostic hash binds normalized pre-encode stereo samples. No human pixel threshold is invented.

Install the pinned workspace dependencies, use the pinned Node, then bind a clean exact commit:

```sh
node scripts/browser-render-comparison/static-check.mjs
node scripts/browser-render-comparison/runner.mjs --prepare --expected-head COMMIT
node scripts/browser-render-comparison/runner.mjs --execute --serial-slot-confirmed --expected-head COMMIT --case vertical-blur-in-source --output tmp/browser-render-comparison/single
```

Only after the single case succeeds, omit --case to execute the bounded four-case matrix. Run it with other expensive project checks stopped and record competing work when present. Every run preserves source/lock/fixture/adapter hashes, partial failures and unexecuted arms, output metadata/clock/audio/contract identity, sampled PNGs, named phases, logical peaks and cleanup. MP4 bytes are streamed to Node for SHA and discarded by default. --keep-mp4 retains them explicitly within the bounded diagnostic output.

Historical external execution at 26fce813 is separate from this portable recipe. Its four synthetic cases completed sixteen cold/warm runs; packaging changes to this runner are not falsely labeled as those already executed bytes. See the retained design report and source-binding evidence before making a new comparison. All activation/distribution/analysis/voice/hardware/human qualification gates remain false.
