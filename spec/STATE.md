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
| QUOTA | 8 | 8 | - | 0 |
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
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | todo |
| T082 | Approved clip credit quotes and admission | QUOTA CLIP ARCH | T081 | todo |
| T083 | Bounded inline video and workflow routing | CLIP QUOTA VIDEO ARCH | T082 | todo |
| T084 | Prepared inline clip generation | CLIP QUOTA ARCH | T083 | todo |
| T085 | Clip credit approval and preview lifecycle | CLIP QUOTA ARCH | T084 | todo |
| T086 | Clip credit and media release regressions | CLIP QUOTA VIDEO ARCH | T085 | todo |

## next
- implement-task T082, then T083 → T084 → T085 → T086; T081 is complete; enforce approved ceilings before inline execution, then connect approval/previews and verify the whole workflow; T008 stays separate
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TMPL for the retired SLOT `label` vs `<ask label>` conflict, and update-ssot LANG to add the `clips` namespace
- T008 is the last PUB task and needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job. Read T042 T043 T045 T046 results for the live surveys and the wiring's typed preflight; update-ssot PUB for VIDEO-17 + TMPL-39 after T008 closes
## log
- 260910 T081 done; evidence-based zero-charge failed clips, durable-outcome settlement and same-lot recovery; all local gates plus usage/job race suites pass, no live provider call or historical adjustment
- 260910 T081 claimed (crd); implement and verify T081–T086 sequentially with one completed-task commit each, preserve unrelated work
- 260910 create-task CLIP QUOTA VIDEO done; T081–T086 consume CLIP r3 QUOTA r8 VIDEO r2 in credit-first order; CLIP-35 stays deferred without an implementation task, T008/archived tasks and concurrent nav work preserved, no code changes
- 260910 create-task CLIP QUOTA VIDEO start; decompose approved ceilings, bounded inline analysis and preview lifecycle; preserve doing T008 and archived tasks, no implementation
- 260910 update-ssot CLIP QUOTA VIDEO done; CLIP r3 QUOTA r8 VIDEO r2 pending task breakdown, no affected doing task or implementation changes; bounded inline analysis, approved ceilings, zero-charge unused failures and attempt-long previews
- 260910 update-ssot CLIP QUOTA VIDEO start; apply approved bounded inline analysis, visible credit ceilings, no-usage failure protection and attempt-long local previews; documents only, preserve T008
- 260910 update-ssot VIDEO CLIP start; paused for approval of bounded inline clip analysis proxies versus VIDEO-10 URL-only delivery; production read-only diagnostics found Gemini permission failures, zero recorded AI usage and 2-credit base settlements; no code, provider retries or production writes
- 260910 clip bugfix start (clp); investigate reported analyze failure through PEM-authenticated read-only diagnostics; preserve credit caps and concurrent T008 edits
- 260910 T080 done; grouped navigation and responsive credit-safe header; all local gates and ordinary/master browser checks pass
- 260910 T080 claimed (clp); CLIP r2 admission delta is unrelated to grouped navigation
- 260910 T079 done; accessible correction workspace, exact source reselection and guarded free rerender; all local gates and responsive browser checks pass
- 260910 T079 claimed (clp); CLIP r2 admission delta does not change this credit-free correction UI
- 260910 T078 done; optimistic corrections, exact source-subset rerender, zero credit/provider path and recovery; all local gates and targeted race tests pass
- 260910 T078 claimed (clp)
- 260910 T077 done; durable progress, guarded generation, preview/download and localized credit refusal; all local gates and isolated responsive browser checks pass
- 260910 T077 claimed (clp)
- 260910 T076 done; durable streaming pipeline, failure-safe cleanup/result retention and strict reservation-capped credits; all local gates, concurrent/race tests and actual nonroot renderer smoke pass
- 260910 T046 done; the daemon has a real publisher factory (a browser per job so the activation latch is never reused, typed preflight failures for release, browser, login and account), install NOT run live because it would claim the owner's queued job — that belongs in T008
- 260910 T046 claimed (wir)
- 260910 T045 done; CDPPort implements CommitPort (arming bound to its observation, one latched activation, post-view readback reporting the canonical permalink) and PUB-13 r4's filling_settings landed end to end — proto, migration 0037 rebuilding publish_jobs, backend, agent and FE; the appended enum number broke two order checks that read stage order off it, both now use an explicit rank
