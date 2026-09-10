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
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 9 | 9 | - | 0 |
| POST | 5 | 5 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 9 | 9 | - | 0 |
| TMPL | 5 | 5 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 4 | 4 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 9 | 9 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 3 | 3 | - | 0 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T094 | 모델 관리: 등급 Listbox, 등급순 sort, unset mark, document diff level changes | MODEL | T093 T095 | todo |
| T096 | Complete multi-source clip planning and playable generation | CLIP QUOTA ARCH | T091 | doing@260911.fix |

## next
- Complete T096 through actual multi-source analysis, composition and playable rendering; additional paid verification must remain strictly below USD 0.10 in aggregate, preserve concurrent model work
- implement-task T094 (last of the level series); then re-teach `/recomend-models` to emit `<id> <level>` lines — its id-only block clears every set level under MODEL-59
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
- 260911 T095 done; per-stage grade on CatalogModel, ordering composed into filterForStage so all three selector call sites share it, grade leads every option label; full FE gate passes
- 260911 T095 claimed (lvl)
- 260911 T093 done; `<id> [level]` grammar, relevel preview, level-aware sync/export; tightened looksLikeModelID to need a slash so a bullet is malformed_line again, not unknown_level
- 260911 T096 working (fix); owner caps all additional paid verification strictly below USD 0.10, reserve each next call before dispatch and retain unknown usage at its maximum; multi-source and safe diagnostic regressions pass
- 260911 T093 claimed (lvl)
- 260911 T092 done; level column 0040, per-registration domain/store/RPC and the stage-keyed llm+ModelInfo wire; ARCH-26 and gen idempotence pass, FE untouched and still builds
- 260911 T091 delivered (fix); 2d484e8 pushed, CI https://github.com/hetarho/postpilot/actions/runs/34506698657 and rollout https://github.com/hetarho/postpilot/actions/runs/34506698910 pass; exact production image running, /health ok, restarts=0 and OOM=false; no further paid call or user-media replay
- 260911 T092 claimed (lvl); chaining T092→T093→T095→T094, commit per task on main
- 260911 create-task MODEL done (tier); r9 → T092 BE column+wire · T093 BE document · T095 FE selectors · T094 FE admin; MODEL tasked=9
- 260911 T091 done (fix); corrected constrained output schemas, actual observation/planning and 15s Korean-captioned MP4 passed; all local gates/races green, reported test cost USD 0.003055 and conservative total USD 0.056321 within approval; no commit/push/deploy
- 260911 create-task MODEL start (tier); r9 delta MODEL-57+ 58+ 59+ and the eight ✎ lines
- 260911 update-ssot MODEL done (tier); r9 adds MODEL-57+ 58+ 59+ and touches 20 27 28 44 52 53 54 55 — no →MODEL ref elsewhere is affected and no doing task holds MODEL
- 260911 update-ssot MODEL start (tier); registered models carry a user-facing level label, shown beside the model and used for ordering
- 260911 T091 resumed (fix); owner approved synthetic live verification up to USD 0.10 total, no historical replay or production settings change
- 260911 T091 blocked (fix); inspected the actual wire and official contracts without an evidenced cause; ask for bounded synthetic live verification, no paid call or speculative code change
- 260911 T091 claimed (fix); prioritize the actual analyze 400 and a playable end-to-end result over additional defensive features; preserve concurrent work and existing credit ceilings, no historical replay
- 260911 T090 done; the push-scope hold cleared, 61fe2d2 is on main and CI/deploy are green on 7327d93
- 260911 update-ssot VOICE TMPL LANG done (mnt); VOICE-42 drops the tag count and GEN-46✎ excludes rule comparison, TMPL scopes the retired `label` to `slot`, LANG-7 gains `billing` `clips`; all four revs are wording-only so tasked=rev
- 260911 update-ssot VOICE TMPL LANG start (mnt); the three spec-maintenance items owed in next
- 260911 T089 done; the 후보 queue is a counted disclosure below the list with sequential bulk accept/dismiss and per-row refusals; all local gates and day/night browser checks pass
