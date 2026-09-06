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
| PUBLISH | 2 | 2 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 2 | 2 | - | 0 |
| MARKETING | 1 | 1 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T007 | Agent automated test suite and LaunchAgent packaging | PUBLISH | T006 | doing@260905.cx |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T019 | todo |
| T014 | Observe videos, write from them and validate VIDEO blocks | VIDEO GEN MODEL | T012 T013 T010 | todo |
| T015 | Video upload in the browser, the attachment strip and the contact sheet | VIDEO POST GEN | T012 | todo |
| T016 | The VIDEO block in the reading view, editor and exports, plus the capability badges and refusals | VIDEO EXPORT MODEL POST | T013 T015 | todo |
| T017 | The `원문` source mode, paste import and the copyable format guide | TEMPLATE | T011 | todo |
| T018 | The bound CDP session and the deterministic Naver observation port | PUBLISH | T005 | doing@260906.pt |
| T019 | Naver editor mutations, the commit port and the daemon publisher wiring | PUBLISH | T018 T007 | todo |

## next
- implement-task T017 (the `원문` source mode on top of T011's screen)
- implement-task T014 (needs T010 first) → T015 → T016; T014 BEFORE T015 — T012's WARN: generation still reads a video as a photo
- implement-task T018 → T019 → T008 (the deterministic publisher's real CDP driver; nothing can publish until T019 wires it) · T007 in parallel (cx)

## log
- 260906 T011 done — the FE grammar fixture is green again (T010's WARN closed)
- 260906 T018 claimed (pt)
- 260906 create-task PUBLISH → T018 T019 (T005 shipped the publisher state machine but never its port; ssot row unchanged — unbuilt scope, not a delta); T008 dep T007→T007 T019
- 260906 create-task PUBLISH start (gap: naver.Port/CommitPort has only test fakes — T005 done left the real CDP driver unbuilt, so `run` refuses and no job can ever be claimed)
- 260906 T011 claimed (vd)
- 260906 WARN T010 left the FE grammar fixture red on purpose (6 of 52, all count cases) — T011 makes it green; do not push before T011
- 260906 T010 done
- 260906 T010 claimed (vd)
- 260906 WARN spec lint: TEMPLATE PUBLISH MARKETING exceed the new FORMAT's 6-char ssot id limit — all 44 violations trace to that one rename, spec-wide, not done inside a task
- 260906 T013 done
- 260906 spec migrated to haeram-spec-creator 0.2.3 FORMAT: tasks/done/ introduced (T001-T006 T009 T012 moved, their STATE rows dropped; T004 T005 T006 T012 file st doing->done per STATE)
- 260906 T013 claimed (vd)
- 260906 WARN T012 put videos inside generation.PostInput.Images; generation still batches that list as photos — T014 must land before T015 makes videos attachable
- 260906 T012 done (videos as the post's second attachment kind: proto, migration 0026, kind-aware handshake, cascade, sweep, VIDEO block; ARCH-26 + ARCH-28 pass)
- 260906 create-task TEMPLATE → T017 (dep T011; FE only, no server change); TEMPLATE tasked=3; T011 base → TEMPLATE@3 with a hand-off note
- 260906 create-task TEMPLATE start (r3 delta: 26✎ 30✎ 34✎ 41+ 42+)
- 260906 T012 claimed (vd)
- 260906 update-ssot TEMPLATE r3 done (TEMPLATE-26✎ 30✎ 34✎ 41+ 42+: `원문` mode back, paste import, copyable format guide; T010/T011 todo touch the same screen — no doing task affected)
- 260906 update-ssot TEMPLATE start (raw body source view + paste import, reversing TEMPLATE-26/34)
- 260906 create-task VIDEO POST GEN MODEL EXPORT → T012–T016; VIDEO tasked=1, POST GEN MODEL EXPORT tasked=rev; VIDEO-17 stays open (no task)
