# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 1 | 1 | - | 0 |
| AUTH | 1 | 1 | - | 0 |
| QUOTA | 2 | 2 | - | 0 |
| POST | 2 | 2 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 2 | 2 | - | 0 |
| MODEL | 3 | 3 | - | 0 |
| TEMPLATE | 3 | 3 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 3 | 3 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 2 | 2 | - | 0 |
| MARKETING | 1 | 1 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T007 | Agent automated test suite and LaunchAgent packaging | PUBLISH | T006 | doing@260907.ix |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T019 | todo |
| T019 | Naver editor mutations, the commit port and the daemon publisher wiring | PUBLISH | T018 T007 | blocked@260907 |

## next
- update-ssot PUBLISH after T008 closes: VIDEO-17 (agent video publishing) and TEMPLATE-39 (photo-row transport); VIDEO is otherwise shipped
- T019 blocked: live survey on the Mac (block appending, format menus, image insertion position, settings-layer opening, the published post frame) → update-ssot PUBLISH (frame-scoped readback in PUBLISH-20, the settings-layer opening step) → create-task re-decomposes T019
- T007 needs one ARCH-27 run on the Mac (and CI green once pushed) to close; everything else in it is verified

## log
- 260907 T007 tests+packaging complete and green on linux; only the macOS ARCH-27 / CI half is unverified. WARN launchd.Uninstall used to bootout unconditionally, so `go test` on a Mac would have stopped a real agent — now behind a test seam
- 260907 update-ssot QUOTA MARKETING start (pricing rework: grant bonus, always-on credit purchase, lot order, /plans entry + comparison, plans copy drift)
- 260907 T007 claimed (ix), base PUBLISH@2→@3 per create-task's r3 ruling that its tests+packaging scope is unaffected
- 260907 T019 blocked: Prepare's editor model contradicts the live SmartEditor in 3 places (no image placeholder, nothing opens the settings layer so tags/category/visibility never resolve, readback needs frame-scoped observation PUBLISH-20 does not enumerate); daemon wiring deliberately left unwired
- 260907 T019 claimed (ix)
- 260907 T019 (mu) and T007 (cx) reclaimed to todo on the owner's explicit instruction — both sessions ended with their work committed and the tree clean at 111bf53
- 260907 WARN T019: SmartEditor has no image placeholder (the photo button IS the file chooser), so Prepare's placeholder-then-upload phase cannot be observed — images must be inserted at their position during uploading_photos, keeping PUBLISH-13 stages monotonic
- 260907 T019 claimed (mu)
- 260907 create-task PUBLISH r3 → T019 updated in place (todo: base@2→@3, +PUBLISH-36, locator-derived pointer input in impl notes); no new task
- 260907 create-task PUBLISH start (r3 delta into T019, todo)
- 260907 update-ssot PUBLISH r3 done (PUBLISH-19✎ 20✎ 36+: recorded/replayed coordinates stay banned, a locator-derived pointer event does not)
- 260907 WARN T007 doing (cx) is inside PUBLISH but its scope is tests+packaging — r3 changes no behaviour it covers; T019 todo must take the new base
- 260907 update-ssot PUBLISH start (what "no screen coordinates" forbids)
- 260907 T019 unclaimed: SmartEditor has no semantic caret placement (visible body is not contenteditable; DOM.focus on its 17px proxy is stolen back), so PUBLISH-19/20 forbid the only working input path — update-ssot first
- 260907 T019 claimed (mu) — T007 still doing (cx) and holds uncommitted main.go; wiring edit kept minimal and last
- 260907 T017 done — TEMPLATE r3 is shipped; the grammar is now visible in 원문 and nowhere else
- 260907 T017 claimed (vd)
- 260907 T018 done (bound CDP page + live-verified observation port; 3 manifest locators were unmatched against the real editor, signature a1→a2)
- 260906 T016 done — VIDEO is complete except VIDEO-17 (agent publishing), still open for update-ssot PUBLISH after T008
- 260906 T016 claimed (vd)
