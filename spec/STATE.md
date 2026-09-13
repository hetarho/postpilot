# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ideation
| id | st |
|---|---|
| clip-source-observation-visibility | converted@260912 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 11 | 11 | - | 0 |
| POST | 6 | 6 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 12 | 12 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 16 | 16 | - | 0 |
| CDS | 10 | 10 | - | 2 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T140 | Fix clip writer input admission and verify the complete pipeline | CLIP QUOTA ARCH LANG | T139 | doing@260913.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |

## next
- T140 code 5c18ab5 is deployed and CI/Workers/runtime verified; keep doing until the pending owner approval for one isolated live writer call (max $0.10) permits actual saved-input planning/original rendering. Synthetic complete-pipeline verification passed; do not ask the owner to repeat the 20 analyses.
- T110 must refresh CLIP@16 / CDS@10 before resuming because mandatory cards, disclosure and fact QA changed; it remains owner-blocked: privately load a 9:16 clip with both cards in the Naver picker without publishing, capture the clip-tab overlays and supply the images for CDS-11 measurement. Remaining CDS interpretations are recorded in the results of T107, T108, T109 and T115 (contrast/card geometry, audio seams, exposure and frequency, footage-bound duration).
- T008 needs the owner present: refresh PUB@5 / ARCH@2, rerun postpilot-agent setup before installation for driver signature smarteditor-one-20260910-a6, and cancel or deliberately reuse the queued 20260905-test job; after completion, update PUB for VIDEO-17 and TMPL-39.

## log
- 260913 T140 deployed (e2e); 5c18ab5 CI/Workers/backend and exact production health pass; keep doing, actual-input live writer approval still pending
- 260913 T140 verification (e2e); real-input admission replay and 20-source synthetic full pipeline pass; paid writer preflight capped at $0.08544, owner approval pending
- 260913 T140 claimed (e2e); implement frozen writer allowance, early input checks and complete pipeline validation
- 260913 create-task CLIP QUOTA done (e2e); T140 consumes CLIP r16 / QUOTA r11 and requires complete pipeline verification
- 260913 create-task CLIP QUOTA start (e2e)
- 260913 update-ssot CLIP r16 QUOTA r11 done (e2e); separate immutable writer allowance, early validation and real-input verification
- 260913 update-ssot CLIP QUOTA start (e2e); freeze a separate writer input allowance and require real-input pipeline verification
- 260913 clip failure investigation start (e2e); reproduce job 5176c61 planning failure with actual saved inputs before defining the follow-up fix
- 260913 T139 done (obs); 5f4a9c1 shipped, CI/Workers/backend rollout pass and exact live image/health verified; observation diagnostics and worker location preservation complete
- 260913 T139 verification (obs); local gates and 16 browser cases pass, prepare push and await CI/Workers/backend rollout before done
- 260913 T139 claimed (obs); implement observation diagnostic coverage and checkpoint locator preservation
- 260913 create-task CLIP done (obs); T139 consumes r15, preserve existing rejection and accounting behavior
- 260913 create-task CLIP start (obs)
- 260913 update-ssot CLIP r15 done (obs); observation diagnostic detail, no validation relaxation or partial-generation policy change
- 260913 update-ssot CLIP start (obs); close missing observation-validation diagnostics exposed by source 20 failure
- 260913 T138 done (diag); b4f5b9a shipped, CI/Workers/backend rollout green, migration 50 and live health verified; partial inspection, safe diagnostics and timeline repair pass all gates
- 260913 T138 verification (diag); local gates and 30 browser cases pass, b4f5b9a pushed; await CI/backend rollout before done
- 260913 T138 claimed (diag); implement checkpoint inspection, failure diagnostics and timeline repair
- 260913 create-task CLIP done (diag); T138 consumes CLIP r14 with bounded latest-attempt checkpoints and bidirectional timing repair
- 260913 create-task CLIP start (diag)
