# REVIEW audio-true-peak-261117
> st:converted@261117 | scope:frontend/src/shared/lib/media/audio/loudness.ts | at:e9becca5 | base:ARCH@25

## summary
- One behavior-preserving true-peak performance improvement adopted under the owner's explicit self-improve delegation; one lower-impact backend candidate deferred. No product, precision, API, storage or verification-policy change.

## findings
- F1 [o] P3 `frontend/src/shared/lib/media/audio/loudness.ts:121-126`: each of four FIR phases repeats the same twelve sample-index bounds checks and sample reads for every PCM frame; normalization and encoded-output verification call this function over complete audio tracks ← reuse each tap's sample across four scalar accumulators while preserving each phase's addition order, coefficients, sample peak and complete filter tail →T887
- F2 [?] P3 `backend/internal/clip/capture_neighbors.go:18-52`: every source repeats photo metadata collection and filename regex matching ← precompute per-call photo groups; deferred because the production call freezes hints once, the default source cap is twenty, and no meaningful end-to-end cost has been measured

## notes
- Authority: direct invocation of gusta-self-improve asking for performance/refactoring and main publication before 06:00 Asia/Seoul on 2026-11-17. This cycle is limited to one problem and sixty minutes from 2026-11-16T16:14Z, reserving independent review and the shared final gate. No recurring schedule or external paid calls.
- Read-only native scout sessions: /root/scout_frontend and /root/scout_backend. Raw pointers checked by the coordinator: loudness.ts:113-132 and capture_neighbors.go:16-83. Existing review/tidy-261116 and tasks T851-T886 were excluded; no same-cause true-peak finding was found.
- Base: e9becca53376079187f488473c8ceb0d9dc5fa39. F1 allowed touches: loudness.ts and its adjacent loudness.test.ts only. The runtime benchmark and frozen baseline live outside the worker's source write scope at .git/gusta/v1/artifacts/perf6 in the shared Git directory.
- Immutable behavior: 48 kHz mono/stereo shape validation, finite-sample errors, silence returning negative infinity, unmodified inputs, four oversampling phases, existing FIR coefficients, eleven-frame tail, per-phase tap addition order and sample-peak inclusion. Preserve CDS-35 and CDS-52 output validation; no tolerance or sample reduction.
- Frozen measurement: Node v24.18.0 on the same host/process; fixed seeded PCM and input hash; exact Object.is comparison to base across tone, noise, transients, silence, impulses, boundaries and error cases. Representative 3-second stereo noise (288000 samples), two warmups per implementation, seven alternating pairs. Accept only at least 15% median elapsed improvement exceeding full-range/median spread with exact results and passing affected regressions. Setup/loading excluded, actual results consumed; preserve raw samples and hashes. No end-to-end/browser-export claim from this local benchmark.
- Implementation time cap: twenty minutes; individual commands at most fifteen minutes. A failed equality or performance criterion is a blocker and cannot be published as an improvement. Independent reviewer validates the raw measurement and final source delta.
- Routing: SETTINGS agents=codex,claude; new task recommends Codex, independent review prefers Claude with fallback allowed. Existing hdref/wopts/vcx frozen integration cohort remains owned by hdref; this session joins the next candidate. A frozen-gate refusal of plan-push preserves these local IDs until publication can resume.
