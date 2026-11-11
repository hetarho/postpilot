# Windows render comparison and final-render investigation (T750)

Windows Chrome and Edge are **unmeasured**. This checkout is on Darwin/ARM64; Windows Node and an executable Windows host are unavailable. The bounded exploration began at 2026-11-11T14:49Z, with a 90-minute upper bound. The available implementation and transport investigation ends early once that missing execution environment is established; no historical number is substituted for a fresh observation.

[browser-render-windows.json](browser-render-windows.json) contains 48 explicit unexecuted arm runs (2 browsers × 4 fixtures × 3 arms × cold/warm), 32 unexecuted residual comparisons and 16 unexecuted production renders. Windows device identity, build, browser versions, WebGL renderer, phases, peaks, residuals and verdicts are unknown/null. All qualification flags remain false. The RTX 2080 Ti named in the task is a planned **discrete** device, neither integrated-GPU nor lower-resource evidence; it was not detected on this host. [The 261029 Windows Chrome report](browser-render-windows-chrome-261029.json) is unchanged historical evidence.

## Driver and source binding

The checked-in CommonJS driver is `scripts/browser-render-comparison/windows-driver.cjs`. Both processes enforce `.node-version` (24.18.0); the Windows process derives the single `playwright-core` version from `pnpm-lock.yaml` and loads its absolute workspace `.pnpm` path (currently 1.63.0). It does not depend on the Windows current directory for package lookup.

The WSL server binds `0.0.0.0`; the Windows client uses its printed `http://localhost:<port>` secure-context origin. Each server gets a new artifact UUID and Vite cache, with `watch: null`. Restart on a new port/cache after any source edit. Expected Git HEAD, clean tracked sources, frontend/fixture/driver/lockfile hashes, generated adapter hashes and start/live/end source bindings must agree. The client additionally compares its local manifest, driver and lockfile with the server.

The driver uses the shared comparison arm list and frozen fixture plan. Its private production adapter AST-extracts the exact comparison `planFor` and `freezeBrowserComposition` call, and substitutes only the old export fixture's plan/speech/snapshot setup. It retains the existing production `renderBrowserAudio`, `renderBrowserVideo`, mux, `measureMp4Output`, audio verification, `browserRenderVerdict`, three DOM playback seeks and recovery path. The control and production snapshot fingerprints must match. Generated TypeScript/JavaScript and both original entries are hashed. No production frontend module or fixture input is changed.

This avoids reusing the old export fixture's repeated 3–6s source cuts and 64×32/7s assumption for the comparison's 320×180/30s fixture. The shared plan has non-overlapping source ranges and the same speed/transition decisions; the production output is 450 frames/15s. The narrated case uses only the checked-in synthetic speech fixture. No owner footage, provider request, upload or private-storage promotion occurs.

Chrome and Edge use Playwright's default launch arguments and managed temporary profiles, never the owner's profile. Each arm gets a fresh browser context, then cold/warm runs in that page. Production repeats disclose that worker lifecycle is unchanged; a warm label does not imply a retained production worker. Browser/context ownership and recovery remain explicit. `--headed` is optional and must be disclosed in a later measurement. The monotonic timebox is capped at 90 minutes; unavailable/failed/skipped combinations stay unmeasured.

