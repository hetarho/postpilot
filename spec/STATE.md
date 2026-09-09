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
| QUOTA | 7 | 7 | - | 0 |
| POST | 5 | 5 | - | 0 |
| VOICE | 2 | 2 | - | 1 |
| GEN | 5 | 5 | - | 0 |
| MODEL | 8 | 8 | - | 0 |
| TMPL | 4 | 4 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
| THEME | 8 | 8 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |
| CLIP | 2 | 2 | - | 0 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | todo |
| T076 | Durable clip generation and result retention | CLIP QUOTA VIDEO MODEL | T071 T074 T075 | doing@260910.clp |
| T077 | Clip generation progress, preview and download | CLIP THEME LANG | T072 T076 | todo |
| T078 | Manual clip-plan save and credit-free rerender | CLIP QUOTA | T071 T074 T076 | todo |
| T079 | Clip correction workspace | CLIP THEME LANG | T077 T078 | todo |
| T080 | Grouped writing and video navigation | CLIP THEME | T070 T072 | todo |

## next
- Implement and commit T076 through T080 sequentially; clip AI must remain within a successful full-run credit reservation
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TMPL for the retired SLOT `label` vs `<ask label>` conflict, and update-ssot LANG to add the `clips` namespace
- T008 is the last PUB task and needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job. Read T042 T043 T045 T046 results for the live surveys and the wiring's typed preflight; update-ssot PUB for VIDEO-17 + TMPL-39 after T008 closes
## log
- 260910 T046 done; the daemon has a real publisher factory (a browser per job so the activation latch is never reused, typed preflight failures for release, browser, login and account), install NOT run live because it would claim the owner's queued job — that belongs in T008
- 260910 T046 claimed (wir)
- 260910 T045 done; CDPPort implements CommitPort (arming bound to its observation, one latched activation, post-view readback reporting the canonical permalink) and PUB-13 r4's filling_settings landed end to end — proto, migration 0037 rebuilding publish_jobs, backend, agent and FE; the appended enum number broke two order checks that read stage order off it, both now use an explicit rank
- 260910 T076 claimed (clp); approved deferred admission with reservation-capped credit usage
- 260910 create-task CLIP QUOTA done; T076/T077 absorb deferred admission and strict reserved-credit protection, T078 remains credit-free
- 260910 create-task CLIP QUOTA start; T076 returned to todo after the owner's order decision
- 260910 update-ssot CLIP QUOTA done; durable preparation before admission, fail-closed AI and reservation-capped clip debit
- 260910 update-ssot CLIP QUOTA start; owner approves durable preparation before admission and prioritizes preventing credit overuse
- 260910 update-ssot CLIP QUOTA start; awaiting the owner's preparation/admission order decision before any revision
- 260910 T076 blocked: exact probed-duration holds must precede job insertion, but source probing is long-running worker work; no T076 implementation or completion commit
- 260910 T076 claimed (clp)
- 260910 T075 done; strict timecoded AI contracts, safe caption placement/exposure, typed budgets and all local gates pass
- 260910 T075 claimed (clp)
- 260910 T074 done; exact caption/video renderer, every local gate and nonroot Docker smoke pass; owner confirms Naver web acceptance of all three ratios
- 260910 T045 claimed (fnc)
- 260910 T074 reclaimed (clp): owner confirms web acceptance of all three MP4s with a screenshot showing their 15-second durations; final freshness and gates before commit
- 260910 T043 done; photos interleave at their manifest positions, and four live defects fixed — the photo-library sidebar occluding every caret point, points read outside the viewport, an upload returning before the editor settled, and insertText leaving its last word uncommitted (4 of 5 writes lost it); driver signature → smarteditor-one-20260910-a5
- 260910 T074 gate rechecked (clp): owner picker result is still absent; preserve blocked status and the sequential commit boundary
- 260910 T074 blocked: all local gates and actual nonroot renderer smoke pass; mandatory owner-assisted Naver picker acceptance remains unverified, so no completion commit
- 260910 T074 claimed (clp)
