# UI and video work: portable T758 handoff

T753–T757 and T758's approved plan are on main. **T758 remains todo and its application source is not integrated.** The additional-hour continuation recovered and expanded the implementation, then received a second independent request for changes. The explicit implementation cutoff passed, so the coordinator preserves the full result rather than starting another implementation round.

## Restore the latest source

Read current `spec/STATE.md`, `spec/reports/261112-ui12.md`, `spec/tasks/T758.guide-video-preparation-and-review.md`, `spec/review/video-preparation-guide-261112.md` and `T758-review-latest.json`. Establish a fresh worker assignment before restoring. Never apply this patch on top of an integrated T758 implementation.

The patch's source base is `b7fe8778df41fa6da0636ea5a6768c172fee9e2e`; its source checkpoint is `a0c7b3a35733876273c461de201c7b17d60f1b68`. Main's subsequent handoff changes are documents only. `T758.patch` contains **40 changed/new files**, including the worker task result, with 190552 bytes and SHA-256 `fb6d12ee4a7e7c9804d0fc391fbc24b52c66b3a37e69e78f655c57e025febc6f`. `T758-snapshot.json` records every file hash. `T758-recovery-proof.json` proves clean application and exact isolated-index reconstruction, not functional acceptance.

From the newly assigned checkout's repository root:

```sh
git apply --check docs/handoff/ui12/T758.patch
git apply docs/handoff/ui12/T758.patch
```

If the check fails, inspect the actual differences. Do not force rejected hunks or import old `.git` runtime/ownership. Do not start over: finish the preserved implementation and the remaining review correction.

## Exact remaining work

- P2: the confirmed voice sample in **Style** must stop when its preparation screen, lifecycle or focused-work view becomes inactive. Cover a sample fetch that completes and tries to play after the screen was hidden. Keep fields, uploads, approval and voice ownership lifetimes; add the regression and obtain a new independent immutable-source review. `T758-review-latest.json` has the exact source pointer and actual reviewer session.
- P3: at320px the emphasized generation label wraps as several narrow lines. Preserve360px behavior and distribute the shared step bar's width more clearly. This does not block pointer operation, but is an observed design issue.
- Full frontend validation still fails5 tests in3 files (4784 pass,603 files total): see `T758-validation.json` for exact names/errors. Diagnose and correct them without weakened assertions or hidden-control shortcuts.
- Complete affected/full/browser checks against the corrected complete delta, then independent approval, actual main integration, ARCH-31 publication gates and exact-revision remote workflow inspection. Existing failed or pending candidate checks are never counted as acceptance. The patch is not a completed whole-video redesign.

## Evidence

The genuine scout/approval and CLIP r75/T758 contract remain authoritative. The first fresh checkpoint passed250 isolated unit tests plus type/lint/format, but the coordinator's expanded91-file/661-test run failed49 tests in10 files. Independent review requested hidden source playback, invalid-region correction and visible-screen test fixes. A distinct correction session implemented those and passed24 isolated tests/type/lint/format. The second reviewer confirmed the earlier P2 fixes and identified the remaining hidden voice sample issue. Full candidate validation completed with5 failing/4784 passing tests in603 files; later build/Python/Go stages did not run. `T758-validation.json` records the exact failures.

`T758-browser-final.json` records25 actual Chromium fixture cases on the frozen corrected source: both channels × new/from-post/resume × ordinary/narrated at360/1440px, plus320px dark/reduced-motion/save+eligibility failure. Native lifecycle hit tests, latest-local summaries/change routes, retained input/file/cursor identity, one dock, no horizontal overflow and no generation on navigation pass. The source360, review360 and dark320 screenshots were viewed. Fixture media are unavailable; no live provider, playback/render/export, hardware, human-listening or distribution qualification is claimed. The independent review remains blocking despite the browser passes.

## Completed and excluded scope

- T753: authenticated home/logo and Make share `/create`, with public entry/deep links preserved.
- T754: stable My work hub/title/filter/list/loading geometry with purposeful animation retained.
- T755: truthful storyline-prepared status across both channels and aggregate history.
- T756 and return-filter repair: one compact accented in-progress section; storyline filters survive workspace returns.
- T757: unsent storyline requests survive same-workspace preview/tool/lifecycle visits; independently reviewed, published and verified.

T758 is the retained **Sources → Story → Style → Review** preparation slice. Later storyline/build, editing and delivery/resume redesign require separate evidence and approval; none is introduced in this bounded run. The journey table remains in `spec/review/video-preparation-guide-261112.md`. Preserve backend generation, quotes/ceilings, owner material, queues/flush, rendering/export/finalization contracts and excluded T539/T550/T603/T604/T697/INFRA work. No paid generation or new dependencies are authorized here.

## Execution history

The owner first requested a device handoff; the original16-file snapshot and stop/release records are preserved in Git commitb7fe8778 and prior report entries. The owner then explicitly authorized about one additional hour. This run used a fresh assigned worker, a separate read-only reviewer, a fresh correction worker and another independent review. The original released dirty worktree was never reused or modified. Source implementation stopped at the latest bounded cutoff; the remaining interval is validation/publication and this recoverable handoff, without an overnight recurrence.
