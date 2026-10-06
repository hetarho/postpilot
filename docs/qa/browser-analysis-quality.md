# Browser analysis quality diagnostic

T603 has a private offline harness. Browser analysis qualification remains disabled. The retained [offline report](browser-analysis-quality-offline.json) demonstrates parser compatibility, report arithmetic and failure gates with synthetic responses and a synthetic media-verifier stub. It establishes no actual Korean audiovisual accuracy, human approval, browser/native equivalence, supplier sampling, or voice readiness.

## Operator use

The `api analysis-quality` subcommand runs before API configuration/database boot. It reads explicit local files and uses the existing production `clip/ai.ObserveChunk` prompt, schema and parser. It has no provider client, credential loading, model-pricing discovery, storage fetch or accepted live route.

```sh
api analysis-quality --mode inspect --input /private/corpus/corpus.json --output /private/new-inspection --ffmpeg /validated/ffmpeg --ffprobe /validated/ffprobe
api analysis-quality --mode replay --input /private/corpus/corpus.json --output /private/new-replay --ffmpeg /validated/ffmpeg --ffprobe /validated/ffprobe
api analysis-quality --mode audit --input /private/corpus/reviewed-corpus.json --output /private/new-audit --ffmpeg /validated/ffmpeg --ffprobe /validated/ffprobe
```

Input directories are canonical private directories (0700). Manifest, original, proxy and replay files are private regular files (0600), referenced by flat basenames in that directory. URLs, symlinks, traversal, devices, unknown/trailing/duplicate JSON keys, excessive nesting, incompatible profiles, changed hashes and malformed metadata are refused. Outputs use a new 0700 directory and exclusively created 0600 files. A failed run retains bounded private failure evidence where writing is possible. Stdout and `summary.json` contain owned status/gate codes and counts, without original paths/names, speech, response/truth text, reviewers, private rights documents or opaque routes. `report.json` is private and contains the full diagnostic provenance and normalized observation evidence.

`--live` fails before any file/configuration/factory/reservation access. A JSON cap, claimed review or synthetic policy cannot enable it. No normal generation job, credit allowance or account access is fabricated. A future live implementation needs separately reviewed real registry/account admission and accounting wiring, authorized audiovisual sources, grounded truth, exact route/settings approval and an explicit cumulative spend ceiling. Credential availability is unverified; the harness neither requests nor reads credentials.

## Versioned input and binding

The `analysisquality.Corpus` type is the executable schema: `format=postpilot-analysis-quality`, `version=1`, origin `synthetic_mock` or `recorded_replay`, corpus/truth versions, a Git commit, language, an offline declared `llm.CallPolicy`, 1–4 predefined replicates, cases, inputs, replay references and optional human assessments/review. Nested clip/llm values use their existing Go JSON representation. Limits are six cases, 147 inputs, 256 labels per case, 128 MiB per original, 8 MiB per proxy, 2 MiB per replay document, 128 MiB total proxy bytes, 16 MiB total replay bytes and 4 MiB per report. Speech edit-distance work is additionally bounded.

Each case binds a source identity/fingerprint to its original's bytes/hash, declared measurement provenance, explicit operator-owned or licensed rights scope/evidence digest/issuer/expiry, annotator/time/original digest, category tags and source-time labels. Required/critical flags, known/unknown truth, evidence references, original intervals/tolerances and equivalence rules are declared before response review. Possession of a file is not evidence of semantic accuracy or a product owner's authorization.

Each input names `reference`, `native` or `browser`, immutable artifact bytes/hash, source interval, production profile/preparation version and declared encoder/device/build telemetry. Current arms obey the existing 60-second, 720-edge, 15-file-FPS, H.264/yuv420p/mono AAC48k and 8 MiB copy bounds. The native/browser geometry and source clocks match. The actual local media adapter reuses T602's packet-EOF/full-frame/audio verifier in a 32 MiB private workspace. Original metadata and encoder bitrate telemetry remain explicitly declared; the harness hashes originals without independently decoding them. A higher-resolution reference needs a separately reviewed bounded validator/admission and cannot be relabeled as the production profile.

Inspect emits production system/user/schema hashes and a per-input replay key. Replay `Record` files bind that exact key, origin, raw normalized `llm.Response`, finish reason and optional owned failure code. Keys bind the source, artifact, encoder/profile, language/prompt/contract and frozen model/route/policy. An incompatible fixture invalidates its own rows; compatible independent rows remain readable. Old captures and synthetic responses always remain replay evidence. The harness performs no response migration or automatic retry, and production parsing adds source offsets once.

