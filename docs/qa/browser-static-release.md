# Browser rendering and static delivery

The local technical delivery is reviewable. Browser final rendering is active under ARCH-68; complete real-device, output and source/license qualification remains open. Browser analysis preparation and real voice remain disabled, and distribution approval remains with the owner. No live deployment, domain purchase, provider call or desktop installation was performed.

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

## Original resvg WASM Rust investigation (T748)

The npm package remains `@resvg/resvg-wasm@2.6.2`, with its original 2,478,606-byte
WASM SHA-256 `22bf6e9f9a100d972da0411a69c5ba504367fc1fa87b3b64e3f35e53926d2d70`.
The separately rebuilt candidate remains unapplied. No Cargo installation,
replacement lockfile, license template or invented copyright holder is used.

The exact npm gitHead `9ca058462ac529120c8cc84ddcd6fef644cc5406` contains **no
Cargo.lock anywhere in its Git tree**; `.gitignore` explicitly excludes it.
Its root Cargo.toml declares the Rust wrapper as `resvg-js@1.0.0`, which is not
the npm version. Common/wasm-target dependency names and version ranges do not
resolve features, transitive versions or licenses. The Rust `rustGraph` is
therefore `[]` with an explicit unresolved build-graph entry; this does not mean
the WASM contains no Rust crates.

The same revision's checked-in `wasm/index_bg.wasm` has SHA-256
`f757ffe05209651fef04cb456bbe42c3defc71738b2aa6754a76ede9e4d6840d`, different from the shipped npm binary. Its CI builds WASM
with `wasm-pack build --target web --out-name index --out-dir wasm/dist --release`
and replaces `wasm/*.wasm` with that artifact before publication. The inspected
revision retains neither the release Cargo.lock nor an exact target/feature
closure. Rebuilding or resolving today's dependencies cannot recover that proof.

The manifest records the direct exclusions from the original Cargo.toml/build.rs:
`napi`, `napi-derive` and `mimalloc` belong to non-wasm targets; `napi-build` is a
host build dependency whose setup is compiled only for non-wasm. These are
source-table exclusions, not a claim that the full wasm closure was resolved.
No transitive dev/build/feature exclusion is guessed.

The installed binary exposes 39 crate/version panic-path strings. They are a
**lower bound**, not a dependency graph. Each is unresolved against the absent
original graph and lacks independently retrieved exact-version source, declared
license and original notice in this investigation:

