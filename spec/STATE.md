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
| POST | 1 | 1 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 1 | 1 | - | 0 |
| MODEL | 2 | 2 | - | 0 |
| TEMPLATE | 1 | 1 | - | 0 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 1 | 1 | - | 0 |
| PUBLISH | 2 | 2 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 2 | 2 | - | 0 |
| MARKETING | 1 | 1 | - | 0 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T001 | Price credit holds from each call's own completion budget | QUOTA GEN | - | done@260905 |
| T002 | Give the write budget reasoning headroom and let the adapter disable reasoning | MODEL GEN QUOTA | T001 | done@260905 |
| T003 | Persist the dedicated browser session across pairing retries | PUBLISH | - | todo |
| T004 | Mac pairing, profile setup and the non-publishing compatibility probe | PUBLISH | T003 | todo |
| T005 | The deterministic Naver publisher | PUBLISH EXPORT | T004 | todo |
| T006 | Agent execution, the commit fence, readback and cleanup | PUBLISH | T005 | todo |
| T007 | Agent automated test suite and LaunchAgent packaging | PUBLISH | T006 | todo |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 | todo |
| T009 | Pointer affordances for clickable controls | THEME | - | done@260905 |

## next
- implement-task T003

## log
- 260905 T002 done (native-effort write headroom, explicit reasoning disable, and per-purpose truncation counts; every ARCH gate passes)
- 260905 T002 claimed (cx)
- 260905 T001 done (per-call completion budgets now price holds and quotes; ARCH-26 and ARCH-28 pass)
- 260905 T001 claimed (cx)
- 260905 T009 done (global pointer affordances; 988 FE tests and every ARCH frontend gate pass on Node 24.18.0)
- 260905 T009 claimed (cx)
- 260905 create-task THEME → T009; THEME tasked=2
- 260905 create-task THEME start
- 260905 update-ssot THEME r2 done (THEME-36+ global pointer affordance; no doing-task impact)
- 260905 update-ssot THEME start
- 260905 migration complete (six skills only; active references clean; narrative reviewed; all ARCH verification gates pass on Node 24.18.0)
- 260905 create-narrative done (spec/NARRATIVE.md; SSOT conversion-loss read-through complete)
- 260905 create-narrative start (audience: new team member; use: onboarding and current-status review)
- 260905 migration Phase 4 start (six-skill, reference, workflow, and conversion-loss verification)
- 260905 migration Phase 3 done (legacy tree archived; 10 legacy skills and spec scaffolding retired; active references swept; CI runs haeram-spec-creator check)
- 260905 migration Phase 3 start (retire the legacy tree, legacy skills, and spec scaffolding)
- 260905 create-task QUOTA MODEL PUBLISH → T001–T008; QUOTA MODEL PUBLISH tasked=2; code-review 01's findings were all closed by legacy jobs 26–31, nothing to carry
- 260905 create-ssot PUBLISH r1 + r2 (legacy change 09), LANG, THEME, MARKETING r1 — all 14 domains migrated; MARKETING is shipped (job 34), the legacy overview row was stale
- 260905 create-ssot TEMPLATE, GUIDE, EXPORT r1 (shipped, no pending work)
- 260905 create-ssot MODEL r1 (shipped) + r2 (legacy change 29) — pending for create-task, after QUOTA
- 260905 create-ssot GEN r1 (shipped, no pending work)
- 260905 create-ssot VOICE r1 (shipped; one open decision VOICE-7 on retiring 규칙으로 저장)
