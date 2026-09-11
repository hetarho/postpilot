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
| VOICE | 2 | 2 | - | 1 |
| GEN | 5 | 5 | - | 0 |
| MODEL | 9 | 9 | - | 0 |
| TMPL | 4 | 4 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
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
| T096 | Complete multi-source clip planning and playable generation | CLIP QUOTA ARCH | T091 | doing@260911.fix |

## next
- T096 functional reproduction and original-footage output now pass; finish local/remote delivery gates, archive only on verified completion, and make no further paid calls (cumulative USD 0.020052)
- the MODEL r9 level series (T092–T095) is complete and `/recomend-models` emits graded lines; nothing further owed on it
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
- 260911 T096 functional fix verified (fix); eight actual observations succeeded, composition reproduced invalid caption exposure and 12700ms/15200ms arithmetic mismatch; bounded local timeline compilation renders a real 15s Korean-captioned MP4, offline authenticated queue/storage/download regression passes; actual paid aggregate USD 0.020052, no retry
- 260911 T096 resumed (fix); owner supplied the exact eight failed videos and requested actual generation testing; keep cumulative paid verification strictly below USD 0.10, preserve private captures outside git, complete the functional fix rather than diagnostics alone
- 260911 recomend-models re-taught (tier); its tiers are now the product's four levels and the paste block emits `<id> <level>`; 최고성능 split into 고급/최고 so `premium` is reachable
- 260911 T096 delivery requested (fix); commit/push diagnostics and verified regressions while keeping original-failure reproduction blocked; rerun local gates, preserve unrelated uncommitted edits, no paid calls
- 260911 T094 done; 등급 Listbox + unset mark per registration, 등급순 sort, 등급 변경 diff group; canApply now counts a relevel, since a re-grade-only document was previewable but not committable
- 260911 T096 blocked recheck (fix); same production plan failure with no raw output; two T094 test files now pass but web build has concurrent fixture type errors; remote main unchanged, original input still needed, paid total unchanged USD 0.011057
- 260911 T096 blocked (fix); actual failed output unavailable and synthetic cases succeed, requested original input; paid total USD 0.011057, no more calls; BE/agent/race/media/codegen pass, concurrent T094 FE gates non-green; no T096 commit/push
- 260911 T094 claimed (lvl)
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