| Embedded crate/version | Unresolved evidence |
| --- | --- |
| `adler32@1.2.0` | Original lock-graph membership, exact source/license/notice. |
| `alloc-no-stdlib@2.0.4` | Original lock-graph membership, exact source/license/notice. |
| `arrayvec@0.7.4` | Original lock-graph membership, exact source/license/notice. |
| `base64@0.21.7` | Original lock-graph membership, exact source/license/notice. |
| `bitvec@1.0.1` | Original lock-graph membership, exact source/license/notice. |
| `brotli-decompressor@2.5.1` | Original lock-graph membership, exact source/license/notice. |
| `bytemuck@1.15.0` | Original lock-graph membership, exact source/license/notice. |
| `bytes@1.6.0` | Original lock-graph membership, exact source/license/notice. |
| `data-url@0.2.0` | Original lock-graph membership, exact source/license/notice. |
| `deflate@1.0.0` | Original lock-graph membership, exact source/license/notice. |
| `flate2@1.0.28` | Original lock-graph membership, exact source/license/notice. |
| `fontdb@0.14.1` | Original lock-graph membership, exact source/license/notice. |
| `gif@0.12.0` | Original lock-graph membership, exact source/license/notice. |
| `hashbrown@0.14.3` | Original lock-graph membership, exact source/license/notice. |
| `imagesize@0.12.0` | Original lock-graph membership, exact source/license/notice. |
| `jpeg-decoder@0.3.1` | Original lock-graph membership, exact source/license/notice. |
| `kurbo@0.9.5` | Original lock-graph membership, exact source/license/notice. |
| `miniz_oxide@0.5.4` | Original lock-graph membership, exact source/license/notice. |
| `miniz_oxide@0.7.2` | Original lock-graph membership, exact source/license/notice. |
| `pathfinder_content@0.5.0` | Original lock-graph membership, exact source/license/notice. |
| `png@0.17.5` | Original lock-graph membership, exact source/license/notice. |
| `rctree@0.5.0` | Original lock-graph membership, exact source/license/notice. |
| `roxmltree@0.18.1` | Original lock-graph membership, exact source/license/notice. |
| `rustybuzz@0.7.0` | Original lock-graph membership, exact source/license/notice. |
| `serde_json@1.0.115` | Original lock-graph membership, exact source/license/notice. |
| `simplecss@0.2.1` | Original lock-graph membership, exact source/license/notice. |
| `slotmap@1.0.7` | Original lock-graph membership, exact source/license/notice. |
| `smallvec@1.13.2` | Original lock-graph membership, exact source/license/notice. |
| `svgtypes@0.11.0` | Original lock-graph membership, exact source/license/notice. |
| `svgtypes@0.14.0` | Original lock-graph membership, exact source/license/notice. |
| `tiny-skia@0.10.0` | Original lock-graph membership, exact source/license/notice. |
| `tiny-skia-path@0.10.0` | Original lock-graph membership, exact source/license/notice. |
| `tinyvec@1.6.0` | Original lock-graph membership, exact source/license/notice. |
| `ttf-parser@0.18.1` | Original lock-graph membership, exact source/license/notice. |
| `unicode-bidi@0.3.15` | Original lock-graph membership, exact source/license/notice. |
| `unicode-general-category@0.6.0` | Original lock-graph membership, exact source/license/notice. |
| `weezl@0.1.8` | Original lock-graph membership, exact source/license/notice. |
| `xmlparser@0.13.6` | Original lock-graph membership, exact source/license/notice. |
| `xmlwriter@0.1.0` | Original lock-graph membership, exact source/license/notice. |

Crates with no embedded panic path remain unknown until the original release
lockfile and build/feature metadata are supplied. Every known missing component
and the two original-build gaps are retained in both generated manifests, the
combined notice inventory and the notices page. `sourceAccessVerified`,
`binaryDependencyGraphVerified` and `distributionApproved` remain false; owner
review in T604 must not treat a fetched wrapper license or these 39 strings as
complete source access.

Reproduce the investigation with Git only for source retrieval (use a private
session directory for `SOURCE_DIR`):

```sh
git clone --filter=blob:none https://github.com/thx/resvg-js SOURCE_DIR
git -C SOURCE_DIR checkout 9ca058462ac529120c8cc84ddcd6fef644cc5406
git -C SOURCE_DIR ls-tree -r --name-only HEAD
git -C SOURCE_DIR show HEAD:.gitignore
git -C SOURCE_DIR show HEAD:Cargo.toml
git -C SOURCE_DIR show HEAD:.github/workflows/CI.yaml
node scripts/browser-media-licenses.mjs --output docs/design/browser-media-licenses.json --write-distribution
pnpm test:browser-media
pnpm build:web
node scripts/build-browser-media-distribution.mjs --report tmp/browser-media-emitted-bundle.json
pnpm run lint
pnpm run format:check
```

The retained `resvg-rust-evidence.json` binds that source revision, original
Cargo.toml text/hash, excluded target tables and exact npm WASM identity. The
generator reads the installed WASM anew, checks this binding, records missing
or mismatched crates and verifies any retained notice files against their
hashes. It parses a supplied original Cargo.lock through explicit wasm runtime
roots/exclusions; absent original input keeps graph completeness false. Small
fixtures cover that subset, binary-string extraction, version mismatch and
missing notices. A fresh generated manifest does not claim the publisher's
unretained build provenance or distribution approval.

Validation at source commit `9fabeddd3711fa90d00b991bc756f82dc01d60f9`:
all 10 browser-media inventory/report tests, required builds, lint/format and
syntax checks pass. The existing static-delivery consumer passes 13 checks in
Chromium 157.0.8103.40, including actual served notice/source body hashes; the
four generated files regenerate byte-identically. This is local loopback
technical evidence with qualification false, not missing original-build proof
or live deployment success. Task T748 retains the exact verification log.
