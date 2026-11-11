# UI and video work: device handoff

This is the preserved device-handoff snapshot. The owner subsequently authorized a bounded T758 continuation; consult current `spec/STATE.md`, the task (including `tasks/done/`) and `spec/reports/261112-ui12.md` before applying this historical patch. Never apply it on top of an already integrated T758 implementation.

The completed implementation through T757 and the approved T758 plan are on `main`. T758 was interrupted at the owner's request to move work to another device. Its partial implementation is preserved in `T758.patch`, including all six new files and the worker's task edits. It is not integrated product code and has not passed task acceptance or independent review.

The original preparation task starts at `ada6afc201602a5332264d06925ea9cf8737fc0b`. `T758-snapshot.json` records all 16 file hashes and the patch SHA-256. The source on main has not changed since that base; subsequent handoff commits contain records and recovery artifacts only.

## Continue on the other device

Update the existing main checkout from `origin/main`, preserving any unrelated local changes. Read `spec/STATE.md`, `spec/reports/261112-ui12.md`, `spec/tasks/T758.guide-video-preparation-and-review.md`, `spec/review/video-preparation-guide-261112.md` and this directory before starting implementation.

In the checkout assigned to T758, from its repository root, restore the partial work once:

```sh
git apply --check docs/handoff/ui12/T758.patch
git apply docs/handoff/ui12/T758.patch
```

If the check fails, inspect the relevant differences rather than using a forced or rejected-hunk application. The task file remains the current acceptance contract; unchecked or partially implemented behavior still needs completion and verification. Do not count this recovery snapshot as a completed implementation.

Run `/start-vibe` in automatic mode and explicitly continue T758 from the recovered patch. A fresh clone does not inherit local work ownership, `.git` runtime, runner processes, installed dependencies or absolute tool paths. Establish ownership using the installed skills on the new device; do not import the old runtime as live assignments. On this device, the coordinator and runner were stopped, attempt `b1df8608-7122-4dba-8647-1299ab2ddd5a` was released, and its original dirty worktree was preserved.

## What is already done

- T753: authenticated logo/home and Make share `/create`; public entry and valid deep links remain available.
- T754: stable My work hub, toolbar, list/loading geometry and scrollbar space, with purposeful motion retained.
- T755: a prepared storyline has its own truthful history status and filters, including aggregate and both clip channels.
- T756 and the return-filter repair: compact, counted in-progress items replace duplicate oversized continuation cards; storyline filters survive workspace returns.
- T757: unsent storyline requests survive same-workspace preview/tool/lifecycle visits. Independent review, complete local CI, fixture-browser acceptance and source-revision CI passed.

The later metadata CI delay for `0af0cf99` resolved successfully: https://github.com/hetarho/postpilot/actions/runs/43094556625. The resumed planning revision is `ada6afc2`. Actual remote workflow conclusions for the final handoff revision must be inspected separately.

## Remaining work

T758 introduces retained preparation screens **Sources → Story → Style → Review**, latest-local summaries with routes back to the original inputs, shared step navigation and explicit generation approval. Preserve both channels, ordinary/narrated modes, source-post and resume paths, owner isolation, autosave/flush, source audio, exact material, quote/credit checks and preview/render/export/finalization behavior. The task and CLIP-227 carry the full contract.

The baseline 360px fixture also shows the Delete control intercepting the centers of the Edit/Finish lifecycle tabs in both channels. `video3-pointer-geometry.json` preserves the actual hit-test evidence. Fix and verify the native pointer route within the existing task; keyboard-only or forced clicks do not establish this acceptance. `video3-scout.json` and `video3-baseline.json` preserve the independent proposal and browser baseline without depending on `.git` artifacts.

T758 is only the first preparation slice of the broader requested video-process redesign. Later storyline review/build, editor action placement and delivery/resume improvements require their own evidence, scoped planning, implementation and independent review. The journey table in `spec/review/video-preparation-guide-261112.md` records that direction. Do not treat T757 or this interrupted slice as the finished redesign.

The owner's unattended decisions and original deadline were recorded in the report; the device handoff stops automatic work on this machine. No paid/live model calls or owner-held hardware/listening/distribution qualification are included. Preserve unrelated T539/T550/T603/T604/T697 and INFRA/ideation work.

## Verification

The partial patch was checked for clean application and exact recovery of all recorded file bytes. Those checks establish recoverability only. Finish T758's changed/affected tests and fast checks, independent review and browser acceptance, then the full pre-push gates in `docs/verification.md` and exact-revision remote workflow inspection before publishing its implementation. The handoff artifacts themselves do not change the implemented application on main.