Windows metadata is a whitelist projection of OS Version/BuildNumber, processor Name, TotalPhysicalMemory and video-controller Name/DriverVersion. PowerShell runs non-interactively with terminating errors and UTF-8 output. A missing required field leaves device metadata unmeasured. Serial numbers, host names and device identifiers are not selected. Pure fixtures verify that incomplete metadata cannot become observed and that extra identifier fields are discarded; these fixtures do not prove actual WMI execution. The underlying field definitions are in Microsoft's [Win32_OperatingSystem](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-operatingsystem), [Win32_Processor](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-processor), [Win32_ComputerSystem](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-computersystem) and [Win32_VideoController](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-videocontroller) references. Browser ownership follows Playwright's [launch](https://playwright.dev/docs/api/class-browsertype#browser-type-launch) and [newContext](https://playwright.dev/docs/api/class-browser#browser-new-context) interfaces.

Artifact chunks are ordered, hashed and bounded before base64 allocation. MP4 bytes are hashed and discarded; sampled PNGs go through the explicit UNC workspace to the WSL server, whose route admits only current known browser/run/sample names. Logical memory counters do not measure physical process/GPU/private codec/DSP peaks. Software or masked renderers cannot become hardware qualification, and pixel residuals have no invented human acceptance threshold.

## Reproduce on the Windows/WSL host

Use a clean checkout at the chosen commit, the pinned Node on both systems, frozen workspace dependencies and installed Chrome/Edge. In WSL, start the server in one terminal; replace `<commit>` with a full source commit. Pick a fresh free port after every edit.

```sh
node scripts/browser-render-comparison/windows-driver.cjs --serve --workspace "$PWD" --expected-head <commit> --port 5187
```

In another WSL terminal at the same checkout, run Windows Node over stdin from a Windows-drive current directory. The driver and workspace are absolute; no user name is embedded. Keep the server running until the client report is written.

```sh
postpilot_workspace_windows="$(wslpath -w "$PWD")"
postpilot_driver="$PWD/scripts/browser-render-comparison/windows-driver.cjs"
(
  cd /mnt/c
  "/mnt/c/Program Files/nodejs/node.exe" - --workspace "$postpilot_workspace_windows" --base-url http://localhost:5187 --expected-head <commit> < "$postpilot_driver"
)
```

The server prints its UUID artifact directory; the client writes `windows-report.json` there. Missing Chrome/Edge, codec failures, browser errors, unknown renderers and cleanup errors are retained as findings. Default browser executable locations follow ProgramFiles/ProgramFiles(x86), with the task's standard locations as fallback. Known historical Windows AAC-rate and null-format HEVC limitations do not authorize changing product settings.

For the non-Windows availability report and the pure metadata/privacy guards:

```sh
node scripts/browser-render-comparison/windows-driver.cjs --workspace "$PWD" --expected-head <commit> --output tmp/browser-render-windows-unmeasured.json
node scripts/browser-render-comparison/windows-driver.cjs --self-test
node --check scripts/browser-render-comparison/windows-driver.cjs
node scripts/browser-render-comparison/static-check.mjs
pnpm run lint
pnpm run format:check
```

## Actual Mac transport and production-path validation

The separate `macTransportValidation` section retains measured phases, logical peaks, all 16 sampled-pixel comparisons, production output measurements and cleanup. Its source is `6569b88ebd6b32c514e6c8955e729688e17a09f2`. The later Windows-only metadata projector/privacy guards are verified separately at `e2f40117a48a0b27aa93a2078be2d16ec9df711b`; the real Mac transport/adapter path is unchanged. This is diagnostic validation on Apple M1 Max/32 GiB, Darwin kernel 25.5.0, Node24.18.0 and bundled Chromium153.0.8010.12. Actual Worker WebGL2 reports unmasked ANGLE Metal Apple M1 Max, `softwareRasterizer=false`. It establishes neither Windows execution nor hardware release approval.

The real driver was read over stdin with a current directory different from `--workspace`, exercising the same explicit-workspace client and artifact transport. A single-case probe passed 6 arm runs, 4 comparisons and 2 real production renders. The full matrix passed **24 arm runs, 16 comparisons and 8 real production renders**, including mixed synthetic narration. Every comparison has matching snapshot/clock/component/background contracts and normalized PCM. Every production render matches its comparison snapshot, passes CDS-52 technical output checks and finishes with no owned OPFS residue. All start/end source hashes agree. Physical peaks and human listening/visual acceptance remain unmeasured.

| Fixture | Production dimensions | Cold / repeat elapsed ms | Output verdicts | Snapshot matches |
|---|---|---|---|---|
| horizontal-neon-source | 1920×1080 | 3878.4 / 3828.2 | pass / pass | pass / pass |
| square-glitch-source | 1080×1080 | 3665.0 / 3659.9 | pass / pass | pass / pass |
| vertical-blur-in-source | 1080×1920 | 3733.7 / 3692.5 | pass / pass | pass / pass |
| vertical-ember-mixed-synthetic | 1080×1920 | 4092.4 / 4076.9 | pass / pass | pass / pass |

All outputs are 15s, 450 frames, H.264 with presentation-valid timestamps and 48 kHz AAC. The durations above include diagnostic render/verification/recovery instrumentation and do not claim production throughput. The JSON preserves detailed verdicts, audio observations, packet/source resource counters, sampled residuals and cleanup. Full raw report SHA and its local artifact path are retained for this checkout; the checked-in projection includes the measured fields needed for review.

Validation selection covers the new transport, every synthetic style/ratio/audio branch, exact shared snapshots, real production consumers, metadata privacy and the all-null Windows missing-evidence path. Logs: `check-97079881-69ba-49a6-b10a-a67120660bd7.log` (single, 75514ms), `check-cc458fb0-62fe-45b0-b515-597068ee283b.log` (matrix, 301859ms), `check-ac02aacc-3c9f-4bee-9ef4-3117f82f732e.log` (syntax, privacy/incomplete-WMI fixtures and real stdin missing report), `check-3148ce95-a4ed-4c60-8916-3c95a688078d.log` (comparison guards/own diagnostic types; existing imported-scope diagnostics are separately reported), `check-f4090cfb-967f-4261-acab-043c08b2f541.log` (lint/format). The complete pre-push project CI independently checks the actual frontend types/build and all existing suites. Windows runtime acceptance stays unchecked; remaining host/human/distribution qualification belongs to T604.