Assessments bind each label to the exact response-file hash and truth-label digest, reviewer/time, status, parsed segment and exact field/rune span. Semantic matching of facts/events and quality labels is supplied by a human; no model judge or keyword classifier is used. Known important text/numbers and usability claims are checked against the mapped output. Speech uses the full mapped utterance, and silence checks all intersecting parsed speech. A final review binds plan, truth and normalized output digests; an incomplete comparison cannot claim complete review. An attached offline review still cannot qualify actual current-provider output.

## Independent results and hard gates

Reports preserve separate fact, event, scene, important text, number, speech, quality and usability counts with known/required denominators, explicit unknown/unreviewed/omitted/invented/not-applicable outcomes and critical failures. Raw exact and predeclared normalized exact matches are separate; a wrong Korean price cannot pass by fuzzy matching. Speech reports character edits/reference runes and unscored reference runes, plus silence correctness/hallucinations. Missing speech is retained in the denominator. Matched event/speech boundaries report signed/absolute original-source errors and tolerance failures; unknown/unmatched boundaries remain explicit.

Native→browser and reference→each-arm comparisons retain missing/unrun/failed arms, required-content loss and timing regressions. Every predefined response remains in the matrix. Repeated outputs report per-label distributions/disagreement, failed/unrun counts and boundary spreads; one replicate reports variance unmeasured. There is no aggregate acceptance score, best-response selection or population-level confidence claim.

File FPS does not describe supplier sampling. Current requests send no temperature/seed/top-p/custom sampling-FPS/media-resolution options. Reports retain those controls as not sent/unsupported and effective supplier sampling, internal resolution and served revision as unknown when the llm boundary does not expose them. The fixed production prompt has no standalone OCR transcript: `ReadableText=true` cannot substitute for an omitted important label/price. Benchmark-only OCR prompting is not added. Supplier documentation is available in [OpenRouter video inputs](https://openrouter.ai/docs/guides/overview/multimodal/videos) and [Google video understanding](https://ai.google.dev/gemini-api/docs/video-understanding); it does not certify the actual nominated endpoint's internal output.

The optional budget primitive requires trusted local admission before constructing its injected Models factory. It copies the approved call matrix/cap, binds the state path/policy/plan/evidence/expiry, owns an exclusive session lock, and persists each in-flight worst-case reservation before entering `Models.Complete`. Cumulative confirmed cost plus unresolved reservations plus the next bound cannot exceed approval. Returned `llm.Usage.CostReported/CostMicrousd` is the only measured cost source; reasoning is not double-counted. Paid errors retain usage, missing cost remains null with its unresolved upper bound, and above-bound usage, failed writes, lost locks or uncertain restarts prevent another call. Existing production response correction is metered per underlying completion. Tests exercise this primitive with stubs; it is not connected to a live CLI/registry or account ledger.

## Verification and remaining work

Code checkpoint `a49d82ba41d05cc8d2e79d43e61d49e4291b3234` retains the initial implementation and two independently reproduced corrections: caller/callback mutation of approval data and unavailable pairwise arms. Focused diagnostic/budget/correction/operator tests and the independent checkpoint overlay pass. Root coordinates the full local CI and execution-image verifier checks against the unchanged code source.

```sh
cd backend
GOCACHE=/private/tmp/postpilot-go-cache GOMAXPROCS=2 go test -p 2 -timeout 5m ./internal/clip/diagnostic/analysisquality ./cmd/api -run 'Test(Budget|ProductionReplay|ProductionResponseCorrection|ReplayRefuses|ParserFailures|CompatibleIndependent|PrivateCLI|AnalysisQuality|MissingArms|StrictPrivate|LocalVerifier)' -count=1
```

The optional `TestLocalVerifierExistingAuthorizedSyntheticCopy` smoke reuses a small, explicitly selected existing operator-owned synthetic MP4 via a private `Input` JSON; it generates no media. Root supplies `CLIP_MEDIA_SMOKE=1`, `T603_VERIFIER_INPUT_JSON`, and validated `FFMPEG_PATH`/`FFPROBE_PATH` in the execution environment. This tests real EOF/frame/audio verification only.

Still required: real rights-cleared Korean text/numbers/brief actions/fast cuts/scenes/speech/focus/shake corpus and human source-time truth; actual matched reference/native/browser captures and approved repeated current-provider responses; supported route/control evidence and trusted live admission/accounting with an explicit cumulative cap; final actual human semantic review. T603 remains open/blocked for these gates. `BrowserAnalysisQualified` remains false, the observation contract is unchanged, and T539/T550 voice/narration gates remain independent.
