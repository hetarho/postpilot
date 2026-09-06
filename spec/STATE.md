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
| T017 | The `원문` source mode, paste import and the copyable format guide | TEMPLATE | T011 | todo |
| T019 | Naver editor mutations, the commit port and the daemon publisher wiring | PUBLISH | T018 T007 | todo |

## next
- implement-task T017 (the `원문` source mode on top of T011's screen)
- update-ssot PUBLISH after T008 closes: VIDEO-17 (agent video publishing) and TEMPLATE-39 (photo-row transport); VIDEO is otherwise shipped
- implement-task T019 NEXT (T018 done: the editor is observable; T019 adds the mutations, the commit port and the daemon wiring) → T008 · T007 in parallel (cx)

## log
- 260907 T018 done (bound CDP page + live-verified observation port; 3 manifest locators were unmatched against the real editor, signature a1→a2)
- 260906 T016 done — VIDEO is complete except VIDEO-17 (agent publishing), still open for update-ssot PUBLISH after T008
- 260906 T016 claimed (vd)
- 260906 T015 done — videos are attachable in the browser; only T016 (blocks, exports, badges) is left of VIDEO
- 260906 T015 claimed (vd)
- 260906 T014 done — T012's WARN closed: generation tells a clip from a photo, so T015 may make videos attachable
- 260906 T014 claimed (vd)
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
