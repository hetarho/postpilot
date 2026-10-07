# Browser rendering and static delivery

The local technical delivery is reviewable. Final rendering, browser analysis preparation, real voice and distribution activation remain disabled. No live deployment, domain purchase, provider call or desktop installation was performed.

## Reproduce the static checks

Use the pinned Node and installed locked workspace dependencies. Cloudflare remains a static-assets Vite SPA; these commands introduce no production Node service.

```sh
pnpm install --offline --frozen-lockfile --ignore-scripts
pnpm build:web
node scripts/build-browser-media-distribution.mjs --report tmp/browser-media-emitted-bundle.json
node scripts/browser-static-deployment-check.mjs --fixture docs/design/fixtures/browser-render-comparison/source-with-audio.mp4 --bundle-inventory tmp/browser-media-emitted-bundle.json --output tmp/browser-static-delivery
```

The diagnostic creates private, temporary loopback TLS credentials and serves the built artifact. Its self-signed certificate and `ignoreHTTPSErrors` establish a local secure-context exercise, not public-domain TLS or a live Cloudflare/R2 deployment result. The local `_headers` interpreter is separately disclosed. Current checks execute both built export/preview module Workers through their named rejection fences, compile the exact bundled WASM, load all eight verified FontFaces and fetch the original notice/source-manifest bodies.

The mock private storage applies the configured origin, method, requested-header and exposed-header policy. Only the explicitly configured production origin maps to the app's loopback origin. Real browser tests exercise exact206 bounded bytes and reject hidden Content-Range, wrong offsets, whole200 responses and an unmapped foreign origin. Policy variants separately reject wildcard-origin release scope, foreign-only origin, missingGET, missingRange preflight admission and missing exposed Content-Range. A single Range GET is CORS-safelisted; the explicit OPTIONS test checks the configured AllowedHeaders contract separately.

`deploy/r2-cors.json` preserves conditional PUT and owner-origin scoping while allowing Range and exposing Content-Range. The deployment workflow checks Range preflight admission. Neither a preflight nor this mock origin certifies a production private-object206 transfer. Live rollout remains a separate operator action.

## Notices and covered source

All fingerprinted JS/WASM assets carry Link headers pointing to `/licenses/browser-media/index.html` and `/licenses/browser-media/source-access.json`. The page exposes twenty original dependency notices, the combined text, exact-version MPL preferred-source paths and existing OFL/renamed-ink notices. The emitted Vite inventory distinguishes actual main/Worker modules from type-only installed dependencies. Source and served-body hashes remain reviewable.

The original npm resvg2.6.2 WASM still lacks a verified complete original Rust dependency graph. Known wrapper/core preferred sources do not reconstruct it. The independently rebuilt candidate is unapplied and is not attributed to this binary. The source-access/distribution flags remain false; fetching upstream license text alone no longer claims complete source access. Missing original/candidate notice evidence is retained rather than inventing a copyright holder or substituting a standard template for an original notice.

## Paired Mac observations

`docs/design/browser-render-comparison-current.json` records Apple M1 Max, MacBookPro18,2, 32GiB, macOS26.5.2/build25F84 and Chrome154. Four synthetic fifteen-second,450-frame cases cover blur-in/neon/glitch/ember, three ratios, three cut rates, dissolve/fadeblack, source sound and one mixed synthetic tone. Sixteen cold/warm runs completed, with eight matching frozen/source-clock/component/background contracts, normalized pre-encode PCM hashes and AAC packet identities. Output bounds and OPFS/Worker cleanup passed.

The arms are the shared Canvas2D composition and Pixi caption scenes copied into that Canvas2D composition. This is a hybrid diagnostic with equal full final-Canvas readback, not a complete GPU compositor or the production video.worker path. Prepare/draw/copy/completion, audio, encoder, packet/spool and managed allocation phases are retained; physical driver/process/private codec/DSP peaks and hardware codec execution remain unknown. Upload is unexecuted/null. Timing instrumentation and diagnostic copies are not ordinary production throughput.

Sampled pixels have measured residuals; byte identity and a human acceptance threshold are not asserted. Decoded maximum mean-channel residuals on0–255 are about 0.0507 blur, 0.2096 neon, 0.3153 glitch and 0.1233 ember. No default changed on these observations. Historical reports remain distinct from current measurements.

The checked-in diagnostic/fixtures are runnable. Its actual portable single-case smoke at d029abea completed all four cold/warm arm runs with unchanged source/adapter/fixture bytes and clean output disposal; this is separate from the sixteen-run historical26fce813 report:

```sh
node scripts/browser-render-comparison/static-check.mjs
node scripts/browser-render-comparison/runner.mjs --prepare --expected-head COMMIT
node scripts/browser-render-comparison/runner.mjs --execute --serial-slot-confirmed --expected-head COMMIT --case vertical-blur-in-source --output tmp/browser-render-comparison/single
```

After a successful single case, omit `--case` for the bounded four-case matrix. Keep source/lock/fixture/adapter binding intact. Portable path changes are disclosed separately from the historical external26fce813 execution; the prior Worker environment failure and unexecuted warm arm remain preserved.

## Independent gates

The versioned readiness record binds renderer/component/font/asset/WASM identity, empty missing gates, explicit enablement and distribution approval. Analysis preparation has its own disabled record and hardfalse backend gate; final rendering cannot authorize analysis or real voice.

Still absent: a reviewed complete original WASM source/license graph, physical Windows integrated-GPU/Edge and lower-resource runs, full release-output/style/pace/extreme human review, uninstrumented production/default performance qualification, actual semantic corpus/current-provider comparison with approved spend and human truth, and real voice qualification. T539/T550 and T603 remain closed independently. Missing evidence keeps release acceptance open.

`browser-static-release-checks.json` binds current local checks and exact-source reuse. The final deploy Python62 and dev11 checks passed; an earlier300-second outer timeout remains a separate failure. Native production bytes and generator inputs/outputs retain the actual full-Go, nonroot-image and bounded-operation proofs; new gated126-layout fixtures and current CORS/workflow/harness checks are separately executed.
