# UI and video work: portable T758 handoff

T753–T757 and T758's approved plan are on main. **T758 remains todo and its application source is not integrated.** The additional-hour continuation recovered and expanded the implementation, then received a second independent request for changes. The main coordinator preserved the result at its cutoff. The parent then used the remaining bounded time for the last voice-playback P2 and a narrow regression diagnosis. The complete latest snapshot remains unintegrated because whole-task validation is incomplete.

## Restore the latest source

Read current `spec/STATE.md`, `spec/reports/261112-ui12.md`, `spec/tasks/T758.guide-video-preparation-and-review.md`, `spec/review/video-preparation-guide-261112.md` and `T758-review-latest.json`. Establish a fresh worker assignment before restoring. Never apply this patch on top of an integrated T758 implementation.

The patch's published source base is `ada6afc201602a5332264d06925ea9cf8737fc0b`; its latest combined snapshot is `99b79a9592b1d6a77ffe0358f07da4f4e7711da8`. Main's subsequent changes are handoff data only. `T758.patch` contains **41 changed/new files**, including the worker task result, with 199445 bytes and SHA-256 `a859b5b861b915eb403a407f7c305c819befd93e316f558e9350a27da4422243`. `T758-snapshot.json` records every file hash, and `T758-recovery-proof.json` proves exact isolated-index reconstruction. These establish recoverability, not task acceptance.

`T758.bundle` also preserves the eight source/checkpoint commits and exact review identities above the published base. Verify it with `git bundle verify docs/handoff/ui12/T758.bundle`. This is an archive of unfinished work, not an approved branch to merge into the product. Use the patch below for normal task recovery; the bundle is available if source/review history is needed.

From the newly assigned checkout's repository root:

```sh
git apply --check docs/handoff/ui12/T758.patch
git apply docs/handoff/ui12/T758.patch
```

If the check fails, inspect the actual differences. Do not force rejected hunks or import old `.git` runtime/ownership. Do not start over: finish the preserved implementation and the remaining review correction.

## Exact remaining work

- The former hidden-style voice-playback P2 is fixed in `74e456ca` and independently approved for that exact narrow delta. It covers playback already active and fetch/play completing after hide, with six actual new regressions, 86 affected tests and type/lint/format checks. `T758-voice-correction.json` and `T758-voice-review.json` preserve the evidence. This does **not** approve the complete T758 source or its final gates.
- P3: at320px the emphasized generation label wraps as several narrow lines. Preserve360px behavior and distribute the shared step bar's width more clearly. This does not block pointer operation, but is an observed design issue.
- The earlier full frontend candidate had5 failures and4784 passes in603 files (`T758-validation.json`). One server-refusal assertion was subsequently scoped to the visible editor alert, preserving its refusal/usable-edit meaning; its whole31-test file passes and the exact narrow assertion change is independently approved (`T758-regression-review.json`). The latest combined snapshot has **four unresolved named cases** and has not rerun the full suite. See `T758-regression-diagnosis.json`: invalid-slot refusal precedence/composition fixture; the available-originals fixture omitting actualBytes; and two cross-tab ownership cases that stop at the guide helper while the focused-job view has no guide. Correct the actual cause without weakened assertions or hidden-control shortcuts. Do not infer either a production defect or a passing whole suite from the partial diagnosis.
- Complete affected/full/browser checks against the corrected complete delta, then independent approval, actual main integration, ARCH-31 publication gates and exact-revision remote workflow inspection. Existing failed or pending candidate checks are never counted as acceptance. The patch is not a completed whole-video redesign.

## Evidence

The genuine scout/approval and CLIP r75/T758 contract remain authoritative. The first fresh checkpoint passed250 isolated unit tests plus type/lint/format, but the coordinator's expanded91-file/661-test run failed49 tests in10 files. Independent review requested hidden source playback, invalid-region correction and visible-screen test fixes. A distinct correction session implemented those and passed24 isolated tests/type/lint/format. The second reviewer confirmed the earlier P2 fixes and identified the hidden voice sample issue, subsequently fixed and independently reviewed in the narrow parent continuation. Full candidate validation completed with5 failing/4784 passing tests in603 files; later build/Python/Go stages did not run. `T758-validation.json` records the exact failures.

`T758-browser-final.json` records25 actual Chromium fixture cases on the frozen corrected source: both channels × new/from-post/resume × ordinary/narrated at360/1440px, plus320px dark/reduced-motion/save+eligibility failure. Native lifecycle hit tests, latest-local summaries/change routes, retained input/file/cursor identity, one dock, no horizontal overflow and no generation on navigation pass. The source360, review360 and dark320 screenshots were viewed. Fixture media are unavailable; no live provider, playback/render/export, hardware, human-listening or distribution qualification is claimed. These25 browser cases predate the final voice/test deltas; new whole-task approval and final-candidate validation remain required. The six new voice regressions validate the async media behavior in unit fixtures, not live media qualification.

## Completed and excluded scope

- T753: authenticated home/logo and Make share `/create`, with public entry/deep links preserved.
- T754: stable My work hub/title/filter/list/loading geometry with purposeful animation retained.
- T755: truthful storyline-prepared status across both channels and aggregate history.
- T756 and return-filter repair: one compact accented in-progress section; storyline filters survive workspace returns.
- T757: unsent storyline requests survive same-workspace preview/tool/lifecycle visits; independently reviewed, published and verified.

T758 is the retained **Sources → Story → Style → Review** preparation slice. Later storyline/build, editing and delivery/resume redesign require separate evidence and approval; none is introduced in this bounded run. The journey table remains in `spec/review/video-preparation-guide-261112.md`. Preserve backend generation, quotes/ceilings, owner material, queues/flush, rendering/export/finalization contracts and excluded T539/T550/T603/T604/T697/INFRA work. No paid generation or new dependencies are authorized here.

## Execution history

The owner first requested a device handoff; the original16-file snapshot and stop/release records are preserved in Git commitb7fe8778 and prior report entries. The owner then explicitly authorized about one additional hour. This run used a fresh assigned worker, a separate read-only reviewer, a fresh correction worker and another independent review. The original released dirty worktree was never reused or modified. After its first cutoff, the parent assigned two isolated bounded follow-ups: the final P2 media correction and a five-case test diagnosis, with real separate read-only reviews. All current code/checkpoints are preserved in this41-file snapshot. No overnight recurrence remains. T758 is still todo and the application on main is unchanged through T757.
